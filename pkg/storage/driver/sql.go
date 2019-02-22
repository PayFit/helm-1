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
	"encoding/json"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	// Import pq for potgres dialect
	_ "github.com/lib/pq"

	rspb "k8s.io/helm/pkg/proto/hapi/release"
	storageerrors "k8s.io/helm/pkg/storage/errors"
)

var _ Driver = (*SQL)(nil)

// SQLDriverName is the string name of this driver.
const SQLDriverName = "SQL"

// SQL is the sql storage driver implementation.
type SQL struct {
	db *sqlx.DB
}

// Release describes a Helm release
type Release struct {
	UUID    uuid.UUID `db:"uuid"`
	Name    string    `db:"name"`
	Version int       `db:"version"`
	Body    []byte    `db:"body"`
}

// Label describes KV pair associated with a given helm release, used for filtering only
type Label struct {
	ReleaseUUID uuid.UUID `db:"release_uuid"`
	Key         string    `db:"key"`
	Value       string    `db:"value"`
}

// NewSQL initializes a new memory driver.
func NewSQL(dialect, connectionString string) (*SQL, error) {
	db, err := sqlx.Connect(dialect, connectionString)
	if err != nil {
		return nil, err
	}
	return &SQL{
		db: db,
	}, nil
}

// Name returns the name of the driver.
func (s *SQL) Name() string {
	return SQLDriverName
}

// Get returns the release named by key.
func (s *SQL) Get(key string) (*rspb.Release, error) {
	var elems []string
	if elems = strings.Split(key, ".v"); len(elems) != 2 {
		return nil, storageerrors.ErrInvalidKey(key)
	}

	name, version := elems[0], elems[1]
	if _, err := strconv.Atoi(version); err != nil {
		return nil, storageerrors.ErrInvalidKey(key)
	}

	// Get will return an error if the result is empty
	var record = &Release{}
	if err := s.db.Get(record, "SELECT body FROM releases WHERE name=? AND version=?", name, version); err != nil {
		return nil, storageerrors.ErrReleaseNotFound(key)
	}

	var release rspb.Release
	if err := json.Unmarshal(record.Body, &release); err != nil {
		return nil, err
	}

	return &release, nil
}

// List returns the list of all releases such that filter(release) == true
func (s *SQL) List(filter func(*rspb.Release) bool) ([]*rspb.Release, error) {
	var records = []Release{}
	if err := s.db.Select(&records, "SELECT * FROM releases"); err != nil {
		return nil, err
	}

	var releases []*rspb.Release
	for _, record := range records {
		var release rspb.Release
		if err := json.Unmarshal(record.Body, &release); err != nil {
			return nil, err
		}
		if filter(&release) {
			releases = append(releases, &release)
		}
	}

	return releases, nil
}

// Query returns the set of releases that match the provided set of labels.
func (s *SQL) Query(keyvals map[string]string) ([]*rspb.Release, error) {
	filters := ""
	for _, key := range keyvals {
		filters = strings.Join([]string{
			filters,
			"name=" + key + " AND value=" + keyvals[key],
		}, " OR ")
	}

	query := strings.Join([]string{
		"SELECT r.rls",
		"FROM",
		"releases r",
		"INNER JOIN",
		"labels l ON r.id = l.release_id",
		"WHERE",
		filters,
	}, " ")

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}

	var releases []*rspb.Release
	for rows.Next() {
		var release rspb.Release
		if err = rows.Scan(&release); err != nil {
			return nil, err
		}
		releases = append(releases, &release)
	}

	return releases, nil
}

// Create creates a new release.
func (s *SQL) Create(key string, rls *rspb.Release) error {
	var elems []string
	if elems = strings.Split(key, ".v"); len(elems) != 2 {
		return storageerrors.ErrInvalidKey(key)
	}

	data, err := json.Marshal(rls)
	if err != nil {
		return err
	}

	version, err := strconv.Atoi(elems[1])
	if err != nil {
		return storageerrors.ErrInvalidKey(key)
	}

	tx := s.db.MustBegin()

	var record Release
	if err := tx.Get(&record, "SELECT (*) FROM releases WHERE name=? AND version=?", elems[0], version); err == nil {
		return storageerrors.ErrReleaseExists(key)
	}

	tx.NamedExec("INSERT INTO releases (uuid, name, version, body) VALUES (:uuid, :name, :version, :body)", &Release{
		UUID:    uuid.New(),
		Name:    elems[0],
		Version: version,
		Body:    data,
	})
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

// Update updates a release.
func (s *SQL) Update(key string, rls *rspb.Release) error {
	var elems []string
	if elems = strings.Split(key, ".v"); len(elems) != 2 {
		return storageerrors.ErrInvalidKey(key)
	}

	version, err := strconv.Atoi(elems[1])
	if err != nil {
		return storageerrors.ErrInvalidKey(key)
	}

	tx := s.db.MustBegin()

	var record Release
	if err := tx.Get(&record, "COUNT (*) FROM releases WHERE name=? AND version=?", elems[0], version); err != nil {
		return storageerrors.ErrReleaseNotFound(key)
	}

	if _, err := tx.NamedExec("UPDATE releases SET body = ? WHERE uuid = ?", record.UUID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

// Delete deletes a release or returns ErrReleaseNotFound.
func (s *SQL) Delete(key string) (*rspb.Release, error) {
	var elems []string
	if elems = strings.Split(key, ".v"); len(elems) != 2 {
		return nil, storageerrors.ErrInvalidKey(key)
	}

	version, err := strconv.Atoi(elems[1])
	if err != nil {
		return nil, storageerrors.ErrInvalidKey(key)
	}

	if _, err := s.db.Exec("DELETE FROM releases WHERE name = ? AND version = ?", elems[0], version); err != nil {
		return nil, err
	}

	return nil, nil
}
