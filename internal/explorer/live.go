package explorer

import (
	"context"
	"time"

	"btcd.watch/internal/node"
)

// The live feeds — stats, fees, and the mempool update — are recomputed
// off the request path and served from a shared cache. btcd blocks the
// RPCs they depend on (getrawmempoolverbose touches the UTXO cache,
// header lookups take the chain lock) for minutes at a time while
// flushing its UTXO cache, and the landing page, fee ticker, and WS
// pushes must keep answering from the last computed values instead of
// hanging past the proxy's origin timeout.

// liveRefreshEvery bounds how often the cache is recomputed; matches the
// public cache TTL on the stats/fees endpoints.
const liveRefreshEvery = 5 * time.Second

// liveRefreshStuck is how long an in-flight recompute may run before a
// fresh attempt may start alongside it (same watchdog as the sync check).
const liveRefreshStuck = 2 * time.Minute

// liveData is the cached result set; fields are nil until their first
// successful compute.
type liveData struct {
	stats  *Stats
	fees   *FeeEstimate
	update *MempoolUpdate
}

// liveSnapshot returns the cached feeds and, when one is due, kicks a
// background recompute. It never waits on the node.
func (s *Service) liveSnapshot() liveData {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()

	if time.Since(s.liveAttemptAt) >= liveRefreshEvery &&
		(!s.liveInFlight || time.Since(s.liveKickedAt) >= liveRefreshStuck) {

		s.liveInFlight = true
		s.liveKickedAt = time.Now()
		go s.refreshLive()
	}

	return s.live
}

// refreshLive recomputes every feed once with no locks held across the
// RPC calls. Feeds that fail keep their previous value — stale numbers
// beat a dead dashboard while the node is flushing.
func (s *Service) refreshLive() {
	stats, statsErr := s.computeStats()
	fees, feesErr := s.computeFees()
	update, updateErr := s.computeMempoolUpdate()

	s.liveMu.Lock()
	defer s.liveMu.Unlock()
	s.liveInFlight = false
	s.liveAttemptAt = time.Now()
	if statsErr == nil {
		s.live.stats = stats
	}
	if feesErr == nil {
		s.live.fees = fees
	}
	if updateErr == nil {
		s.live.update = update
	}
}

// WarmLive kicks a background recompute so the first requests after a
// (re)connect serve fresh values instead of node_unavailable.
func (s *Service) WarmLive() {
	s.liveSnapshot()
}

// RunLiveRefresh keeps the live cache warm regardless of traffic. Without
// it the cache only refreshes on demand, so the first visitor after a
// quiet stretch was served numbers frozen at whatever was current when
// the previous visitor left — a stale block height on first paint,
// corrected only when the lazily kicked recompute landed and the next hub
// tick delivered it. Cancel the context to stop.
func (s *Service) RunLiveRefresh(ctx context.Context) {
	ticker := time.NewTicker(s.liveEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.liveSnapshot()
		}
	}
}

// Stats returns the cached dashboard payload. ErrUnavailable until the
// first compute lands (the node was never reachable yet).
func (s *Service) Stats() (*Stats, error) {
	if d := s.liveSnapshot(); d.stats != nil {
		return d.stats, nil
	}
	return nil, node.ErrUnavailable
}

// Fees returns the cached fee tiers (see Stats for the caching contract).
func (s *Service) Fees() (*FeeEstimate, error) {
	if d := s.liveSnapshot(); d.fees != nil {
		return d.fees, nil
	}
	return nil, node.ErrUnavailable
}

// MempoolUpdate returns the cached live-mempool payload (see Stats for
// the caching contract).
func (s *Service) MempoolUpdate() (*MempoolUpdate, error) {
	if d := s.liveSnapshot(); d.update != nil {
		return d.update, nil
	}
	return nil, node.ErrUnavailable
}
