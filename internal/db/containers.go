package db

import (
	"database/sql"
	"fmt"
	"time"
)

type Container struct {
	ID         string
	UserID     int64
	Name       string
	Namespace  string
	Status     string
	ExternalIP sql.NullString
	MemoryMB   int
	StorageGB  int
	Image      string
	CreatedAt  time.Time
	StoppedAt  sql.NullTime
}

func (db *DB) CreateContainer(c *Container) error {
	_, err := db.Exec(`
		INSERT INTO containers (id, user_id, name, namespace, status, memory_mb, storage_gb, image)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.UserID, c.Name, c.Namespace, c.Status, c.MemoryMB, c.StorageGB, c.Image,
	)
	if err != nil {
		return fmt.Errorf("insert container: %w", err)
	}
	return nil
}

func (db *DB) GetContainer(id string) (*Container, error) {
	c := &Container{}
	err := db.QueryRow(`
		SELECT id, user_id, name, namespace, status, external_ip, memory_mb, storage_gb, image, created_at, stopped_at
		FROM containers WHERE id = ?`, id,
	).Scan(&c.ID, &c.UserID, &c.Name, &c.Namespace, &c.Status, &c.ExternalIP, &c.MemoryMB, &c.StorageGB, &c.Image, &c.CreatedAt, &c.StoppedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query container: %w", err)
	}
	return c, nil
}

func (db *DB) ListContainersByUser(userID int64) ([]*Container, error) {
	rows, err := db.Query(`
		SELECT id, user_id, name, namespace, status, external_ip, memory_mb, storage_gb, image, created_at, stopped_at
		FROM containers WHERE user_id = ? ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query containers: %w", err)
	}
	defer rows.Close()

	var containers []*Container
	for rows.Next() {
		c := &Container{}
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Namespace, &c.Status, &c.ExternalIP, &c.MemoryMB, &c.StorageGB, &c.Image, &c.CreatedAt, &c.StoppedAt); err != nil {
			return nil, fmt.Errorf("scan container: %w", err)
		}
		containers = append(containers, c)
	}
	return containers, nil
}

func (db *DB) UpdateContainerStatus(id, status string) error {
	_, err := db.Exec(`UPDATE containers SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("update container status: %w", err)
	}
	return nil
}

func (db *DB) UpdateContainerIP(id, ip string) error {
	_, err := db.Exec(`UPDATE containers SET external_ip = ? WHERE id = ?`, ip, id)
	if err != nil {
		return fmt.Errorf("update container ip: %w", err)
	}
	return nil
}

func (db *DB) UpdateContainerStopped(id string) error {
	_, err := db.Exec(`UPDATE containers SET status = 'stopped', stopped_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("update container stopped: %w", err)
	}
	return nil
}

func (db *DB) DeleteContainer(id string) error {
	_, err := db.Exec(`DELETE FROM containers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete container: %w", err)
	}
	return nil
}

func (db *DB) CountContainersByUser(userID int64) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM containers WHERE user_id = ? AND status != 'deleted'`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count containers: %w", err)
	}
	return count, nil
}
