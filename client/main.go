package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"chatap.com/shared"
)

const (
	appDataDir = "appData"
)

// Client represents the chat client
type Client struct {
	conn              net.Conn
	serverAddr        string
	username          string
	currentRoom       string
	isAuthenticated   bool
	shouldExit        bool
	mutex             sync.Mutex
	pendingFileChunks map[string][]shared.FileMessage
}

func NewClient(serverAddr string) *Client {
	return &Client{
		serverAddr:        serverAddr,
		isAuthenticated:   false,
		pendingFileChunks: make(map[string][]shared.FileMessage),
	}
}

// Connect establishes a connection to the chat server
func (c *Client) Connect() error {
	conn, err := net.Dial("tcp", c.serverAddr)
	if err != nil {
		return fmt.Errorf("error connecting to server: %v", err)
	}
	c.conn = conn
	return nil
}

// Close closes the connection to the server
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// SetAuthenticated sets the authentication status
func (c *Client) SetAuthenticated(username string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.isAuthenticated = true
	c.username = username
}

// IsAuthenticated returns the authentication status
func (c *Client) IsAuthenticated() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.isAuthenticated
}

// SetCurrentRoom sets the current room
func (c *Client) SetCurrentRoom(room string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.currentRoom = room
}

// GetCurrentRoom gets the current room
func (c *Client) GetCurrentRoom() string {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.currentRoom
}

// SendMessage sends a message to the server
func (c *Client) SendMessage(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = c.conn.Write(append(data, '\n'))
	return err
}

// processMessage handles incoming server messages
func (c *Client) processMessage(message []byte) {
	var msg shared.Message
	if err := json.Unmarshal(message, &msg); err != nil {
		fmt.Printf("Error parsing message: %v\n", err)
		return
	}

	switch msg.Type {
	case shared.MessageTypeText:
		// Display regular chat message
		if msg.Room != "" {
			fmt.Printf("[%s] [%s] %s: %s\n",
				msg.Timestamp.Format("15:04:05"),
				msg.Room,
				msg.Sender,
				msg.Content)
		} else {
			fmt.Printf("[%s] %s: %s\n",
				msg.Timestamp.Format("15:04:05"),
				msg.Sender,
				msg.Content)
		}

	case shared.MessageTypeCommand:
		// Handle command responses
		fmt.Printf("%s\n", msg.Content)

		// Check for specific command responses
		if strings.HasPrefix(msg.Content, "SUCCESS: Logged in") ||
			strings.HasPrefix(msg.Content, "SUCCESS: Registered") {
			c.SetAuthenticated(c.username)
		} else if strings.HasPrefix(msg.Content, "SUCCESS: Room created and joined:") ||
			strings.HasPrefix(msg.Content, "SUCCESS: Joined room:") {
			parts := strings.Split(msg.Content, ":")
			if len(parts) > 1 {
				roomName := strings.TrimSpace(parts[1])
				c.SetCurrentRoom(roomName)
			}
		} else if strings.HasPrefix(msg.Content, "SUCCESS: Left room") {
			c.SetCurrentRoom("")
		} else if strings.HasPrefix(msg.Content, "SUCCESS: Goodbye!") {
			c.shouldExit = true
		}

	case shared.MessageTypeDirect:
		// Handle direct messages
		fmt.Printf("[%s] [DM from %s]: %s\n",
			msg.Timestamp.Format("15:04:05"),
			msg.Sender,
			msg.Content)

	case shared.MessageTypeEncrypted:
		// Handle encrypted messages
		if msg.Encrypted {
			// Simple demo key - in production, use secure key exchange
			key := []byte("0123456789abcdef")

			decrypted, err := shared.Decrypt(msg.Content, key)
			if err != nil {
				fmt.Printf("[%s] [Encrypted from %s]: Error decrypting: %v\n",
					msg.Timestamp.Format("15:04:05"),
					msg.Sender,
					err)
				return
			}

			fmt.Printf("[%s] [Encrypted from %s]: %s\n",
				msg.Timestamp.Format("15:04:05"),
				msg.Sender,
				decrypted)
		}

	case shared.MessageTypeFile:
		// Handle file messages
		var fileMsg shared.FileMessage
		if err := json.Unmarshal(message, &fileMsg); err != nil {
			fmt.Printf("Error parsing file message: %v\n", err)
			return
		}

		c.handleFileChunk(fileMsg)
	}
}

