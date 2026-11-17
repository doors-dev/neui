package nekit

import (
	"database/sql"

	"neui.dev/nekit/internal/sqlite"
)

// SQLite keeps every user's threads in one SQLite database. Give each session
// its user's part with User. Uploaded files are not stored here; set
// neui.OptionFileStorage for them.
type SQLite = sqlite.Store

// NewSQLite creates or migrates neui's tables in db and turns on WAL; it
// refuses a database written by a newer neui. Import a driver, open db and
// close it after the app stops. Give neui a database file of its own: the
// schema version is kept in PRAGMA user_version.
func NewSQLite(db *sql.DB) (*SQLite, error) {
	return sqlite.New(db)
}
