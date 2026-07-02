module github.com/global-torque/go-common/logger

go 1.25.0

require (
	github.com/global-torque/go-common/configurator v1.0.20
	github.com/global-torque/go-common/context v1.0.18
	github.com/global-torque/go-common/tests v1.0.23
	github.com/pkg/errors v0.9.1
	github.com/rs/zerolog v1.35.1
	go.uber.org/fx v1.24.0
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/global-torque/go-common/httputils v1.0.20 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/kelseyhightower/envconfig v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.28.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/global-torque/go-common/configurator => ../configurator

replace github.com/global-torque/go-common/context => ../context

replace github.com/global-torque/go-common/tests => ../tests

replace github.com/global-torque/go-common/httputils => ../httputils
