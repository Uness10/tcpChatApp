package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"chatap.com/shared"
)

const (
	MessageHistoryDir = "message_history"
	MaxRoomMessages   = 100 // Maximum messages to store per room
)

// MessageStore handles persisting and retrieving message history
type MessageStore struct {
	roomMessages   map[string][]shared.Message
	directMessages map[string][]shared.Message // Key is "user1_user2" (always alphabetically sorted)
	mu             sync.RWMutex
	server         *Server
}

// NewMessageStore creates a new message store
func NewMessageStore(server *Server) *MessageStore {
	store := &MessageStore{
		roomMessages:   make(map[string][]shared.Message),
		directMessages: make(map[string][]shared.Message),
		server:         server,
	}

	// Create message history directory if it doesn't exist
	if err := os.MkdirAll(MessageHistoryDir, 0755); err != nil {
		log.Printf("Failed to create message history directory: %v", err)
	}

	// Load message history from disk
	store.loadHistory()

	// Start periodic saving
	go store.periodicSave()

	return store
}

// AddRoomMessage adds a message to a room's history
func (ms *MessageStore) AddRoomMessage(roomName string, msg shared.Message) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	messages, exists := ms.roomMessages[roomName]
	if !exists {
		messages = make([]shared.Message, 0, MaxRoomMessages)
	}

	// Add message to history
	messages = append(messages, msg)

	// Trim if we have too many messages
	if len(messages) > MaxRoomMessages {
		messages = messages[len(messages)-MaxRoomMessages:]
	}

	ms.roomMessages[roomName] = messages
}

// GetRoomHistory returns the message history for a room
func (ms *MessageStore) GetRoomHistory(roomName string) []shared.Message {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	messages, exists := ms.roomMessages[roomName]
	if !exists {
		return []shared.Message{}
	}

	// Return a copy to avoid race conditions
	result := make([]shared.Message, len(messages))
	copy(result, messages)

	return result
}

// AddDirectMessage adds a direct message to the history
func (ms *MessageStore) AddDirectMessage(from, to string, msg shared.Message) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Always sort users alphabetically for consistent key
	key := from + "_" + to
	if from > to {
		key = to + "_" + from
	}

	messages, exists := ms.directMessages[key]
	if !exists {
		messages = make([]shared.Message, 0, MaxRoomMessages)
	}

	// Add message to history
	messages = append(messages, msg)

	// Trim if we have too many messages
	if len(messages) > MaxRoomMessages {
		messages = messages[len(messages)-MaxRoomMessages:]
	}

	ms.directMessages[key] = messages
}

// GetDirectMessageHistory returns the direct message history between two users
func (ms *MessageStore) GetDirectMessageHistory(user1, user2 string) []shared.Message {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	// Always sort users alphabetically for consistent key
	key := user1 + "_" + user2
	if user1 > user2 {
		key = user2 + "_" + user1
	}

	messages, exists := ms.directMessages[key]
	if !exists {
		return []shared.Message{}
	}

	// Return a copy to avoid race conditions
	result := make([]shared.Message, len(messages))
	copy(result, messages)

	return result
}

// saveHistory saves the message history to disk
func (ms *MessageStore) saveHistory() error {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	// Save room messages
	for room, messages := range ms.roomMessages {
		data, err := json.Marshal(messages)
		if err != nil {
			log.Printf("Failed to marshal room messages for %s: %v", room, err)
			continue
		}

		filename := filepath.Join(MessageHistoryDir, "room_"+room+".json")
		if err := ioutil.WriteFile(filename, data, 0644); err != nil {
			log.Printf("Failed to save room messages for %s: %v", room, err)
		}
	}

	// Save direct messages
	for key, messages := range ms.directMessages {
		data, err := json.Marshal(messages)
		if err != nil {
			log.Printf("Failed to marshal direct messages for %s: %v", key, err)
			continue
		}

		filename := filepath.Join(MessageHistoryDir, "dm_"+key+".json")
		if err := ioutil.WriteFile(filename, data, 0644); err != nil {
			log.Printf("Failed to save direct messages for %s: %v", key, err)
		}
	}

	log.Println("Message history saved successfully")
	return nil
}

// loadHistory loads the message history from disk
func (ms *MessageStore) loadHistory() {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(MessageHistoryDir, 0755); err != nil {
		log.Printf("Failed to create message history directory: %v", err)
		return
	}

	files, err := ioutil.ReadDir(MessageHistoryDir)
	if err != nil {
		log.Printf("Failed to read message history directory: %v", err)
		return
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := filepath.Join(MessageHistoryDir, file.Name())
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			log.Printf("Failed to read message history file %s: %v", filename, err)
			continue
		}

		var messages []shared.Message
		if err := json.Unmarshal(data, &messages); err != nil {
			log.Printf("Failed to unmarshal message history from %s: %v", filename, err)
			continue
		}

		// Determine if this is a room or direct message history
		if len(file.Name()) > 5 && file.Name()[:5] == "room_" {
			roomName := file.Name()[5 : len(file.Name())-5] // Remove "room_" prefix and ".json" suffix
			ms.roomMessages[roomName] = messages
		} else if len(file.Name()) > 3 && file.Name()[:3] == "dm_" {
			key := file.Name()[3 : len(file.Name())-5] // Remove "dm_" prefix and ".json" suffix
			ms.directMessages[key] = messages
		}
	}

	log.Println("Message history loaded successfully")
}

// periodicSave saves the message history periodically
func (ms *MessageStore) periodicSave() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if err := ms.saveHistory(); err != nil {
			log.Printf("Periodic message history save failed: %v", err)
		}
	}
}
