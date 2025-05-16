package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"

	"chatap.com/shared"
)

type Room struct {
	Name               string
	Clients            map[*Client]bool
	Broadcast          chan []byte
	mu                 sync.RWMutex
	Server             *Server // Add reference to server
	receivedFileChunks map[string][]shared.FileMessage
}

func NewRoom(name string, server *Server) *Room {
	return &Room{
		Name:               name,
		Clients:            make(map[*Client]bool),
		Broadcast:          make(chan []byte),
		Server:             server,
		receivedFileChunks: make(map[string][]shared.FileMessage),
	}
}

func (r *Room) AddClient(client *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Clients[client] = true
}

func (r *Room) RemoveClient(client *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.Clients, client)
}

func (r *Room) BroadcastMessage(message []byte, sender *Client) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clientCount := 0
	for client := range r.Clients {
		// If sender is provided, don't send to the original sender
		// If sender is nil, send to everyone
		if sender != nil && client == sender {
			continue
		}

		select {
		case client.Send <- message:
			clientCount++
		default:
			// Client's send buffer is full
			log.Printf("Message dropped for client %s (username: %s) in room %s: send buffer full.",
				client.Conn.RemoteAddr(), client.Username, r.Name)
		}
	}

	if clientCount > 0 {
		log.Printf("Broadcast message to %d clients in room %s", clientCount, r.Name)
	}
}

// BroadcastEvent broadcasts a standard event to all clients in the room
func (r *Room) BroadcastEvent(eventType int, username string, extraInfo string) {
	notification := shared.CreateEventMessage(eventType, username, r.Name, extraInfo)
	notificationBytes, _ := json.Marshal(notification)
	r.BroadcastMessage(notificationBytes, nil) // nil means broadcast to everyone
}

// HandleFileChunk stores file chunks and processes complete files
func (r *Room) HandleFileChunk(fileMsg shared.FileMessage) {
	fileKey := fileMsg.Sender + "_" + fileMsg.Filename // Create unique key per user and filename

	r.mu.Lock()
	if _, ok := r.receivedFileChunks[fileKey]; !ok {
		r.receivedFileChunks[fileKey] = make([]shared.FileMessage, 0, fileMsg.TotalChunks)
	}
	r.receivedFileChunks[fileKey] = append(r.receivedFileChunks[fileKey], fileMsg)
	currentChunks := len(r.receivedFileChunks[fileKey])
	r.mu.Unlock()

	// Check if all chunks are received
	if currentChunks == fileMsg.TotalChunks {
		go r.saveCompleteFile(fileKey, fileMsg.Filename)
	}
}

// saveCompleteFile saves the assembled file chunks to the server's uploads directory
func (r *Room) saveCompleteFile(fileKey string, filename string) {
	r.mu.Lock()
	chunks := r.receivedFileChunks[fileKey]
	delete(r.receivedFileChunks, fileKey) // Remove from memory after processing
	r.mu.Unlock()

	// Extract username from fileKey (format is "username_filename")
	parts := strings.SplitN(fileKey, "_", 2)
	username := parts[0]

	// Get the upload directory path from the server
	uploadDir := r.Server.GetRoomUploadPath(r.Name)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Printf("Failed to create upload directory for room %s: %v", r.Name, err)
		return
	}

	// Save the file
	if err := shared.SaveFileFromChunks(chunks, uploadDir); err != nil {
		log.Printf("Failed to save file %s in room %s: %v", filename, r.Name, err)
		return
	}

	log.Printf("File %s successfully saved in room %s at %s", filename, r.Name, uploadDir)

	// Notify room that file is available using the new event system
	r.BroadcastEvent(shared.EventFileUploaded, username, filename)
}

type RoomManager struct {
	Rooms  map[string]*Room
	mu     sync.RWMutex
	Server *Server // Add reference to server
}

func NewRoomManager(server *Server) *RoomManager {
	rm := &RoomManager{
		Rooms:  make(map[string]*Room),
		Server: server,
	}

	// Create a default room
	rm.CreateRoom("general")

	return rm
}

func (rm *RoomManager) CreateRoom(name string) *Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.Rooms[name]; !exists {
		rm.Rooms[name] = NewRoom(name, rm.Server)
	}

	return rm.Rooms[name]
}

func (rm *RoomManager) GetRoom(name string) *Room {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	room, exists := rm.Rooms[name]
	if !exists {
		return nil
	}

	return room
}

func (rm *RoomManager) DeleteRoom(name string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	delete(rm.Rooms, name)
}

func (rm *RoomManager) GetAllRooms() []string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	rooms := make([]string, 0, len(rm.Rooms))
	for name := range rm.Rooms {
		rooms = append(rooms, name)
	}

	return rooms
}
