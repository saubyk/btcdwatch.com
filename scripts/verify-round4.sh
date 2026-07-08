#!/usr/bin/env bash
# Live verification of round 4 (inputs/outputs + address links) against
# the regtest harness on :8480.
set -u

API="${BTCDWATCH_URL:-http://127.0.0.1:8480}"
pass=0 fail=0

check() { # check <exit-code> <label> <detail>
  if [ "$1" = 0 ]; then pass=$((pass + 1)); echo "  ✓ $2  $3"
  else fail=$((fail + 1)); echo "  ✗ $2  $3"; fi
}

get() { # get <url> <dot.path>
  curl -sf "$1" | python3 -c "
import sys, json, functools
try:
    d = json.load(sys.stdin)
    v = functools.reduce(
        lambda o, k: o[int(k)] if isinstance(o, list) else o[k],
        '$2'.split('.'), d)
    print(json.dumps(v) if isinstance(v, (dict, list, bool)) else v)
except Exception:
    pass" 2>/dev/null
}

tip=$(get "$API/api/stats" blockHeight)
h=$((tip - 1))
txid=$(get "$API/api/block/$h" txs.1.txid)
[ -n "$txid" ] || txid=$(get "$API/api/block/$h" txs.0.txid)

echo "— tx inputs/outputs payload —"
nin=$(get "$API/api/tx/$txid" inputs | python3 -c 'import sys,json;print(len(json.load(sys.stdin)))' 2>/dev/null)
nout=$(get "$API/api/tx/$txid" outputs | python3 -c 'import sys,json;print(len(json.load(sys.stdin)))' 2>/dev/null)
[ -n "$nin" ] && [ "$nin" -ge 1 ] && [ -n "$nout" ] && [ "$nout" -ge 1 ] 2>/dev/null
check $? "inputs/outputs rows present" "in=$nin out=$nout"

addr0=$(get "$API/api/tx/$txid" inputs.0.address)
amt0=$(get "$API/api/tx/$txid" inputs.0.amountSats)
[ -n "$addr0" ] && [ -n "$amt0" ]; check $? "row fields" "addr=${addr0:0:16}… sats=$amt0"

# Fee identity: sum(inputs) − sum(outputs) == feeSats.
python3 - "$API" "$txid" <<'PYEOF'
import json, sys, urllib.request
d = json.load(urllib.request.urlopen(f"{sys.argv[1]}/api/tx/{sys.argv[2]}"))
if d.get("isCoinbase"):
    print("  (sampled the coinbase — fee identity skipped)"); sys.exit(0)
fee = sum(i["amountSats"] for i in d["inputs"]) - sum(o["amountSats"] for o in d["outputs"])
ok = fee == (d["feeSats"] or -1)
print(f"  {'✓' if ok else '✗'} inputs − outputs = feeSats  ({fee} vs {d['feeSats']})")
sys.exit(0 if ok else 1)
PYEOF
check $? "fee identity" ""

cb=$(get "$API/api/block/$h" txs.0.txid)
cbin=$(get "$API/api/tx/$cb" inputs | python3 -c 'import sys,json;print(len(json.load(sys.stdin)))' 2>/dev/null)
[ "$cbin" = 0 ]; check $? "coinbase has no input rows" "in=$cbin"

echo
echo "RESULT: $pass passed, $fail failed"
echo
echo "Manual UI spot-checks at $API:"
echo "  1. Landing order: hero → dark stats bar → 'The line right now' → node CTA."
echo "  2. Confirmed tx, Detailed tab: Inputs/Outputs card under the detail table — full"
echo "     addresses as orange underlined links, amounts right, 'recipient' / 'change — back"
echo "     to sender' chips, and the 'Inputs − outputs = … the fee, kept by the miner' footer."
echo "  3. Pending tx: 'RECEIVING ADDRESS — CHECK IT MATCHES YOURS' block under the chips with"
echo "     the full address as a link + working copy button."
echo "  4. from→to line on both views: truncated addresses are links; clicking any address"
echo "     (including IO-card rows) opens that address's view through the loading skeleton."