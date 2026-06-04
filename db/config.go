package db

import (
	"fmt"

	valid "github.com/go-playground/validator/v10"
)

// Type represent storage engine type
type (
	Type string
)

const (
	Postgres Type = "postgres"
)

// Config is a struct to configure postgresql
type Config struct {
	Type     Type   `required:"true" split_words:"true" validate:"required,oneof=postgres"`
	Host     string `required:"false" default:"localhost" split_words:"true"`
	Port     uint16 `default:"5432" split_words:"true"`
	User     string `required:"true" split_words:"true" validate:"required"`
	Password string `required:"true" split_words:"true" validate:"required"`
	Database string `required:"true" split_words:"true" validate:"required"`
	AppName  string `required:"true" split_words:"true" validate:"required"`
	//nolint:lll // Validator oneof tag must contain the full pgx sslmode allowlist.
	SslMode         string `default:"disable" split_words:"true" validate:"required,oneof=disable allow prefer require verify-ca verify-full"`
	MinConnections  int    `default:"4" split_words:"true" validate:"gte=0,lte=2147483647,ltefield=MaxConnections"`
	MaxConnections  int    `default:"16" split_words:"true" validate:"gt=0,lte=2147483647"`
	MaxConnLifetime int    `default:"3600" split_words:"true" validate:"gte=0"` // seconds
	MaxRetries      int    `default:"5" split_words:"true" validate:"gte=0"`    // initial-connection retry ceiling
	LogLevel        string `default:"error" split_words:"true"`
}

func validateDBConfig(cfg Config) error {
	err := valid.New().Struct(cfg)
	if err != nil {
		return fmt.Errorf("validate db config: %w", err)
	}

	return nil
}
