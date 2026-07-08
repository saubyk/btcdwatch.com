package explorer

import (
	"testing"

	"github.com/btcsuite/btcd/btcjson"
)

// TestGetTxInputsOutputs covers the detail-card rows: one row per input
// and output with full address + amount, change flagged on outputs.
func TestGetTxInputsOutputs(t *testing.T) {
	m := newMockBackend()
	p1, p2 := hexID("ioparent1"), hexID("ioparent2")
	installParent(m, p1, vout(0, 0.04, addrA1))
	installParent(m, p2, vout(0, 0.02, addrA2))

	txid := hexID("iotx")
	m.txs[txid] = &btcjson.TxRawResult{
		Txid:  txid,
		Vsize: 208,
		Vin:   []btcjson.Vin{vin(p1, 0), vin(p2, 0)},
		Vout: []btcjson.Vout{
			vout(0, 0.0425, addrB1),    // payment
			vout(1, 0.0174874, addrA1), // change back to a sender address
		},
	}
	confirm(m, m.txs[txid], 4, 1735000000)
	installChain(m, 4, 60)

	tx, err := newTestService(m).GetTx(txid)
	if err != nil {
		t.Fatal(err)
	}

	if len(tx.Inputs) != 2 {
		t.Fatalf("inputs = %d, want 2", len(tx.Inputs))
	}
	if tx.Inputs[0].Address != addrA1 || tx.Inputs[0].AmountSats != 400_0000 {
		t.Errorf("input 0 = %+v", tx.Inputs[0])
	}

	if len(tx.Outputs) != 2 {
		t.Fatalf("outputs = %d, want 2", len(tx.Outputs))
	}
	pay, chg := tx.Outputs[0], tx.Outputs[1]
	if pay.Address != addrB1 || pay.Change || pay.AmountSats != 425_0000 {
		t.Errorf("payment row = %+v", pay)
	}
	if chg.Address != addrA1 || !chg.Change || chg.AmountSats != 174_8740 {
		t.Errorf("change row = %+v", chg)
	}
}

// TestGetTxInputsOutputsSelfSend: when every output returns to the
// sender, the headline counts them all — the rows must not contradict it
// with change flags.
func TestGetTxInputsOutputsSelfSend(t *testing.T) {
	m := newMockBackend()
	p := hexID("selfparent")
	installParent(m, p, vout(0, 1.0, addrA1))

	txid := hexID("selftx")
	m.txs[txid] = &btcjson.TxRawResult{
		Txid:  txid,
		Vsize: 141,
		Vin:   []btcjson.Vin{vin(p, 0)},
		Vout:  []btcjson.Vout{vout(0, 0.999, addrA1)},
	}
	confirm(m, m.txs[txid], 2, 1735000000)
	installChain(m, 2, 60)

	tx, err := newTestService(m).GetTx(txid)
	if err != nil {
		t.Fatal(err)
	}
	if tx.AmountSats != 9990_0000 {
		t.Errorf("amount = %d, want full self-send", tx.AmountSats)
	}
	if len(tx.Outputs) != 1 || tx.Outputs[0].Change {
		t.Errorf("self-send outputs = %+v, want change unflagged", tx.Outputs)
	}
}

// TestGetTxInputsOutputsCoinbase: no input rows; outputs never flagged
// change.
func TestGetTxInputsOutputsCoinbase(t *testing.T) {
	m := newMockBackend()
	txid := hexID("iocb")
	m.txs[txid] = &btcjson.TxRawResult{
		Txid:  txid,
		Vsize: 100,
		Vin:   []btcjson.Vin{{Coinbase: "abcd"}},
		Vout:  []btcjson.Vout{vout(0, 50, addrB1)},
	}
	confirm(m, m.txs[txid], 2, 1735000000)
	installChain(m, 2, 60)

	tx, err := newTestService(m).GetTx(txid)
	if err != nil {
		t.Fatal(err)
	}
	if len(tx.Inputs) != 0 {
		t.Errorf("coinbase inputs = %+v, want none", tx.Inputs)
	}
	if len(tx.Outputs) != 1 || tx.Outputs[0].Change {
		t.Errorf("coinbase outputs = %+v", tx.Outputs)
	}
}
