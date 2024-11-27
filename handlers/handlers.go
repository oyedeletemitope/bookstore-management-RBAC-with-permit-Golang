package handlers

import (
	"bookstore/middleware"
	"bookstore/models" // Use your models package here
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/permitio/permit-golang/pkg/config"
	"github.com/permitio/permit-golang/pkg/enforcement"
	permitModels "github.com/permitio/permit-golang/pkg/models"
	"github.com/permitio/permit-golang/pkg/permit"
)

var tmpl = template.Must(template.ParseGlob("templates/*.html"))

// Helper function to convert string to *string
func StringPtr(s string) *string {
	return &s
}

type Handlers struct {
	db           *sql.DB
	permitClient *permit.Client
}

func NewHandlers(db *sql.DB, apiKey string) *Handlers {
	permitConfig := config.NewConfigBuilder(apiKey).
		WithPdpUrl("http://localhost:7766").
		Build()
	permitClient := permit.NewPermit(permitConfig)
	if permitClient == nil {
		log.Fatalf("Failed to initialize Permit.io client")
	}

	return &Handlers{
		db:           db,
		permitClient: permitClient,
	}
}

func (h *Handlers) LoginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			if err := tmpl.ExecuteTemplate(w, "login.html", nil); err != nil {
				http.Error(w, "Error rendering template", http.StatusInternalServerError)
				return
			}
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		user, err := middleware.LoginUser(h.db, username, password)
		if err != nil {
			log.Printf("Login failed for user %s: %v\n", username, err)
			http.Error(w, "Invalid login credentials", http.StatusUnauthorized)
			return
		}
		role := user.Role // Extract the role string

		// Set username in a cookie to persist across requests
		http.SetCookie(w, &http.Cookie{
			Name:     "username",
			Value:    username,
			Path:     "/",
			Expires:  time.Now().Add(24 * time.Hour),
			HttpOnly: true,
			Secure:   r.TLS != nil, // Only secure if using HTTPS
		})

		// Create context for syncing user with Permit.io
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Create Permit.io User object with attributes
		permitUser := permitModels.NewUserCreate(username)
		permitUser.SetAttributes(map[string]interface{}{
			"role": role,
		})

		_, err = h.permitClient.SyncUser(ctx, *permitUser)
		if err != nil {
			log.Printf("Permit sync failed: %v\n", err)
		}

		// Render the index.html page with the user's username and role
		data := struct {
			Username string
			Role     string
		}{
			Username: username,
			Role:     role,
		}

		if err := tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
			log.Printf("Template execution error: %v\n", err)
			http.Error(w, "Error displaying page", http.StatusInternalServerError)
		}

	}
}

