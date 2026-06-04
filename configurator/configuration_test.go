package configurator

import (
	"fmt"
	"os"
	"testing"
)

// will load .env automatically
// and check if all required envs are exists
func TestNewConfiguration(t *testing.T) {
	type Config struct {
		Host     string `required:"false" default:"localhost" split_words:"true"`
		Port     uint16 `default:"5432" split_words:"true"`
		User     string `required:"true" split_words:"true"`
		Password string `required:"true" split_words:"true"`
	}

	cfg := &Config{}
	err := NewConfiguration(cfg, "DB")
	if err != nil {
		t.Fatalf("cannot process %s", err.Error())
	}
}

// will load .env automatically
// and check if all required envs are exists
func TestAlternativeEnvPath(t *testing.T) {
	type ServerConfig struct {
		Host string `required:"true"`
		Port uint16 `required:"true"`
	}

	t.Setenv("ENV_FILE", ".env.tests")
	cfg := &ServerConfig{}
	err := NewConfiguration(cfg, "")
	if err != nil {
		t.Fatalf("cannot process %s", err.Error())
	}
}

// will load .env automatically
// and check if all required envs are exists
func TestRequiredEnvs(t *testing.T) {
	type ServerConfig struct {
		MyHost string `required:"true"`
		Port   int16  `required:"true"`
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
		}
	}()

	cfg := &ServerConfig{}
	err := NewConfiguration(cfg, "")
	if err == nil {
		t.Errorf("no env set, should return an error")
	}
}

func TestRequiredString(t *testing.T) {
	t.Setenv("ENV_FILE", "")
	t.Setenv("CFG_REQUIRED", "  value  ")
	t.Setenv("CFG_OPTIONAL", "")

	type Config struct {
		Required RequiredString
		Optional string `required:"true"`
	}

	cfg := &Config{}
	if err := NewConfiguration(cfg, "CFG"); err != nil {
		t.Fatalf("cannot process config: %v", err)
	}
	if cfg.Required.String() != "value" {
		t.Fatalf("expected trimmed required string, got %q", cfg.Required.String())
	}
	if cfg.Optional != "" {
		t.Fatalf("expected regular string to keep empty value, got %q", cfg.Optional)
	}
}

func TestRequiredStringRejectsBlankValue(t *testing.T) {
	t.Setenv("ENV_FILE", "")
	t.Setenv("CFG_REQUIRED", "   ")

	type Config struct {
		Required RequiredString
	}

	cfg := &Config{}
	if err := NewConfiguration(cfg, "CFG"); err == nil {
		t.Fatal("expected blank required string to fail")
	}
}

func TestRequiredStringRejectsMissingValueWithoutRequiredTag(t *testing.T) {
	t.Setenv("ENV_FILE", "")
	unsetEnv(t, "CFG_REQUIRED")

	type Config struct {
		Required RequiredString
	}

	cfg := &Config{}
	if err := NewConfiguration(cfg, "CFG"); err == nil {
		t.Fatal("expected missing required string to fail")
	}
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()

	oldValue, exists := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("cannot unset %s: %v", key, err)
	}

	t.Cleanup(func() {
		if exists {
			_ = os.Setenv(key, oldValue)
			return
		}
		_ = os.Unsetenv(key)
	})
}
