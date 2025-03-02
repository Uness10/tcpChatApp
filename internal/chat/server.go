package chat

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strings"

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
		case CMD_NICK:
			s.nick(cmd.client, cmd.args)
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
	log.Printf("new client has connected : %s", conn.RemoteAddr().String())
	c := &client{
		conn:     conn,
		nick:     "anonymous",
		commands: s.commands,
	}

	s.showMenu(c)
	c.readInput()
}

func (s *server) nick(c *client, args []string) {
	if len(args) < 2 {
		c.err(errors.New("missing field for nickname"))
		return
	}
	c.nick = args[1]
	_, err := s.repo.FindUserByNickname(c.nick)
	if err != nil {
		err = s.repo.CreateUser(&models.User{Nickname: c.nick})
		if err != nil {
			c.err(fmt.Errorf("failed to create user: %v", err))
			return
		}
	}
	c.msg(fmt.Sprintf("all right, I will call you %s", c.nick))
}
func (s *server) join(c *client, args []string) {
	if len(args) < 2 {
		c.err(errors.New("missing field for room name"))
		return
	}
	roomName := args[1]
	_, err := s.repo.FindRoomByName(roomName)

	if err != nil {
		r := &models.Room{
			Name: roomName,
		}
		s.repo.CreateRoom(r)

	}
	if c.room != nil {
		s.quitCurrentRoom(c)
	}
	// c.room = r
	// r.broadcast(c, fmt.Sprintf("%s has joined the room", c.nick))
	// c.msg(fmt.Sprintf("welcome to %s", r.name))
}
func (s *server) listRooms(c *client, args []string) {
	rooms, err := s.repo.FindAllRooms()
	if err != nil {
		log.Printf("error querying rooms: %s", err)
		c.msg("⚠️ Error retrieving rooms. Please try again later.")
		return
	}
	if len(rooms) == 0 {
		c.msg("No available rooms at the moment. Create one with /join <room_name>")
		return
	}
	c.msg(fmt.Sprintf("Available rooms: %s", strings.Join(rooms, ", ")))
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
	c.room.broadcast(c, c.nick+": "+strings.Join(args[1:len(args)], " "))
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
