# btcdwatch.com — Implementation Plan

This plan turns the design in [`ARCHITECTURE.md`](./ARCHITECTURE.md) into working software in
five independently verifiable milestones. Read the architecture doc first; this document does
not restate API schemas or derivation rules — it says what gets built when, and how each step is
proven against a live regtest network.

---

## 1. Approach & ground rules

- **Milestone-driven.** Each milestone (M1–M5) ends in a state that can be demonstrated
  end-to-end against the local btcd regtest node. The regtest harness
  (`btc-regtest-env/scripts/`: `start-network.sh`, `miner.sh`, `txgen.sh`, …) keeps a miner and
  transaction generator churning, so real pending/confirmed data exists from day one.
- **Design fidelity.** The design handoff (`design_handoff_btcdwatch/`) is final:
  `README.md` is the authoritative spec and token source; `BlockPeek.dc.html` is the pixel
  reference. The UI is **recreated in React** — the reference file's custom runtime tags are
  never copied. Fonts: Plus Jakarta Sans (UI) + IBM Plex Mono (hashes/amounts), loaded via
  Google Fonts links in `index.html`. Single light theme; brand orange `#F7931A`; cream
  background `#FCF6EE`. Styling is plain CSS custom properties (no Tailwind) for pixel fidelity.
- **Design views to implement** (state-driven SPA, one `view` state): landing (search, example
  chips, fee estimator with 3 tiers × 3 vB presets, dark stats bar, "run your own node" CTA),
  loading, notfound, confirmed-tx (Simple/Detailed toggle, 6-segment confirmation progress,
  celebration banner), pending-tx (ETA/queue panel, **Watch** button → live flip to confirmed),
  address (balance, received/sent/count tiles, activity list with pagination).
