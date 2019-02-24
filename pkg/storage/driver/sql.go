/*
Copyright The Helm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driver

import (
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	migrate "github.com/rubenv/sql-migrate"

	// Import pq for potgres dialect
	_ "github.com/lib/pq"

	rspb "k8s.io/helm/pkg/proto/hapi/release"
	storageerrors "k8s.io/helm/pkg/storage/errors"
)

var _ Driver = (*SQL)(nil)

var labelMap = map[string]string{
	"MODIFIED_AT": "modified_at",
	"CREATED_AT":  "created_at",
	"VERSION":     "version",
	"STATUS":      "status",
	"OWNER":       "owner",
	"NAME":        "name",
}

// SQLDriverName is the string name of this driver.
const SQLDriverName = "SQL"

// SQL is the sql storage driver implementation.
type SQL struct {
	db *sqlx.DB
}

// Name returns the name of the driver.
func (s *SQL) Name() string {
	return SQLDriverName
}

func (s *SQL) ensureDBSetup() error {
	// Populate the database with the relations we need if they don't exist yet
	// TODO: create smart indices (labels and key and... ?)
	migrations := &migrate.MemoryMigrationSource{
		Migrations: []*migrate.Migration{
			&migrate.Migration{
				Id: "init",
				Up: []string{
					`
            CREATE TABLE releases (
		      		key VARCHAR(67) PRIMARY KEY,
              body STRING NOT NULL,

              name VARCHAR(64) NOT NULL,
              version INTEGER NOT NULL,
		      		status STRING NOT NULL,
		      		owner STRING NOT NULL,
		      		created_at INTEGER NOT NULL,
		      		modified_at INTEGER NOT NULL DEFAULT 0,
            );

            CREATE INDEX ON releases (key);
            CREATE INDEX ON releases (version);
            CREATE INDEX ON releases (status);
            CREATE INDEX ON releases (owner);
            CREATE INDEX ON releases (created_at);
            CREATE INDEX ON releases (modified_at);
		      `,
				},
				Down: []string{
					`
            DROP TABLE releases;
          `,
				},
			},
		},
	}

	_, err := migrate.Exec(s.db.DB, "postgres", migrations, migrate.Up)
	return err
}

// Release describes a Helm release
type Release struct {
	Key  string `db:"key"`
	Body string `db:"body"`

	Name       string `db:"name"`
	Version    int    `db:"version"`
	Status     string `db:"status"`
	Owner      string `db:"owner"`
	CreatedAt  int    `db:"created_at"`
	ModifiedAt int    `db:"modified_at"`
}

// NewSQL initializes a new memory driver.
func NewSQL(dialect, connectionString string) (*SQL, error) {
	db, err := sqlx.Connect(dialect, connectionString)
	if err != nil {
		return nil, err
	}

	driver := &SQL{
		db: db,
	}

	if err := driver.ensureDBSetup(); err != nil {
		return nil, err
	}

	return driver, nil
}

// Get returns the release named by key.
func (s *SQL) Get(key string) (*rspb.Release, error) {
	var record Release
	// Get will return an error if the result is empty
	err := s.db.Get(&record, "SELECT body FROM releases WHERE key=?", key)
	if err != nil {
		return nil, storageerrors.ErrReleaseNotFound(key)
	}

	release, err := decodeRelease(record.Body)
	if err != nil {
		return nil, err
	}

	return release, nil
}

// List returns the list of all releases such that filter(release) == true
func (s *SQL) List(filter func(*rspb.Release) bool) ([]*rspb.Release, error) {
	var records = []Release{}
	if err := s.db.Select(&records, "SELECT body FROM releases WHERE owner='TILLER'"); err != nil {
		return nil, err
	}

	var releases []*rspb.Release
	for _, record := range records {
		release, err := decodeRelease(record.Body)
		if err != nil {
			continue
		}
		if filter(release) {
			releases = append(releases, release)
		}
	}

	return releases, nil
}

// Query returns the set of releases that match the provided set of labels.
func (s *SQL) Query(labels map[string]string) ([]*rspb.Release, error) {
	var filters []string
	for key, val := range labels {
		// Build a slice of where filters e.g
		// labels = map[string]string{ "foo": "foo", "bar": "bar" }
		// []string{ "foo=:foo", "bar=:bar" }
		if dbField, ok := labelMap[key]; ok {
			filters = append(filters, strings.Join([]string{
				dbField, "=:", dbField,
			}, ""))
		}
	}

	query := strings.Join([]string{
		"SELECT body FROM releases",
		"WHERE",
		strings.Join(filters, " AND "),
	}, " ")

	rows, err := s.db.NamedQuery(query, labels)
	if err != nil {
		return nil, err
	}

	var releases []*rspb.Release
	for rows.Next() {
		var record Release
		if err = rows.Scan(&record); err != nil {
			return nil, err
		}

		release, err := decodeRelease(record.Body)
		if err != nil {
			continue
		}
		releases = append(releases, release)
	}

	if len(releases) == 0 {
		return nil, storageerrors.ErrReleaseNotFound(labels["NAME"])
	}

	return releases, nil
}

// Create creates a new release.
func (s *SQL) Create(key string, rls *rspb.Release) error {
	body, err := encodeRelease(rls)
	if err != nil {
		return err
	}

	transaction, err := s.db.Beginx()
	if err != nil {
		return fmt.Errorf("error beginning transaction: %v", err)
	}

	if _, err := transaction.NamedExec("INSERT INTO releases (key, body, name, version, status, owner, created_at) VALUES (:key, :body, :name, :version, :status, :owner, :created_at)",
		&Release{
			Key:  key,
			Body: body,

			Name:      rls.Name,
			Version:   int(rls.Version),
			Status:    rspb.Status_Code_name[int32(rls.Info.Status.Code)],
			Owner:     "TILLER",
			CreatedAt: int(time.Now().Unix()),
		},
	); err != nil {
		defer transaction.Rollback()
		var record Release
		if err := transaction.Get(&record, "SELECT key FROM releases WHERE key=?", key); err == nil {
			return storageerrors.ErrReleaseExists(key)
		}

		return err
	}
	defer transaction.Commit()

	return nil
}

// Update updates a release.
func (s *SQL) Update(key string, rls *rspb.Release) error {
	body, err := encodeRelease(rls)
	if err != nil {
		return err
	}

	if _, err := s.db.NamedExec("UPDATE releases WHERE key=:key SET body=:body, name=:name, version=:version, status=:status, owner=:owner, modified_at=:modified_at",
		&Release{
			Key:  key,
			Body: body,

			Name:       rls.Name,
			Version:    int(rls.Version),
			Status:     rspb.Status_Code_name[int32(rls.Info.Status.Code)],
			Owner:      "TILLER",
			ModifiedAt: int(time.Now().Unix()),
		},
	); err != nil {
		return err
	}

	return nil
}

// Delete deletes a release or returns ErrReleaseNotFound.
func (s *SQL) Delete(key string) (*rspb.Release, error) {
	_, err := s.db.Exec("DELETE FROM releases WHERE key=?", key)
	return nil, err
}
