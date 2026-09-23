package store

import (
	"backend/internal/config"
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresStore struct {
	db       *pgxpool.Pool
	putQuery string
	getQuery string
}

func newPostgresStore(ctx context.Context, postgresCfg *config.PostgresConfig) (*postgresStore, error) {
	if err := ensureDatabaseExists(ctx, postgresCfg); err != nil {
		return nil, err
	}

	poolConfig, err := pgxpool.ParseConfig("")
	if err != nil {
		return nil, err
	}

	poolConfig.ConnConfig.Host = postgresCfg.Host
	poolConfig.ConnConfig.Port = postgresCfg.Port
	poolConfig.ConnConfig.User = postgresCfg.User
	poolConfig.ConnConfig.Password = postgresCfg.Password
	poolConfig.ConnConfig.Database = postgresCfg.Name

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	putQuery := fmt.Sprintf(
		`INSERT INTO %s (%s, %s) VALUES ($1, $2)`,
		postgresCfg.Table, postgresCfg.Columns.ID, postgresCfg.Columns.URL,
	)

	getQuery := fmt.Sprintf(
		`SELECT %s FROM %s WHERE %s = $1`,
		postgresCfg.Columns.URL, postgresCfg.Table, postgresCfg.Columns.ID,
	)

	store := &postgresStore{
		db:       pool,
		putQuery: putQuery,
		getQuery: getQuery,
	}

	if err := store.initSchema(ctx, postgresCfg); err != nil {
		pool.Close()
		slog.Error("initializing schema failed", "err", err)
		return nil, err
	}

	return store, nil
}

func (p *postgresStore) PutUrl(
	ctx context.Context,
	id string,
	url string,
) bool {

	if _, err := p.db.Exec(ctx, p.putQuery, id, url); err != nil {
		slog.Error("postgres put failed", "err", err)
		return false
	}

	return true
}

func (p *postgresStore) GetUrl(
	ctx context.Context,
	id string,
) (string, bool) {
	var originalURL string

	err := p.db.QueryRow(ctx, p.getQuery, id).Scan(&originalURL)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", true
		}
		slog.Error("postgres get failed", "err", err)
		return "", false
	}

	return originalURL, true
}

func ensureDatabaseExists(ctx context.Context, postgresCfg *config.PostgresConfig) error {
	maintConfig, err := pgx.ParseConfig("")
	if err != nil {
		return err
	}

	maintConfig.Host = postgresCfg.Host
	maintConfig.Port = postgresCfg.Port
	maintConfig.User = postgresCfg.User
	maintConfig.Password = postgresCfg.Password
	maintConfig.Database = "postgres" // Connect to default maintenance database

	conn, err := pgx.ConnectConfig(ctx, maintConfig)
	if err != nil {
		slog.Error("maintenance database connection failed", "err", err)
		return err
	}
	defer conn.Close(ctx)

	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`
	if err := conn.QueryRow(ctx, checkQuery, postgresCfg.Name).Scan(&exists); err != nil {
		return  err
	}

	if exists {
		return nil
	}

	query := fmt.Sprintf("CREATE DATABASE %s", postgresCfg.Name)
	if _, err := conn.Exec(ctx, query); err != nil {
		var pgErr *pgconn.PgError
		// Handle concurrent startup race conditions (SQLSTATE 42P04: duplicate_database)
		if errors.As(err, &pgErr) && pgErr.Code == "42P04" {
			return nil
		}
		return err
	}

	slog.Info("database created successfully", "database", postgresCfg.Name)
	return nil
}

func (p *postgresStore) initSchema(
	ctx context.Context,
	postgresCfg *config.PostgresConfig,
) error {

	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			%s TEXT PRIMARY KEY,
			%s TEXT NOT NULL
		)
	`,
		postgresCfg.Table,
		postgresCfg.Columns.ID,
		postgresCfg.Columns.URL,
	)

	if _, err := p.db.Exec(ctx, query); err != nil {
		return err
	}

	return nil
}
