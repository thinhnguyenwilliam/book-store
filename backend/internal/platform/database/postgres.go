package database

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	URL                   string
	MaxOpenConnections    int
	MaxIdleConnections    int
	ConnectionMaxLifetime string
	ConnectionMaxIdleTime string
}

func Open(ctx context.Context, config Config) (*gorm.DB, error) {
	connectionMaxLifetime, err := time.ParseDuration(config.ConnectionMaxLifetime)
	if err != nil {
		return nil, fmt.Errorf("parse postgres connection max lifetime: %w", err)
	}
	connectionMaxIdleTime, err := time.ParseDuration(config.ConnectionMaxIdleTime)
	if err != nil {
		return nil, fmt.Errorf("parse postgres connection max idle time: %w", err)
	}

	db, err := gorm.Open(postgres.Open(config.URL), &gorm.Config{
		TranslateError:         true,
		PrepareStmt:            true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres with GORM: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get postgres sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(config.MaxOpenConnections)
	sqlDB.SetMaxIdleConns(config.MaxIdleConnections)
	sqlDB.SetConnMaxLifetime(connectionMaxLifetime)
	sqlDB.SetConnMaxIdleTime(connectionMaxIdleTime)

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}

func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
