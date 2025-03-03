package chat

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"tcpServer.com/internal/auth"
	"tcpServer.com/internal/db"
	"tcpServer.com/internal/models"
)

type server struct {
	rooms         map[string]*room
	commands      chan command
	repo          *db.Repository
	activeClients map[string]*client // Track active clients by nickname
}

func NewServer(repo *db.Repository) *server {
	return &server{
		rooms:         make(map[string]*room),
		commands:      make(chan command),
		repo:          repo,
		activeClients: make(map[string]*client), // Initialize active clients map
	}
}

func (s *server) Run() {
	for cmd := range s.commands {
		switch cmd.id {
		case CMD_JOIN:
			s.join(cmd.client, cmd.args)
		case CMD_ROOMS:
			s.listRooms(cmd.client, cmd.args)
		case CMD_MSG:
			s.msg(cmd.client, cmd.args)
		case CMD_FILE:
			s.handleFile(cmd.client, cmd.args)
		case CMD_QUIT:
			s.quit(cmd.client, cmd.args)

		}
	}
}

func (s *server) NewClient(conn net.Conn) {
	log.Printf("new client connected: %s", conn.RemoteAddr().String())

	c := &client{
		conn:     conn,
		commands: s.commands,
	}

	defer func() {
		log.Printf("client disconnected: %s", conn.RemoteAddr().String())
		s.quitCurrentRoom(c)
		conn.Close()
	}()
	c.info("Welcome! You can either log in to your account or create a new one.")

	for attempts := 0; attempts < 3; attempts++ {
		s.handleAuth(c)
		if c.nick != "" {
			break
		}
		c.err(fmt.Errorf("authentication failed. Attempts remaining: %d", 2-attempts))
	}

	if c.nick == "" {
		c.err(errors.New("too many failed attempts. Disconnecting..."))
		return
	}

	s.showMenu(c)
	c.readInput()
}

func (s *server) handleAuth(c *client) {
	c.info("- Type 1: Log in")
	c.info("- Type 2: Create an account")

	num, _ := bufio.NewReader(c.conn).ReadString('\n')
	num = strings.TrimSpace(num)

	switch num {
	case "1":
		s.login(c)
	case "2":
		s.createAccount(c)
	default:
		c.err(errors.New("invalid choice"))
		c.conn.Close()
	}
}

func (s *server) login(c *client) {
	c.msg("Enter your nickname:")
	nickname, _ := bufio.NewReader(c.conn).ReadString('\n')
	nickname = strings.TrimSpace(nickname)

	c.msg("Enter your password:")
	password, _ := bufio.NewReader(c.conn).ReadString('\n')
	password = strings.TrimSpace(password)

	user, err := s.repo.FindUserByNickname(nickname)
	if err != nil {
		c.err(errors.New("user not found"))
		return
	}

	if !auth.CheckPassword(password, user.Password) {
		c.err(errors.New("invalid password"))
		return
	}

	// Check if the user is already logged in
	if existingClient, exists := s.activeClients[nickname]; exists {
		// Disconnect the existing client
		existingClient.msg("You have been logged out from another device.")
		s.quitCurrentRoom(existingClient)
		existingClient.conn.Close()
		delete(s.activeClients, nickname)
	}

	// Add the new client to active sessions
	s.activeClients[nickname] = c
	c.nick = nickname
	c.msg(fmt.Sprintf("Welcome back, %s!", nickname))
}
func (s *server) createAccount(c *client) {
	c.msg("Choose a nickname:")
	nickname, _ := bufio.NewReader(c.conn).ReadString('\n')
	nickname = strings.TrimSpace(nickname)

	_, err := s.repo.FindUserByNickname(nickname)
	if err == nil {
		c.err(errors.New("nickname already taken"))
		return
	}

	c.msg("Enter a password:")
	password, _ := bufio.NewReader(c.conn).ReadString('\n')
	password = strings.TrimSpace(password)

	hashedPassword := auth.HashPassword(password)
	user := &models.User{
		Nickname: nickname,
		Password: hashedPassword,
	}

	if err := s.repo.CreateUser(user); err != nil {
		c.err(fmt.Errorf("failed to create user: %v", err))
		return
	}
	c.nick = nickname
	c.msg(fmt.Sprintf("Welcome, %s! Account created successfully.", nickname))
}

