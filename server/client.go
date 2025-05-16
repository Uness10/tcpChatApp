package main

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
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
}

func NewClient(conn net.Conn, server *Server) *Client {
	return &Client{
		Conn:       conn,
		Send:       make(chan []byte, 256),
		Server:     server,
		isLoggedIn: false,
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

	default:
		c.sendError("Unknown command: " + cmd)
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
