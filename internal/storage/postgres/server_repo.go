package postgres

import (
	"context"

	"github.com/Meedoeed/ssh-sync-automation/internal/domain"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ServerRepo struct {
	db *pgxpool.Pool
}

func NewServerRepo(db *pgxpool.Pool) *ServerRepo {
	return &ServerRepo{db: db}
}

func (r *ServerRepo) Create(ctx context.Context, server *domain.Server) error {
	query := `
		INSERT INTO servers (id, name, host, port, username, auth_type, password, private_key, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`

	if server.ID == uuid.Nil {
		server.ID = uuid.New()
	}

	err := r.db.QueryRow(ctx, query,
		server.ID,
		server.Name,
		server.Host,
		server.Port,
		server.Username,
		server.AuthType,
		server.Password,
		server.PrivateKey,
		server.IsActive,
	).Scan(&server.CreatedAt, &server.UpdatedAt)

	if err != nil {
		logger.Get().Error().
			Err(err).
			Str("server_name", server.Name).
			Msg("Failed to create server")
		return err
	}

	logger.Get().Info().
		Str("server_id", server.ID.String()).
		Str("server_name", server.Name).
		Msg("Server created")

	return nil
}

func (r *ServerRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Server, error) {
	query := `
		SELECT id, name, host, port, username, auth_type, password, private_key,
		       created_at, updated_at, last_seen, is_active
		FROM servers
		WHERE id = $1
	`

	var server domain.Server
	err := r.db.QueryRow(ctx, query, id).Scan(
		&server.ID,
		&server.Name,
		&server.Host,
		&server.Port,
		&server.Username,
		&server.AuthType,
		&server.Password,
		&server.PrivateKey,
		&server.CreatedAt,
		&server.UpdatedAt,
		&server.LastSeen,
		&server.IsActive,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &server, nil
}

func (r *ServerRepo) GetByName(ctx context.Context, name string) (*domain.Server, error) {
	query := `
		SELECT id, name, host, port, username, auth_type, password, private_key,
		       created_at, updated_at, last_seen, is_active
		FROM servers
		WHERE name = $1
	`

	var server domain.Server
	err := r.db.QueryRow(ctx, query, name).Scan(
		&server.ID,
		&server.Name,
		&server.Host,
		&server.Port,
		&server.Username,
		&server.AuthType,
		&server.Password,
		&server.PrivateKey,
		&server.CreatedAt,
		&server.UpdatedAt,
		&server.LastSeen,
		&server.IsActive,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &server, nil
}

func (r *ServerRepo) List(ctx context.Context, activeOnly bool) ([]*domain.Server, error) {
	query := `
		SELECT id, name, host, port, username, auth_type, password, private_key,
		       created_at, updated_at, last_seen, is_active
		FROM servers
	`
	if activeOnly {
		query += " WHERE is_active = true"
	}
	query += " ORDER BY name"

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*domain.Server
	for rows.Next() {
		var server domain.Server
		err := rows.Scan(
			&server.ID,
			&server.Name,
			&server.Host,
			&server.Port,
			&server.Username,
			&server.AuthType,
			&server.Password,
			&server.PrivateKey,
			&server.CreatedAt,
			&server.UpdatedAt,
			&server.LastSeen,
			&server.IsActive,
		)
		if err != nil {
			return nil, err
		}
		servers = append(servers, &server)
	}

	return servers, nil
}

func (r *ServerRepo) Update(ctx context.Context, server *domain.Server) error {
	query := `
		UPDATE servers
		SET name = $2, host = $3, port = $4, username = $5,
		    auth_type = $6, password = $7, private_key = $8, is_active = $9
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query,
		server.ID,
		server.Name,
		server.Host,
		server.Port,
		server.Username,
		server.AuthType,
		server.Password,
		server.PrivateKey,
		server.IsActive,
	)

	if err != nil {
		logger.Get().Error().
			Err(err).
			Str("server_id", server.ID.String()).
			Msg("Failed to update server")
		return err
	}

	return nil
}

func (r *ServerRepo) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM servers WHERE id = $1`

	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		logger.Get().Error().
			Err(err).
			Str("server_id", id.String()).
			Msg("Failed to delete server")
		return err
	}

	logger.Get().Info().
		Str("server_id", id.String()).
		Msg("Server deleted")

	return nil
}

func (r *ServerRepo) UpdateLastSeen(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE servers SET last_seen = NOW() WHERE id = $1`

	_, err := r.db.Exec(ctx, query, id)
	return err
}
