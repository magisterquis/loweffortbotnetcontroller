// Package callbackfiles - Watch for files to indicate a callback is requested
package callbackfiles

/*
 * callbackfiles.go
 * Watch for files to indicate a callback is requested
 * By J. Stuart McMurray
 * Created 20240807
 * Last Modified 20240807
 */

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/magisterquis/loweffortbotnetcontroller/internal/idmanager"
)

// GetCallbackRequests lists the files in dir and calls idm.RequestCallback
// with each filename.  The files are then deleted.  It keeps trying until
// there are no more files in case it's called in the middle of a batch of
// files being created.
func GetCallbackRequests(
	sl *slog.Logger,
	idm *idmanager.Manager,
	dir string,
) error {
	var (
		empty bool
		err   error
	)
	/* Keep going until the directory is empty. */
	for !empty {
		if empty, err = getCallbackRequests(sl, idm, dir); nil != err {
			return err
		}
	}
	return nil
}

// getCallbackRequets does what GetCallbackRequests says it does, but only one
// iteration.  It returns true, nil if the directory was empty.
func getCallbackRequests(
	sl *slog.Logger,
	idm *idmanager.Manager,
	dir string,
) (empty bool, err error) {
	/* Get the list of IDs we want to call us back. */
	des, err := os.ReadDir(dir)
	if nil != err {
		return false, fmt.Errorf("listing directory: %w", err)
	}
	/* If we don't have any, we're done. */
	if 0 == len(des) {
		return true, nil
	}
	/* Request a callback from each ID. */
	for _, de := range des {
		/* Only worry about regular files. */
		if !de.Type().IsRegular() {
			continue
		}
		/* Request the callback. */
		id := de.Name()
		idm.RequestCallback(de.Name())
		sl.Info("Callback request noted", "id", id)
		fn := filepath.Join(dir, id)
		if err := os.Remove(fn); nil != err {
			return false, fmt.Errorf("removing %s: %w", fn, err)
		}
	}

	return false, nil
}
