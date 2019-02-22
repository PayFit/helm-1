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

// Get returns the release named by key or returns ErrReleaseNotFound.
func (s *SQL) Get(key string) (*rspb.Release, error) {
	var elems []string
	if elems = strings.Split(key, ".v"); len(elems) != 2 {
		return nil, storageerrors.ErrInvalidKey(key)
	}

	name, version := elems[0], elems[1]
	if _, err := strconv.Atoi(version); err != nil {
		return nil, storageerrors.ErrInvalidKey(key)
	}

	var release = Release{}
	if err := s.db.Get(&release, "SELECT body FROM releases WHERE name=? AND version=?", name, version); err != nil {
		return nil, storageerrors.ErrReleaseNotFound(key)
	}
	// TODO handle not found

	// TODO unmarshal body

	return &release.Rls, nil
}

// List returns the list of all releases such that filter(release) == true
func (s *SQL) List(filter func(*rspb.Release) bool) ([]*rspb.Release, error) {
	var records = []Release{}
	if err := s.db.Select(&records, "SELECT * FROM releases"); err != nil {
		return nil, err
	}

	var releases []*rspb.Release
	for _, release := range records {
		if filter(&release.Rls) {
			releases = append(releases, &release.Rls)
		}
	}

	return releases, nil
}

// Query returns the set of releases that match the provided set of labels
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
		"release r",
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
		var rls rspb.Release
		if err = rows.Scan(&rls); err != nil {
			return nil, err
		}
		releases = append(releases, &rls)
	}

	return releases, nil
}

// Create creates a new release or returns ErrReleaseExists.
func (s *SQL) Create(key string, rls *rspb.Release) error {
	// defer unlock(mem.wlock())

	// if recs, ok := mem.cache[rls.Name]; ok {
	// 	if err := recs.Add(newRecord(key, rls)); err != nil {
	// 		return err
	// 	}
	// 	mem.cache[rls.Name] = recs
	// 	return nil
	// }
	// mem.cache[rls.Name] = records{newRecord(key, rls)}
	// return nil
	return nil
}

// Update updates a release or returns ErrReleaseNotFound.
func (s *SQL) Update(key string, rls *rspb.Release) error {
	// defer unlock(mem.wlock())

	// if rs, ok := mem.cache[rls.Name]; ok && rs.Exists(key) {
	// 	rs.Replace(key, newRecord(key, rls))
	// 	return nil
	// }
	// return storageerrors.ErrReleaseNotFound(rls.Name)
	return nil
}

// Delete deletes a release or returns ErrReleaseNotFound.
func (s *SQL) Delete(key string) (*rspb.Release, error) {
	// defer unlock(mem.wlock())

	// elems := strings.Split(key, ".v")

	// if len(elems) != 2 {
	// 	return nil, storageerrors.ErrInvalidKey(key)
	// }

	// name, ver := elems[0], elems[1]
	// if _, err := strconv.Atoi(ver); err != nil {
	// 	return nil, storageerrors.ErrInvalidKey(key)
	// }
	// if recs, ok := mem.cache[name]; ok {
	// 	if r := recs.Remove(key); r != nil {
	// 		// recs.Remove changes the slice reference, so we have to re-assign it.
	// 		mem.cache[name] = recs
	// 		return r.rls, nil
	// 	}
	// }
	// return nil, storageerrors.ErrReleaseNotFound(key)
	return nil, nil
}
