package idmanager

/*
 * idmanager_test.go
 * Tests for idmanager.go
 * By J. Stuart McMurray
 * Created 20240806
 * Last Modified 20240807
 */

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
	"time"
)

const maxIDs = 4

func newTestManager(t *testing.T) *Manager {
	m, err := New(filepath.Join(t.TempDir(), "ids.json"), maxIDs)
	if nil != err {
		t.Fatalf("Making new ID manager: %s", err)
	}
	return m
}

func TestNew_Smoketest(t *testing.T) {
	m := newTestManager(t)
	if maxIDs != m.maxIDs {
		t.Errorf("Incorrect max IDs: got:%d want:%d", m.maxIDs, maxIDs)
	}
}

func TestManagerCheckIn(t *testing.T) {
	m := newTestManager(t)

	/* Make sure a single checkin works. */
	t.Run("first_checkin", func(t *testing.T) {
		/* Check in an initial ID. */
		id := "id_init"
		from := "from_init"
		m.CheckIn(id, from)
		/* Make sure it worked. */
		if !m.dirty {
			t.Errorf("Manager should be dirty, but isn't")
		}
		if got := len(m.ids); 1 != got {
			t.Errorf("Expect 1 seen ID, got %d", got)
		}
		/* And that the info looks right. */
		info, ok := m.ids[id]
		if !ok {
			t.Fatalf("Did not get info for ID %s", id)
		}
		if from != info.From {
			t.Errorf(
				"From incorrect: got:%s want:%s",
				from,
				info.From,
			)
		}
		if info.RequestCallback {
			t.Error("RequestCallback set but shouldn't be")
		}
		if info.When.IsZero() {
			t.Error("When should not be zero")
		}
	})

	/* Make sure if we have too many checking in, the old ones are
	deleted. */
	t.Run("multiple_checkins", func(t *testing.T) {
		/* Make one more ID than we can hold. */
		ids := make([]string, maxIDs+1)
		for i := range ids {
			ids[i] = fmt.Sprintf("id_%d", i)
		}
		froms := make([]string, len(ids))
		for i := range froms {
			froms[i] = fmt.Sprintf("from_%d", i)
		}

		/* Check in more IDs than we can handle. */
		for i := range ids {
			m.CheckIn(ids[i], froms[i])
		}

		/* Make sure we have as many as we can hold. */
		if got := len(m.ids); maxIDs != got {
			t.Errorf(
				"Incorrect IDs count: got:%d want:%d",
				got,
				maxIDs,
			)
		}

		/* Make sure it's the last maxIDs worth. */
		for i := len(ids) - maxIDs; i < len(ids); i++ {
			t.Run(ids[i], func(t *testing.T) {
				id := ids[i]
				from := froms[i]
				info, ok := m.ids[id]
				if !ok {
					t.Fatalf("No info")
				}
				if from != info.From {
					t.Errorf(
						"From incorrect: "+
							"got:%s want:%s",
						info.From,
						from,
					)
				}
			})
		}

		/* Make sure the IDs are in order. */
		t.Run("order_check", func(t *testing.T) {
			type seenID struct {
				id   string
				when time.Time
			}
			var (
				expectedIDs = ids[len(ids)-maxIDs:]
				idsSorted   = make([]seenID, 0, len(m.ids))
			)
			for id, info := range m.ids {
				idsSorted = append(idsSorted, seenID{
					id:   id,
					when: info.When,
				})
			}
			slices.SortFunc(idsSorted, func(a, b seenID) int {
				return a.when.Compare(b.when)
			})
			if len(idsSorted) != len(expectedIDs) {
				t.Fatalf(
					"Slice length mismatch:\n"+
						"    idsSorted: %d\n"+
						"  expectedIDs: %d",
					len(idsSorted),
					len(expectedIDs),
				)
			}
			for i, wantID := range expectedIDs {
				if gotID := idsSorted[i].id; gotID != wantID {
					t.Errorf(
						"Sorted ID %d incorrect:\n"+
							" got: %s\n"+
							"want: %s\n",
						i,
						gotID,
						wantID,
					)
				}
			}
		})
	})
}

