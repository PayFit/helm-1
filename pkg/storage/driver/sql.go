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

type Release struct {
	ID      int
	Name    string
	Version int
	Rls     rspb.Release
}

type Label struct {
	ID   int
	Name string
}

type ReleaseLabelAssociation struct {
	ID        int
	ReleaseID int
	LabelID   int
}

// NewSQL initializes a new memory driver.
func NewSQL(dialect, connectionString string) *SQL {
	db, err := sqlx.Connect(dialect, connectionString)
	if err != nil {
		panic(err)
	}
	return &SQL{
		db: db,
	}
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

	releases := []Release{}
	s.db.Select(&releases, "SELECT * FROM releases WHERE name=$1 AND version=$2", name, version)

	if len(releases) == 0 {
		return nil, storageerrors.ErrReleaseNotFound(key)
	}

	return &releases[0].Rls, nil
}

// List returns the list of all releases such that filter(release) == true
func (s *SQL) List(filter func(*rspb.Release) bool) ([]*rspb.Release, error) {
	// defer unlock(mem.rlock())

	// var ls []*rspb.Release
	// for _, recs := range mem.cache {
	// 	recs.Iter(func(_ int, rec *record) bool {
	// 		if filter(rec.rls) {
	// 			ls = append(ls, rec.rls)
	// 		}
	// 		return true
	// 	})
	// }
	// return ls, nil
	return nil, nil
}

// Query returns the set of releases that match the provided set of labels
func (s *SQL) Query(keyvals map[string]string) ([]*rspb.Release, error) {
	// defer unlock(mem.rlock())

	// var lbs labels

	// lbs.init()
	// lbs.fromMap(keyvals)

	// var ls []*rspb.Release
	// for _, recs := range mem.cache {
	// 	recs.Iter(func(_ int, rec *record) bool {
	// 		// A query for a release name that doesn't exist (has been deleted)
	// 		// can cause rec to be nil.
	// 		if rec == nil {
	// 			return false
	// 		}
	// 		if rec.lbs.match(lbs) {
	// 			ls = append(ls, rec.rls)
	// 		}
	// 		return true
	// 	})
	// }
	// return ls, nil
	return nil, nil
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
