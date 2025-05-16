package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"chatap.com/shared"
)

type Client struct {
	conn               net.Conn
	reader             *bufio.Reader
	username           string
	isLoggedIn         bool
	currentRoom        string
	receivedFileChunks map[string][]shared.FileMessage
}

func NewClient(serverAddr string) (*Client, error) {
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:               conn,
		reader:             bufio.NewReader(conn),
		receivedFileChunks: make(map[string][]shared.FileMessage),
	}, nil
}

func (c *Client) Start() {
	go c.readMessages()
	c.handleInput()
}

func (c *Client) readMessages() {
	for {
		message, err := c.reader.ReadBytes('\n')
		if err != nil {
			log.Printf("Connection closed: %v", err)
			os.Exit(1)
		}

		var msg shared.Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error parsing message: %v", err)
			continue
		}

		c.handleMessage(msg, message)
	}
}

func (c *Client) handleMessage(msg shared.Message, rawMsg []byte) {
	switch msg.Type {
	case shared.MessageTypeText:
		// Format room messages with timestamp, sender, and content
		if msg.Sender == "Server" {
			// Server notifications (like user joined/left)
			fmt.Println(shared.FormatEventMessage(msg.Timestamp, msg.Content))
		} else {
			// Regular user messages
			fmt.Printf("[%s] [%s] %s: %s\n",
				msg.Timestamp.Format("15:04:05"),
				msg.Room,
				msg.Sender,
				msg.Content)
		}

	case shared.MessageTypeCommand:
		fmt.Println(msg.Content)

	case shared.MessageTypeFile:
		var fileMsg shared.FileMessage
		if err := json.Unmarshal(rawMsg, &fileMsg); err != nil {
			log.Printf("Error parsing file message: %v", err)
			return
		}

		// Only print status message if it's from someone else (not echo of our own message)
		if fileMsg.Sender != c.username {
			fmt.Printf("[%s] %s is sending file: %s (chunk %d/%d)\n",
				fileMsg.Timestamp.Format("15:04:05"),
				fileMsg.Sender,
				fileMsg.Filename,
				fileMsg.ChunkID+1,
				fileMsg.TotalChunks)
		}

		// Store the chunk if it's from someone else
		if fileMsg.Sender != c.username {
			if _, ok := c.receivedFileChunks[fileMsg.Filename]; !ok {
				c.receivedFileChunks[fileMsg.Filename] = make([]shared.FileMessage, 0, fileMsg.TotalChunks)
			}
			c.receivedFileChunks[fileMsg.Filename] = append(c.receivedFileChunks[fileMsg.Filename], fileMsg)

			// Check if all chunks are received
			if len(c.receivedFileChunks[fileMsg.Filename]) == fileMsg.TotalChunks {
				fmt.Printf("All chunks received for %s. Assembling file...\n", fileMsg.Filename)

				downloadsDir := "appData"
				if err := os.MkdirAll(downloadsDir, 0755); err != nil {
					fmt.Printf("Error creating downloads directory: %v\n", err)
				} else {
					err := shared.SaveFileFromChunks(c.receivedFileChunks[fileMsg.Filename], downloadsDir)
					if err != nil {
						fmt.Printf("Error saving file %s: %v\n", fileMsg.Filename, err)
					} else {
						fmt.Printf("File %s saved successfully to %s directory.\n", fileMsg.Filename, downloadsDir)
					}
					// Clean up stored chunks for this file
					delete(c.receivedFileChunks, fileMsg.Filename)
				}
			}
		}
	}
}

func (c *Client) handleInput() {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Welcome to the TCP Chat Client!")
	fmt.Println("Commands:")
	fmt.Println("  /register <username> <password>")
	fmt.Println("  /login <username> <password>")
	fmt.Println("  /rooms")
	fmt.Println("  /create <room-name>")
	fmt.Println("  /join <room-name>")
	fmt.Println("  /leave")
	fmt.Println("  /file <filepath>")
	fmt.Println("  /exit")

	for scanner.Scan() {
		input := scanner.Text()

		if strings.HasPrefix(input, "/") {
			c.handleCommand(input)
		} else if c.isLoggedIn {
			if c.currentRoom == "" {
				fmt.Println("You need to join a room first. Use /rooms and /join <name>")
				continue
			}

			// Send regular text message to current room
			msg := shared.Message{
				Type:      shared.MessageTypeText,
				Content:   input,
				Room:      c.currentRoom,
				Timestamp: time.Now(),
			}

			msgBytes, _ := json.Marshal(msg)
			if _, err := c.conn.Write(msgBytes); err != nil {
				fmt.Println("Failed to send message:", err)
				continue
			}
			if _, err := c.conn.Write([]byte("\n")); err != nil {
				fmt.Println("Failed to send message:", err)
				continue
			}

			// Message sent to server (no local echo needed as server will broadcast back)
		} else {
			fmt.Println("You need to log in first. Use /login <username> <password>")
		}
	}
}

