package db

import (
	"context"
	"database/sql"

	"github.com/ICan-TC/lib/logging"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

func New(dsn string) (*bun.DB, error) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	db := bun.NewDB(sqldb, pgdialect.New())
	l := logging.L()

	if err := sqldb.Ping(); err != nil {
		l.Warn().Err(err).Msg("Failed to connect to database")
		return nil, err
	}
	l.Info().Msg("Connected to database")
	return db, nil
}

func Ping(db *bun.DB) error {
	l := logging.L()
	ctx := context.Background()

	l.Debug().Msg("Pinging database")
	err := db.PingContext(ctx)
	if err != nil {
		l.Error().Err(err).Msg("Database ping failed")
	} else {
		l.Debug().Msg("Database ping successful")
	}
	return err
}
