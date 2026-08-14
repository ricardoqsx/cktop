//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package state

import "os"

func lockFile(*os.File) (func() error, error) {
	return func() error { return nil }, nil
}

func syncDirectory(string) error {
	return nil
}
