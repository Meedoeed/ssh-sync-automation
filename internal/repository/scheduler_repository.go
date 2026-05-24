package repository

import (
	"context"
	"time"
)

type SchedulerLeader struct {
	LeaderID      string
	LastHeartbeat time.Time
	ExpiresAt     time.Time
}

type SchedulerLeaderRepository interface {
	TryBecomeLeader(ctx context.Context, schedulerID string, ttl time.Duration) (bool, error)
	RenewLeader(ctx context.Context, schedulerID string, ttl time.Duration) (bool, error)
	ReleaseLeader(ctx context.Context, schedulerID string) error
	GetLeader(ctx context.Context) (*SchedulerLeader, error)
}
