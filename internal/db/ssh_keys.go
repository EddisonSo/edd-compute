package db

import (
	"database/sql"
	"fmt"
	"time"
)

type SSHKey struct {
	ID          int64
	UserID      int64
	Name        string
	PublicKey   string
	Fingerprint string
	CreatedAt   time.Time
}

func (db *DB) CreateSSHKey(key *SSHKey) error {
	result, err := db.Exec(`
		INSERT INTO ssh_keys (user_id, name, public_key, fingerprint)
		VALUES (?, ?, ?, ?)`,
		key.UserID, key.Name, key.PublicKey, key.Fingerprint,
	)
	if err != nil {
		return fmt.Errorf("insert ssh key: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	key.ID = id
	return nil
}

func (db *DB) GetSSHKey(id int64) (*SSHKey, error) {
	key := &SSHKey{}
	err := db.QueryRow(`
		SELECT id, user_id, name, public_key, fingerprint, created_at
		FROM ssh_keys WHERE id = ?`, id,
	).Scan(&key.ID, &key.UserID, &key.Name, &key.PublicKey, &key.Fingerprint, &key.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query ssh key: %w", err)
	}
	return key, nil
}

func (db *DB) ListSSHKeysByUser(userID int64) ([]*SSHKey, error) {
	rows, err := db.Query(`
		SELECT id, user_id, name, public_key, fingerprint, created_at
		FROM ssh_keys WHERE user_id = ? ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query ssh keys: %w", err)
	}
	defer rows.Close()

	var keys []*SSHKey
	for rows.Next() {
		key := &SSHKey{}
		if err := rows.Scan(&key.ID, &key.UserID, &key.Name, &key.PublicKey, &key.Fingerprint, &key.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan ssh key: %w", err)
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func (db *DB) GetSSHKeysByIDs(userID int64, ids []int64) ([]*SSHKey, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	// Build query with placeholders
	query := `SELECT id, user_id, name, public_key, fingerprint, created_at
		FROM ssh_keys WHERE user_id = ? AND id IN (`
	args := []any{userID}
	for i, id := range ids {
		if i > 0 {
			query += ","
		}
		query += "?"
		args = append(args, id)
	}
	query += ")"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query ssh keys by ids: %w", err)
	}
	defer rows.Close()

	var keys []*SSHKey
	for rows.Next() {
		key := &SSHKey{}
		if err := rows.Scan(&key.ID, &key.UserID, &key.Name, &key.PublicKey, &key.Fingerprint, &key.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan ssh key: %w", err)
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func (db *DB) DeleteSSHKey(id int64, userID int64) error {
	result, err := db.Exec(`DELETE FROM ssh_keys WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("delete ssh key: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("ssh key not found or not owned by user")
	}
	return nil
}

func (db *DB) CountSSHKeysByUser(userID int64) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM ssh_keys WHERE user_id = ?`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count ssh keys: %w", err)
	}
	return count, nil
}
