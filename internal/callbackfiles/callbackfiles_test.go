package callbackfiles

/*
 * callbackfiles_test.go
 * Tests for callbackfiles.go
 * By J. Stuart McMurray
 * Created 20240807
 * Last Modified 20240807
 */

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/magisterquis/loweffortbotnetcontroller/internal/idmanager"
)

func TestGetCallbackRequests(t *testing.T) {
	var (
		dir     = t.TempDir()
		cbDir   = filepath.Join(dir, "cbreq")
		idmFile = filepath.Join(dir, "id.json")
		nCB     = 10
		idm     *idmanager.Manager
		id      = make([]string, nCB)
		err     error
	)
	if idm, err = idmanager.New(idmFile, uint(nCB)); nil != err {
		t.Fatalf("Error making ID manager: %s", err)
	}

	/* Make the callback request directory. */
	if err := os.MkdirAll(cbDir, 0770); nil != err {
		t.Fatalf("Error making callback directory %s: %s", cbDir, err)
	}

	/* Make some files for callback-requesting. */
	for i := range id {
		id[i] = fmt.Sprintf("id_%d", i)
	}
	for _, id := range id {
		fn := filepath.Join(cbDir, id)
		if err := os.WriteFile(fn, nil, 0660); nil != err {
			t.Fatalf("Creating %s: %s", fn, err)
		}
	}

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

	/* Request callbacks. */
	if err := GetCallbackRequests(sl, idm, cbDir); nil != err {
		t.Fatalf("GetCallbackRequests failed: %s", err)
	}

	/* Make sure we asked for callbacks. */
	for _, id := range id {
		if !idm.CheckIn(id, "") {
			t.Fatalf("Callback for %s not requested", id)
		}
	}

	/* Make sure logs are correct. */
	want := `{"level":"INFO","msg":"Callback request noted","id":"id_0"}
{"level":"INFO","msg":"Callback request noted","id":"id_1"}
{"level":"INFO","msg":"Callback request noted","id":"id_2"}
{"level":"INFO","msg":"Callback request noted","id":"id_3"}
{"level":"INFO","msg":"Callback request noted","id":"id_4"}
{"level":"INFO","msg":"Callback request noted","id":"id_5"}
{"level":"INFO","msg":"Callback request noted","id":"id_6"}
{"level":"INFO","msg":"Callback request noted","id":"id_7"}
{"level":"INFO","msg":"Callback request noted","id":"id_8"}
{"level":"INFO","msg":"Callback request noted","id":"id_9"}
`
	if got := lb.String(); got != want {
		t.Fatalf("Incorrect log:\ngot:\n%s\nwant:\n%s", got, want)
	}
}
