// internal/storage/postgres/server_repo.go
package postgres

import (
	"context"

	"github.com/Meedoeed/ssh-sync-automation/internal/domain"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/encryption"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ServerRepo struct {
	db        *pgxpool.Pool
	encryptor *encryption.Encryptor // Добавляем
}

func NewServerRepo(db *pgxpool.Pool, encryptor *encryption.Encryptor) *ServerRepo {
	return &ServerRepo{
		db:        db,
		encryptor: encryptor,
	}
}

func (r *ServerRepo) Create(ctx context.Context, server *domain.Server) error {
	// Шифруем пароль и ключ перед сохранением
	var encryptedPassword, encryptedPrivateKey *string

	if server.Password != nil && *server.Password != "" {
		encrypted, err := r.encryptor.Encrypt(*server.Password)
		if err != nil {
			logger.Get().Error().Err(err).Msg("Failed to encrypt password")
			return err
		}
		encryptedPassword = &encrypted
	}

	if server.PrivateKey != nil && *server.PrivateKey != "" {
		encrypted, err := r.encryptor.Encrypt(*server.PrivateKey)
		if err != nil {
			logger.Get().Error().Err(err).Msg("Failed to encrypt private key")
			return err
		}
		encryptedPrivateKey = &encrypted
	}

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
		encryptedPassword,
		encryptedPrivateKey,
		server.IsActive,
	).Scan(&server.CreatedAt, &server.UpdatedAt)

	if err != nil {
		logger.Get().Error().
			Err(err).
			Str("server_name", server.Name).
			Msg("Failed to create server")
		return err
	}

	server.Password = nil
	server.PrivateKey = nil

	return nil
}

// internal/storage/postgres/server_repo.go

func (r *ServerRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Server, error) {
	query := `
        SELECT id, name, host, port, username, auth_type, password, private_key,
               created_at, updated_at, last_seen, is_active
        FROM servers
        WHERE id = $1
    `

	var server domain.Server
	var encryptedPassword, encryptedPrivateKey *string

	err := r.db.QueryRow(ctx, query, id).Scan(
		&server.ID,
		&server.Name,
		&server.Host,
		&server.Port,
		&server.Username,
		&server.AuthType,
		&encryptedPassword,
		&encryptedPrivateKey,
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

	logger.Get().Debug().
		Str("server_id", server.ID.String()).
		Bool("has_encrypted_password", encryptedPassword != nil).
		Msg("Retrieved server from DB")

	if encryptedPassword != nil && *encryptedPassword != "" {
		decrypted, err := r.encryptor.Decrypt(*encryptedPassword)
		if err != nil {
			logger.Get().Error().Err(err).Msg("Failed to decrypt password")
			return nil, err
		}
		server.Password = &decrypted
		logger.Get().Debug().
			Str("server_id", server.ID.String()).
			Int("password_length", len(decrypted)).
			Msg("Password decrypted successfully")
	} else {
		logger.Get().Warn().
			Str("server_id", server.ID.String()).
			Msg("No encrypted password found")
	}

	if encryptedPrivateKey != nil && *encryptedPrivateKey != "" {
		decrypted, err := r.encryptor.Decrypt(*encryptedPrivateKey)
		if err != nil {
			logger.Get().Error().Err(err).Msg("Failed to decrypt private key")
			return nil, err
		}
		server.PrivateKey = &decrypted
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
	var encryptedPassword, encryptedPrivateKey *string

	err := r.db.QueryRow(ctx, query, name).Scan(
		&server.ID,
		&server.Name,
		&server.Host,
		&server.Port,
		&server.Username,
		&server.AuthType,
		&encryptedPassword,
		&encryptedPrivateKey,
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

	if encryptedPassword != nil && *encryptedPassword != "" {
		decrypted, err := r.encryptor.Decrypt(*encryptedPassword)
		if err != nil {
			return nil, err
		}
		server.Password = &decrypted
	}

	if encryptedPrivateKey != nil && *encryptedPrivateKey != "" {
		decrypted, err := r.encryptor.Decrypt(*encryptedPrivateKey)
		if err != nil {
			return nil, err
		}
		server.PrivateKey = &decrypted
	}

	return &server, nil
}

func (r *ServerRepo) Update(ctx context.Context, server *domain.Server) error {
	var encryptedPassword, encryptedPrivateKey *string

	if server.Password != nil && *server.Password != "" {
		encrypted, err := r.encryptor.Encrypt(*server.Password)
		if err != nil {
			return err
		}
		encryptedPassword = &encrypted
	}

	if server.PrivateKey != nil && *server.PrivateKey != "" {
		encrypted, err := r.encryptor.Encrypt(*server.PrivateKey)
		if err != nil {
			return err
		}
		encryptedPrivateKey = &encrypted
	}

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
		encryptedPassword,
		encryptedPrivateKey,
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
		var encryptedPassword, encryptedPrivateKey *string

		err := rows.Scan(
			&server.ID,
			&server.Name,
			&server.Host,
			&server.Port,
			&server.Username,
			&server.AuthType,
			&encryptedPassword,
			&encryptedPrivateKey,
			&server.CreatedAt,
			&server.UpdatedAt,
			&server.LastSeen,
			&server.IsActive,
		)
		if err != nil {
			return nil, err
		}

		if encryptedPassword != nil && *encryptedPassword != "" {
			decrypted, err := r.encryptor.Decrypt(*encryptedPassword)
			if err != nil {
				logger.Get().Error().
					Err(err).
					Str("server_id", server.ID.String()).
					Msg("Failed to decrypt password")
				return nil, err
			}
			server.Password = &decrypted
		}

		// Дешифруем приватный ключ
		if encryptedPrivateKey != nil && *encryptedPrivateKey != "" {
			decrypted, err := r.encryptor.Decrypt(*encryptedPrivateKey)
			if err != nil {
				logger.Get().Error().
					Err(err).
					Str("server_id", server.ID.String()).
					Msg("Failed to decrypt private key")
				return nil, err
			}
			server.PrivateKey = &decrypted
		}

		servers = append(servers, &server)
	}

	return servers, nil
}

func (r *ServerRepo) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM servers WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}

func (r *ServerRepo) UpdateLastSeen(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE servers SET last_seen = NOW() WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}
