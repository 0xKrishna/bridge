package application

import "github.com/ledgerwatch/erigon-lib/kv"

const (
	BridgeEventsBucket   = "bridge_events"   // bridgeId -> BridgeEvent
	BridgePendingBucket  = "bridge_pending"  // destChainId -> []bridgeId
	BridgeCompletedBucket = "bridge_completed" // bridgeId -> completion timestamp
)

func Tables() kv.TableCfg {
	return kv.TableCfg{
		BridgeEventsBucket:   {},
		BridgePendingBucket:  {},
		BridgeCompletedBucket: {},
	}
}
