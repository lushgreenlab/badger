package badger

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
)

func TestConcurrentSyncAcrossWALAndValueLogRotation(t *testing.T) {
	for _, threshold := range []int64{1024, 32768} {
		t.Run(fmt.Sprint(threshold), func(t *testing.T) {
			path := t.TempDir()
			opts := DefaultOptions(path).WithLogger(nil).WithSyncWrites(false).
				WithMemTableSize(1 << 20).WithValueLogFileSize(1 << 20).WithValueThreshold(threshold)
			db, err := Open(opts)
			if err != nil {
				t.Fatal(err)
			}
			var wg sync.WaitGroup
			for worker := range 4 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					value := bytes.Repeat([]byte{byte(worker + 1)}, 8192)
					for i := range 128 {
						if err := db.Update(func(txn *Txn) error {
							for j := range 2 {
								if err := txn.Set([]byte(fmt.Sprintf("%d/%d/%d", worker, i, j)), value); err != nil {
									return err
								}
							}
							return nil
						}); err != nil {
							t.Error(err)
							return
						}
						if err := db.Sync(); err != nil {
							t.Error(err)
							return
						}
					}
				}()
			}
			wg.Wait()
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = Open(opts)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := db.View(func(txn *Txn) error {
				for worker := range 4 {
					want := bytes.Repeat([]byte{byte(worker + 1)}, 8192)
					for i := range 128 {
						for j := range 2 {
							item, err := txn.Get([]byte(fmt.Sprintf("%d/%d/%d", worker, i, j)))
							if err != nil {
								return err
							}
							got, err := item.ValueCopy(nil)
							if err != nil {
								return err
							}
							if !bytes.Equal(got, want) {
								return fmt.Errorf("corrupt paired document %d/%d/%d", worker, i, j)
							}
						}
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
