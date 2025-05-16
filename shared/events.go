package shared

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"time"
)

// Event types
const (
	EventUserJoined = iota
	EventUserLeft
	EventUserDisconnected
	EventFileUploaded
	EventFileSending
	EventRoomCreated
	EventMessageSent
	EventServerNotice
	EventStatusChange
	EventTypingIndicator
)

// CreateEventMessage creates a standardized event message
func CreateEventMessage(eventType int, username, roomName, extraInfo string) Message {
	var content string

	switch eventType {
	case EventUserJoined:
		content = username + " has joined the room"
	case EventUserLeft:
		content = username + " has left the room"
	case EventUserDisconnected:
		content = username + " has disconnected from the server"
	case EventFileUploaded:
		content = "File " + extraInfo + " uploaded by " + username + " is available"
	case EventFileSending:
		content = username + " is sending file: " + extraInfo
	case EventRoomCreated:
		content = "Room " + roomName + " has been created by " + username
	case EventStatusChange:
		content = username + " is now " + extraInfo
	case EventTypingIndicator:
		content = username + " is typing..."
	case EventServerNotice:
		content = extraInfo
	default:
		content = extraInfo
	}

	return Message{
		Type:      MessageTypeText,
		Content:   content,
		Sender:    "Server",
		Room:      roomName,
		Timestamp: time.Now(),
	}
}

// FormatEventMessage formats an event message for console display
func FormatEventMessage(timestamp time.Time, content string) string {
	return "[" + timestamp.Format("15:04:05") + "] " + content
}

// Encrypt encrypts a message with the given key
func Encrypt(text string, key []byte) (string, error) {
	plaintext := []byte(text)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a message with the given key
func Decrypt(cryptoText string, key []byte) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(cryptoText)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return string(ciphertext), nil
}
