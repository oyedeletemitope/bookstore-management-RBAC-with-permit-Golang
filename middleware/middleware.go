package middleware

import (
	"bookstore/models"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes the password for secure storage
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash compares a plain password with its hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// LoginUser authenticates a user and returns the full user object
func LoginUser(db *sql.DB, username, password string) (*models.User, error) {
	fmt.Printf("Attempting login for username: %s\n", username)

	var user models.User
	var passwordHash string

	err := db.QueryRow(`
		SELECT id, username, password_hash, role, email, first_name, last_name, created_at 
		FROM users 
		WHERE username = $1
	`, username).Scan(
		&user.ID,
		&user.Username,
		&passwordHash,
		&user.Role,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Printf("No user found with username: %s\n", username)
			return nil, fmt.Errorf("invalid credentials")
		}
		fmt.Printf("Database error: %v\n", err)
		return nil, err
	}

	// Check password match
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		fmt.Printf("Password comparison failed: %v\n", err)
		return nil, fmt.Errorf("invalid credentials")
	}

	user.PasswordHash = "" // Clear password hash before returning
	return &user, nil
}

// GetBooks retrieves all books from the database
func GetBooks(db *sql.DB) ([]models.Book, error) {
	rows, err := db.Query(`
		SELECT id, title, author, published_at, created_by, created_at 
		FROM books
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var books []models.Book
	for rows.Next() {
		var book models.Book
		err := rows.Scan(
			&book.ID,
			&book.Title,
			&book.Author,
			&book.PublishedAt,

			&book.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		books = append(books, book)
	}

	return books, nil
}

// CreateBook creates a new book in the database
func CreateBook(db *sql.DB, book *models.Book) error {
	book.ID = uuid.New()
	book.CreatedAt = time.Now()

	_, err := db.Exec(`
		INSERT INTO books (id, title, author, published_at, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`,
		book.ID,
		book.Title,
		book.Author,
		book.PublishedAt,

		book.CreatedAt,
	)

	return err
}

// UpdateBook updates an existing book in the database
func UpdateBook(db *sql.DB, book *models.Book) error {
	result, err := db.Exec(`
		UPDATE books 
		SET title = $1, author = $2, published_at = $3
		WHERE id = $4 AND created_by = $5
	`,
		book.Title,
		book.Author,
		book.PublishedAt,
		book.ID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("book not found or user not authorized")
	}

	return nil
}

// DeleteBook deletes a book from the database
func DeleteBook(db *sql.DB, bookID uuid.UUID, userID uuid.UUID) error {
	result, err := db.Exec(`
		DELETE FROM books 
		WHERE id = $1 AND created_by = $2
	`, bookID, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("book not found or user not authorized")
	}

	return nil
}

func GetUserRole(db *sql.DB, username string) (string, error) {
	var role string
	err := db.QueryRow("SELECT role FROM users WHERE username = $1", username).Scan(&role)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("no user found with username %s", username)
		}
		return "", err
	}
	return role, nil
}