func (s *server) join(c *client, args []string) {
	if len(args) < 2 {
		c.err(errors.New("missing field for room name"))
		return
	}
	roomName := args[1]

	// Get or create in-memory room
	r, exists := s.rooms[roomName]
	if !exists {
		r = &room{
			name:    roomName,
			members: make(map[net.Addr]*client),
		}
		s.rooms[roomName] = r
	}

	// Remove client from previous room
	if c.room != nil {
		s.quitCurrentRoom(c)
	}

	// Add client to the new room
	r.members[c.conn.RemoteAddr()] = c
	c.room = r

	// Notify room
	r.broadcast(c, fmt.Sprintf("%s joined the room", c.nick))
	c.msg(fmt.Sprintf("Joined %s", roomName))
}

func (s *server) listRooms(c *client, args []string) {
	var roomList []string
	for name := range s.rooms {
		roomList = append(roomList, name)
	}

	if len(roomList) == 0 {
		c.msg("No rooms found. Use /join <name> to create one.")
	} else {
		c.msg("Rooms: " + strings.Join(roomList, ", "))
	}
}

func (s *server) msg(c *client, args []string) {
	if len(args) < 2 {
		c.err(errors.New("missing field for message"))
		return
	}
	if c.room == nil {
		c.err(errors.New("you must join the room first"))
		return
	}
	c.room.broadcast(c, c.nick+": "+strings.Join(args[1:], " "))
}

func (s *server) quit(c *client, args []string) {
	log.Printf("client has disconnected: %s", c.conn.RemoteAddr().String())
	s.quitCurrentRoom(c)
	c.msg("sad to see you go :(")
	c.conn.Close()
}

func (s *server) quitCurrentRoom(c *client) {
	if c.room != nil {
		delete(c.room.members, c.conn.RemoteAddr())
		c.room.broadcast(c, fmt.Sprintf("%s has left the room", c.nick))
		c.room = nil
	}
}

func (s *server) handleFile(c *client, args []string) {
	if len(args) < 2 {
		c.err(errors.New("missing field for file name"))
		return
	}
	if c.room == nil {
		c.err(errors.New("you must join the room first"))
		return
	}

	filename := args[1]
	filePath := fmt.Sprintf("./uploads/%s", filename)

	// Create the uploads directory if it doesn't exist
	if err := os.MkdirAll("./uploads", os.ModePerm); err != nil {
		c.err(fmt.Errorf("failed to create uploads directory: %v", err))
		return
	}

	// Create the file
	file, err := os.Create(filePath)
	if err != nil {
		c.err(fmt.Errorf("failed to create file: %v", err))
		return
	}
	defer file.Close()

	// Read the file data from the client
	_, err = io.Copy(file, c.conn)
	if err != nil {
		c.err(fmt.Errorf("failed to receive file: %v", err))
		return
	}

	// Notify the room about the file
	c.room.broadcast(c, fmt.Sprintf("%s sent a file: %s", c.nick, filename))

	// Send the file to all room members
	for _, member := range c.room.members {
		if member.conn.RemoteAddr() != c.conn.RemoteAddr() {
			member.msg(fmt.Sprintf("Receiving file: %s", filename))
			s.sendFileToClient(member, filePath)
		}
	}
}

func (s *server) sendFileToClient(c *client, filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		c.err(fmt.Errorf("failed to open file: %v", err))
		return
	}
	defer file.Close()

	// Send the file command to the client
	c.conn.Write([]byte("/file " + filePath + "\n"))

	// Send the file contents
	_, err = io.Copy(c.conn, file)
	if err != nil {
		c.err(fmt.Errorf("failed to send file: %v", err))
		return
	}
}

func (s *server) showMenu(c *client) {
	c.info("Welcome to the TCP chat app!")
	c.info("Menu: ")
	c.info("	- /join <room> : join a room")
	c.info("	- /rooms: list rooms")
	c.info("	- /msg <message> : send message in a room")
	c.info("	- /file <filename> : send a file")
	c.info("	- /quit: quit")
}
