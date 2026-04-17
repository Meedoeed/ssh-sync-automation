package postgres

import (
	"context"
	"time"

	"github.com/Meedoeed/ssh-sync-automation/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TaskRepo struct {
	db *pgxpool.Pool
}

func NewTaskRepo(db *pgxpool.Pool) *TaskRepo {
	return &TaskRepo{db: db}
}

func (r *TaskRepo) Create(ctx context.Context, task *domain.SyncTask) error {
	query := `
		INSERT INTO sync_tasks (
			id, server_id, direction, file_name, remote_path, local_path,
			file_size, status, max_attempts
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`

	if task.ID == uuid.Nil {
		task.ID = uuid.New()
	}

	if task.MaxAttempts == 0 {
		task.MaxAttempts = 5
	}

	return r.db.QueryRow(ctx, query,
		task.ID,
		task.ServerID,
		task.Direction,
		task.FileName,
		task.RemotePath,
		task.LocalPath,
		task.FileSize,
		task.Status,
		task.MaxAttempts,
	).Scan(&task.CreatedAt, &task.UpdatedAt)
}

func (r *TaskRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.SyncTask, error) {
	query := `
		SELECT id, server_id, direction, file_name, remote_path, local_path,
		       file_size, bytes_transferred, status, attempt_count, max_attempts,
		       error_message, started_at, completed_at, created_at, updated_at
		FROM sync_tasks
		WHERE id = $1
	`

	var task domain.SyncTask
	err := r.db.QueryRow(ctx, query, id).Scan(
		&task.ID,
		&task.ServerID,
		&task.Direction,
		&task.FileName,
		&task.RemotePath,
		&task.LocalPath,
		&task.FileSize,
		&task.BytesTransferred,
		&task.Status,
		&task.AttemptCount,
		&task.MaxAttempts,
		&task.ErrorMessage,
		&task.StartedAt,
		&task.CompletedAt,
		&task.CreatedAt,
		&task.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &task, nil
}

func (r *TaskRepo) ListByServer(ctx context.Context, serverID uuid.UUID, status *domain.SyncStatus, limit int) ([]*domain.SyncTask, error) {
	query := `
		SELECT id, server_id, direction, file_name, remote_path, local_path,
		       file_size, bytes_transferred, status, attempt_count, max_attempts,
		       error_message, started_at, completed_at, created_at, updated_at
		FROM sync_tasks
		WHERE server_id = $1
	`
	args := []any{serverID}

	if status != nil {
		query += " AND status = $2"
		args = append(args, *status)
	}

	query += " ORDER BY created_at DESC LIMIT $"
	if status != nil {
		query += "3"
	} else {
		query += "2"
	}
	args = append(args, limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*domain.SyncTask
	for rows.Next() {
		var task domain.SyncTask
		err := rows.Scan(
			&task.ID,
			&task.ServerID,
			&task.Direction,
			&task.FileName,
			&task.RemotePath,
			&task.LocalPath,
			&task.FileSize,
			&task.BytesTransferred,
			&task.Status,
			&task.AttemptCount,
			&task.MaxAttempts,
			&task.ErrorMessage,
			&task.StartedAt,
			&task.CompletedAt,
			&task.CreatedAt,
			&task.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, &task)
	}

	return tasks, nil
}

func (r *TaskRepo) ListPending(ctx context.Context, limit int) ([]*domain.SyncTask, error) {
	query := `
		SELECT id, server_id, direction, file_name, remote_path, local_path,
		       file_size, bytes_transferred, status, attempt_count, max_attempts,
		       error_message, started_at, completed_at, created_at, updated_at
		FROM sync_tasks
		WHERE status = $1 AND attempt_count < max_attempts
		ORDER BY created_at
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`

	rows, err := r.db.Query(ctx, query, domain.StatusPending, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*domain.SyncTask
	for rows.Next() {
		var task domain.SyncTask
		err := rows.Scan(
			&task.ID,
			&task.ServerID,
			&task.Direction,
			&task.FileName,
			&task.RemotePath,
			&task.LocalPath,
			&task.FileSize,
			&task.BytesTransferred,
			&task.Status,
			&task.AttemptCount,
			&task.MaxAttempts,
			&task.ErrorMessage,
			&task.StartedAt,
			&task.CompletedAt,
			&task.CreatedAt,
			&task.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, &task)
	}

	return tasks, nil
}

func (r *TaskRepo) Update(ctx context.Context, task *domain.SyncTask) error {
	query := `
		UPDATE sync_tasks
		SET file_size = $2, bytes_transferred = $3, status = $4,
		    attempt_count = $5, error_message = $6, started_at = $7, completed_at = $8
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query,
		task.ID,
		task.FileSize,
		task.BytesTransferred,
		task.Status,
		task.AttemptCount,
		task.ErrorMessage,
		task.StartedAt,
		task.CompletedAt,
	)

	return err
}

func (r *TaskRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.SyncStatus, errMsg *string) error {
	query := `
        UPDATE sync_tasks
        SET status = $2,
            error_message = $3,
            updated_at = NOW()
        WHERE id = $1
    `

	_, err := r.db.Exec(ctx, query, id, string(status), errMsg)
	if err != nil {
		return err
	}

	if status == domain.StatusCompleted || status == domain.StatusFailed || status == domain.StatusCancelled {
		queryComplete := `
            UPDATE sync_tasks
            SET completed_at = NOW()
            WHERE id = $1 AND completed_at IS NULL
        `
		_, err = r.db.Exec(ctx, queryComplete, id)
	}

	return err
}
func (r *TaskRepo) UpdateProgress(ctx context.Context, id uuid.UUID, bytesTransferred int64) error {
	query := `
		UPDATE sync_tasks
		SET bytes_transferred = $2
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, id, bytesTransferred)
	return err
}

func (r *TaskRepo) IncrementAttempt(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE sync_tasks
		SET attempt_count = attempt_count + 1,
		    started_at = COALESCE(started_at, NOW())
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, id)
	return err
}

func (r *TaskRepo) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM sync_tasks WHERE id = $1`

	_, err := r.db.Exec(ctx, query, id)
	return err
}

func (r *TaskRepo) DeleteCompletedOlderThan(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `
		DELETE FROM sync_tasks
		WHERE status IN ('completed', 'failed', 'cancelled')
		  AND completed_at < NOW() - $1
	`

	result, err := r.db.Exec(ctx, query, olderThan)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected(), nil
}
