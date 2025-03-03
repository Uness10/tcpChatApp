package chat

import (
	"fmt"
	"net"
	"os"
)

type room struct {
	name    string
	members map[net.Addr]*client
}

func (r *room) broadcast(sender *client, msg string) {
	for addr, m := range r.members {
		if addr != sender.conn.RemoteAddr() {
			m.msg(msg)
		}
	}
}

func (r *room) broadcastFile(sender *client, filename string) {
	file, err := os.Open("uploads/" + filename)
	if err != nil {
		sender.err(fmt.Errorf("failed to open file for broadcast: %v", err))
		return
	}
	defer file.Close()

	buffer := make([]byte, 1024)
	for _, member := range r.members {
		if member != sender {
			member.msg(fmt.Sprintf("%s sent a file: %s", sender.nick, filename))

			// Send file data
			for {
				n, err := file.Read(buffer)
				if n == 0 || err != nil {
					break
				}
				member.conn.Write(buffer[:n])
			}
		}
	}
}
