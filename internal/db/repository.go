package db

import (
	"database/sql"
	"fmt"

	"tcpServer.com/internal/models"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateUser(user *models.User) error {
	return r.db.QueryRow(
		"INSERT INTO users (nickname) VALUES ($1) RETURNING id,created_at",
		user.Nickname,
	).Scan(&user.ID, &user.CreatedAt)
}

func (r *Repository) FindUserByNickname(nickname string) (*models.User, error) {
	var user models.User
	err := r.db.QueryRow(
		"SELECT id, nickname, created_at FROM users WHERE nickname = $1",
		nickname,
	).Scan(&user.ID, &user.Nickname, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) CreateRoom(room *models.Room) error {
	return r.db.QueryRow(
		"INSERT INTO rooms (name) VALUES ($1) RETURNING id, created_at",
		room.Name,
	).Scan(&room.ID, &room.CreatedAt)
}

func (r *Repository) FindAllRooms() ([]string, error) {
	rows, err := r.db.Query("SELECT name FROM rooms")
	if err != nil {
		return nil, fmt.Errorf("failed to query rooms: %w", err)
	}
	defer rows.Close()

	var rooms []string
	for rows.Next() {
		var roomName string
		if err := rows.Scan(&roomName); err != nil {
			return nil, fmt.Errorf("error scanning room name: %w", err)
		}
		rooms = append(rooms, roomName)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iteration error: %w", err)
	}

	return rooms, nil
}

func (r *Repository) FindRoomByName(name string) (*models.Room, error) {
	var room models.Room
	err := r.db.QueryRow(
		"SELECT id,name,created_at from rooms where name = $1",
		name,
	).Scan(&room.ID, &room.Name, &room.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &room, nil
}
func (r *Repository) SaveMessage(message *models.Message) error {
	_, err := r.db.Exec(
		"INSERT INTO messages (content, room_id, user_id) VALUES ($1,$2,$3)",
		message.Content, message.RoomID, message.UserID,
	)
	return err
}

func (r *Repository) EnrollUser(enrollement *models.Enrollement) error {
	_, err := r.db.Exec(
		"INSERT INTO enrollements (user_id,room_id) VALUES ($1,$2)",
		enrollement.UserID, enrollement.RoomID,
	)
	return err
}
