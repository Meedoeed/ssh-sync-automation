package domain

import (
	"time"

	"github.com/google/uuid"
)

type ProbeStatus string

const (
	ProbeStatusPending    ProbeStatus = "pending"
	ProbeStatusProcessing ProbeStatus = "processing"
	ProbeStatusCompleted  ProbeStatus = "completed"
	ProbeStatusFailed     ProbeStatus = "failed"
)

type ProbeTask struct {
	ID          uuid.UUID
	ServerID    uuid.UUID
	TaskType    string
	Status      ProbeStatus
	CreatedAt   time.Time
	CompletedAt *time.Time
	WorkerID    *string
	FilesFound  []string
	Error       *string
}
