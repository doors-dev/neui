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
