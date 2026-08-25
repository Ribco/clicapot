package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserExists         = errors.New("user already exists")
	ErrSessionInvalid     = errors.New("invalid session")
)

type User struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

func Register(db *sql.DB, email, username, password string) (User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)

	if email == "" || username == "" || len(password) < 8 {
		return User{}, errors.New("invalid registration data")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}

	result, err := db.Exec(`
		INSERT INTO users (email, username, password_hash)
		VALUES (?, ?, ?)
	`, email, username, string(hash))

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return User{}, ErrUserExists
		}
		return User{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return User{}, err
	}

	var user User
	err = db.QueryRow(`
		SELECT id, email, username, created_at
		FROM users
		WHERE id = ?
	`, id).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.CreatedAt,
	)

	return user, err
}

func Login(db *sql.DB, email, password string) (User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	var (
		user         User
		passwordHash string
	)

	err := db.QueryRow(`
		SELECT id, email, username, password_hash, created_at
		FROM users
		WHERE email = ?
	`, email).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&passwordHash,
		&user.CreatedAt,
	)

	if err != nil {
		return User{}, "", ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(passwordHash),
		[]byte(password),
	); err != nil {
		return User{}, "", ErrInvalidCredentials
	}

	token, err := randomToken(32)
	if err != nil {
		return User{}, "", err
	}

	tokenHash := hashToken(token)
	expires := time.Now().Add(30 * 24 * time.Hour)

	_, err = db.Exec(`
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES (?, ?, ?)
	`, user.ID, tokenHash, expires)

	if err != nil {
		return User{}, "", err
	}

	return user, token, nil
}

func GetUserByToken(db *sql.DB, token string) (User, error) {
	tokenHash := hashToken(token)

	var user User

	err := db.QueryRow(`
		SELECT u.id, u.email, u.username, u.created_at
		FROM users u
		JOIN sessions s ON s.user_id = u.id
		WHERE s.token_hash = ?
		  AND s.expires_at > CURRENT_TIMESTAMP
	`, tokenHash).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.CreatedAt,
	)

	if err != nil {
		return User{}, ErrSessionInvalid
	}

	return user, nil
}

func Logout(db *sql.DB, token string) error {
	_, err := db.Exec(`
		DELETE FROM sessions
		WHERE token_hash = ?
	`, hashToken(token))

	return err
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)

	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return hex.EncodeToString(buf), nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
