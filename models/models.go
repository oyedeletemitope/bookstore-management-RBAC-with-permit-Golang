package models

import (
	"time"

	"database/sql/driver"

	"github.com/google/uuid"
)

// NullUUID represents a UUID that may be null.
// NullUUID implements the Scanner interface so
// it can be used as a scan destination, similar to sql.NullString.
type NullUUID struct {
	UUID  uuid.UUID
	Valid bool // Valid is true if UUID is not NULL
}

// Scan implements the Scanner interface for NullUUID.
func (n *NullUUID) Scan(value interface{}) error {
	if value == nil {
		n.UUID, n.Valid = uuid.UUID{}, false
		return nil
	}
	n.Valid = true
	return n.UUID.Scan(value)
}

// Value implements the driver Valuer interface for NullUUID.
func (n NullUUID) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.UUID[:], nil
}

type User struct {
	ID           uuid.UUID `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	Email        string    `json:"email"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	CreatedAt    time.Time `json:"created_at"`
}

type Book struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Author      string     `json:"author"`
	PublishedAt *time.Time `json:"published_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
