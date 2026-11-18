// Copyright 2026 doors dev LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nekit

import (
	"database/sql"

	"neui.dev/nekit/internal/sqlite"
)

// SQLiteMessages keeps every user's threads in one SQLite database. Give each session
// its user's part with User. Uploaded files are not stored here; set
// neui.OptionUploadStorage for them.
type SQLiteMessages = *sqlite.Store

// NewSQLiteMessages creates or migrates neui's tables in db and turns on WAL; it
// refuses a database written by a newer neui. Import a driver, open db and
// close it after the app stops. Give neui a database file of its own: the
// schema version is kept in PRAGMA user_version.
func NewSQLiteMessages(db *sql.DB) (SQLiteMessages, error) {
	return sqlite.New(db)
}
