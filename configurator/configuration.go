package configurator

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// Configuration  is a struct for storing configuration
type Configuration struct {
	Name   string
	Config any
}

// NewConfiguration sets conf from env
func NewConfiguration(conf interface{}, prefixes ...string) error {
	prefix := ""

	err := LoadDotEnv()
	if err != nil {
		return err
	}

	if len(prefixes) > 0 {
		prefix = prefixes[0]
	}

	if err := envconfig.Process(prefix, conf); err != nil {
		_ = envconfig.Usage(prefix, conf)

		return err
	}
	if err := validateRequiredStrings(conf); err != nil {
		return err
	}

	return nil
}

func LoadDotEnv() error {
	envPath := os.Getenv("ENV_FILE")

	var err error
	if envPath == "" {
		// if .env exists
		if _, err1 := os.Stat(".env"); err1 == nil {
			err = godotenv.Load(".env")
		}
	} else {
		err = godotenv.Load(envPath) // if path to env file defined, check error
	}

	if err != nil {
		return err
	}

	return nil
}

func Parse[T any](prefixes ...string) (T, error) {
	var t T
	return t, NewConfiguration(&t, prefixes...)
}
