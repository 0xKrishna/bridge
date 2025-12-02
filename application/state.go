package application

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog/log"
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
		pendingKey := []byte("chain_" + string(rune(destChainID)))
		data, err := tx.GetOne(BridgePendingBucket, pendingKey)
		if err != nil {
			return err
		}

		if len(data) == 0 {
			return nil
		}

		var bridgeIDs []string
		if err := json.Unmarshal(data, &bridgeIDs); err != nil {
			return err
		}

		for _, bridgeID := range bridgeIDs {
			eventData, err := tx.GetOne(BridgeEventsBucket, []byte(bridgeID))
			if err != nil {
				log.Warn().Err(err).Str("bridgeId", bridgeID).Msg("Failed to get bridge event")
				continue
			}

			if len(eventData) == 0 {
				continue
			}

			var event BridgeEvent
			if err := json.Unmarshal(eventData, &event); err != nil {
				log.Warn().Err(err).Str("bridgeId", bridgeID).Msg("Failed to unmarshal bridge event")
				continue
			}

			events = append(events, event)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return events, nil
}

// GetBridgeStatus returns the status of a bridge event
func GetBridgeStatus(ctx context.Context, db kv.RoDB, bridgeID string) (string, error) {
	event, err := GetBridgeEvent(ctx, db, bridgeID)
	if err != nil {
		return "", err
	}

	return event.Status, nil
}

// IsBridgeClaimed checks if a bridge has been claimed
func IsBridgeClaimed(ctx context.Context, db kv.RoDB, bridgeID string) (bool, error) {
	err := db.View(ctx, func(tx kv.Tx) error {
		data, err := tx.GetOne(BridgeCompletedBucket, []byte(bridgeID))
		if err != nil {
			return err
		}

		if len(data) > 0 {
			return nil // Claimed
		}

		return ErrBridgeNotFound
	})

	if err == nil {
		return true, nil
	}

	if errors.Is(err, ErrBridgeNotFound) {
		return false, nil
	}

	return false, err
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
