package internal

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Repository struct {
	db *sql.DB
}

type RepositoryOptions struct {
	DBPath string
}

func NewRepository(options RepositoryOptions) (*Repository, error) {
	db, err := sql.Open("sqlite", options.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	repo := &Repository{
		db: db,
	}

	err = repo.init()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}

	return repo, nil
}

func (r *Repository) init() error {
	_, err := r.db.Exec(`
		CREATE TABLE IF NOT EXISTS planks_reckoning_state (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			last_message_sent INTEGER
		)
	`)

	return err
}

func (r *Repository) GetLastMessageSent() (*time.Time, error) {
	var lastMessageSent int64
	err := r.db.QueryRow("SELECT last_message_sent FROM planks_reckoning_state WHERE id = 1").Scan(&lastMessageSent)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get last message sent: %v", err)
	}

	t := time.Unix(lastMessageSent, 0)
	return &t, nil
}

func (r *Repository) SetLastMessageSent(timestamp time.Time) error {
	_, err := r.db.Exec(`
		INSERT INTO planks_reckoning_state (id, last_message_sent)
		VALUES (1, ?)
		ON CONFLICT(id) DO UPDATE SET last_message_sent = excluded.last_message_sent
	`, timestamp.Unix())

	return err
}
