package models

import "time"

type Enrollement struct {
	UserID   int
	RoomID   int
	JoinedAt time.Time
}
