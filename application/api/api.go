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
	c.rpcServer.AddMethod("getBridgeEvent", c.GetBridgeEvent)
	c.rpcServer.AddMethod("getBridgeStatus", c.GetBridgeStatus)
	c.rpcServer.AddMethod("listPendingBridges", c.ListPendingBridges)
	c.rpcServer.AddMethod("getBridgeStats", c.GetBridgeStats)
}

// GetBridgeEvent retrieves a bridge event by ID
func (c *CustomRPC) GetBridgeEvent(ctx context.Context, params []any) (any, error) {
	if len(params) == 0 {
		return nil, application.ErrMissingParameters
	}

	paramBytes, err := json.Marshal(params[0])
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameter: %w", err)
	}

	var req GetBridgeEventRequest
	if err := json.Unmarshal(paramBytes, &req); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if req.BridgeID == "" {
		return nil, application.ErrInvalidBridgeID
	}

	event, err := application.GetBridgeEvent(ctx, c.db, req.BridgeID)
	if err != nil {
		if errors.Is(err, application.ErrBridgeNotFound) {
			return nil, fmt.Errorf("bridge not found: %s", req.BridgeID)
		}
		return nil, fmt.Errorf("failed to get bridge event: %w", err)
	}

	return GetBridgeEventResponse{
		BridgeID:     event.BridgeID,
		SourceChain:  event.SourceChain,
		DestChain:    event.DestChain,
		Token:        event.Token,
		Amount:       event.Amount,
		Sender:       event.Sender,
		Recipient:    event.Recipient,
		Status:       event.Status,
		SourceTxHash: event.SourceTxHash,
		ClaimTxHash:  event.ClaimTxHash,
	}, nil
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
	if err := json.Unmarshal(paramBytes, &req); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if req.BridgeID == "" {
		return nil, application.ErrInvalidBridgeID
	}

	event, err := application.GetBridgeEvent(ctx, c.db, req.BridgeID)
	if err != nil {
		if errors.Is(err, application.ErrBridgeNotFound) {
			return nil, fmt.Errorf("bridge not found: %s", req.BridgeID)
		}
		return nil, fmt.Errorf("failed to get bridge status: %w", err)
	}

	isClaimed, _ := application.IsBridgeClaimed(ctx, c.db, req.BridgeID)

	return GetBridgeStatusResponse{
		BridgeID:     req.BridgeID,
		Status:       event.Status,
		Claimed:      isClaimed,
		SourceTxHash: event.SourceTxHash,
		ClaimTxHash:  event.ClaimTxHash,
	}, nil
}

// ListPendingBridges retrieves all pending bridges for a destination chain
func (c *CustomRPC) ListPendingBridges(ctx context.Context, params []any) (any, error) {
	if len(params) == 0 {
		return nil, application.ErrMissingParameters
	}

	paramBytes, err := json.Marshal(params[0])
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameter: %w", err)
	}

	var req ListPendingBridgesRequest
	if err := json.Unmarshal(paramBytes, &req); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if req.DestChainID == 0 {
		return nil, application.ErrInvalidChainID
	}

	events, err := application.GetPendingBridges(ctx, c.db, req.DestChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending bridges: %w", err)
	}

	var bridgeList []BridgeSummary
	for _, event := range events {
		bridgeList = append(bridgeList, BridgeSummary{
			BridgeID:    event.BridgeID,
			SourceChain: event.SourceChain,
			DestChain:   event.DestChain,
			Token:       event.Token,
			Amount:      event.Amount,
			Recipient:   event.Recipient,
			Status:      event.Status,
		})
	}

	return ListPendingBridgesResponse{
		DestChainID: req.DestChainID,
		Count:       len(bridgeList),
		Bridges:     bridgeList,
	}, nil
}

// GetBridgeStats retrieves statistics about bridge events
func (c *CustomRPC) GetBridgeStats(ctx context.Context, _ []any) (any, error) {
	stats, err := application.GetBridgeStats(ctx, c.db)
	if err != nil {
		return nil, fmt.Errorf("failed to get bridge stats: %w", err)
	}

	return stats, nil
}

// Request/Response Types

type GetBridgeEventRequest struct {
	BridgeID string `json:"bridgeId"`
}

type GetBridgeEventResponse struct {
	BridgeID     string `json:"bridgeId"`
	SourceChain  uint64 `json:"sourceChain"`
	DestChain    uint64 `json:"destChain"`
	Token        string `json:"token"`
	Amount       string `json:"amount"`
	Sender       string `json:"sender"`
	Recipient    string `json:"recipient"`
	Status       string `json:"status"`
	SourceTxHash string `json:"sourceTxHash,omitempty"`
	ClaimTxHash  string `json:"claimTxHash,omitempty"`
}

type GetBridgeStatusRequest struct {
	BridgeID string `json:"bridgeId"`
}

type GetBridgeStatusResponse struct {
	BridgeID     string `json:"bridgeId"`
	Status       string `json:"status"`
	Claimed      bool   `json:"claimed"`
	SourceTxHash string `json:"sourceTxHash,omitempty"`
	ClaimTxHash  string `json:"claimTxHash,omitempty"`
}

type ListPendingBridgesRequest struct {
	DestChainID uint64 `json:"destChainId"`
}

type BridgeSummary struct {
	BridgeID    string `json:"bridgeId"`
	SourceChain uint64 `json:"sourceChain"`
	DestChain   uint64 `json:"destChain"`
	Token       string `json:"token"`
	Amount      string `json:"amount"`
	Recipient   string `json:"recipient"`
	Status      string `json:"status"`
}

type ListPendingBridgesResponse struct {
	DestChainID uint64          `json:"destChainId"`
	Count       int             `json:"count"`
	Bridges     []BridgeSummary `json:"bridges"`
}
