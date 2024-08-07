package hsrv

/*
 * checkin.go
 * Handle /checkin requests
 * By J. Stuart McMurray
 * Created 20240806
 * Last Modified 20240806
 */

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/magisterquis/loweffortbotnetcontroller/internal/idmanager"
)

const (
	maxIDs         = 4
	id             = "kittens"
	callbackString = "moose"
)

func newTestCheckInHandler(t *testing.T) (*bytes.Buffer, checkInHandler) {
	dir := t.TempDir()
	/* Logger which logs to a buffer, without a timestamp. */
	var lv slog.LevelVar
	lv.Set(slog.LevelDebug)
	lb := new(bytes.Buffer)
	sl := slog.New(slog.NewJSONHandler(lb, &slog.HandlerOptions{
		ReplaceAttr: func(
			groups []string,
			a slog.Attr,
		) slog.Attr {
			if 0 == len(groups) && "time" == a.Key {
				return slog.Attr{}
			}
			return a
		},
		Level: &lv,
	}))

	/* ID Manager. */
	idm, err := idmanager.New(filepath.Join(dir, "id.json"), maxIDs)
	if nil != err {
		t.Fatalf("Creating ID manager: %s", err)
	}

	/* Checkin directory. */
	cid := filepath.Join(dir, "checkins")
	if err := os.MkdirAll(cid, 0770); nil != err {
		t.Fatalf("Making checkin directory %s: %s", cid, err)
	}

	return lb, checkInHandler{
		sl:             sl,
		idm:            idm,
		checkInDir:     cid,
		callbackString: callbackString,
	}
}

func TestCheckInHandler_Smoketest(t *testing.T) { newTestCheckInHandler(t) }

func TestCheckInHandler_Simple(t *testing.T) {
	lb, cih := newTestCheckInHandler(t)
	wantLog := `{"level":"DEBUG","msg":"Check-in",` +
		`"remote_address":"192.0.2.1","user_agent":"","id":"kittens",` +
		`"callback_request":false}`
	checkResponseAndLog(t, cih, lb, "", "", wantLog)
}

func TestCheckInHandler_ProcessListingAndCallbackRequest(t *testing.T) {
	lb, cih := newTestCheckInHandler(t)
	haveBody := `A process listing`
	wantLog := `{"level":"INFO","msg":"Check-in",` +
		`"remote_address":"192.0.2.1","user_agent":"","id":"kittens",` +
		`"callback_request":true}`
	/* Ask for a callback. */
	cih.idm.RequestCallback(id)
	/* Check in. */
	checkResponseAndLog(t, cih, lb, haveBody, callbackString, wantLog)
	/* See if we got the process listing. */
	cfn := filepath.Join(cih.checkInDir, id)
	b, err := os.ReadFile(cfn)
	if nil != err {
		t.Fatalf("Could not read checkin file %s: %s", cfn, err)
	}
	if got := string(b); got != haveBody {
		t.Fatalf(
			"Incorrect checking file %s:\n got: %s\nwant: %s",
			cfn,
			got,
			haveBody,
		)
	}
}

func checkResponseAndLog(
	t *testing.T,
	cih checkInHandler,
	lb *bytes.Buffer,
	body string,
	wantResponse string,
	wantLog string,
) (failed bool) {
	t.Helper()
	/* Muxify, for the pattern. */
	mux := http.NewServeMux()
	mux.Handle(CheckInPattern, cih)

	/* Make the request itself. */
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(
		http.MethodPost,
		"/checkin/"+id,
		strings.NewReader(body),
	))
	/* Should always get a 200. */
	if http.StatusOK != rr.Code {
		t.Errorf("Non-OK status code: %d", rr.Code)
	}
	/* See if the response is what we expect. */
	resBody, err := io.ReadAll(rr.Result().Body)
	if nil != err { /* Unpossible. */
		t.Fatalf("Error reading response body: %s", err)
	}
	if got := string(resBody); got != wantResponse {
		t.Errorf(
			"Incorrect response body:\n got: %s\nwant: %s",
			got,
			wantResponse,
		)
	}
	/* See if the log's correct. */
	wantLog = strings.TrimSpace(wantLog) /* Newlines. */
	if got := strings.TrimSpace(lb.String()); got != wantLog {
		t.Errorf("Log ircorrect:\ngot:\n%s\nwant:\n%s", got, wantLog)
	}

	return t.Failed()
}
