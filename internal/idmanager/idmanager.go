// Package idmanager - Keeps track of checking-in IDs
package idmanager

/*
 * idmanager.go
 * Keeps track of checking-in IDs
 * By J. Stuart McMurray
 * Created 20240806
 * Last Modified 20240807
 */

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync"
	"time"
)

// idMap holds the set of IDs which have checked in.
type idMap map[string]struct { /* ID -> info */
	When            time.Time /* Last checkin */
	From            string    /* IP Address */
	RequestCallback bool      /* Request a callback next time */
}

// Manager keeps track of the IDs which have checked in.
type Manager struct {
	mu       sync.Mutex
	filename string /* Backing file */
	dirty    bool   /* Changed since last write */
	maxIDs   uint   /* Maximum number of IDs */
	ids      idMap  /* Checked-in IDs */
}

// New returns a new Manager backed by the file name fn which tracks at most
// n IDs.  New panics if n == 0.
func New(fn string, n uint) (*Manager, error) {
	/* Gotta have at least one ID. */
	if 0 == n {
		panic("must track at least one ID")
	}

	/* Roll a new manager. */
	m := Manager{
		filename: fn,
		maxIDs:   n,
		ids:      make(idMap), /* for just in case. */
	}
	m.mu.Lock() /* Should be unnecessary. */
	defer m.mu.Unlock()

	/* Read the initial file. */
	if err := m.read(); nil != err {
		return nil, fmt.Errorf("reading underlying file: %w", err)
	}

	/* Make sure we can write it back. */
	m.dirty = true
	if err := m.write(); nil != err {
		return nil, fmt.Errorf("re-writing underlying file: %w", err)
	}

	return &m, nil
}

// read (re-)raeds m's underlying file.  read's caller must hold m.mu.
func (m *Manager) read() error {
	/* Map for eventual reading. */
	idm := make(idMap)

	/* Get hold of the underlying file. */
	f, err := os.Open(m.filename)
	if errors.Is(err, fs.ErrNotExist) { /* No underlying file is ok. */
		m.ids = idm
		return nil
	}
	if nil != err { /* Other errors aren't ok. */
		return fmt.Errorf(
			"opening underlying file %s: %w",
			m.filename,
			err,
		)
	}
	defer f.Close()

	/* Read into a new map. */
	if err := json.NewDecoder(f).Decode(&idm); nil != err &&
		!errors.Is(err, io.EOF) {
		return fmt.Errorf(
			"un-JSONing underlying file %s: %w",
			m.filename,
			err,
		)
	}
	m.ids = idm

	/* No change since the last update, more or less. */
	m.dirty = false

	return nil
}

// Write flushes m's checked-in IDs to m's underlying file, if there have been
// changes since the last write.
func (m *Manager) Write() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.write()
}

// write does what Write says it does, but requires its caller to hold m.mu.
func (m *Manager) write() error {
	/* If we haven't changed since the last write, not much to do. */
	if !m.dirty {
		return nil
	}

	/* Grab the file to which to write. */
	f, err := os.OpenFile(
		m.filename,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0660,
	)
	if nil != err {
		return fmt.Errorf(
			"opening underlying file %s: %w",
			m.filename,
			err,
		)
	}
	defer f.Close()

	/* Write the JSON to it. */
	enc := json.NewEncoder(f)
	enc.SetIndent("", "\t")
	if err := enc.Encode(m.ids); nil != err {
		return fmt.Errorf(
			"JSONing underlying file %s: %w",
			m.filename,
			err,
		)
	}

	/* No longer dirty since the last write. */
	m.dirty = false

	return nil
}

// CheckIn processes a check-in.  It returns true if the RequestCallback flag
// is set for the given ID.
func (m *Manager) CheckIn(id, from string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	/* Update this ID's entry. */
	info := m.ids[id]
	info.From = from
	info.When = time.Now()
	rc := info.RequestCallback
	info.RequestCallback = false
	m.ids[id] = info
	m.dirty = true

	/* Make sure we don't have too many IDs. */
	m.ensureNotTooManyIDs()

	return rc
}

// RequestCallback sets id's RequestCallback flag to true.  The id need not
// already exist.
func (m *Manager) RequestCallback(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	/* Set this IDs RequestCallback flag. */
	info := m.ids[id]
	info.RequestCallback = true

	/* If this is a new ID, set the timestamp.  The lack of a From address
	will tell us it's not a callback timestamp.  Hopefully. */
	if "" == info.From {
		info.When = time.Now()
	}

	/* Update the ID list and make sure we don't have too many IDs. */
	m.ids[id] = info
	m.dirty = true
	m.ensureNotTooManyIDs()
}

// ensureNotTooManyIDs ensures m doesn't have too many IDs by deleting the
// least-recently-seen ID.  ensureNotTooManyIDs caller must hold m.mu.
func (m *Manager) ensureNotTooManyIDs() {
	/* If we don't have too many IDs, life's easy. */
	if uint(len(m.ids)) <= m.maxIDs {
		return
	}

	/* Find and delete the oldest ID. */
	var (
		oldestID   string
		oldestWhen time.Time
	)
	for id, info := range m.ids {
		if "" == oldestID || info.When.Before(oldestWhen) {
			oldestID = id
			oldestWhen = info.When
		}
	}

	/* If we didn't find one, something's gone terribly wrong. */
	if "" == oldestID {
		panic("BUG: could not find the oldest ID")
	}
	delete(m.ids, oldestID)
}
