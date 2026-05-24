package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Meedoeed/ssh-sync-automation/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SchedulerLeaderRepo struct {
	db *pgxpool.Pool
}

func NewSchedulerLeaderRepo(db *pgxpool.Pool) *SchedulerLeaderRepo {
	return &SchedulerLeaderRepo{db: db}
}

func (r *SchedulerLeaderRepo) TryBecomeLeader(ctx context.Context, schedulerID string, ttl time.Duration) (bool, error) {
	query := `
		INSERT INTO scheduler_leader (id, leader_id, last_heartbeat, expires_at)
		VALUES ('default', $1, NOW(), NOW() + $2::INTERVAL)
		ON CONFLICT (id) DO UPDATE SET
			leader_id = EXCLUDED.leader_id,
			last_heartbeat = EXCLUDED.last_heartbeat,
			expires_at = EXCLUDED.expires_at
		WHERE scheduler_leader.expires_at < NOW()
	`

	interval := fmt.Sprintf("%d seconds", int(ttl.Seconds()))

	tag, err := r.db.Exec(ctx, query, schedulerID, interval)
	if err != nil {
		return false, fmt.Errorf("failed to acquire leader: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

func (r *SchedulerLeaderRepo) RenewLeader(ctx context.Context, schedulerID string, ttl time.Duration) (bool, error) {
	query := `
		UPDATE scheduler_leader
		SET last_heartbeat = NOW(),
		    expires_at = NOW() + $2::INTERVAL,
		    updated_at = NOW()
		WHERE id = 'default' AND leader_id = $1 AND expires_at > NOW()
	`

	interval := fmt.Sprintf("%d seconds", int(ttl.Seconds()))

	tag, err := r.db.Exec(ctx, query, schedulerID, interval)
	if err != nil {
		return false, fmt.Errorf("failed to renew leader: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

func (r *SchedulerLeaderRepo) ReleaseLeader(ctx context.Context, schedulerID string) error {
	query := `
		UPDATE scheduler_leader
		SET expires_at = NOW()
		WHERE id = 'default' AND leader_id = $1
	`

	_, err := r.db.Exec(ctx, query, schedulerID)
	return err
}

func (r *SchedulerLeaderRepo) GetLeader(ctx context.Context) (*repository.SchedulerLeader, error) {
	query := `
		SELECT leader_id, last_heartbeat, expires_at
		FROM scheduler_leader
		WHERE id = 'default'
	`

	var leader repository.SchedulerLeader
	err := r.db.QueryRow(ctx, query).Scan(&leader.LeaderID, &leader.LastHeartbeat, &leader.ExpiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &leader, nil
}