// handleFileChunk processes incoming file chunks
func (c *Client) handleFileChunk(fileMsg shared.FileMessage) {
	fileKey := fileMsg.Sender + "_" + fileMsg.Filename

	// First chunk notification
	if fileMsg.ChunkID == 0 {
		fmt.Printf("[%s] %s is sending file: %s\n",
			fileMsg.Timestamp.Format("15:04:05"),
			fileMsg.Sender,
			fileMsg.Filename)
	}

	// Store the chunk
	c.mutex.Lock()
	if _, exists := c.pendingFileChunks[fileKey]; !exists {
		c.pendingFileChunks[fileKey] = make([]shared.FileMessage, 0, fileMsg.TotalChunks)
	}
	c.pendingFileChunks[fileKey] = append(c.pendingFileChunks[fileKey], fileMsg)
	chunksReceived := len(c.pendingFileChunks[fileKey])
	c.mutex.Unlock()

	// Check if we have all chunks
	if chunksReceived == fileMsg.TotalChunks {
		fmt.Printf("All chunks received for %s. Assembling file...\n", fileMsg.Filename)
		go c.saveFile(fileKey, fileMsg.Filename)
	}
}

// saveFile assembles and saves a complete file from chunks
func (c *Client) saveFile(fileKey, filename string) {
	// Ensure app data directory exists
	if err := os.MkdirAll(appDataDir, 0755); err != nil {
		fmt.Printf("Error creating appData directory: %v\n", err)
		return
	}

	// Get file chunks
	c.mutex.Lock()
	chunks := c.pendingFileChunks[fileKey]
	delete(c.pendingFileChunks, fileKey)
	c.mutex.Unlock()

	// Save file
	if err := shared.SaveFileFromChunks(chunks, appDataDir); err != nil {
		fmt.Printf("Error saving file %s: %v\n", filename, err)
		return
	}

	fmt.Printf("File %s saved successfully to %s directory.\n", filename, appDataDir)
}

// sendFile sends a file to the current room
func (c *Client) sendFile(filePath string) error {
	if !c.IsAuthenticated() {
		return fmt.Errorf("you must be logged in to send files")
	}

	currentRoom := c.GetCurrentRoom()
	if currentRoom == "" {
		return fmt.Errorf("you must join a room before sending files")
	}

	chunks, err := shared.EncodeFileToChunks(filePath)
	if err != nil {
		return fmt.Errorf("error encoding file: %v", err)
	}

	// Set message metadata for each chunk
	for i := range chunks {
		chunks[i].Sender = c.username
		chunks[i].Room = currentRoom
		chunks[i].Timestamp = time.Now()
	}

	// Send each chunk
	for i, chunk := range chunks {
		if err := c.SendMessage(chunk); err != nil {
			return fmt.Errorf("error sending chunk %d: %v", i, err)
		}

		// Small delay to prevent flooding
		time.Sleep(10 * time.Millisecond)
	}

	return nil
}

// parseCommand processes user commands
func (c *Client) parseCommand(input string) error {
	input = strings.TrimSpace(input)
	if len(input) == 0 {
		return nil
	}

	// Handle commands
	if input[0] == '/' {
		cmd := strings.TrimPrefix(input, "/")
		return c.executeCommand(cmd)
	}

	// Regular chat message
	if !c.IsAuthenticated() {
		return fmt.Errorf("you must be logged in to send messages")
	}

	currentRoom := c.GetCurrentRoom()
	if currentRoom == "" {
		return fmt.Errorf("you must join a room before sending messages")
	}

	// Create and send text message
	msg := shared.Message{
		Type:      shared.MessageTypeText,
		Content:   input,
		Room:      currentRoom,
		Timestamp: time.Now(),
	}

	return c.SendMessage(msg)
}

