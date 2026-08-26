package apikeys

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound = errors.New("API key not found")
	ErrInvalid  = errors.New("invalid API key")
)

type APIKey struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"user_id"`
	Name      string     `json:"name"`
	Prefix    string     `json:"prefix"`
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
}

func Create(db *sql.DB, userID int64, name string) (APIKey, string, error) {
	name = strings.TrimSpace(name)

	if name == "" {
		return APIKey{}, "", errors.New("API key name is required")
	}

	raw, err := randomHex(32)
	if err != nil {
		return APIKey{}, "", err
	}

	key := "cp_" + raw
	prefix := key[:11]
	keyHash := hashKey(key)

	result, err := db.Exec(`
		INSERT INTO api_keys (user_id, name, key_hash, prefix)
		VALUES (?, ?, ?, ?)
	`, userID, name, keyHash, prefix)

	if err != nil {
		return APIKey{}, "", err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return APIKey{}, "", err
	}

	var apiKey APIKey

	err = db.QueryRow(`
		SELECT id, user_id, name, prefix, created_at, last_used
		FROM api_keys
		WHERE id = ?
	`, id).Scan(
		&apiKey.ID,
		&apiKey.UserID,
		&apiKey.Name,
		&apiKey.Prefix,
		&apiKey.CreatedAt,
		&apiKey.LastUsed,
	)

	if err != nil {
		return APIKey{}, "", err
	}

	return apiKey, key, nil
}

func Authenticate(db *sql.DB, key string) (int64, error) {
	key = strings.TrimSpace(key)

	if !strings.HasPrefix(key, "cp_") {
		return 0, ErrInvalid
	}

	var userID int64

	err := db.QueryRow(`
		SELECT user_id
		FROM api_keys
		WHERE key_hash = ?
	`, hashKey(key)).Scan(&userID)

	if err == sql.ErrNoRows {
		return 0, ErrInvalid
	}

	if err != nil {
		return 0, err
	}

	_, err = db.Exec(`
		UPDATE api_keys
		SET last_used = CURRENT_TIMESTAMP
		WHERE key_hash = ?
	`, hashKey(key))

	if err != nil {
		return 0, err
	}

	return userID, nil
}

func List(db *sql.DB, userID int64) ([]APIKey, error) {
	rows, err := db.Query(`
		SELECT id, user_id, name, prefix, created_at, last_used
		FROM api_keys
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := []APIKey{}

	for rows.Next() {
		var key APIKey

		if err := rows.Scan(
			&key.ID,
			&key.UserID,
			&key.Name,
			&key.Prefix,
			&key.CreatedAt,
			&key.LastUsed,
		); err != nil {
			return nil, err
		}

		keys = append(keys, key)
	}

	return keys, rows.Err()
}

func Delete(db *sql.DB, userID, id int64) error {
	result, err := db.Exec(`
		DELETE FROM api_keys
		WHERE id = ? AND user_id = ?
	`, id, userID)

	if err != nil {
		return err
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if n == 0 {
		return ErrNotFound
	}

	return nil
}

func randomHex(size int) (string, error) {
	buf := make([]byte, size)

	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return hex.EncodeToString(buf), nil
}

func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
