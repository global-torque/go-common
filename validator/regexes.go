package validator

import "regexp"

var pathRegex = regexp.MustCompile(`^[A-Za-z0-9/_-]+$`)
