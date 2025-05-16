package main

import (
	"log"
)

func main() {
	server := NewServer(":8080")

	// Register some test users
	server.AuthManager.RegisterUser("admin", "admin123")
	server.AuthManager.RegisterUser("test", "test123")

	log.Fatal(server.Run())
}
