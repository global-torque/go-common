package db

import (
	"math"
	"strings"
	"testing"
)

func validDBConfig() Config {
	return Config{
		Type:            Postgres,
		Host:            "localhost",
		Port:            5432,
		User:            "user",
		Password:        "password",
		Database:        "database",
		AppName:         "test",
		SslMode:         "disable",
		MinConnections:  4,
		MaxConnections:  16,
		MaxConnLifetime: 3600,
		MaxRetries:      5,
		LogLevel:        "error",
	}
}

func TestValidateDBConfig(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "valid",
		},
		{
			name: "invalid type",
			mutate: func(cfg *Config) {
				cfg.Type = "mysql"
			},
			wantErr: "Type",
		},
		{
			name: "invalid ssl mode",
			mutate: func(cfg *Config) {
				cfg.SslMode = "invalid"
			},
			wantErr: "SslMode",
		},
		{
			name: "min connections greater than max",
			mutate: func(cfg *Config) {
				cfg.MinConnections = 17
			},
			wantErr: "MinConnections",
		},
		{
			name: "max connections over int32",
			mutate: func(cfg *Config) {
				cfg.MaxConnections = math.MaxInt32 + 1
			},
			wantErr: "MaxConnections",
		},
		{
			name: "negative max retries",
			mutate: func(cfg *Config) {
				cfg.MaxRetries = -1
			},
			wantErr: "MaxRetries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validDBConfig()
			if tt.mutate != nil {
				tt.mutate(&cfg)
			}

			err := validateDBConfig(cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}

				return
			}
			if err == nil {
				t.Fatalf("expected validation error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
