package domain

import (
	"time"

	"github.com/google/uuid"
)

type Server struct {
	ID         uuid.UUID
	Name       string
	Host       string
	Port       int
	Username   string
	AuthType   string
	Password   *string
	PrivateKey *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	LastSeen   *time.Time
	IsActive   bool
}

type ServerStatus struct {
	ID           uuid.UUID
	ServerID     uuid.UUID
	Status       string //  TODO "online" или "offline" или "syncing" или "error"
	LastChecked  time.Time
	ErrorMessage *string
	CreatedAt    time.Time
}
