package models

import "time"

type User struct {
	ID        int
	Nickname  string
	CreatedAt time.Time
}
