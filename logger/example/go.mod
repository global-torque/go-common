module github.com/global-torque/go-common/logger/example

go 1.25.0

require (
	github.com/global-torque/go-common/logger v1.0.21
	github.com/global-torque/go-common/verser v1.0.19
	github.com/labstack/echo/v4 v4.15.2
	github.com/pkg/errors v0.9.1
)

require (
	github.com/global-torque/go-common/configurator v1.0.20 // indirect
	github.com/global-torque/go-common/context v1.0.18 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/kelseyhightower/envconfig v1.4.0 // indirect
	github.com/labstack/gommon v0.5.0 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/rs/zerolog v1.35.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	golang.org/x/crypto v0.52.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
)

replace github.com/global-torque/go-common/logger => ..

replace github.com/global-torque/go-common/verser => ../../verser

replace github.com/global-torque/go-common/configurator => ../../configurator

replace github.com/global-torque/go-common/context => ../../context

replace github.com/global-torque/go-common/tests => ../../tests

replace github.com/global-torque/go-common/httputils => ../../httputils
