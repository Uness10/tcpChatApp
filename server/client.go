package main

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"strings"
	"time"

	"chatap.com/shared"
)

type Client struct {
	Conn       net.Conn
	Send       chan []byte
	Username   string
	Room       *Room
	Server     *Server
	isLoggedIn bool
	Status     shared.UserStatus
}

func NewClient(conn net.Conn, server *Server) *Client {
	return &Client{
		Conn:       conn,
		Send:       make(chan []byte, 256),
		Server:     server,
		isLoggedIn: false,
		Status:     shared.StatusOnline,
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.Server.Unregister <- c
		c.Conn.Close()
	}()

	reader := bufio.NewReader(c.Conn)

	for {
		message, err := reader.ReadBytes('\n')
		if err != nil {
			log.Printf("Error reading from client: %v", err)
			break
		}

		var msg shared.Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		c.handleMessage(msg, message)
	}
}

func (c *Client) WritePump() {
	defer func() {
		c.Conn.Close()
	}()

	for {
		message, ok := <-c.Send
		if !ok {
			return
		}

		c.Conn.Write(message)
		c.Conn.Write([]byte("\n"))
	}
}

func (c *Client) joinRoom(room *Room) {
	// Leave current room if any
	if c.Room != nil {
		c.Room.RemoveClient(c)
		c.Room.BroadcastEvent(shared.EventUserLeft, c.Username, "")
	}

	c.Room = room
	room.AddClient(c)

	// Notify room about new user
	room.BroadcastEvent(shared.EventUserJoined, c.Username, "")
}