func TestManagerRequestCallback(t *testing.T) {
	m := newTestManager(t)
	id := "test_id"

	/* Make sure we can pre-queue a callback request. */
	t.Run("pre-checkin", func(t *testing.T) {
		/* Request the callback and make sure it worked. */
		m.RequestCallback(id)
		if got := len(m.ids); 1 != got {
			t.Errorf("Incorrect IDs count: got:%d want:1", got)
		}
		info, ok := m.ids[id]
		if !ok {
			t.Fatalf("No info for ID %s", id)
		}
		if "" != info.From {
			t.Errorf("From unexpectedly set")
		}
		if !info.RequestCallback {
			t.Errorf("RequestCallback not set")
		}
		if info.When.IsZero() {
			t.Errorf("When is zero")
		}

		/* Make sure checking in works. */
		if !m.CheckIn(id, "") {
			t.Errorf("Did not get callback request on checkin")
		}
		if m.CheckIn(id, "") {
			t.Errorf("Got uneexpected second callback request")
		}
	})

	/* Make sure we can queue a reqest for a known ID. */
	t.Run("post-checkin", func(t *testing.T) {
		/* Clear the state. */
		m.CheckIn(id, "")
		if m.CheckIn(id, "") {
			t.Fatalf("RequestCallback set on second CheckIn")
		}
		/* Request a callback and make sure it worked. */
		m.RequestCallback(id)
		if !m.CheckIn(id, "") {
			t.Fatalf("Did not get callback request")
		}
		if m.CheckIn(id, "") {
			t.Fatalf("Got callback request on second check-in")
		}
	})

	/* Make sure we don't get a request for an unknown ID. */
	t.Run("unknown_id", func(t *testing.T) {
		if m.CheckIn(time.Now().String(), "") {
			t.Errorf("Got callback request for unknown ID")
		}
	})
}

