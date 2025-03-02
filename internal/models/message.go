package models

import "time"

type Message struct {
	ID        int
	UserID    int
	RoomID    int
	Content   string
	CreatedAt time.Time
}