// executeCommand processes specific commands
func (c *Client) executeCommand(cmd string) error {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	command := parts[0]

	switch command {
	case "login":
		if len(parts) < 3 {
			return fmt.Errorf("usage: /login <username> <password>")
		}
		username, password := parts[1], parts[2]
		c.username = username // Store tentatively, confirmed when server responds

		authMsg := shared.AuthMessage{
			Message: shared.Message{
				Type:      shared.MessageTypeAuth,
				Content:   "login",
				Timestamp: time.Now(),
			},
			Username: username,
			Password: password,
		}

		return c.SendMessage(authMsg)

	case "register":
		if len(parts) < 3 {
			return fmt.Errorf("usage: /register <username> <password>")
		}
		username, password := parts[1], parts[2]
		c.username = username // Store tentatively, confirmed when server responds

		authMsg := shared.AuthMessage{
			Message: shared.Message{
				Type:      shared.MessageTypeAuth,
				Content:   "register",
				Timestamp: time.Now(),
			},
			Username: username,
			Password: password,
		}

		return c.SendMessage(authMsg)

	case "create":
		if !c.IsAuthenticated() {
			return fmt.Errorf("you must be logged in to create rooms")
		}

		if len(parts) < 2 {
			return fmt.Errorf("usage: /create <room-name>")
		}

		roomName := parts[1]
		msg := shared.Message{
			Type:      shared.MessageTypeCommand,
			Content:   "create",
			Room:      roomName,
			Timestamp: time.Now(),
		}

		return c.SendMessage(msg)

	case "join":
		if !c.IsAuthenticated() {
			return fmt.Errorf("you must be logged in to join rooms")
		}

		if len(parts) < 2 {
			return fmt.Errorf("usage: /join <room-name>")
		}

		roomName := parts[1]
		msg := shared.Message{
			Type:      shared.MessageTypeCommand,
			Content:   "join",
			Room:      roomName,
			Timestamp: time.Now(),
		}

		return c.SendMessage(msg)

	case "leave":
		if !c.IsAuthenticated() {
			return fmt.Errorf("you must be logged in to leave rooms")
		}

		msg := shared.Message{
			Type:      shared.MessageTypeCommand,
			Content:   "leave",
			Timestamp: time.Now(),
		}

		return c.SendMessage(msg)

	case "rooms":
		if !c.IsAuthenticated() {
			return fmt.Errorf("you must be logged in to list rooms")
		}

		msg := shared.Message{
			Type:      shared.MessageTypeCommand,
			Content:   "rooms",
			Timestamp: time.Now(),
		}

		return c.SendMessage(msg)

	case "list":
		if !c.IsAuthenticated() {
			return fmt.Errorf("you must be logged in to list users")
		}

		msg := shared.Message{
			Type:      shared.MessageTypeCommand,
			Content:   "list",
			Timestamp: time.Now(),
		}

		return c.SendMessage(msg)

	case "msg":
		if !c.IsAuthenticated() {
			return fmt.Errorf("you must be logged in to send direct messages")
		}

		if len(parts) < 3 {
			return fmt.Errorf("usage: /msg <username> <message>")
		}

		recipient := parts[1]
		content := strings.Join(parts[2:], " ")

		msg := shared.Message{
			Type:      shared.MessageTypeDirect,
			Content:   content,
			Recipient: recipient,
			Timestamp: time.Now(),
		}

		return c.SendMessage(msg)

	case "encrypt":
		if !c.IsAuthenticated() {
			return fmt.Errorf("you must be logged in to send encrypted messages")
		}

		if len(parts) < 3 {
			return fmt.Errorf("usage: /encrypt <username> <message>")
		}

		recipient := parts[1]
		content := strings.Join(parts[2:], " ")

		// Simple demo key - in production, use secure key exchange
		key := []byte("0123456789abcdef")

		encrypted, err := shared.Encrypt(content, key)
		if err != nil {
			return fmt.Errorf("encryption failed: %v", err)
		}

		msg := shared.Message{
			Type:      shared.MessageTypeEncrypted,
			Content:   encrypted,
			Recipient: recipient,
			Timestamp: time.Now(),
			Encrypted: true,
		}

		return c.SendMessage(msg)

	case "file":
		if !c.IsAuthenticated() {
			return fmt.Errorf("you must be logged in to send files")
		}

		if len(parts) < 2 {
			return fmt.Errorf("usage: /file <filepath>")
		}

		filePath := parts[1]
		return c.sendFile(filePath)

	case "status":
		if !c.IsAuthenticated() {
			return fmt.Errorf("you must be logged in to change status")
		}

		if len(parts) < 2 {
			return fmt.Errorf("usage: /status <online|away|busy|offline>")
		}

		statusStr := parts[1]
		var statusValue shared.UserStatus

		switch strings.ToLower(statusStr) {
		case "online":
			statusValue = shared.StatusOnline
		case "away":
			statusValue = shared.StatusAway
		case "busy":
			statusValue = shared.StatusBusy
		case "offline":
			statusValue = shared.StatusOffline
		default:
			return fmt.Errorf("invalid status. Use: online, away, busy, or offline")
		}

		statusMsg := shared.StatusMessage{
			Message: shared.Message{
				Type:      shared.MessageTypeStatus,
				Content:   statusStr,
				Timestamp: time.Now(),
			},
			Status: statusValue,
		}

		return c.SendMessage(statusMsg)

	case "history":
		if !c.IsAuthenticated() {
			return fmt.Errorf("you must be logged in to view history")
		}

		var msgContent string
		if len(parts) > 1 {
			// Direct message history with specific user
			msgContent = "history " + parts[1]
		} else {
			// Room history
			msgContent = "history"
		}

		msg := shared.Message{
			Type:      shared.MessageTypeCommand,
			Content:   msgContent,
			Timestamp: time.Now(),
		}

		return c.SendMessage(msg)

	case "exit":
		msg := shared.Message{
			Type:      shared.MessageTypeCommand,
			Content:   "exit",
			Timestamp: time.Now(),
		}

		return c.SendMessage(msg)

	case "help":
		printHelp()
		return nil

	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// printHelp displays available commands
func printHelp() {
	fmt.Println("\n=== TCP Chat Client Help ===")
	fmt.Println("Authentication:")
	fmt.Println("  /register <username> <password> - Register a new account")
	fmt.Println("  /login <username> <password>    - Log in with existing account")

	fmt.Println("\nRoom Management:")
	fmt.Println("  /rooms                          - List available rooms")
	fmt.Println("  /create <room-name>             - Create and join a new room")
	fmt.Println("  /join <room-name>               - Join an existing room")
	fmt.Println("  /leave                          - Leave current room")
	fmt.Println("  /list                           - List users in current room")

	fmt.Println("\nMessaging:")
	fmt.Println("  <message>                       - Send message to current room")
	fmt.Println("  /msg <username> <message>       - Send direct message to user")
	fmt.Println("  /encrypt <username> <message>   - Send encrypted message to user")

	fmt.Println("\nFile Sharing:")
	fmt.Println("  /file <filepath>                - Send file to current room")

	fmt.Println("\nOther Commands:")
	fmt.Println("  /status <online|away|busy|offline> - Change your status")
	fmt.Println("  /history                        - View room message history")
	fmt.Println("  /history <username>             - View direct message history with user")
	fmt.Println("  /help                           - Show this help message")
	fmt.Println("  /exit                           - Exit the chat client")
	fmt.Println("===============================\n")
}

func main() {
	// Define command-line flags
	serverAddr := flag.String("server", "localhost:8080", "Chat server address")
	flag.Parse()

	// Create app data directory
	if err := os.MkdirAll(appDataDir, 0755); err != nil {
		log.Fatalf("Error creating data directory: %v", err)
	}

	// Initialize client
	client := NewClient(*serverAddr)

	// Display welcome message
	fmt.Println("TCP Chat Client")
	fmt.Println("Type /help for available commands")
	fmt.Printf("Connecting to %s...\n", *serverAddr)

	// Connect to server
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	fmt.Println("Connected to server!")

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start reader goroutine
	go func() {
		reader := bufio.NewReader(client.conn)
		for {
			message, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					fmt.Println("\nDisconnected from server")
				} else {
					fmt.Printf("\nError reading from server: %v\n", err)
				}
				sigCh <- syscall.SIGTERM
				return
			}
			client.processMessage(message)

			// Check if we should exit
			if client.shouldExit {
				sigCh <- syscall.SIGTERM
				return
			}
		}
	}()

	// Input handler
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			input := scanner.Text()
			if err := client.parseCommand(input); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		}

		if err := scanner.Err(); err != nil {
			fmt.Printf("Error reading input: %v\n", err)
		}

		sigCh <- syscall.SIGTERM
	}()

	// Wait for termination signal
	<-sigCh
	fmt.Println("\nShutting down client...")
}
