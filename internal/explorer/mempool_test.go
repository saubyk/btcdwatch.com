package explorer

import (
	"testing"

	"github.com/btcsuite/btcd/btcjson"
)

func TestMempoolSnapshotCachesWithinMaxAge(t *testing.T) {
	m := newMockBackend()
	m.mempool[hexID("tx1")] = btcjson.GetRawMempoolVerboseResult{
		Vsize: 200, Fee: 0.001, Time: 100,
	}

	mp := NewMempool(m)
	for range 3 {
		if _, err := mp.Snapshot(); err != nil {
			t.Fatal(err)
		}
	}
	if m.mempoolFetches != 1 {
		t.Errorf("mempool fetched %d times within max age, want 1",
			m.mempoolFetches)
	}

	mp.Invalidate()
	if _, err := mp.Snapshot(); err != nil {
		t.Fatal(err)
	}
	if m.mempoolFetches != 2 {
		t.Errorf("mempool fetched %d times after invalidate, want 2",
			m.mempoolFetches)
	}
}

func TestMempoolEntryDerivation(t *testing.T) {
	m := newMockBackend()
	m.mempool[hexID("tx1")] = btcjson.GetRawMempoolVerboseResult{
		Vsize: 250, Fee: 0.00025, Time: 42,
	}

	snap, err := NewMempool(m).Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	e := snap[hexID("tx1")]
	if e.FeeSats != 25_000 || e.VSize != 250 || e.Time != 42 {
		t.Fatalf("entry = %+v", e)
	}
	if e.FeeRate != 100 {
		t.Errorf("feeRate = %v, want 100 sat/vB", e.FeeRate)
	}
}
