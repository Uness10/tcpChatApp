package shared

import (
	"time"
)

const (
	MessageTypeText = iota
	MessageTypeCommand
	MessageTypeFile
	MessageTypeAuth
	MessageTypeDirect    // Add type for direct messages
	MessageTypeStatus    // Add type for status updates
	MessageTypeEncrypted // Add type for encrypted messages
)

// UserStatus represents a user's online status
type UserStatus int

const (
	StatusOnline UserStatus = iota
	StatusAway
	StatusBusy
	StatusOffline
)

type Message struct {
	Type      int       `json:"type"`
	Content   string    `json:"content"`
	Sender    string    `json:"sender"`
	Room      string    `json:"room"`
	Timestamp time.Time `json:"timestamp"`
	Recipient string    `json:"recipient,omitempty"` // For direct messages
	Encrypted bool      `json:"encrypted,omitempty"` // For encrypted messages
}

type FileMessage struct {
	Message
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	ChunkID     int    `json:"chunk_id"`
	TotalChunks int    `json:"total_chunks"`
	Data        []byte `json:"data"`
}

type AuthMessage struct {
	Message
	Username string `json:"username"`
	Password string `json:"password"`
}

// DirectMessage type for private user-to-user messaging
type DirectMessage struct {
	Message
	Encrypted bool `json:"encrypted"`
}

// StatusMessage for user status updates
type StatusMessage struct {
	Message
	Status UserStatus `json:"status"`
}
