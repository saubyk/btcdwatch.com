package explorer

import (
	"errors"
	"testing"
	"time"

	"btcdwatch.com/internal/node"
)

// The live feeds are cache-served: requests must never wait on node RPC
// (see live.go). These tests drive refreshLive directly, the way the
// background goroutine does.
func TestLiveCache(t *testing.T) {
	m := newMockBackend()
	installChain(m, 20, 60*time.Second)
	s := newTestService(m)

	// Before the first compute lands, every feed is unavailable.
	if _, err := s.Stats(); !errors.Is(err, node.ErrUnavailable) {
		t.Errorf("Stats before first compute: err = %v, want ErrUnavailable", err)
	}
	if _, err := s.Fees(); !errors.Is(err, node.ErrUnavailable) {
		t.Errorf("Fees before first compute: err = %v, want ErrUnavailable", err)
	}
	if _, err := s.MempoolUpdate(); !errors.Is(err, node.ErrUnavailable) {
		t.Errorf("MempoolUpdate before first compute: err = %v, want ErrUnavailable", err)
	}

	s.refreshLive()
	stats, err := s.Stats()
	if err != nil || stats.BlockHeight != 20 {
		t.Fatalf("Stats after refresh = %+v, %v; want height 20", stats, err)
	}
	if _, err := s.Fees(); err != nil {
		t.Errorf("Fees after refresh: %v", err)
	}
	if _, err := s.MempoolUpdate(); err != nil {
		t.Errorf("MempoolUpdate after refresh: %v", err)
	}

	// A failing recompute keeps the previous values — stale numbers beat
	// a dead dashboard while the node is flushing or unreachable.
	m.failAll(errors.New("node down"))
	s.refreshLive()
	stats, err = s.Stats()
	if err != nil || stats.BlockHeight != 20 {
		t.Errorf("Stats after failed refresh = %+v, %v; want cached height 20",
			stats, err)
	}
}

// Stats must answer instantly from cache even when the node has stopped
// answering RPC — /api/stats hung past Cloudflare's 100s origin timeout
// when btcd stalled mid-flush, because it computed on the request path.
func TestStatsDoesNotBlockOnStalledNode(t *testing.T) {
	m := newMockBackend()
	installChain(m, 20, 60*time.Second)
	b := &stalledBackend{mockBackend: m, release: make(chan struct{})}
	defer close(b.release)

	s := newTestService(b)
	done := make(chan error, 1)
	go func() {
		_, err := s.Stats()
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, node.ErrUnavailable) {
			t.Errorf("empty cache: err = %v, want ErrUnavailable", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stats blocked on a stalled node RPC")
	}
}
