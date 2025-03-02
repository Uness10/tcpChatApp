package main

import (
	"log"
	"net"

	"tcpServer.com/config"
	"tcpServer.com/internal/chat"
	"tcpServer.com/internal/db"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize PostgreSQL connection
	pgDB, err := db.NewPostgresConnection(cfg.DB)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pgDB.Close()

	// Create repository
	repo := db.NewRepository(pgDB)

	// Create and start the chat server
	chatServer := chat.NewServer(repo)
	go chatServer.Run()

	// Start TCP listener
	listener, err := net.Listen("tcp", cfg.Server.Address)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()
	log.Printf("Server started on %s", cfg.Server.Address)

	// Accept incoming connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}
		go chatServer.NewClient(conn)
	}
}
