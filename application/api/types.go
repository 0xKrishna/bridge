package api

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
