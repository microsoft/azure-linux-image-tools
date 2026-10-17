// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package file

import (
	"context"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type LockHandle struct {
	unlocked bool
	f        *os.File
}

func (h *LockHandle) Unlock() error {
	if h.unlocked {
		return nil
	}

	err := unix.Flock(int(h.f.Fd()), unix.LOCK_UN)
	if err != nil {
		return err
	}

	h.unlocked = true
	return nil
}

// Locks a file for exclusive access.
// Note: Lock is attached to file handle. So, the returned LockHandle can be ignored if the file is closed.
func LockExclusive(ctx context.Context, f *os.File, timeout time.Duration) (*LockHandle, error) {
	return lockHelper(ctx, f, timeout, unix.LOCK_EX)
}

// Locks a file for shared access.
// Note: Lock is attached to file handle. So, the returned LockHandle can be ignored if the file is closed.
func LockShared(ctx context.Context, f *os.File, timeout time.Duration) (*LockHandle, error) {
	return lockHelper(ctx, f, timeout, unix.LOCK_SH)
}

func lockHelper(ctx context.Context, f *os.File, timeout time.Duration, op int) (*LockHandle, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	for {
		err := unix.Flock(int(f.Fd()), op|unix.LOCK_NB)
		if err == nil {
			return &LockHandle{false, f}, nil
		}
		if err != syscall.EWOULDBLOCK {
			return nil, err
		}

		// Sleep before trying again.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case <-time.After(100 * time.Millisecond):
			continue
		}
	}
}