func (h *Handlers) BooksHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("username")
		if err != nil {
			log.Printf("Cookie error: %v\n", err)
			http.Error(w, "Unauthorized access: no username found", http.StatusUnauthorized)
			return
		}
		username := cookie.Value

		role, err := middleware.GetUserRole(h.db, username)
		if err != nil {
			log.Printf("Database role lookup error: %v\n", err)
			http.Error(w, "Error retrieving user role", http.StatusInternalServerError)
			return
		}

		user := enforcement.UserBuilder(username).
			WithAttributes(map[string]interface{}{
				"role": role,
			}).
			Build()

		resource := enforcement.ResourceBuilder("books").
			WithTenant("default").
			Build()

		permitted, err := h.permitClient.Check(user, "view", resource)
		if err != nil {
			log.Printf("Permission check error: %v\n", err)
			http.Error(w, "Error checking permissions", http.StatusInternalServerError)
			return
		}

		if !permitted {
			log.Printf("Access denied for user %s with role %s\n", username, role)
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		rows, err := h.db.Query("SELECT id, title, author, published_at, created_at FROM books")
		if err != nil {
			log.Printf("Database query error: %v\n", err)
			http.Error(w, "Error fetching books", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var books []models.Book
		for rows.Next() {
			var book models.Book
			var publishedAt sql.NullTime

			err := rows.Scan(&book.ID, &book.Title, &book.Author, &publishedAt, &book.CreatedAt)
			if err != nil {
				log.Printf("Row scan error: %v\n", err)
				http.Error(w, "Error reading book data", http.StatusInternalServerError)
				return
			}

			if publishedAt.Valid {
				book.PublishedAt = &publishedAt.Time
			}

			books = append(books, book)
		}

		if err = rows.Err(); err != nil {
			log.Printf("Row iteration error: %v\n", err)
			http.Error(w, "Error reading book data", http.StatusInternalServerError)
			return
		}

		// Render the books template
		if err := tmpl.ExecuteTemplate(w, "books.html", books); err != nil {
			log.Printf("Template execution error: %v\n", err)
			http.Error(w, "Error displaying books", http.StatusInternalServerError)
		}
	}
}

func (h *Handlers) AddBookHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("Entered AddBookHandler") // Log entry into handler

		// Retrieve username from session or cookie
		cookie, err := r.Cookie("username")
		if err != nil {
			http.Error(w, "Unauthorized access: no username found", http.StatusUnauthorized)
			return
		}
		username := cookie.Value

		// Retrieve user's role from the database
		role, err := middleware.GetUserRole(h.db, username)
		if err != nil {
			log.Printf("Database role lookup error: %v\n", err)
			http.Error(w, "Error retrieving user role", http.StatusInternalServerError)
			return
		}

		// Permission check (using Permit.io) - only allow users with "create" permission
		user := enforcement.UserBuilder(username).
			WithAttributes(map[string]interface{}{
				"role": role,
			}).
			Build()

		resource := enforcement.ResourceBuilder("books").
			WithTenant("default").
			Build()

		permitted, err := h.permitClient.Check(user, "create", resource)
		if err != nil {
			log.Printf("Permission check error: %v\n", err)
			http.Error(w, "Error checking permissions", http.StatusInternalServerError)
			return
		}

		// If not permitted, display alert and deny access
		if !permitted {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `
				<!DOCTYPE html>
				<html lang="en">
				<head>
					<meta charset="UTF-8">
					<meta name="viewport" content="width=device-width, initial-scale=1.0">
					<title>Access Denied</title>
					<script>alert('You cannot add a book as you are an admin.')</script>
				</head>
				<body>
					<p>You do not have permission to add books.</p>
					<a href="/books">Back to Books</a>
				</body>
				</html>
			`)
			return
		}

		// Handle GET request to render add.html
		if r.Method == http.MethodGet {
			log.Println("Rendering add.html for GET request")
			if err := tmpl.ExecuteTemplate(w, "add.html", nil); err != nil {
				log.Printf("Template execution error: %v\n", err)
				http.Error(w, "Error displaying page", http.StatusInternalServerError)
			}
			return
		}

		// Handle POST request to add a new book
		if r.Method == http.MethodPost {
			title := strings.TrimSpace(r.FormValue("title"))
			author := strings.TrimSpace(r.FormValue("author"))
			publishedAt := strings.TrimSpace(r.FormValue("published_at"))

			// Debugging: Print received values
			log.Printf("Received values - Title: %s, Author: %s, Published At: %s\n", title, author, publishedAt)

			// Parse published_at date
			var pubDate sql.NullTime
			if publishedAt != "" {
				parsedDate, err := time.Parse("2006-01-02", publishedAt)
				if err == nil {
					pubDate = sql.NullTime{Time: parsedDate, Valid: true}
				}
			}

			// Insert book into the database
			_, err := h.db.Exec("INSERT INTO books (id, title, author, published_at, created_at) VALUES ($1, $2, $3, $4, NOW())",
				uuid.New(), title, author, pubDate)
			if err != nil {
				log.Printf("Error adding book to database: %v\n", err)
				http.Error(w, "Error adding book", http.StatusInternalServerError)
				return
			}

			// Redirect to books page after successful addition
			http.Redirect(w, r, "/books", http.StatusSeeOther)
		}
	}
}

