package main

import (
	"bookstore/handlers"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func connectDB() *sql.DB {

	connStr := "user=bookstore_user password=1111 dbname=bookstore_db sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Error connecting to the database:", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatal("Cannot ping the database:", err)
	}
	fmt.Println("Successfully connected to the database!")
	return db
}

func main() {
	// Load environment variables from .env file

	err := godotenv.Load()
	if err != nil {
		log.Print("Error loading .env file")
	}

	// Get Permit.io API key from environment variable
	permitApiKey := os.Getenv("PERMIT_API_KEY")
	if permitApiKey == "" {
		log.Fatal("PERMIT_API_KEY environment variable is not set")
	}

	db := connectDB()
	defer db.Close()

	r := mux.NewRouter()

	// Create handlers with the API key
	h := handlers.NewHandlers(db, permitApiKey)

	// Register routes
	r.HandleFunc("/login", h.LoginHandler()).Methods("GET", "POST")
	r.HandleFunc("/books", h.BooksHandler()).Methods("GET")
	r.HandleFunc("/add", h.AddBookHandler()).Methods("GET", "POST")
	r.HandleFunc("/delete", h.DeleteBookHandler()).Methods("POST")
	r.HandleFunc("/update", h.UpdateBookHandler()).Methods("GET", "POST")

	fmt.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