func (c *Client) handleMessage(msg shared.Message, rawMsg []byte) {
	switch msg.Type {
	case shared.MessageTypeAuth:
		var authMsg shared.AuthMessage
		if err := json.Unmarshal(rawMsg, &authMsg); err != nil {
			log.Printf("Error unmarshaling auth message: %v", err)
			return
		}
		c.handleAuth(authMsg)

	case shared.MessageTypeCommand:
		c.handleCommand(msg)

	case shared.MessageTypeText:
		if !c.isLoggedIn {
			c.sendError("Not authenticated")
			return
		}

		if c.Room == nil {
			c.sendError("You are not in a room. Join a room first.")
			return
		}

		// Set message metadata
		msg.Sender = c.Username
		msg.Timestamp = time.Now()
		msg.Room = c.Room.Name // Ensure room name is set correctly

		log.Printf("Room message from %s in %s: %s", c.Username, c.Room.Name, msg.Content)

		// Re-encode message with updated metadata
		updatedMsg, err := json.Marshal(msg)
		if err != nil {
			log.Printf("Error marshaling message: %v", err)
			return
		}

		// Broadcast to everyone in the room (including back to sender for confirmation)
		c.Room.BroadcastMessage(updatedMsg, nil) // Use nil instead of c to send to everyone

		// Store message in history
		c.Server.MessageStore.AddRoomMessage(c.Room.Name, msg)

	case shared.MessageTypeFile:
		if !c.isLoggedIn {
			c.sendError("Not authenticated")
			return
		}

		var fileMsg shared.FileMessage
		if err := json.Unmarshal(rawMsg, &fileMsg); err != nil {
			log.Printf("Error unmarshaling file message: %v", err)
			return
		}

		fileMsg.Sender = c.Username
		fileMsg.Timestamp = time.Now()
		fileMsg.Room = c.Room.Name // Ensure room name is set properly

		if c.Room != nil {
			// Log receipt of file chunk
			log.Printf("Received file chunk %d/%d for %s from %s in room %s",
				fileMsg.ChunkID+1, fileMsg.TotalChunks, fileMsg.Filename,
				c.Username, c.Room.Name)

			// Announce file transfer to the room on first chunk
			if fileMsg.ChunkID == 0 {
				c.Room.BroadcastEvent(shared.EventFileSending, c.Username, fileMsg.Filename)
			}

			// Process the file chunk on the server
			c.Room.HandleFileChunk(fileMsg)

			// Forward the file chunk to other clients
			updatedMsg, _ := json.Marshal(fileMsg)
			log.Printf("Broadcasting file chunk %d/%d for %s to room %s",
				fileMsg.ChunkID+1, fileMsg.TotalChunks, fileMsg.Filename, c.Room.Name)
			c.Room.BroadcastMessage(updatedMsg, c)
		} else {
			c.sendError("You are not in a room. Join a room first.")
		}

	case shared.MessageTypeDirect:
		if !c.isLoggedIn {
			c.sendError("Not authenticated")
			return
		}

		// Set message metadata
		msg.Sender = c.Username
		msg.Timestamp = time.Now()

		// Find the recipient
		recipient := c.Server.FindClientByUsername(msg.Recipient)
		if recipient == nil {
			c.sendError("User not found: " + msg.Recipient)
			return
		}

		// Store in message history
		c.Server.MessageStore.AddDirectMessage(c.Username, msg.Recipient, msg)

		// Encode and send
		msgBytes, _ := json.Marshal(msg)
		recipient.SendDirectMessage(msgBytes)

		// Also send a copy back to sender for confirmation
		c.SendDirectMessage(msgBytes)

		log.Printf("Direct message from %s to %s", c.Username, msg.Recipient)

	case shared.MessageTypeEncrypted:
		if !c.isLoggedIn {
			c.sendError("Not authenticated")
			return
		}

		// Set message metadata
		msg.Sender = c.Username
		msg.Timestamp = time.Now()
		msg.Encrypted = true

		// Find the recipient
		recipient := c.Server.FindClientByUsername(msg.Recipient)
		if recipient == nil {
			c.sendError("User not found: " + msg.Recipient)
			return
		}

		// Note: For encrypted messages, we store only metadata in history, not content
		historyMsg := msg
		historyMsg.Content = "[Encrypted message]"
		c.Server.MessageStore.AddDirectMessage(c.Username, msg.Recipient, historyMsg)

		// Pass through the encrypted message
		msgBytes, _ := json.Marshal(msg)
		recipient.SendDirectMessage(msgBytes)

		// Also send a copy back to sender
		c.SendDirectMessage(msgBytes)

		log.Printf("Encrypted message from %s to %s", c.Username, msg.Recipient)

	case shared.MessageTypeStatus:
		if !c.isLoggedIn {
			c.sendError("Not authenticated")
			return
		}

		var statusMsg shared.StatusMessage
		if err := json.Unmarshal(rawMsg, &statusMsg); err != nil {
			log.Printf("Error unmarshaling status message: %v", err)
			return
		}

		c.Status = statusMsg.Status

		// Notify all rooms the user is in
		if c.Room != nil {
			var statusText string
			switch c.Status {
			case shared.StatusOnline:
				statusText = "online"
			case shared.StatusAway:
				statusText = "away"
			case shared.StatusBusy:
				statusText = "busy"
			case shared.StatusOffline:
				statusText = "offline"
			}

			c.Room.BroadcastEvent(shared.EventStatusChange, c.Username, statusText)
		}

		log.Printf("User %s changed status to %d", c.Username, c.Status)

	default:
		log.Printf("Unknown message type: %v", msg.Type)
	}
}

// Add this new method to send a message directly to this client
func (c *Client) SendDirectMessage(message []byte) {
	select {
	case c.Send <- message:
		// Message sent successfully
	default:
		// Client's message buffer is full
		c.Server.Unregister <- c
		close(c.Send)
	}
}

func (c *Client) handleAuth(authMsg shared.AuthMessage) {
	if authMsg.Content == "register" {
		success := c.Server.AuthManager.RegisterUser(authMsg.Username, authMsg.Password)
		if success {
			c.Username = authMsg.Username
			c.isLoggedIn = true
			c.sendSuccess("Registered and logged in successfully")

			// Remove automatic room joining
			// No longer joining default room
		} else {
			c.sendError("Username already exists")
		}
	} else {
		success := c.Server.AuthManager.AuthenticateUser(authMsg.Username, authMsg.Password)
		if success {
			c.Username = authMsg.Username
			c.isLoggedIn = true
			c.sendSuccess("Logged in successfully")

			// Remove automatic room joining
			// No longer joining default room
		} else {
			c.sendError("Invalid credentials")
		}
	}
}

