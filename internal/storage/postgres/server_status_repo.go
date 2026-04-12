package postgres

import (
	"context"
	"time"

	"github.com/Meedoeed/ssh-sync-automation/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ServerStatusRepo struct {
	db *pgxpool.Pool
}

func NewServerStatusRepo(db *pgxpool.Pool) *ServerStatusRepo {
	return &ServerStatusRepo{db: db}
}

func (r *ServerStatusRepo) Create(ctx context.Context, status *domain.ServerStatus) error {
	query := `
		INSERT INTO server_status (id, server_id, status, last_checked, error_message)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at
	`

	if status.ID == uuid.Nil {
		status.ID = uuid.New()
	}

	if status.LastChecked.IsZero() {
		status.LastChecked = time.Now()
	}

	return r.db.QueryRow(ctx, query,
		status.ID,
		status.ServerID,
		status.Status,
		status.LastChecked,
		status.ErrorMessage,
	).Scan(&status.CreatedAt)
}

func (r *ServerStatusRepo) GetLatestForServer(ctx context.Context, serverID uuid.UUID) (*domain.ServerStatus, error) {
	query := `
		SELECT id, server_id, status, last_checked, error_message, created_at
		FROM server_status
		WHERE server_id = $1
		ORDER BY last_checked DESC
		LIMIT 1
	`

	var status domain.ServerStatus
	err := r.db.QueryRow(ctx, query, serverID).Scan(
		&status.ID,
		&status.ServerID,
		&status.Status,
		&status.LastChecked,
		&status.ErrorMessage,
		&status.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &status, nil
}

func (r *ServerStatusRepo) ListByServer(ctx context.Context, serverID uuid.UUID, limit int) ([]*domain.ServerStatus, error) {
	query := `
		SELECT id, server_id, status, last_checked, error_message, created_at
		FROM server_status
		WHERE server_id = $1
		ORDER BY last_checked DESC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, serverID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statuses []*domain.ServerStatus
	for rows.Next() {
		var status domain.ServerStatus
		err := rows.Scan(
			&status.ID,
			&status.ServerID,
			&status.Status,
			&status.LastChecked,
			&status.ErrorMessage,
			&status.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, &status)
	}

	return statuses, nil
}
