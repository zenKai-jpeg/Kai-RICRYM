package main

import (
	"fmt"
	"log"
	"net/http"

	"backendGo/auth"
	"backendGo/cache"
	"backendGo/database"
	"backendGo/handlers"
	"backendGo/session"
	"backendGo/utils"
) // Add this new import

func main() {
	// Initialize the cache
	cache.InitializeCache()

	// Connect to the database
	db := database.ConnectDB()
	defer db.Close()

	// Create tables if needed
	database.CreateTables(db)

	// Populate the database with fake data if it is empty
	database.GenerateDataIfNeeded(db)

	// Generate sessions for all accounts
	session.GenerateSessionsForAllAccounts(db)

	// Set up HTTP routes
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		utils.WriteJSONResponse(w, http.StatusOK, map[string]string{
			"message": "Welcome to the API!",
		})
	})
	http.HandleFunc("/accounts", func(w http.ResponseWriter, r *http.Request) {
		handlers.PaginatedHandler(w, r, db)
	})
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		auth.LoginHandler(w, r, db)
	})
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		auth.RegisterHandler(w, r, db)
	})
	http.HandleFunc("/verify-email", func(w http.ResponseWriter, r *http.Request) {
		auth.VerifyEmailHandler(w, r, db)
	}) // Add this new route

	// Start HTTP server
	port := ":8080"
	fmt.Printf("Server is running at %s\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
