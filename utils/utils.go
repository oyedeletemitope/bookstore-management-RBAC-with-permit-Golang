package utils

import "golang.org/x/crypto/bcrypt"

// HashPassword hashes a plain-text password using bcrypt.
func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

$2a$10$Ur5ZdxBvLXbcATh4hSq8ge3v55jnap05si9YxyIlwvPezLaIaLcXm

$2a$10$wSrr4DwBrXdlfF1alDdpcufJj53Bbn0xNub4HUyX80xFer54IHtZ


INSERT INTO users (username, password_hash, role, email, first_name, last_name, created_at)
VALUES
    
    ('standard_user', '$2a$10$1GRNeox2V8AhAW2B/3Sgv.6Z8OJFZeK/r7TTITYs0BN3kzunT2juO', 'user', 'standard_user@example.com', 'John', 'Smith', NOW());

	DELETE FROM users WHERE username = 'standard_user';

	func (h *Handlers) BooksHandler() http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Step 1: Verify user via Permit.io
			cookie, err := r.Cookie("username")
			if err != nil {
				fmt.Printf("Cookie error: %v\n", err)
				http.Error(w, "Unauthorized access: no username found", http.StatusUnauthorized)
				return
			}
			username := cookie.Value
	
			role, err := middleware.GetUserRole(h.db, username)
			if err != nil {
				fmt.Printf("Database role lookup error: %v\n", err)
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
				WithAttributes(map[string]interface{}{
					"public": true,
				}).
				Build()
	
			permitted, err := h.permitClient.Check(user, "view", resource)
			if err != nil {
				fmt.Printf("Permission check error: %v\n", err)
				http.Error(w, "Error checking permissions", http.StatusInternalServerError)
				return
			}
	
			if !permitted {
				fmt.Printf("Access denied for user %s with role %s\n", username, role)
				http.Error(w, "Access denied", http.StatusForbidden)
				return
			}
	
			// Step 2: Fetch books from the database
			rows, err := h.db.Query("SELECT id, title, author, published_at, created_by, created_at FROM books")
			if err != nil {
				log.Printf("Database query error: %v\n", err)
				http.Error(w, "Error fetching books", http.StatusInternalServerError)
				return
			}
			defer rows.Close()
	
			var books []models.Book
			for rows.Next() {
				var book models.Book
				var id, createdBy string
				var publishedAt, createdAt sql.NullTime
	
				err := rows.Scan(&id, &book.Title, &book.Author, &publishedAt, &createdBy, &createdAt)
				if err != nil {
					log.Printf("Row scan error: %v\n", err)
					http.Error(w, "Error reading book data", http.StatusInternalServerError)
					return
				}
	
				book.ID, err = uuid.Parse(id)
				if err != nil {
					log.Printf("Error parsing book ID: %v\n", err)
					continue
				}
	
				book.CreatedBy, err = uuid.Parse(createdBy)
				if err != nil {
					log.Printf("Error parsing created by ID: %v\n", err)
					continue
				}
	
				if publishedAt.Valid {
					book.PublishedAt = &publishedAt.Time
				}
				if createdAt.Valid {
					book.CreatedAt = createdAt.Time
				}
	
				books = append(books, book)
			}
	
			if err = rows.Err(); err != nil {
				log.Printf("Row iteration error: %v\n", err)
				http.Error(w, "Error reading book data", http.StatusInternalServerError)
				return
			}
	
			// Step 3: Display books or "no books" message
			if len(books) == 0 {
				tmpl.ExecuteTemplate(w, "books.html", "no books to fetch")
				return
			}
	
			err = tmpl.ExecuteTemplate(w, "books.html", books)
			if err != nil {
				log.Printf("Template execution error: %v\n", err)
				http.Error(w, "Error displaying books", http.StatusInternalServerError)
			}
		}
	}
	