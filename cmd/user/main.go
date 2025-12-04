package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/crueber/home-links/internal/auth"
	"github.com/crueber/home-links/internal/db"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	dbPath := getEnv("DATABASE_PATH", "./data/bookmarks.db")

	// Initialize database
	database, err := db.New(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	command := os.Args[1]

	switch command {
	case "create":
		handleCreate(database)
	case "delete":
		handleDelete(database)
	case "list":
		handleList(database)
	case "reset-password":
		handleResetPassword(database)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func handleCreate(database *db.DB) {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: user create <username>")
		os.Exit(1)
	}

	username := os.Args[2]

	// Check if user already exists
	existingUser, err := database.GetUserByUsername(username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Database error: %v\n", err)
		os.Exit(1)
	}
	if existingUser != nil {
		fmt.Fprintf(os.Stderr, "User '%s' already exists\n", username)
		os.Exit(1)
	}

	// Prompt for password
	fmt.Print("Enter password: ")
	password, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read password: %v\n", err)
		os.Exit(1)
	}

	if len(password) < 8 {
		fmt.Fprintln(os.Stderr, "Password must be at least 8 characters")
		os.Exit(1)
	}

	fmt.Print("Confirm password: ")
	confirmPassword, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read password: %v\n", err)
		os.Exit(1)
	}

	if string(password) != string(confirmPassword) {
		fmt.Fprintln(os.Stderr, "Passwords do not match")
		os.Exit(1)
	}

	// Hash password
	passwordHash, err := auth.HashPassword(string(password))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to hash password: %v\n", err)
		os.Exit(1)
	}

	// Create user
	user, err := database.CreateUser(username, passwordHash)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create user: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("User '%s' created successfully (ID: %d)\n", user.Username, user.ID)
}

func handleDelete(database *db.DB) {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: user delete <username>")
		os.Exit(1)
	}

	username := os.Args[2]

	// Confirm deletion
	fmt.Printf("Are you sure you want to delete user '%s'? This will also delete all their data. (yes/no): ", username)
	var confirm string
	fmt.Scanln(&confirm)

	if confirm != "yes" {
		fmt.Println("Deletion cancelled")
		return
	}

	// Delete user
	if err := database.DeleteUser(username); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to delete user: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("User '%s' deleted successfully\n", username)
}

func handleList(database *db.DB) {
	users, err := database.ListUsers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list users: %v\n", err)
		os.Exit(1)
	}

	if len(users) == 0 {
		fmt.Println("No users found")
		return
	}

	fmt.Println("Users:")
	for _, user := range users {
		fmt.Printf("  - %s (ID: %d, Created: %s)\n", user.Username, user.ID, user.CreatedAt.Format("2006-01-02 15:04:05"))
	}
}

func handleResetPassword(database *db.DB) {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: user reset-password <username>")
		os.Exit(1)
	}

	username := os.Args[2]

	// Check if user exists
	user, err := database.GetUserByUsername(username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Database error: %v\n", err)
		os.Exit(1)
	}
	if user == nil {
		fmt.Fprintf(os.Stderr, "User '%s' not found\n", username)
		os.Exit(1)
	}

	// Prompt for new password
	fmt.Print("Enter new password: ")
	password, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read password: %v\n", err)
		os.Exit(1)
	}

	if len(password) < 8 {
		fmt.Fprintln(os.Stderr, "Password must be at least 8 characters")
		os.Exit(1)
	}

	fmt.Print("Confirm password: ")
	confirmPassword, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read password: %v\n", err)
		os.Exit(1)
	}

	if string(password) != string(confirmPassword) {
		fmt.Fprintln(os.Stderr, "Passwords do not match")
		os.Exit(1)
	}

	// Hash password
	passwordHash, err := auth.HashPassword(string(password))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to hash password: %v\n", err)
		os.Exit(1)
	}

	// Update password
	if err := database.UpdateUserPassword(username, passwordHash); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to reset password: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Password for user '%s' reset successfully\n", username)
}

func printUsage() {
	fmt.Println("User Management Tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  user create <username>          Create a new user")
	fmt.Println("  user delete <username>          Delete a user")
	fmt.Println("  user list                       List all users")
	fmt.Println("  user reset-password <username>  Reset a user's password")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  DATABASE_PATH   Path to the SQLite database (default: ./data/bookmarks.db)")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
