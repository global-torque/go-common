# configurator

Import path: `github.com/global-torque/go-common/configurator/v2`

Loads startup configuration structs from environment variables with
`github.com/kelseyhightower/envconfig`. It also supports `.env` files through
`github.com/joho/godotenv`.

## Use For

- Parsing service config during startup.
- Loading `.env` automatically when present.
- Loading a custom dotenv file through `ENV_FILE`.
- Reusing parsed config instances through `Configurator`.
- Required string env values that must reject blank or whitespace-only input.

## Do Not Use For

- Hot-path config reads or dynamic reloads.
- Long-running refresh loops. `NewConfiguration` may read dotenv files.
- Secret management beyond env loading.

## Key APIs

- `NewConfiguration(conf, prefixes...) error`
- `Parse[T](prefixes ...string) (T, error)`
- `LoadDotEnv() error`
- `NewConfigurator(configs ...Configuration) *Configurator`
- `(*Configurator).Get`, `MustGet`, `New`, `MustNew`, `Print`, `MustPrint`
- `RequiredString`

## Configuration

Use normal `envconfig` struct tags:

```go
type Config struct {
	Host     string `required:"false" default:"localhost" split_words:"true"`
	Port     uint16 `default:"5432" split_words:"true"`
	User     string `required:"true" split_words:"true"`
	Password string `required:"true" split_words:"true"`
}
```

`LoadDotEnv` reads `ENV_FILE` when set. If `ENV_FILE` is empty and `.env`
exists in the current directory, it loads `.env`.

## Wiring Pattern

For one-off startup config:

```go
cfg, err := configurator.Parse[Config]("DB")
if err != nil {
	return err
}
```

For shared config:

```go
conf := configurator.NewConfigurator()
cfg, err := conf.New("logger", &LoggerConfig{}, "log")
if err != nil {
	return err
}
```

Use `RequiredString` when empty strings are not allowed:

```go
type PubSubConfig struct {
	Topic configurator.RequiredString `split_words:"true"`
}
```

## Testing

Use `t.Setenv("ENV_FILE", "")` to avoid loading a real dotenv file, or set
`ENV_FILE=.env.tests` when tests should load fixture env values.

## Gotchas

- `MustGet`, `MustNew`, and `MustPrint` panic. Keep them in application startup
  code, not reusable libraries.
- `Configurator.Get` copies cached configs with `copier` and ignores empty
  values.
- `RequiredString` is required by type; do not add redundant `required:"true"`.
