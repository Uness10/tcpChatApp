package models

import "time"

type Room struct {
	ID        int
	Name      string
	CreatedAt time.Time
}
