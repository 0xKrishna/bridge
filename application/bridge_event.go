package application

// BridgeEvent represents a cross-chain bridge event from external chains.
// This is NOT an AppTransaction - it's an external event we store for record-keeping.
type BridgeEvent struct {
	BridgeID    string `json:"bridgeId"`    // Unique bridge identifier from contract event
	SourceChain uint64 `json:"sourceChain"` // Source chain ID
	DestChain   uint64 `json:"destChain"`   // Destination chain ID
	Token       string `json:"token"`       // Token address (address(0) for native)
	Amount      uint64 `json:"amount"`      // Amount bridged
	Sender      string `json:"sender"`      // Original sender address
	Recipient   string `json:"recipient"`   // Recipient on destination chain
	Status      string `json:"status"`      // Event status: Confirmed, Completed
	SourceTxHash string `json:"sourceTxHash,omitempty"` // Tx hash on source chain (initiation)
	ClaimTxHash  string `json:"claimTxHash,omitempty"`  // Tx hash on dest chain (claim)
}

// BridgeEvent status constants
const (
	BridgeStatusConfirmed = "Confirmed" // Event received and external tx generated
	BridgeStatusCompleted = "Completed" // Claimed on destination chain
)
