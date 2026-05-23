package domain

import (
	"time"

	"github.com/google/uuid"
)

type SyncDirection string

const (
	DirectionUpload   SyncDirection = "upload"
	DirectionDownload SyncDirection = "download"
)

type SyncStatus string

const (
	StatusPending    SyncStatus = "pending"
	StatusProcessing SyncStatus = "processing"
	StatusCompleted  SyncStatus = "completed"
	StatusFailed     SyncStatus = "failed"
	StatusCancelled  SyncStatus = "cancelled"
)

type SyncTask struct {
	ID               uuid.UUID
	ServerID         uuid.UUID
	WorkerID         *string
	Direction        SyncDirection
	FileName         string
	RemotePath       string
	LocalPath        string
	FileSize         int64
	BytesTransferred int64
	Status           SyncStatus
	AttemptCount     int
	MaxAttempts      int
	ErrorMessage     *string
	StartedAt        *time.Time
	CompletedAt      *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (t *SyncTask) Progress() float64 {
	if t.FileSize == 0 {
		return 0
	}
	return float64(t.BytesTransferred) / float64(t.FileSize) * 100
}
