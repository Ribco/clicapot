package dns

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound = errors.New("not found")
	ErrExists   = errors.New("already exists")
)

type Zone struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type Record struct {
	ID        int64     `json:"id"`
	ZoneID    int64     `json:"zone_id"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	TTL       int       `json:"ttl"`
	Priority  *int      `json:"priority,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func Migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS dns_zones (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, name)
		);

		CREATE TABLE IF NOT EXISTS dns_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			zone_id INTEGER NOT NULL,
			type TEXT NOT NULL,
			name TEXT NOT NULL,
			content TEXT NOT NULL,
			ttl INTEGER NOT NULL DEFAULT 300,
			priority INTEGER,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(zone_id) REFERENCES dns_zones(id) ON DELETE CASCADE
		);
	`)
	return err
}

func normalizeZone(name string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
}

func ListZones(db *sql.DB, userID int64) ([]Zone, error) {
	rows, err := db.Query(`
		SELECT id, user_id, name, status, created_at
		FROM dns_zones
		WHERE user_id = ?
		ORDER BY id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Zone
	for rows.Next() {
		var z Zone
		if err := rows.Scan(&z.ID, &z.UserID, &z.Name, &z.Status, &z.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, z)
	}

	if result == nil {
		result = []Zone{}
	}

	return result, rows.Err()
}

func CreateZone(db *sql.DB, userID int64, name string) (Zone, error) {
	name = normalizeZone(name)
	if name == "" {
		return Zone{}, errors.New("zone name is required")
	}

	res, err := db.Exec(`
		INSERT INTO dns_zones (user_id, name)
		VALUES (?, ?)
	`, userID, name)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return Zone{}, ErrExists
		}
		return Zone{}, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return Zone{}, err
	}

	var z Zone
	err = db.QueryRow(`
		SELECT id, user_id, name, status, created_at
		FROM dns_zones WHERE id = ? AND user_id = ?
	`, id, userID).Scan(&z.ID, &z.UserID, &z.Name, &z.Status, &z.CreatedAt)

	return z, err
}

func DeleteZone(db *sql.DB, userID, id int64) error {
	res, err := db.Exec(`
		DELETE FROM dns_zones
		WHERE id = ? AND user_id = ?
	`, id, userID)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}

	return nil
}

func ListRecords(db *sql.DB, userID, zoneID int64) ([]Record, error) {
	var exists int
	if err := db.QueryRow(`
		SELECT 1 FROM dns_zones WHERE id = ? AND user_id = ?
	`, zoneID, userID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	rows, err := db.Query(`
		SELECT id, zone_id, type, name, content, ttl, priority, created_at
		FROM dns_records
		WHERE zone_id = ?
		ORDER BY id DESC
	`, zoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Record
	for rows.Next() {
		var r Record
		if err := rows.Scan(
			&r.ID, &r.ZoneID, &r.Type, &r.Name,
			&r.Content, &r.TTL, &r.Priority, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, r)
	}

	if result == nil {
		result = []Record{}
	}

	return result, rows.Err()
}

func CreateRecord(db *sql.DB, userID, zoneID int64, record Record) (Record, error) {
	var exists int
	if err := db.QueryRow(`
		SELECT 1 FROM dns_zones WHERE id = ? AND user_id = ?
	`, zoneID, userID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Record{}, ErrNotFound
		}
		return Record{}, err
	}

	record.Type = strings.ToUpper(strings.TrimSpace(record.Type))
	record.Name = strings.TrimSpace(record.Name)
	record.Content = strings.TrimSpace(record.Content)

	switch record.Type {
	case "A", "AAAA", "CNAME", "MX", "TXT", "NS":
	default:
		return Record{}, errors.New("unsupported DNS record type")
	}

	if record.Name == "" || record.Content == "" {
		return Record{}, errors.New("name and content are required")
	}

	if record.TTL <= 0 {
		record.TTL = 300
	}

	res, err := db.Exec(`
		INSERT INTO dns_records
		(zone_id, type, name, content, ttl, priority)
		VALUES (?, ?, ?, ?, ?, ?)
	`, zoneID, record.Type, record.Name, record.Content, record.TTL, record.Priority)
	if err != nil {
		return Record{}, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return Record{}, err
	}

	err = db.QueryRow(`
		SELECT id, zone_id, type, name, content, ttl, priority, created_at
		FROM dns_records WHERE id = ?
	`, id).Scan(
		&record.ID, &record.ZoneID, &record.Type, &record.Name,
		&record.Content, &record.TTL, &record.Priority, &record.CreatedAt,
	)

	return record, err
}

func DeleteRecord(db *sql.DB, userID, zoneID, recordID int64) error {
	res, err := db.Exec(`
		DELETE FROM dns_records
		WHERE id = ?
		AND zone_id IN (
			SELECT id FROM dns_zones WHERE id = ? AND user_id = ?
		)
	`, recordID, zoneID, userID)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}

	return nil
}
