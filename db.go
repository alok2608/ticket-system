package main

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	email         TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	created_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tickets (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id     INTEGER NOT NULL,
	title       TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	status      TEXT NOT NULL,
	created_at  TEXT NOT NULL,
	updated_at  TEXT NOT NULL,
	FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_tickets_user_id ON tickets (user_id);
`

// openDB opens the SQLite file and creates the tables if they do not exist.
// The file is created automatically on first start, so the service needs no
// setup step and no external database.
func openDB(path string) *sql.DB {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	// SQLite allows only one writer at a time; a single connection keeps
	// concurrent requests from hitting "database is locked".
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		log.Fatalf("create schema: %v", err)
	}
	return db
}
