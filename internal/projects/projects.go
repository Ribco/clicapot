package projects

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound = errors.New("project not found")
	ErrExists   = errors.New("project already exists")
)

type Project struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
}

func Create(db *sql.DB, userID int64, name, slug string) (Project, error) {
	name = strings.TrimSpace(name)
	slug = strings.TrimSpace(strings.ToLower(slug))

	if name == "" || slug == "" {
		return Project{}, errors.New("name and slug are required")
	}

	result, err := db.Exec(`
		INSERT INTO projects (user_id, name, slug)
		VALUES (?, ?, ?)
	`, userID, name, slug)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Project{}, ErrExists
		}
		return Project{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Project{}, err
	}

	return Get(db, userID, id)
}

func List(db *sql.DB, userID int64) ([]Project, error) {
	rows, err := db.Query(`
		SELECT id, user_id, name, slug, created_at
		FROM projects
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Project

	for rows.Next() {
		var project Project

		if err := rows.Scan(
			&project.ID,
			&project.UserID,
			&project.Name,
			&project.Slug,
			&project.CreatedAt,
		); err != nil {
			return nil, err
		}

		result = append(result, project)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if result == nil {
		result = []Project{}
	}

	return result, nil
}

func Get(db *sql.DB, userID, id int64) (Project, error) {
	var project Project

	err := db.QueryRow(`
		SELECT id, user_id, name, slug, created_at
		FROM projects
		WHERE id = ? AND user_id = ?
	`, id, userID).Scan(
		&project.ID,
		&project.UserID,
		&project.Name,
		&project.Slug,
		&project.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return Project{}, ErrNotFound
	}

	return project, err
}

func Delete(db *sql.DB, userID, id int64) error {
	result, err := db.Exec(`
		DELETE FROM projects
		WHERE id = ? AND user_id = ?
	`, id, userID)

	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return ErrNotFound
	}

	return nil
}