func TestManagerRequestCallback_JSON(t *testing.T) {
	/* Just request a single callback. */
	m := newTestManager(t)
	id := "test_id"
	m.RequestCallback(id)
	/* Write it to a file. */
	if err := m.Write(); nil != err {
		t.Fatalf("Error writing ID file: %s", err)
	}
	/* Make sure the file looks right. */
	b, err := os.ReadFile(m.filename)
	if nil != err {
		t.Fatalf("Error reading ID file: %s", err)
	}
	want := `{
	"test_id": {
		"When": "",
		"From": "",
		"RequestCallback": true
	}
}
`
	if got := string(regexp.MustCompile(
		`"When": "[^"]+",`,
	).ReplaceAll(
		b,
		[]byte(`"When": "",`),
	)); got != want {
		t.Fatalf("ID file incorrect:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestManager_Write(t *testing.T) {
	m := newTestManager(t)
	want := make(idMap)
	var cbReq bool /* Make sure we've requessted a callback. */
	/* Check-in a bunch of IDs. */
	for i := range maxIDs {
		id := fmt.Sprintf("id_%d", i)
		from := fmt.Sprintf("from_%d", i)
		/* Check in this ID. */
		m.CheckIn(id, from)
		/* Note it for our want later. */
		info := want[id]
		info.From = from
		if !cbReq {
			info.RequestCallback = true
			m.RequestCallback(id)
			cbReq = true
		}
		want[id] = info
	}

	/* Write it out, and try to re-read it. */
	if err := m.Write(); nil != err {
		t.Fatalf("Write error: %s", err)
	}
	got := make(idMap)
	b, err := os.ReadFile(m.filename)
	if nil != err {
		t.Fatalf("Reading back %s: %s", m.filename, err)
	}
	if err := json.Unmarshal(b, &got); nil != err {
		t.Fatalf("Unmarshalling: %s", err)
	}

	/* Make sure all of the entries have a time set and zero them for
	checking. */
	for id, info := range got {
		if info.When.IsZero() {
			t.Errorf("ID %s has a zero When", id)
		}
		info.When = time.Time{}
		got[id] = info
	}
	if t.Failed() {
		return
	}

	/* Make sure we got the same maps. */
	for id, ginfo := range got {
		winfo, ok := want[id]
		if !ok {
			t.Errorf("Got extra ID: %s", id)
			continue
		}
		if winfo != ginfo {
			t.Errorf(
				"Info for %s incorrect:\n"+
					" got: %+v\n"+
					"want: %+v",
				id,
				ginfo,
				winfo,
			)
		}
	}
	for id := range want {
		if _, ok := got[id]; !ok {
			t.Errorf("Did not get info for %s", id)
		}
	}
}

func TestManagerWrite_JSON(t *testing.T) {
	m := newTestManager(t)
	/* Roll a known ID map. */
	when := time.Date(2000, time.January, 02, 03, 04, 05, 06, time.UTC)
	m.CheckIn("id_0", "from_0")
	m.CheckIn("id_1", "from_1")
	m.RequestCallback("id_1")
	m.RequestCallback("id_2")
	for id, info := range m.ids {
		info.When = when
		m.ids[id] = info
	}
	/* Write it out. */
	if err := m.Write(); nil != err {
		t.Fatalf("Write failed: %s", err)
	}
	/* Make sure it's what we expect. */
	want := `{
	"id_0": {
		"When": "2000-01-02T03:04:05.000000006Z",
		"From": "from_0",
		"RequestCallback": false
	},
	"id_1": {
		"When": "2000-01-02T03:04:05.000000006Z",
		"From": "from_1",
		"RequestCallback": true
	},
	"id_2": {
		"When": "2000-01-02T03:04:05.000000006Z",
		"From": "",
		"RequestCallback": true
	}
}
`
	b, err := os.ReadFile(m.filename)
	if nil != err {
		t.Fatalf("Error reading %s: %s", m.filename, err)
	}
	if got := string(b); got != want {
		t.Fatalf(
			"JSON incorrect:\nhave: %+v\ngot:\n%s\nwant:\n%s",
			m.ids,
			got,
			want,
		)
	}
}

func TestManagerRead(t *testing.T) {
	m := newTestManager(t)

	/* Update the backing file's JSON. */
	have := `
{
	"id_0": {
		"From": "from_0",
		"RequestCallback": false,
		"When": "2000-01-02T03:04:05.000000006Z"
	},
	"id_1": {
		"From": "",
		"RequestCallback": true,
		"When": "2000-01-02T03:04:05.000000006Z"
	},
	"id_2": {
		"From": "from_2",
		"RequestCallback": false,
		"When": "2000-01-02T03:04:05.000000006Z"
	},
	"id_3": {
		"From": "from_3",
		"RequestCallback": true,
		"When": "2000-01-02T03:04:05.000000006Z"
	}
}
`
	if err := os.WriteFile(m.filename, []byte(have), 0660); nil != err {
		t.Fatalf("Updating %s: %s", m.filename, err)
	}

	/* Roll the equivalent to the JSON. */
	want := make(idMap)
	when := time.Date(2000, time.January, 02, 03, 04, 05, 06, time.UTC)
	for i := range maxIDs {
		id := fmt.Sprintf("id_%d", i)
		info := want[id]
		info.From = fmt.Sprintf("from_%d", i)
		if 1 == i%2 {
			info.RequestCallback = true
		}
		info.When = when
		want[id] = info
	}
	/* ID 1 is just a callback. */
	info, ok := want["id_1"]
	if !ok {
		t.Fatalf("No ID 1")
	}
	info.From = ""
	want["id_1"] = info

	/* Make sure read works. */
	if err := m.read(); nil != err {
		t.Fatalf("Re-reading %s: %s", m.filename, err)
	}
	if !maps.Equal(m.ids, want) {
		gotj, err := json.Marshal(m.ids)
		if nil != err {
			t.Fatalf("Marshalling got: %s", err)
		}
		wantj, err := json.Marshal(want)
		if nil != err {
			t.Fatalf("Marshalling want: %s", err)
		}
		t.Fatalf(
			"read failed:\nhave:\n%s\nwant:\n%s\ngot:\n%s",
			have,
			gotj,
			wantj,
		)
	}
}
