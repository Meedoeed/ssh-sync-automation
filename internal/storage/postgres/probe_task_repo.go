package postgres

import (
	"context"
	"time"

	"github.com/Meedoeed/ssh-sync-automation/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProbeTaskRepo struct {
	db *pgxpool.Pool
}

func NewProbeTaskRepo(db *pgxpool.Pool) *ProbeTaskRepo {
	return &ProbeTaskRepo{db: db}
}

func (r *ProbeTaskRepo) Create(ctx context.Context, task *domain.ProbeTask) error {
	query := `
		INSERT INTO probe_tasks (id, server_id, task_type, status, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	task.ID = uuid.New()
	task.CreatedAt = time.Now()
	task.Status = domain.ProbeStatusPending

	_, err := r.db.Exec(ctx, query,
		task.ID,
		task.ServerID,
		task.TaskType,
		task.Status,
		task.CreatedAt,
	)
	return err
}

func (r *ProbeTaskRepo) ListPending(ctx context.Context, limit int) ([]*domain.ProbeTask, error) {
	query := `
		SELECT id, server_id, task_type, status, created_at, worker_id, completed_at
		FROM probe_tasks
		WHERE status = $1
		ORDER BY created_at
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`

	rows, err := r.db.Query(ctx, query, domain.ProbeStatusPending, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*domain.ProbeTask
	for rows.Next() {
		var task domain.ProbeTask
		err := rows.Scan(
			&task.ID,
			&task.ServerID,
			&task.TaskType,
			&task.Status,
			&task.CreatedAt,
			&task.WorkerID,
			&task.CompletedAt,
		)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, &task)
	}
	return tasks, nil
}

func (r *ProbeTaskRepo) ReserveTask(ctx context.Context, taskID uuid.UUID, workerID string) error {
	query := `
		UPDATE probe_tasks
		SET status = $1, worker_id = $2
		WHERE id = $3 AND status = $4
	`
	_, err := r.db.Exec(ctx, query, domain.ProbeStatusProcessing, workerID, taskID, domain.ProbeStatusPending)
	return err
}

func (r *ProbeTaskRepo) CompleteTask(ctx context.Context, taskID uuid.UUID, filesFound []string, errMsg *string) error {
	status := domain.ProbeStatusCompleted
	if errMsg != nil {
		status = domain.ProbeStatusFailed
	}

	query := `
		UPDATE probe_tasks
		SET status = $1, completed_at = $2, files_found = $3, error_message = $4
		WHERE id = $5
	`
	_, err := r.db.Exec(ctx, query, status, time.Now(), filesFound, errMsg, taskID)
	return err
}

func (r *ProbeTaskRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ProbeTask, error) {
	query := `
		SELECT id, server_id, task_type, status, created_at, worker_id, completed_at, files_found, error_message
		FROM probe_tasks
		WHERE id = $1
	`

	var task domain.ProbeTask
	var filesFound []string
	var errorMsg *string

	err := r.db.QueryRow(ctx, query, id).Scan(
		&task.ID,
		&task.ServerID,
		&task.TaskType,
		&task.Status,
		&task.CreatedAt,
		&task.WorkerID,
		&task.CompletedAt,
		&filesFound,
		&errorMsg,
	)
	if err != nil {
		return nil, err
	}

	task.FilesFound = filesFound
	if errorMsg != nil {
		task.Error = errorMsg
	}

	return &task, nil
}
