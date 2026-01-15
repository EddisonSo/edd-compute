package db

import (
	"database/sql"
	"fmt"
	"time"
)

type APIKey struct {
	ID        int64
	UserID    int64
	KeyHash   string
	Name      string
	CreatedAt time.Time
	LastUsed  sql.NullTime
}

func (db *DB) CreateAPIKey(key *APIKey) error {
	result, err := db.Exec(`
		INSERT INTO api_keys (user_id, key_hash, name)
		VALUES (?, ?, ?)`,
		key.UserID, key.KeyHash, key.Name,
	)
	if err != nil {
		return fmt.Errorf("insert api key: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	key.ID = id
	return nil
}

func (db *DB) GetAPIKeyByHash(keyHash string) (*APIKey, error) {
	key := &APIKey{}
	err := db.QueryRow(`
		SELECT id, user_id, key_hash, name, created_at, last_used
		FROM api_keys WHERE key_hash = ?`, keyHash,
	).Scan(&key.ID, &key.UserID, &key.KeyHash, &key.Name, &key.CreatedAt, &key.LastUsed)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query api key: %w", err)
	}
	return key, nil
}

func (db *DB) ListAPIKeysByUser(userID int64) ([]*APIKey, error) {
	rows, err := db.Query(`
		SELECT id, user_id, key_hash, name, created_at, last_used
		FROM api_keys WHERE user_id = ? ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query api keys: %w", err)
	}
	defer rows.Close()

	var keys []*APIKey
	for rows.Next() {
		key := &APIKey{}
		if err := rows.Scan(&key.ID, &key.UserID, &key.KeyHash, &key.Name, &key.CreatedAt, &key.LastUsed); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func (db *DB) UpdateAPIKeyLastUsed(id int64) error {
	_, err := db.Exec(`UPDATE api_keys SET last_used = CURRENT_TIMESTAMP WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("update api key last used: %w", err)
	}
	return nil
}

func (db *DB) DeleteAPIKey(id int64, userID int64) error {
	result, err := db.Exec(`DELETE FROM api_keys WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("api key not found or not owned by user")
	}
	return nil
}

func (db *DB) CountAPIKeysByUser(userID int64) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE user_id = ?`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count api keys: %w", err)
	}
	return count, nil
}
