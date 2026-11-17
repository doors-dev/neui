package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strconv"

	"neui.dev/api"
)

var migrations = [][]string{
	{
		`CREATE TABLE threads (
			user        TEXT NOT NULL,
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL DEFAULT '',
			name_fold   TEXT NOT NULL DEFAULT '',
			seq         INTEGER NOT NULL DEFAULT 0,
			created     INTEGER NOT NULL,
			updated     INTEGER NOT NULL,
			next_round  INTEGER NOT NULL DEFAULT 0,
			next_record INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX threads_user_updated ON threads (user, updated, id)`,
		`CREATE TABLE rounds (
			user          TEXT NOT NULL,
			thread        TEXT NOT NULL,
			id            INTEGER NOT NULL,
			created       INTEGER NOT NULL,
			state         TEXT NOT NULL DEFAULT '',
			input         TEXT NOT NULL,
			error         TEXT NOT NULL DEFAULT '',
			usage         TEXT NOT NULL DEFAULT '{}',
			response      TEXT,
			results       TEXT,
			input_fold    TEXT NOT NULL DEFAULT '',
			response_fold TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (user, thread, id)
		)`,
		`CREATE INDEX rounds_user_created ON rounds (user, created, thread, id)`,
		`CREATE TABLE compactions (
			n      INTEGER PRIMARY KEY,
			user   TEXT NOT NULL,
			thread TEXT NOT NULL,
			target INTEGER NOT NULL,
			tail   INTEGER NOT NULL,
			text   TEXT NOT NULL,
			output INTEGER NOT NULL DEFAULT 0,
			base   INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX compactions_user_thread ON compactions (user, thread, n)`,
		`CREATE TABLE records (
			user        TEXT NOT NULL,
			thread      TEXT NOT NULL,
			id          INTEGER NOT NULL,
			origin      INTEGER NOT NULL,
			pin         INTEGER NOT NULL DEFAULT -1,
			kind        TEXT NOT NULL,
			attrs       TEXT,
			data        TEXT NOT NULL,
			data_string INTEGER NOT NULL,
			PRIMARY KEY (user, thread, id)
		)`,
	},
}

// Store keeps every user's threads in one SQLite database. Give each session
// its user's part with User. Uploaded files are not stored here; set
// neui.OptionFileStorage for them.
type Store struct {
	db *sql.DB
}

func New(db *sql.DB) (*Store, error) {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		conn.Close()
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA journal_mode = WAL"); err != nil {
		conn.Close()
		return nil, err
	}
	conn.Close()
	err = write(ctx, db, func(conn *sql.Conn) error {
		var version int
		if err := conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
			return err
		}
		if version > len(migrations) {
			return fmt.Errorf("sqlite: schema version %d is newer than this neui (%d)", version, len(migrations))
		}
		if version == len(migrations) {
			return nil
		}
		for _, migration := range migrations[version:] {
			for _, stmt := range migration {
				if _, err := conn.ExecContext(ctx, stmt); err != nil {
					return err
				}
			}
		}
		_, err := conn.ExecContext(ctx, "PRAGMA user_version = "+strconv.Itoa(len(migrations)))
		return err
	})
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// User returns the threads of one user, for neui.OptionMessageStorage. Users
// never see each other's threads; "" is an id like any other, so every session
// that passes it shares the same threads.
func (s *Store) User(id string) api.MessageStorage {
	return &MessageStorage{db: s.db, user: id}
}

func write(ctx context.Context, db *sql.DB, change func(conn *sql.Conn) error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	err = change(conn)
	if err == nil {
		if _, err = conn.ExecContext(ctx, "COMMIT"); err == nil {
			return nil
		}
	}
	if _, rollback := conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK"); rollback != nil {
		conn.Raw(func(any) error { return driver.ErrBadConn })
	}
	return err
}
