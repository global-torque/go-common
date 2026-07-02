module github.com/global-torque/go-common/db

go 1.25.0

require (
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/global-torque/go-common/configurator v1.0.20
	github.com/global-torque/go-common/context v1.0.18
	github.com/global-torque/go-common/logger v1.0.21
	github.com/global-torque/go-common/tests v1.0.23
	github.com/go-playground/validator/v10 v10.30.3
	github.com/jackc/pgx/v5 v5.10.0
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/global-torque/go-common/httputils v1.0.20 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3-0.20260117141049-eeee8ae54d81 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/kelseyhightower/envconfig v1.4.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rs/zerolog v1.35.1 // indirect
	golang.org/x/crypto v0.52.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/global-torque/go-common/configurator => ../configurator

replace github.com/global-torque/go-common/context => ../context

replace github.com/global-torque/go-common/logger => ../logger

replace github.com/global-torque/go-common/tests => ../tests

replace github.com/global-torque/go-common/httputils => ../httputils
