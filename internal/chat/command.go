package chat

type commandID int

const (
	CMD_JOIN commandID = iota
	CMD_ROOMS
	CMD_MSG
	CMD_FILE
	CMD_QUIT
)

type command struct {
	id     commandID
	client *client
	args   []string
}
