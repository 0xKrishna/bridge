package application

import (
	"context"
	"encoding/json"

	"github.com/ledgerwatch/erigon-lib/kv"
)

// GetBridgeEvent retrieves a bridge event by ID
func GetBridgeEvent(ctx context.Context, db kv.RoDB, bridgeID string) (*BridgeEvent, error) {
	var event BridgeEvent

	err := db.View(ctx, func(tx kv.Tx) error {
		data, err := tx.GetOne(BridgeEventsBucket, []byte(bridgeID))
		if err != nil {
			return err
		}

		if len(data) == 0 {
			return ErrBridgeNotFound
		}

		return json.Unmarshal(data, &event)
	})

	if err != nil {
		return nil, err
	}

	return &event, nil
}

// GetPendingBridges retrieves all pending bridges for a destination chain
func GetPendingBridges(ctx context.Context, db kv.RoDB, destChainID uint64) ([]BridgeEvent, error) {
	var events []BridgeEvent

	err := db.View(ctx, func(tx kv.Tx) error {
		cursor, err := tx.Cursor(BridgeEventsBucket)
		if err != nil {
			return err
		}
		defer cursor.Close()

		for k, v, err := cursor.First(); k != nil; k, v, err = cursor.Next() {
			if err != nil {
				return err
			}

			var event BridgeEvent
			if err := json.Unmarshal(v, &event); err != nil {
				continue
			}

			// Filter by destination chain and pending status
			if event.DestChain == destChainID && event.Status == BridgeStatusConfirmed {
				events = append(events, event)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return events, nil
}

// IsBridgeClaimed checks if a bridge has been claimed
func IsBridgeClaimed(ctx context.Context, db kv.RoDB, bridgeID string) (bool, error) {
	event, err := GetBridgeEvent(ctx, db, bridgeID)
	if err != nil {
		return false, err
	}

	return event.Status == BridgeStatusCompleted, nil
}

// GetBridgeStats returns statistics about bridge events
func GetBridgeStats(ctx context.Context, db kv.RoDB) (map[string]interface{}, error) {
	stats := map[string]interface{}{
		"total":     0,
		"confirmed": 0,
		"completed": 0,
	}

	err := db.View(ctx, func(tx kv.Tx) error {
		cursor, err := tx.Cursor(BridgeEventsBucket)
		if err != nil {
			return err
		}
		defer cursor.Close()

		for k, v, err := cursor.First(); k != nil; k, v, err = cursor.Next() {
			if err != nil {
				return err
			}

			var event BridgeEvent
			if err := json.Unmarshal(v, &event); err != nil {
				continue
			}

			stats["total"] = stats["total"].(int) + 1

			switch event.Status {
			case BridgeStatusConfirmed:
				stats["confirmed"] = stats["confirmed"].(int) + 1
			case BridgeStatusCompleted:
				stats["completed"] = stats["completed"].(int) + 1
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return stats, nil
}
