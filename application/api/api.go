package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/0xAtelerix/sdk/gosdk/rpc"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/example/application"
)

type CustomRPC struct {
	rpcServer *rpc.StandardRPCServer
	db        kv.RoDB
}

func NewCustomRPC(rpcServer *rpc.StandardRPCServer, db kv.RoDB) *CustomRPC {
	return &CustomRPC{
		rpcServer: rpcServer,
		db:        db,
	}
}

func (c *CustomRPC) AddRPCMethods() {
	c.rpcServer.AddMethod("getBridgeStatus", c.GetBridgeStatus)
}

// GetBridgeStatus retrieves the status of a bridge event
func (c *CustomRPC) GetBridgeStatus(ctx context.Context, params []any) (any, error) {
	if len(params) == 0 {
		return nil, application.ErrMissingParameters
	}

	paramBytes, err := json.Marshal(params[0])
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameter: %w", err)
	}

	var req GetBridgeStatusRequest

	if unmarshalErr := json.Unmarshal(paramBytes, &req); unmarshalErr != nil {
		return nil, fmt.Errorf("invalid parameters: %w", unmarshalErr)
	}

	if req.BridgeID == "" {
		return nil, application.ErrInvalidBridgeID
	}

	event, err := application.GetBridgeEvent(ctx, c.db, req.BridgeID)
	if err != nil {
		if errors.Is(err, application.ErrBridgeNotFound) {
			return nil, application.ErrBridgeNotFound
		}

		return nil, fmt.Errorf("failed to get bridge status: %w", err)
	}

	return GetBridgeStatusResponse{
		BridgeID:     req.BridgeID,
		Status:       event.Status,
		Claimed:      event.Status == application.BridgeStatusCompleted,
		SourceTxHash: event.SourceTxHash,
		ClaimTxHash:  event.ClaimTxHash,
	}, nil
}
