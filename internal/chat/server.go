package chat

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"

	"tcpServer.com/internal/auth"
	"tcpServer.com/internal/db"
	"tcpServer.com/internal/models"
)

type server struct {
	rooms    map[string]*room
	commands chan command
	repo     *db.Repository
}

func NewServer(repo *db.Repository) *server {
	return &server{
		rooms:    make(map[string]*room),
		commands: make(chan command),
		repo:     repo,
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
		case CMD_QUIT:
			s.quit(cmd.client, cmd.args)

		}
	}
}
func (s *server) NewClient(conn net.Conn) {
	log.Printf("new client attempting to connect : %s", conn.RemoteAddr().String())

	c := &client{
		conn:     conn,
		commands: s.commands,
	}

	s.handleAuth(c)

	s.showMenu(c)
	c.readInput()
}

func (s *server) handleAuth(c *client) {
	c.info("Welcome! You can either log in to your account or create a new one.")
	c.info("- Type 1: Log in")
	c.info("- Type 2: Create an account")

	num, _ := bufio.NewReader(c.conn).ReadString('\n')
	num = strings.TrimSpace(num)

	switch num {
	case "1":
		c.msg("Enter your nickname:")
		nickname, _ := bufio.NewReader(c.conn).ReadString('\n')
		nickname = strings.TrimSpace(nickname)

		c.msg("Enter your password:")
		password, _ := bufio.NewReader(c.conn).ReadString('\n')
		password = strings.TrimSpace(password)

		user, err := s.repo.FindUserByNickname(nickname)
		if err != nil {
			c.err(errors.New("user not found"))
			log.Printf(err.Error())
			c.conn.Close()
			return
		}

		if !auth.CheckPassword(password, user.Password) {
			c.err(errors.New("invalid password"))
			c.conn.Close()
			return
		}

		// token, err := auth.GenerateJWT(nickname)
		// if err != nil {

		// }
		// c.nick = nickname
		// c.token = token
		// c.msg(fmt.Sprintf("token,%s", token))
		c.msg(fmt.Sprintf("Welcome back, %s", nickname))

	case "2":
		c.msg("Choose a nickname:")
		nickname, _ := bufio.NewReader(c.conn).ReadString('\n')
		nickname = strings.TrimSpace(nickname)

		_, err := s.repo.FindUserByNickname(nickname)
		if err == nil {
			c.err(errors.New("nickname already taken"))
			c.conn.Close()
			return
		}

		c.msg("Enter a password:")
		password, _ := bufio.NewReader(c.conn).ReadString('\n')
		password = strings.TrimSpace(password)
		fmt.Println(password)

		hashedPassword := auth.HashPassword(password)

		user := &models.User{
			Nickname: nickname,
			Password: hashedPassword,
		}

		err = s.repo.CreateUser(user)
		if err != nil {
			c.err(fmt.Errorf("failed to create user: %v", err))
			c.conn.Close()
			return
		}
		// token, err := auth.GenerateJWT(nickname)
		// if err != nil {

		// }
		// c.nick = nickname
		// c.token = token
		// c.msg(fmt.Sprintf("token,%s", token))
		c.msg(fmt.Sprintf("Welcome, %s !", nickname))

	default:
		c.err(errors.New("invalid choice"))
		c.conn.Close()
		return
	}
}

func (s *server) join(c *client, args []string) {
	if len(args) < 2 {
		c.err(errors.New("missing field for room name"))
		return
	}
	roomName := args[1]

	// Ensure room exists in the database
	_, err := s.repo.FindRoomByName(roomName)
	if err != nil {
		// Create room in DB if not found
		if err := s.repo.CreateRoom(&models.Room{Name: roomName}); err != nil {
			c.err(fmt.Errorf("failed to create room: %v", err))
			return
		}
	}

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
	dbRooms, err := s.repo.FindAllRooms()
	if err != nil {
		c.msg("⚠️ Error retrieving rooms")
		return
	}

	activeRooms := make(map[string]bool)
	for name := range s.rooms {
		activeRooms[name] = true
	}

	var roomList []string
	for _, name := range dbRooms {
		if activeRooms[name] {
			roomList = append(roomList, fmt.Sprintf("%s (active)", name))
		} else {
			roomList = append(roomList, name)
		}
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
	delete(c.room.members, c.conn.RemoteAddr())
	c.room.broadcast(c, fmt.Sprintf("%s has left the room", c.nick))

}

func (s *server) showMenu(c *client) {
	c.info("Welcome to the tcp chat app !")
	c.info("Menu: ")
	c.info("	- /nick <nickname> : set a nickname ")
	c.info("	- /join <room> : join a room ")
	c.info("	- /rooms: list rooms ")
	c.info("	- /msg <message> : send msg in a room (you must join it first)")
	c.info("	- /quit: quit")
}
