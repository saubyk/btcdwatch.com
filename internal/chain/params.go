// Package chain provides the network abstraction: mapping config network
// names to chain parameters, query classification, and halving math. All
// network-dependent constants flow from chaincfg.Params — nothing is
// hardcoded per network.
package chain

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/address/v2"
	"github.com/btcsuite/btcd/chaincfg/v2"
)

// ParamsForNetwork maps a config network name to its chain parameters.
func ParamsForNetwork(name string) (*chaincfg.Params, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "mainnet":
		return &chaincfg.MainNetParams, nil
	case "testnet", "testnet3":
		return &chaincfg.TestNet3Params, nil
	case "regtest":
		return &chaincfg.RegressionNetParams, nil
	case "signet":
		return &chaincfg.SigNetParams, nil
	case "simnet":
		return &chaincfg.SimNetParams, nil
	}
	return nil, fmt.Errorf("unknown network %q", name)
}

// QueryKind is the classification of a raw search query.
type QueryKind string

const (
	QueryTx      QueryKind = "tx"
	QueryAddress QueryKind = "address"
	QueryInvalid QueryKind = "invalid"
)

// Query is the result of classifying a raw search string.
type Query struct {
	Kind QueryKind

	// Txid is the normalized (lowercase) transaction id when Kind ==
	// QueryTx.
	Txid string

	// Address is the decoded address when Kind == QueryAddress. It is
	// guaranteed to be valid for the network it was classified against.
	Address address.Address
}

// ClassifyQuery decides whether a raw search string is a txid, an address
// valid for the given network, or neither. The backend owns classification
// so that address validation is always network-correct (a mainnet bc1
// address is rejected on regtest, whose bech32 HRP is different).
func ClassifyQuery(raw string, params *chaincfg.Params) Query {
	q := strings.TrimSpace(raw)

	if len(q) == 64 {
		if _, err := hex.DecodeString(q); err == nil {
			return Query{Kind: QueryTx, Txid: strings.ToLower(q)}
		}
	}

	if addr, err := address.DecodeAddress(q, params); err == nil &&
		addr.IsForNet(params) {

		return Query{Kind: QueryAddress, Address: addr}
	}

	return Query{Kind: QueryInvalid}
}

// BlocksUntilHalving returns how many blocks remain after the given height
// until the next subsidy halving. Halvings occur at heights that are
// multiples of params.SubsidyReductionInterval.
func BlocksUntilHalving(height int64, params *chaincfg.Params) int64 {
	interval := int64(params.SubsidyReductionInterval)
	return interval - height%interval
}