func (c *Client) handleCommand(msg shared.Message) {
	if !c.isLoggedIn {
		c.sendError("Not authenticated")
		return
	}

	cmd := msg.Content

	switch cmd {
	case "rooms":
		rooms := c.Server.RoomManager.GetAllRooms()
		roomList, _ := json.Marshal(rooms)

		response := shared.Message{
			Type:      shared.MessageTypeCommand,
			Content:   string(roomList),
			Sender:    "Server",
			Timestamp: time.Now(),
		}

		respBytes, _ := json.Marshal(response)
		c.Send <- respBytes

	case "create":
		if msg.Room == "" {
			c.sendError("Room name not specified")
			return
		}

		room := c.Server.RoomManager.CreateRoom(msg.Room)
		c.joinRoom(room)
		c.sendSuccess("Room created and joined: " + msg.Room)

	case "join":
		if msg.Room == "" {
			c.sendError("Room name not specified")
			return
		}

		room := c.Server.RoomManager.GetRoom(msg.Room)
		if room == nil {
			c.sendError("Room not found: " + msg.Room)
			return
		}

		c.joinRoom(room)

		// Send recent message history
		history := c.Server.MessageStore.GetRoomHistory(room.Name)
		if len(history) > 0 {
			historyMsg := shared.Message{
				Type:      shared.MessageTypeCommand,
				Content:   "Recent messages:",
				Sender:    "Server",
				Timestamp: time.Now(),
			}
			historyBytes, _ := json.Marshal(historyMsg)
			c.SendDirectMessage(historyBytes)

			// Send the last 10 messages or all if less than 10
			start := 0
			if len(history) > 10 {
				start = len(history) - 10
			}

			for _, msg := range history[start:] {
				msgBytes, _ := json.Marshal(msg)
				c.SendDirectMessage(msgBytes)
			}
		}

		c.sendSuccess("Joined room: " + msg.Room)

	case "leave":
		if c.Room != nil {
			oldRoom := c.Room
			c.Room = nil
			oldRoom.RemoveClient(c)

			// Broadcast to everyone in the room that this user has left
			oldRoom.BroadcastEvent(shared.EventUserLeft, c.Username, "")

			c.sendSuccess("Left room: " + oldRoom.Name)
		} else {
			c.sendError("Not in any room")
		}

	case "msg":
		if len(msg.Content) < 2 {
			c.sendError("Usage: msg <username> <message>")
			return
		}

		parts := strings.SplitN(msg.Content, " ", 3)
		if len(parts) < 3 {
			c.sendError("Usage: msg <username> <message>")
			return
		}

		recipient := parts[1]
		content := parts[2]

		// Find the recipient
		target := c.Server.FindClientByUsername(recipient)
		if target == nil {
			c.sendError("User not found: " + recipient)
			return
		}

		directMsg := shared.Message{
			Type:      shared.MessageTypeDirect,
			Content:   content,
			Sender:    c.Username,
			Recipient: recipient,
			Timestamp: time.Now(),
		}

		// Store in message history
		c.Server.MessageStore.AddDirectMessage(c.Username, recipient, directMsg)

		// Send the message
		msgBytes, _ := json.Marshal(directMsg)
		target.SendDirectMessage(msgBytes)
		c.SendDirectMessage(msgBytes) // Also send to sender

		log.Printf("Direct message from %s to %s", c.Username, recipient)

	case "encrypt":
		if len(msg.Content) < 2 {
			c.sendError("Usage: encrypt <username> <message>")
			return
		}

		parts := strings.SplitN(msg.Content, " ", 3)
		if len(parts) < 3 {
			c.sendError("Usage: encrypt <username> <message>")
			return
		}

		recipient := parts[1]
		content := parts[2]

		// Generate a simple key for demo purposes (in production, use proper key exchange)
		key := []byte("0123456789abcdef") // 16-byte key for AES-128

		// Encrypt the message
		encryptedContent, err := shared.Encrypt(content, key)
		if err != nil {
			c.sendError("Encryption failed: " + err.Error())
			return
		}

		// Find the recipient
		target := c.Server.FindClientByUsername(recipient)
		if target == nil {
			c.sendError("User not found: " + recipient)
			return
		}

		encryptedMsg := shared.Message{
			Type:      shared.MessageTypeEncrypted,
			Content:   encryptedContent,
			Sender:    c.Username,
			Recipient: recipient,
			Timestamp: time.Now(),
			Encrypted: true,
		}

		// Store metadata in history (not the content)
		historyMsg := encryptedMsg
		historyMsg.Content = "[Encrypted message]"
		c.Server.MessageStore.AddDirectMessage(c.Username, recipient, historyMsg)

		// Send the encrypted message
		msgBytes, _ := json.Marshal(encryptedMsg)
		target.SendDirectMessage(msgBytes)
		c.SendDirectMessage(msgBytes) // Also send to sender

		log.Printf("Encrypted message from %s to %s", c.Username, recipient)

	case "status":
		if len(msg.Content) < 2 {
			c.sendError("Usage: status <online|away|busy|offline>")
			return
		}

		parts := strings.Fields(msg.Content)
		statusStr := strings.ToLower(parts[1])

		var newStatus shared.UserStatus
		switch statusStr {
		case "online":
			newStatus = shared.StatusOnline
		case "away":
			newStatus = shared.StatusAway
		case "busy":
			newStatus = shared.StatusBusy
		case "offline":
			newStatus = shared.StatusOffline
		default:
			c.sendError("Invalid status. Use: online, away, busy, or offline")
			return
		}

		c.Status = newStatus

		// Notify current room if any
		if c.Room != nil {
			c.Room.BroadcastEvent(shared.EventStatusChange, c.Username, statusStr)
		}

		c.sendSuccess("Status updated to: " + statusStr)

	case "history":
		parts := strings.Fields(msg.Content)

		if len(parts) > 1 {
			// Direct message history
			otherUser := parts[1]

			history := c.Server.MessageStore.GetDirectMessageHistory(c.Username, otherUser)
			if len(history) == 0 {
				c.sendSuccess("No message history with user: " + otherUser)
				return
			}

			historyMsg := shared.Message{
				Type:      shared.MessageTypeCommand,
				Content:   "Message history with " + otherUser + ":",
				Sender:    "Server",
				Timestamp: time.Now(),
			}
			historyBytes, _ := json.Marshal(historyMsg)
			c.SendDirectMessage(historyBytes)

			// Send all direct messages
			for _, msg := range history {
				msgBytes, _ := json.Marshal(msg)
				c.SendDirectMessage(msgBytes)
			}
		} else {
			// Room history
			if c.Room == nil {
				c.sendError("You are not in a room")
				return
			}

			history := c.Server.MessageStore.GetRoomHistory(c.Room.Name)
			if len(history) == 0 {
				c.sendSuccess("No message history for room: " + c.Room.Name)
				return
			}

			historyMsg := shared.Message{
				Type:      shared.MessageTypeCommand,
				Content:   "Message history for room " + c.Room.Name + ":",
				Sender:    "Server",
				Timestamp: time.Now(),
			}
			historyBytes, _ := json.Marshal(historyMsg)
			c.SendDirectMessage(historyBytes)

			// Send the last 20 messages or all if less than 20
			start := 0
			if len(history) > 20 {
				start = len(history) - 20
			}

			for _, msg := range history[start:] {
				msgBytes, _ := json.Marshal(msg)
				c.SendDirectMessage(msgBytes)
			}
		}

	default:
		c.sendError("Unknown command: " + cmd)
	}
}

func (c *Client) sendError(message string) {
	response := shared.Message{
		Type:      shared.MessageTypeCommand,
		Content:   "ERROR: " + message,
		Sender:    "Server",
		Timestamp: time.Now(),
	}

	respBytes, _ := json.Marshal(response)
	c.SendDirectMessage(respBytes)
}

func (c *Client) sendSuccess(message string) {
	response := shared.Message{
		Type:      shared.MessageTypeCommand,
		Content:   "SUCCESS: " + message,
		Sender:    "Server",
		Timestamp: time.Now(),
	}

	respBytes, _ := json.Marshal(response)
	c.SendDirectMessage(respBytes)
}