- **No secrets in the repo.** RPC credentials come from the environment
  (`scripts/dev.sh` sources the harness's `env.sh`); `config.example.yaml` carries placeholders
  only.
- **No hardcoded network constants.** Everything network-dependent flows from
  `chaincfg.Params` (see ARCHITECTURE §7).
- Handoff-prototype props (`showFeeEstimator`, `showStats`, `defaultDetail`) become constants in
  `web/src/appConfig.ts` rather than runtime props.

---

## 2. Repo layout (target state)

```
btcdwatch.com/
├── go.mod                        # module btcdwatch.com
├── Makefile                      # build / test / dev targets
├── README.md
├── .gitignore
├── config.example.yaml           # placeholder creds only
├── docs/
│   ├── ARCHITECTURE.md
│   └── IMPLEMENTATION_PLAN.md
├── scripts/
│   └── dev.sh                    # sources ../btc-regtest-env/scripts/env.sh → BTCDWATCH_* env → go run
├── cmd/
│   └── btcdwatchd/main.go
├── internal/
│   ├── config/config.go          # YAML + BTCDWATCH_* env overrides
│   ├── chain/params.go           # network name → chaincfg.Params; halving math; ClassifyQuery()
│   ├── node/client.go            # rpcclient ws wrapper behind node.Backend interface (mockable)
│   ├── explorer/
│   │   ├── tx.go        tx_test.go
│   │   ├── address.go   address_test.go
│   │   ├── mempool.go   mempool_test.go     # shared snapshot
│   │   ├── fees.go      fees_test.go
│   │   ├── stats.go     stats_test.go
│   │   └── examples.go
│   ├── price/price.go            # CoinGecko fetch + cache + static fallback
│   └── api/
│       ├── server.go  handlers.go  ws.go  hub.go  static.go
└── web/                          # Vite + React + TypeScript
    ├── embed.go                  # //go:embed all:dist ; dist/.keep committed
    ├── index.html                # Google Fonts links
    ├── vite.config.ts            # /api proxy (ws: true) in dev
    └── src/
        ├── main.tsx  App.tsx  appConfig.ts
        ├── api/client.ts  api/types.ts  api/ws.ts
        ├── styles/tokens.css  global.css  animations.css
        ├── lib/format.ts                         # + format.test.ts (vitest)
        ├── hooks/useSearch.ts  useLiveStats.ts  useWatchTx.ts  useCopy.ts
        ├── components/                           # Header, Footer, Toast, LiveStatusPill,
        │                                         # SearchBar, ExampleChips, FeeEstimator,
        │                                         # StatsBar, NodeCta, StatusCard, AmountDisplay,
        │                                         # FromTo, StatTile, ConfirmationProgress,
        │                                         # DetailToggle, DetailTable, CopyButton,
        │                                         # WatchPanel, ActivityList, Skeleton
        └── views/Landing.tsx  LoadingView.tsx  NotFound.tsx
                 ConfirmedTx.tsx  PendingTx.tsx  AddressView.tsx
```

- `styles/tokens.css` carries the handoff README's full token set as CSS custom properties
  (colors, radii, shadows, fonts) plus the four keyframes (`bpFade`, `bpPulse`, `bpSpin`,
  `bpShimmer`); `prefers-reduced-motion` disables pulse/shimmer.
- **Dev**: Go API on `:8480`, Vite on `:5173` with proxy
  `{ '/api': { target: 'http://127.0.0.1:8480', ws: true } }`.
- **Prod**: `make build` = `npm run build` then `go build` → single binary serving the SPA from
  the `web` embed with SPA fallback; `--static-dir` override flag.

---

## 3. Milestones

### M1 — Backend skeleton + transaction lookup

**Scope**: `go.mod`, `Makefile`, `.gitignore`, `config.example.yaml`, `scripts/dev.sh`,
`cmd/btcdwatchd/main.go`, `internal/config`, `internal/chain` (params + `ClassifyQuery`),
`internal/node` (websocket rpcclient behind `node.Backend`; HTTP-mode-free), and the first three
endpoints: `GET /api/healthz`, `GET /api/tx/{txid}` (full derivation: fee via prevout fetches +
LRU, from/to heuristics, amount, coinbase handling, block height/time via `getblockheader`),
`GET /api/search`. Pending-position fields may return `null` until M2's mempool snapshot lands
if sequencing demands it, but target is to include `internal/explorer/mempool.go` here since
`/api/tx` needs it for pending txs.

**Verification**

1. `scripts/dev.sh` starts the server against the running regtest node; `curl
   localhost:8480/api/healthz` → `nodeConnected: true` with the current height.
2. Grab a fresh txid from `txgen.sh` output, `curl localhost:8480/api/tx/<txid>` → `status:
   "pending"` with fee, feerate, from/to populated.
3. Run `miner.sh` for one block; re-curl → `status: "confirmed"`, `confirmations: 1`, block
   height/time present. Cross-check `feeSats` against manual btcctl math (Σ prevouts − Σ outs).
4. `curl '…/api/search?q=<txid>'`, `?q=<garbage>`, `?q=<mainnet bc1 addr>` → `tx` / `invalid` /
   `invalid` respectively.
5. `go test ./...` green (unit tests below).

### M2 — Frontend shell + landing + transaction views

**Scope**: Vite + React + TS scaffold, `tokens.css`/`global.css`/`animations.css`, fonts,
`api/client.ts` + `types.ts`, `App.tsx` view reducer
(`{view, query, result, detail, justConfirmed, watching}`), `useSearch` (min-400 ms loading
state to avoid flash), Header/Footer/Toast, Landing (LiveStatusPill, SearchBar, ExampleChips,
FeeEstimator, StatsBar, NodeCta), LoadingView, NotFound (network-appropriate example prefix —
`bcrt1` on regtest), ConfirmedTx and PendingTx views with Simple/Detailed toggle, 6-segment
ConfirmationProgress, CopyButton. Backend side: `GET /api/fees` (percentiles + floors),
`GET /api/stats` (REST-only for now; avg-interval from recent headers), `GET /api/examples`,
`internal/price`.

**Verification**

1. `scripts/dev.sh` + `npm run dev`; landing renders with live height/mempool/price and real
   example chips.
2. Paste a `txgen.sh` txid → pending view with queue/ETA panel; re-search after `miner.sh`
   mines → confirmed view.
3. Fee estimator shows three tiers; with the mempool drained (mine everything), tiers fall back
   to configured floors.
4. Visual comparison of every view against `BlockPeek.dc.html` side by side.
5. `tsc -b` and `vitest` green.

### M3 — Address view

**Scope**: `GET /api/address/{addr}` (`searchrawtransactions` + `includePrevOut`, direction/net
math, totals with `max_scan_txs` cap + `approximate`, short-TTL cache invalidated on block,
per-hash header-time cache), AddressView (balance header, received/sent/count StatTiles,
ActivityList with pagination), address branch in `/api/search` and `useSearch`.

**Verification**

1. Pick an address from txgen churn; UI totals match a manual
   `btcctl searchrawtransactions <addr>` tally.
2. Pagination pages through activity; mempool entries appear as pending rows.
3. Send to the address while the page is open, re-search → count/balance move as expected.

### M4 — Live layer (WebSocket + notifications)

**Scope**: btcd notification wiring (`NotifyBlocks`, `NotifyNewTransactions(true)`, dual block
handlers deduped by height, `OnTxAcceptedVerbose` → snapshot dirty flag, re-register in
`OnClientConnected`), hub + `/api/ws` (`ws.go`, `hub.go`), stats push, watch/unwatch, 10 s
pending ticker, frontend `ws.ts` singleton (backoff 1 s→30 s, watch-replay on reopen),
`useLiveStats` + `useWatchTx`, WatchPanel, celebration flow (WS flip → full `/api/tx` refetch →
ConfirmedTx with 🎉 banner + "1 of 6" progress), live StatsBar.

**Verification**

1. Open a pending tx, hit **Watch**, run `miner.sh` once → view flips to Confirmed with the
   celebration banner, **no refresh**, within the block interval.
2. Landing stats bar ticks up on each mined block without reload.
3. Kill and restart btcd → backend reconnects and re-registers (watch a tx across the restart
   and confirm it still flips); kill the Go server → frontend WS reconnects with backoff and
   replays the watch.
4. Two browser tabs watching different txs each receive only their own `tx` messages.

### M5 — Polish + single-binary production build

**Scope**: `web/embed.go` + SPA fallback + `--static-dir`, `?q=` sync via
`history.replaceState`, copy-to-clipboard toasts, `prefers-reduced-motion` behavior,
empty-mempool floor path exercised, consistent error states (node down → friendly banner),
Skeleton loading states, README (quickstart, config reference, E2E recipe), final `Makefile`
targets (`build`, `test`, `dev`). First feature commit / PR into the existing repo (the repo
already exists — no `git init` needed).

**Verification**

1. `make build`; stop Vite; run the single binary → all six views work served from the embed;
   deep-link `http://…/?q=<txid>` cold-loads into the right view.
2. `make test` = `go test ./...` + `tsc -b` + `vitest`, all green.
3. Walk the full E2E recipe (below) on the production binary.

---

## 4. Testing strategy

**Go unit tests** run against a mocked `node.Backend` fed with btcjson fixtures — no live node
needed in CI:

- **Tx derivation** (`explorer/tx_test.go`): fee & from/to & amount for a 2-in/2-out payment
  with change; coinbase (null fee, "newly minted", amount = all outputs); self-send fallback;
  non-standard script placeholder; prevout LRU hit behavior.
- **Fees** (`explorer/fees_test.go`): vsize-weighted percentile math; floor clamping;
  monotonicity enforcement; empty mempool → floors + `source:"floor"`.
- **Address** (`explorer/address_test.go`): direction classification
  (received/sent/self), net amounts, totals incl. mempool entries, `max_scan_txs` cap →
  `approximate: true`.
- **ClassifyQuery** (`chain/params_test.go`): 64-hex → tx; valid `bcrt1…` → address; mainnet
  `bc1…` rejected on regtest; garbage → invalid.
- **Halving math** (`chain`): boundary cases around multiples of
  `SubsidyReductionInterval` (150 on regtest).
- **Hub** (`api/hub_test.go`): watch/unwatch bookkeeping, fan-out targets only watchers,
  slow-client drop.

**Frontend**: vitest for `lib/format.ts` (`formatBtc`, `formatFiat`, `formatRelative`,
`formatEta`, `truncateMiddle`, `formatNumber`); `tsc -b` type-checks the whole app. Both wired
into `make test`.

**E2E recipe** (documented in the README, run against the regtest harness):

1. `txgen.sh` → copy the txid, paste into the UI → pending view appears.
2. Press **Watch**; run `miner.sh` → view flips to confirmed with the celebration banner within
   one block interval, no refresh.
3. Search an address seen in the churn → activity list grows as txgen keeps running.

---

## 5. Risks & mitigations

| # | Risk | Mitigation |
| --- | --- | --- |
| 1 | btcd lacks `estimatesmartfee`, `getmempoolentry`, `getblockstats`, and verbosity-2 prevouts | Percentile fees from `getrawmempool verbose`, shared mempool snapshot, explicit deduped prevout fetches — all verified available on the live 0.26 node (ARCHITECTURE §9). |
| 2 | Address balance is O(history) | Scan cap (`address.max_scan_txs`) + `approximate` flag + short-TTL cache invalidated per block. Fine on regtest; mainnet expectations (and the `addrindex=1` requirement) documented. |
| 3 | btcd script results use `addresses[]`; exotic scripts may yield none | Placeholder labels; never assume Core's singular `address` field; excluded from address accounting. |
| 4 | Block notifications differ across btcd versions/registration modes | Register both `OnFilteredBlockConnected` and legacy `OnBlockConnected`; dedupe by height. Re-register all notifications in `OnClientConnected` after reconnects. |
