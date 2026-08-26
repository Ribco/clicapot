package apikeys

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound  = errors.New("API key not found")
	ErrInvalid   = errors.New("invalid API key")
	ErrForbidden = errors.New("insufficient permissions")
)

var ValidScopes = map[string]bool{
	"account:read":   true,
	"projects:read":  true,
	"projects:write": true,
	"dns:read":       true,
	"dns:write":      true,
	"analytics:read": true,
	"storage:read":   true,
	"storage:write":  true,
	"compute:read":   true,
	"compute:write":  true,
}

type APIKey struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"user_id"`
	Name      string     `json:"name"`
	Prefix    string     `json:"prefix"`
	Scopes    []string   `json:"scopes"`
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
}

func Create(db *sql.DB, userID int64, name string, scopes []string) (APIKey, string, error) {
	name = strings.TrimSpace(name)

	if name == "" {
		return APIKey{}, "", errors.New("API key name is required")
	}

	if len(scopes) == 0 {
		scopes = []string{
			"account:read",
			"projects:read",
			"projects:write",
		}
	}

	for _, scope := range scopes {
		if !ValidScopes[scope] {
			return APIKey{}, "", errors.New("invalid scope: " + scope)
		}
	}

	raw, err := randomHex(32)
	if err != nil {
		return APIKey{}, "", err
	}

	key := "cp_" + raw
	prefix := key[:11]
	keyHash := hashKey(key)

	scopeJSON, err := json.Marshal(scopes)
	if err != nil {
		return APIKey{}, "", err
	}

	result, err := db.Exec(`
		INSERT INTO api_keys (user_id, name, key_hash, prefix, scopes)
		VALUES (?, ?, ?, ?, ?)
	`, userID, name, keyHash, prefix, string(scopeJSON))

	if err != nil {
		return APIKey{}, "", err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return APIKey{}, "", err
	}

	var apiKey APIKey
	var scopesText string

	err = db.QueryRow(`
		SELECT id, user_id, name, prefix, scopes, created_at, last_used
		FROM api_keys
		WHERE id = ?
	`, id).Scan(
		&apiKey.ID,
		&apiKey.UserID,
		&apiKey.Name,
		&apiKey.Prefix,
		&scopesText,
		&apiKey.CreatedAt,
		&apiKey.LastUsed,
	)

	if err != nil {
		return APIKey{}, "", err
	}

	_ = json.Unmarshal([]byte(scopesText), &apiKey.Scopes)

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

func HasScope(db *sql.DB, key string, required string) bool {
	if !ValidScopes[required] {
		return false
	}

	var scopesText string

	err := db.QueryRow(`
		SELECT scopes
		FROM api_keys
		WHERE key_hash = ?
	`, hashKey(key)).Scan(&scopesText)

	if err != nil {
		return false
	}

	var scopes []string

	if json.Unmarshal([]byte(scopesText), &scopes) != nil {
		return false
	}

	for _, scope := range scopes {
		if scope == required {
			return true
		}
	}

	return false
}

func List(db *sql.DB, userID int64) ([]APIKey, error) {
	rows, err := db.Query(`
		SELECT id, user_id, name, prefix, scopes, created_at, last_used
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
		var scopesText string

		if err := rows.Scan(
			&key.ID,
			&key.UserID,
			&key.Name,
			&key.Prefix,
			&scopesText,
			&key.CreatedAt,
			&key.LastUsed,
		); err != nil {
			return nil, err
		}

		_ = json.Unmarshal([]byte(scopesText), &key.Scopes)

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