func (c *Client) handleCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	cmd := parts[0]

	switch cmd {
	case "/register":
		if len(parts) < 3 {
			fmt.Println("Usage: /register <username> <password>")
			return
		}

		username := parts[1]
		password := parts[2]

		authMsg := shared.AuthMessage{
			Message: shared.Message{
				Type:    shared.MessageTypeAuth,
				Content: "register",
			},
			Username: username,
			Password: password,
		}

		msgBytes, _ := json.Marshal(authMsg)
		c.conn.Write(msgBytes)
		c.conn.Write([]byte("\n"))

		c.username = username
		c.isLoggedIn = true

	case "/login":
		if len(parts) < 3 {
			fmt.Println("Usage: /login <username> <password>")
			return
		}

		username := parts[1]
		password := parts[2]

		authMsg := shared.AuthMessage{
			Message: shared.Message{
				Type:    shared.MessageTypeAuth,
				Content: "login",
			},
			Username: username,
			Password: password,
		}

		msgBytes, _ := json.Marshal(authMsg)
		c.conn.Write(msgBytes)
		c.conn.Write([]byte("\n"))

		c.username = username
		c.isLoggedIn = true

	case "/rooms", "/create", "/join", "/leave":
		if !c.isLoggedIn {
			fmt.Println("You need to log in first")
			return
		}

		content := strings.TrimPrefix(cmd, "/")
		room := ""

		if cmd == "/create" || cmd == "/join" {
			if len(parts) < 2 {
				fmt.Printf("Usage: %s <room-name>\n", cmd)
				return
			}
			room = parts[1]
			if cmd == "/join" {
				c.currentRoom = room
			}
		}

		msg := shared.Message{
			Type:    shared.MessageTypeCommand,
			Content: content,
			Room:    room,
		}

		msgBytes, _ := json.Marshal(msg)
		c.conn.Write(msgBytes)
		c.conn.Write([]byte("\n"))

	case "/file":
		if !c.isLoggedIn {
			fmt.Println("You need to log in first")
			return
		}

		if c.currentRoom == "" {
			fmt.Println("You need to join a room first")
			return
		}

		if len(parts) < 2 {
			fmt.Println("Usage: /send-file <filepath>")
			return
		}

		filepath := parts[1]

		// Verify file exists and can be read
		fileInfo, err := os.Stat(filepath)
		if os.IsNotExist(err) {
			fmt.Printf("Error: File does not exist: %s\n", filepath)
			return
		}
		if err != nil {
			fmt.Printf("Error accessing file: %v\n", err)
			return
		}

		// Check if file is empty
		if fileInfo.Size() == 0 {
			fmt.Printf("Error: File is empty: %s\n", filepath)
			return
		}

		chunks, err := shared.EncodeFileToChunks(filepath)
		if err != nil {
			fmt.Printf("Error preparing file: %v\n", err)
			return
		}

		if len(chunks) == 0 {
			fmt.Printf("Error: No chunks generated for file: %s\n", filepath)
			return
		}

		for i, chunk := range chunks {
			chunk.Sender = c.username
			chunk.Room = c.currentRoom

			msgBytes, _ := json.Marshal(chunk)
			_, err := c.conn.Write(msgBytes)
			if err != nil {
				fmt.Printf("Error sending chunk %d: %v\n", i+1, err)
				return
			}
			_, err = c.conn.Write([]byte("\n"))
			if err != nil {
				fmt.Printf("Error sending chunk %d: %v\n", i+1, err)
				return
			}

			time.Sleep(100 * time.Millisecond) // Throttle to avoid flooding
		}

		// Remove "File sent successfully" message as it's misleading
		// The server will respond with success or error

	case "/exit":
		fmt.Println("Exiting...")
		c.conn.Close()
		os.Exit(0)

	default:
		fmt.Println("Unknown command:", cmd)
	}
}

func main() {
	// Replace "localhost:8080" with your server's IP and port
	// For example: "192.168.1.5:8080"
	client, err := NewClient("localhost:8080")
	if err != nil {
		log.Fatal(err)
	}

	client.Start()
}
