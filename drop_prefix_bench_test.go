/*
 * SPDX-FileCopyrightText: © 2017-2026 Istari Digital, Inc.
 * SPDX-License-Identifier: Apache-2.0
 */

package badger

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func benchmarkDropPrefixWholeTableDiscardStats(b *testing.B, entries int, encryptionKey []byte) {
	b.Helper()
	b.ReportMetric(float64(entries), "entries/drop")

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		dir := b.TempDir()
		opts := getTestOptions(dir).
			WithValueThreshold(32).
			WithValueLogFileSize(5 << 20).
			WithBaseTableSize(1 << 20).
			WithCompactL0OnClose(true).
			WithIndexCacheSize(64 << 20).
			WithEncryptionKey(encryptionKey)

		db, err := Open(opts)
		require.NoError(b, err)
		value := bytes.Repeat([]byte("v"), 256)
		wb := db.NewWriteBatch()
		for j := 0; j < entries; j++ {
			require.NoError(b, wb.Set([]byte(fmt.Sprintf("drop/%012d", j)), value))
		}
		require.NoError(b, wb.Flush())
		require.NoError(b, db.Close())

		db, err = Open(opts)
		require.NoError(b, err)
		require.NoError(b, db.Flatten(1))
		require.Positive(b, benchmarkNumNonL0WholePrefixTables(db, []byte("drop/")))
		_, discard := db.vlog.discardStats.MaxDiscard()
		require.Zero(b, discard)

		b.StartTimer()
		err = db.DropPrefix([]byte("drop/"))
		b.StopTimer()
		require.NoError(b, err)
		_, discard = db.vlog.discardStats.MaxDiscard()
		b.ReportMetric(float64(discard), "max-discard-B")
		require.NoError(b, db.Close())
	}
}

func benchmarkNumNonL0WholePrefixTables(db *DB, prefix []byte) int {
	var count int
	for _, level := range db.lc.levels[1:] {
		level.RLock()
		for _, tbl := range level.tables {
			if bytes.HasPrefix(tbl.Smallest(), prefix) && bytes.HasPrefix(tbl.Biggest(), prefix) {
				count++
			}
		}
		level.RUnlock()
	}
	return count
}

func BenchmarkDropPrefixWholeTableDiscardStats(b *testing.B) {
	// Fixture creation is intentionally outside the timer. Use -benchtime=1x for
	// reproducible scale runs without rebuilding large fixtures in one sample.
	for _, entries := range []int{10_000, 100_000, 1_000_000} {
		b.Run(fmt.Sprintf("entries=%d", entries), func(b *testing.B) {
			b.Run("plain", func(b *testing.B) {
				benchmarkDropPrefixWholeTableDiscardStats(b, entries, nil)
			})
			b.Run("encrypted", func(b *testing.B) {
				benchmarkDropPrefixWholeTableDiscardStats(b, entries, []byte("badgerkey16bytes"))
			})
		})
	}
}
