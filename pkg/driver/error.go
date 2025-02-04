package driver

import (
	"errors"
	"fmt"
)

var (
	errNilProtocol       = errors.New("protocol is nil")
	errs3ProtocolMissing = errors.New("S3 protocol not defined")
	errInvalidDriverId   = errors.New(fmt.Sprintf("driver id cannot contain character '%s'", separator))
)
