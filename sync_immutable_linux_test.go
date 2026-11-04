//go:build linux

package badger

import (
	"os"
	"testing"
	"unsafe"

	"github.com/dgraph-io/ristretto/v2/z"
)

// Inject an invalid mapping in an immutable WAL: syncing only the active WAL
// incorrectly reports success. No actual database file is corrupted.
func TestSyncReportsImmutableWALError(t *testing.T) {
	db, err := Open(DefaultOptions(t.TempDir()).WithLogger(nil).WithSyncWrites(false))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	buf := make([]byte, os.Getpagesize()+1)
	offset := 0
	if uintptr(unsafe.Pointer(&buf[0]))%uintptr(os.Getpagesize()) == 0 {
		offset = 1
	}
	bad := &memTable{wal: &logFile{MmapFile: &z.MmapFile{Data: buf[offset : offset+1]}}}
	db.lock.Lock()
	db.imm = append(db.imm, bad)
	db.lock.Unlock()
	defer func() {
		db.lock.Lock()
		db.imm = db.imm[:len(db.imm)-1]
		db.lock.Unlock()
	}()
	if err := db.Sync(); err == nil {
		t.Fatal("Sync ignored immutable WAL failure")
	}
}
