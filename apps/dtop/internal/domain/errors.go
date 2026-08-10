package domain

import "errors"

var ErrRemoteUnsupported = errors.New("remote Docker Engine support is not enabled yet")
