package users

import (
	"database/sql"
	"errors"
	"log"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// User represents a user in the system
type User struct {
	Email        string
	PasswordHash string
	Name         string
	CreatedAt    time.Time
}

// UserStore manages user accounts
type UserStore struct {
	db *sql.DB
}

// NewUserStore creates a new user store with database connection
func NewUserStore(databaseURL string) (*UserStore, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	store := &UserStore{db: db}

	// Create table if not exists
	if err := store.createTable(); err != nil {
		return nil, err
	}

	// Add default admin user if no users exist
	count, err := store.countUsers()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		log.Println("No users found, creating default admin user")
		if err := store.AddUser("admin@indian-store.com", "admin123", "Admin User"); err != nil {
			log.Printf("Warning: Failed to create default admin user: %v", err)
		}
	}

	return store, nil
}

// createTable creates the users table
func (s *UserStore) createTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		email VARCHAR(255) PRIMARY KEY,
		password_hash VARCHAR(255) NOT NULL,
		name VARCHAR(255) NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW()
	)`

	_, err := s.db.Exec(query)
	return err
}

// countUsers returns the number of users
func (s *UserStore) countUsers() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

// AddUser adds a new user with hashed password
func (s *UserStore) AddUser(email, password, name string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	query := `INSERT INTO users (email, password_hash, name) VALUES ($1, $2, $3)`
	_, err = s.db.Exec(query, email, string(hashedPassword), name)
	if err != nil {
		// Check if it's a duplicate key error
		if err.Error() == "pq: duplicate key value violates unique constraint \"users_pkey\"" {
			return errors.New("user already exists")
		}
		return err
	}

	log.Printf("User created: %s (%s)", email, name)
	return nil
}

// Authenticate verifies email and password
func (s *UserStore) Authenticate(email, password string) (*User, error) {
	query := `SELECT email, password_hash, name, created_at FROM users WHERE email = $1`
	
	var user User
	err := s.db.QueryRow(query, email).Scan(&user.Email, &user.PasswordHash, &user.Name, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, errors.New("invalid credentials")
	}
	if err != nil {
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	return &user, nil
}

// GetUser retrieves a user by email
func (s *UserStore) GetUser(email string) (*User, bool) {
	query := `SELECT email, password_hash, name, created_at FROM users WHERE email = $1`
	
	var user User
	err := s.db.QueryRow(query, email).Scan(&user.Email, &user.PasswordHash, &user.Name, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, false
	}
	if err != nil {
		log.Printf("Error fetching user: %v", err)
		return nil, false
	}

	return &user, true
}

// ListUsers returns all users (without password hashes)
func (s *UserStore) ListUsers() ([]*User, error) {
	query := `SELECT email, name, created_at FROM users ORDER BY created_at DESC`
	
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.Email, &user.Name, &user.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, &user)
	}

	return users, nil
}

// DeleteUser removes a user
func (s *UserStore) DeleteUser(email string) error {
	query := `DELETE FROM users WHERE email = $1`
	result, err := s.db.Exec(query, email)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("user not found")
	}

	log.Printf("User deleted: %s", email)
	return nil
}

// Close closes the database connection
func (s *UserStore) Close() error {
	return s.db.Close()
}
