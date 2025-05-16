package main

import (
	"encoding/json"
	"fmt"
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
)

// MessageStore manages all message history for rooms and direct messages
type MessageStore struct {
	mu             sync.RWMutex
	roomMessages   map[string][]shared.Message // map[roomName][]Message
	directMessages map[string][]shared.Message // map[user1_user2][]Message
	server         *Server
}

func NewMessageStore(server *Server) *MessageStore {
	// Create message history directory if it doesn't exist
	if err := os.MkdirAll(MessageHistoryDir, 0755); err != nil {
		log.Printf("Failed to create message history directory: %v", err)
	}

	ms := &MessageStore{
		roomMessages:   make(map[string][]shared.Message),
		directMessages: make(map[string][]shared.Message),
		server:         server,
	}

	// Load existing message history
	ms.loadAllMessages()

	return ms
}

// loadAllMessages loads all saved message history from files
func (ms *MessageStore) loadAllMessages() {
	// Load room messages
	files, err := filepath.Glob(filepath.Join(MessageHistoryDir, "room_*.json"))
	if err != nil {
		log.Printf("Error searching for room history files: %v", err)
		return
	}

	for _, file := range files {
		// Extract room name from filename
		roomName := filepath.Base(file)
		roomName = roomName[5 : len(roomName)-5] // Remove "room_" prefix and ".json" suffix

		// Load messages
		messages, err := ms.loadMessagesFromFile(file)
		if err != nil {
			log.Printf("Error loading room messages for %s: %v", roomName, err)
			continue
		}

		ms.roomMessages[roomName] = messages
		log.Printf("Loaded %d messages for room: %s", len(messages), roomName)
	}

	// Load direct messages
	files, err = filepath.Glob(filepath.Join(MessageHistoryDir, "dm_*.json"))
	if err != nil {
		log.Printf("Error searching for direct message history files: %v", err)
		return
	}

	for _, file := range files {
		// Extract conversation key from filename
		conversationKey := filepath.Base(file)
		conversationKey = conversationKey[3 : len(conversationKey)-5] // Remove "dm_" prefix and ".json" suffix

		// Load messages
		messages, err := ms.loadMessagesFromFile(file)
		if err != nil {
			log.Printf("Error loading direct messages for %s: %v", conversationKey, err)
			continue
		}

		ms.directMessages[conversationKey] = messages
		log.Printf("Loaded %d direct messages for conversation: %s", len(messages), conversationKey)
	}
}

// loadMessagesFromFile loads messages from a JSON file
func (ms *MessageStore) loadMessagesFromFile(filePath string) ([]shared.Message, error) {
	// Read file
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	// Parse JSON
	var messages []shared.Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	return messages, nil
}

// saveMessagesToFile saves messages to a JSON file
func (ms *MessageStore) saveMessagesToFile(filePath string, messages []shared.Message) error {
	// Serialize to JSON
	data, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("error serializing messages: %v", err)
	}

	// Create parent directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating directory: %v", err)
	}

	// Write to file
	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("error writing file: %v", err)
	}

	return nil
}

// AddRoomMessage adds a message to a room's history
func (ms *MessageStore) AddRoomMessage(roomName string, msg shared.Message) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Ensure the message has all required fields
	if msg.Sender == "" || msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	// Store a copy of the message to avoid later modifications affecting stored history
	messageCopy := msg

	// Create the room history if it doesn't exist
	if _, exists := ms.roomMessages[roomName]; !exists {
		ms.roomMessages[roomName] = make([]shared.Message, 0)
	}

	// Add the message to history
	ms.roomMessages[roomName] = append(ms.roomMessages[roomName], messageCopy)

	// Save to file automatically
	filePath := filepath.Join(MessageHistoryDir, fmt.Sprintf("room_%s.json", roomName))
	if err := ms.saveMessagesToFile(filePath, ms.roomMessages[roomName]); err != nil {
		log.Printf("Error saving room message history for %s: %v", roomName, err)
	}
}

// GetRoomHistory returns all messages for a room
func (ms *MessageStore) GetRoomHistory(roomName string) []shared.Message {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if messages, exists := ms.roomMessages[roomName]; exists {
		// Create a copy of messages to avoid race conditions
		result := make([]shared.Message, len(messages))
		copy(result, messages)
		return result
	}

	return []shared.Message{}
}

// AddDirectMessage adds a message to the direct message history between two users
func (ms *MessageStore) AddDirectMessage(sender, recipient string, msg shared.Message) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Create a consistent key for the conversation (alphabetical order)
	key := getConversationKey(sender, recipient)

	// Ensure the message has all required fields
	messageCopy := msg
	if messageCopy.Timestamp.IsZero() {
		messageCopy.Timestamp = time.Now()
	}

	// Create the conversation if it doesn't exist
	if _, exists := ms.directMessages[key]; !exists {
		ms.directMessages[key] = make([]shared.Message, 0)
	}

	// Add the message to history
	ms.directMessages[key] = append(ms.directMessages[key], messageCopy)

	// Save to file automatically
	filePath := filepath.Join(MessageHistoryDir, fmt.Sprintf("dm_%s.json", key))
	if err := ms.saveMessagesToFile(filePath, ms.directMessages[key]); err != nil {
		log.Printf("Error saving direct message history for %s: %v", key, err)
	}
}

// GetDirectMessageHistory returns all direct messages between two users
func (ms *MessageStore) GetDirectMessageHistory(user1, user2 string) []shared.Message {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	key := getConversationKey(user1, user2)

	if messages, exists := ms.directMessages[key]; exists {
		// Create a copy of messages to avoid race conditions
		result := make([]shared.Message, len(messages))
		copy(result, messages)
		return result
	}

	return []shared.Message{}
}

func getConversationKey(user1, user2 string) string {
	if user1 < user2 {
		return user1 + "_" + user2
	}
	return user2 + "_" + user1
}
