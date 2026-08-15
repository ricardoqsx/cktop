//go:build windows

package state

import (
	"context"
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

func lockFile(ctx context.Context, file *os.File) (func() error, error) {
	overlapped := &windows.Overlapped{}
	handle := windows.Handle(file.Fd())
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, overlapped)
		if err == nil {
			break
		}
		if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
	return func() error {
		return windows.UnlockFileEx(handle, 0, 1, 0, overlapped)
	}, nil
}

func syncDirectory(string) error {
	return nil
}
