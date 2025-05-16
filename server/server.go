package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"chatap.com/shared"
)

const (
	UploadsDir = "uploads"
)

type Server struct {
	Addr         string
	AuthManager  *AuthManager
	RoomManager  *RoomManager
	MessageStore *MessageStore
	Clients      map[*Client]bool
	Register     chan *Client
	Unregister   chan *Client
	mu           sync.RWMutex
}

func NewServer(addr string) *Server {
	server := &Server{
		Addr:        addr,
		AuthManager: NewAuthManager(),
		Clients:     make(map[*Client]bool),
		Register:    make(chan *Client),
		Unregister:  make(chan *Client),
	}

	// Initialize message store
	server.MessageStore = NewMessageStore(server)

	// Initialize RoomManager with reference to server
	server.RoomManager = NewRoomManager(server)

	return server
}

func (s *Server) Run() error {
	// Create uploads directory
	if err := os.MkdirAll(UploadsDir, 0755); err != nil {
		return fmt.Errorf("failed to create uploads directory: %v", err)
	}
	log.Printf("Uploads directory initialized at: %s", UploadsDir)

	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	go s.handleChannels()

	log.Printf("TCP Chat Server started on %s", s.Addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}

		// Set timeout for idle connections
		conn.SetDeadline(time.Now().Add(5 * time.Minute))

		client := NewClient(conn, s)
		s.Register <- client

		go client.ReadPump()
		go client.WritePump()
	}
}

func (s *Server) handleChannels() {
	for {
		select {
		case client := <-s.Register:
			s.mu.Lock()
			s.Clients[client] = true
			s.mu.Unlock()
			log.Printf("New client connected: %s", client.Conn.RemoteAddr())

		case client := <-s.Unregister:
			s.mu.Lock()
			if _, ok := s.Clients[client]; ok {
				// Store room reference before removing the client
				room := client.Room
				username := client.Username

				delete(s.Clients, client)
				close(client.Send)

				// If client was in a room, notify other members about the disconnection
				if room != nil && username != "" {
					// First remove client from room's client list
					room.RemoveClient(client)

					// Then broadcast that they've disconnected
					room.BroadcastEvent(shared.EventUserDisconnected, username, "")

					log.Printf("Client %s removed from room %s due to disconnection",
						username, room.Name)
				}

				log.Printf("Client disconnected: %s", client.Conn.RemoteAddr())
			}
			s.mu.Unlock()
		}
	}
}

// Add this new method to find a client by username
func (s *Server) FindClientByUsername(username string) *Client {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for client := range s.Clients {
		if client.Username == username {
			return client
		}
	}
	return nil
}

// Add this new method to send a message to a specific client
func (s *Server) SendToClient(username string, message []byte) bool {
	client := s.FindClientByUsername(username)
	if client == nil {
		return false
	}

	client.SendDirectMessage(message)
	return true
}

// GetRoomUploadPath returns the path where files for a specific room should be stored
func (s *Server) GetRoomUploadPath(roomName string) string {
	return filepath.Join(UploadsDir, roomName)
}

// IsUserLoggedIn checks if a username is already being used by an active client
func (s *Server) IsUserLoggedIn(username string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for client := range s.Clients {
		if client.isLoggedIn && client.Username == username {
			return true
		}
	}
	return false
}
