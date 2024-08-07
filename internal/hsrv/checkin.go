package hsrv

/*
 * checkin.go
 * Handle /checkin requests
 * By J. Stuart McMurray
 * Created 20240806
 * Last Modified 20240806
 */

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/magisterquis/loweffortbotnetcontroller/internal/idmanager"
)

// CheckInPattern is the URL pattern used for checkins.
const (
	CheckInPattern = "/checkin/{id}"
)

// checkInHandler handles requests for checkins.
type checkInHandler struct {
	sl             *slog.Logger
	idm            *idmanager.Manager
	checkInDir     string /* Bodies sent in checkin requests. */
	callbackString string /* How to ask for a callback. */
}

// ServeHTTP implements http.Handler.
func (h checkInHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sl := h.sl.With()

	/* Attempt to add the remote address and user agent to the logger. */
	from, _, err := net.SplitHostPort(r.RemoteAddr)
	if nil != err && "" == from {
		sl.Error(
			"Could not determine remote address",
			"raw_address", r.RemoteAddr,
		)
		from = "unknown"
	}
	sl = sl.With(
		"remote_address", from,
		"user_agent", r.UserAgent(),
	)

	/* Work out who this is. */
	id := strings.Map( /* Only allow LDH. */
		func(r rune) rune {
			if 'A' <= r || r <= 'Z' || /* LDH. */
				'a' <= r || r <= 'z' ||
				'0' <= r || r <= '9' ||
				'-' == r ||
				'.' == r {
				return r
			}
			return -1 /* Illegal character. */
		},
		r.PathValue("id"),
	)
	if "" == id {
		sl.Error("Missing id")
		return
	}
	sl = sl.With("id", id)

	/* Grab the body, hopefully a process listing. */
	b, err := io.ReadAll(r.Body)
	var mbe *http.MaxBytesError
	if nil != err && !errors.As(err, &mbe) {
		sl.Error("Error reading body", "error", err)
		return
	}

	/* Write the body to the appropriate file. */
	fn := filepath.Join(h.checkInDir, id)
	if err := os.WriteFile(fn, b, 0660); nil != err {
		sl.Error(
			"Error writing check-in body",
			"filename", fn,
			"error", err,
		)
	}

	/* Note we've checked in and maybe ask for a callback. */
	rcb := h.idm.CheckIn(id, from)
	sl = sl.With("callback_request", rcb)
	msg := "Check-in"
	if !rcb {
		sl.Debug(msg)
		return
	}
	if _, err := io.WriteString(w, h.callbackString); nil != err {
		sl.Error("Error requesting callback", "error", err)
	}
	sl.Info(msg)
}
