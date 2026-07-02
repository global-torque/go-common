module github.com/global-torque/go-common/tests

go 1.25.0

require (
	github.com/global-torque/go-common/httputils v1.0.20
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/global-torque/go-common/context v1.0.18 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/global-torque/go-common/httputils => ../httputils

replace github.com/global-torque/go-common/context => ../context