func (h *Handlers) DeleteBookHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Retrieve username from session or cookie
		cookie, err := r.Cookie("username")
		if err != nil {
			http.Error(w, "Unauthorized access: no username found", http.StatusUnauthorized)
			return
		}
		username := cookie.Value

		// Retrieve user's role from the database
		role, err := middleware.GetUserRole(h.db, username)
		if err != nil {
			log.Printf("Database role lookup error: %v\n", err)
			http.Error(w, "Error retrieving user role", http.StatusInternalServerError)
			return
		}

		// Permission check for "delete" action using Permit.io
		user := enforcement.UserBuilder(username).
			WithAttributes(map[string]interface{}{
				"role": role,
			}).
			Build()

		resource := enforcement.ResourceBuilder("books").
			WithTenant("default").
			Build()

		permitted, err := h.permitClient.Check(user, "delete", resource)
		if err != nil {
			log.Printf("Permission check error: %v\n", err)
			http.Error(w, "Error checking permissions", http.StatusInternalServerError)
			return
		}

		if !permitted {
			log.Printf("Access denied for user %s with role %s\n", username, role)
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		// Retrieve book ID from the form data
		bookIDStr := r.FormValue("id")
		bookID, err := uuid.Parse(bookIDStr)
		if err != nil {
			log.Printf("Invalid book ID: %v\n", err)
			http.Error(w, "Invalid book ID", http.StatusBadRequest)
			return
		}

		// Delete the book from the database
		_, err = h.db.Exec("DELETE FROM books WHERE id = $1", bookID)
		if err != nil {
			log.Printf("Database delete error: %v\n", err)
			http.Error(w, "Error deleting book", http.StatusInternalServerError)
			return
		}

		// Redirect to the books page after successful deletion
		http.Redirect(w, r, "/books", http.StatusSeeOther)
	}
}

func (h *Handlers) UpdateBookHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Retrieve username from session or cookie
		cookie, err := r.Cookie("username")
		if err != nil {
			http.Error(w, "Unauthorized access: no username found", http.StatusUnauthorized)
			return
		}
		username := cookie.Value

		// Retrieve user's role from the database
		role, err := middleware.GetUserRole(h.db, username)
		if err != nil {
			log.Printf("Database role lookup error: %v\n", err)
			http.Error(w, "Error retrieving user role", http.StatusInternalServerError)
			return
		}

		// Permission check for "update" action using Permit.io
		user := enforcement.UserBuilder(username).
			WithAttributes(map[string]interface{}{
				"role": role,
			}).
			Build()

		resource := enforcement.ResourceBuilder("books").
			WithTenant("default").
			Build()

		permitted, err := h.permitClient.Check(user, "update", resource)
		if err != nil {
			log.Printf("Permission check error: %v\n", err)
			http.Error(w, "Error checking permissions", http.StatusInternalServerError)
			return
		}

		if !permitted {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `
				<!DOCTYPE html>
				<html lang="en">
				<head>
					<meta charset="UTF-8">
					<meta name="viewport" content="width=device-width, initial-scale=1.0">
					<title>Access Denied</title>
					<script>alert('You are not authorized to update books.')</script>
				</head>
				<body>
					<p>Access Denied. You do not have permission to update books.</p>
					<a href="/books">Back to Books</a>
				</body>
				</html>
			`)
			return
		}

		// If request is GET, render the update page with current book details
		if r.Method == http.MethodGet {
			bookID := r.FormValue("id")
			var book models.Book
			err := h.db.QueryRow("SELECT id, title, author, published_at FROM books WHERE id = $1", bookID).
				Scan(&book.ID, &book.Title, &book.Author, &book.PublishedAt)
			if err != nil {
				log.Printf("Error fetching book for update: %v\n", err)
				http.Error(w, "Error fetching book", http.StatusInternalServerError)
				return
			}

			if err := tmpl.ExecuteTemplate(w, "update.html", book); err != nil {
				log.Printf("Template execution error: %v\n", err)
				http.Error(w, "Error displaying update page", http.StatusInternalServerError)
			}
			return
		}

		// Handle POST request for updating book details
		if r.Method == http.MethodPost {
			bookID := r.FormValue("id")
			title := r.FormValue("title")
			author := r.FormValue("author")
			publishedAt := r.FormValue("published_at")

			var pubDate sql.NullTime
			if publishedAt != "" {
				parsedDate, err := time.Parse("2006-01-02", publishedAt)
				if err == nil {
					pubDate = sql.NullTime{Time: parsedDate, Valid: true}
				}
			}

			_, err := h.db.Exec("UPDATE books SET title = $1, author = $2, published_at = $3 WHERE id = $4",
				title, author, pubDate, bookID)
			if err != nil {
				log.Printf("Error updating book: %v\n", err)
				http.Error(w, "Error updating book", http.StatusInternalServerError)
				return
			}

			http.Redirect(w, r, "/books", http.StatusSeeOther)
		}
	}
}
