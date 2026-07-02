module github.com/global-torque/go-common/server

go 1.25.0

require (
	github.com/global-torque/go-common/configurator v1.0.20
	github.com/global-torque/go-common/context v1.0.18
	github.com/global-torque/go-common/httputils v1.0.20
	github.com/global-torque/go-common/logger v1.0.21
	github.com/global-torque/go-common/response v1.0.19
	github.com/global-torque/go-common/validator v1.0.22
	github.com/global-torque/go-common/verser v1.0.19
	github.com/labstack/echo-contrib v0.50.1
	github.com/labstack/echo/v4 v4.15.2
	github.com/labstack/gommon v0.5.0
	github.com/pkg/errors v0.9.1
	github.com/rs/zerolog v1.35.1
	github.com/stretchr/testify v1.11.1
	go.uber.org/fx v1.24.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.30.3 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/kelseyhightower/envconfig v1.4.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.68.1 // indirect
	github.com/prometheus/procfs v0.20.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	go.uber.org/dig v1.19.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.28.0 // indirect
	golang.org/x/crypto v0.52.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/global-torque/go-common/response => ../response

replace github.com/global-torque/go-common/validator => ../validator

replace github.com/global-torque/go-common/logger => ../logger

replace github.com/global-torque/go-common/configurator => ../configurator

replace github.com/global-torque/go-common/context => ../context

replace github.com/global-torque/go-common/httputils => ../httputils

replace github.com/global-torque/go-common/verser => ../verser
