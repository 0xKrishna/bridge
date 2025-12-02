package application

import (
	"context"
	"encoding/json"
	"math/big"
	"strings"
	"time"

	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/evmtypes"
	"github.com/0xAtelerix/sdk/gosdk/external"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog/log"
)

const (
	// Bridge contract addresses (deployed and verified)
	BridgeContractAddressSepolia   = "0x844E740Ea7F404c6208fd85Ee6114a14F8037df7" // Sepolia L1
	BridgeContractAddressStavanger = "0x3C1c8351a09DB0300786148B56EcB7be2FaA322e" // Stavanger L2

	// Event signatures
	// BridgeInitiated(bytes32 indexed bridgeId, uint256 indexed sourceChainId, uint256 indexed destChainId,
	//                 address token, uint256 amount, address sender, address recipient)
	BridgeInitiatedSignature = "0xa43a2e0bb4454dc2f20f4a34be7549f0e1b00e4f5e88805c729900e40471a0cb"

	// AssetClaimed(bytes32 indexed bridgeId, address indexed recipient, address indexed token, uint256 amount)
	AssetClaimedSignature = "0x464f754c10c6c0416dd32b200e8ad0a2c492ef1eb66a3c80e0eb9a66c14850c9"

	// Event ABIs
	bridgeInitiatedEventABI = `[{"anonymous":false,"inputs":[` +
		`{"indexed":true,"internalType":"bytes32","name":"bridgeId","type":"bytes32"},` +
		`{"indexed":true,"internalType":"uint256","name":"sourceChainId","type":"uint256"},` +
		`{"indexed":true,"internalType":"uint256","name":"destChainId","type":"uint256"},` +
		`{"indexed":false,"internalType":"address","name":"token","type":"address"},` +
		`{"indexed":false,"internalType":"uint256","name":"amount","type":"uint256"},` +
		`{"indexed":false,"internalType":"address","name":"sender","type":"address"},` +
		`{"indexed":false,"internalType":"address","name":"recipient","type":"address"}],` +
		`"name":"BridgeInitiated","type":"event"}]`
)

var (
	_ gosdk.StateTransitionSimplified                       = &StateTransition{}
	_ gosdk.StateTransitionInterface[Transaction, Receipt] = gosdk.BatchProcesser[Transaction, Receipt]{}
)

type StateTransition struct {
	msa *gosdk.MultichainStateAccessSQL
}

func NewStateTransition(msa *gosdk.MultichainStateAccessSQL) *StateTransition {
	return &StateTransition{
		msa: msa,
	}
}

// ProcessBlock processes external chain blocks (EVM chains only)
func (st *StateTransition) ProcessBlock(
	b apptypes.ExternalBlock,
	tx kv.RwTx,
) ([]apptypes.ExternalTransaction, error) {
	if !gosdk.IsEvmChain(apptypes.ChainType(b.ChainID)) {
		log.Warn().Uint64("chainID", b.ChainID).Msg("Unsupported chain type, skipping...")
		return nil, nil
	}

	return st.processEVMBlock(b, tx)
}

func (st *StateTransition) processEVMBlock(
	b apptypes.ExternalBlock,
	dbtx kv.RwTx,
) ([]apptypes.ExternalTransaction, error) {
	var externalTxs []apptypes.ExternalTransaction

	block, err := st.msa.EVMBlock(context.Background(), b)
	if err != nil {
		return nil, err
	}

	receipts, err := st.msa.EVMReceipts(context.Background(), b)
	if err != nil {
		return nil, err
	}

	for _, r := range receipts {
		extTxs := st.processReceipt(dbtx, r, b.ChainID)
		if len(extTxs) > 0 {
			externalTxs = append(externalTxs, extTxs...)
		}
	}

	log.Info().
		Uint64("chainID", b.ChainID).
		Uint64("blockNumber", b.BlockNumber).
		Int("transactions", len(block.Transactions)).
		Int("receipts", len(receipts)).
		Msg("EVM External block")

	return externalTxs, nil
}

// processReceipt handles Bridge events from the external chain
func (*StateTransition) processReceipt(
	dbtx kv.RwTx,
	r evmtypes.Receipt,
	chainID uint64,
) []apptypes.ExternalTransaction {
	var externalTxs []apptypes.ExternalTransaction

	for _, vlog := range r.Logs {
		// Check if this log is from our Bridge contracts
		bridgeAddresses := []string{
			BridgeContractAddressSepolia,
			BridgeContractAddressStavanger,
		}

		isBridgeContract := false
		for _, addr := range bridgeAddresses {
			if vlog.Address == common.HexToAddress(addr) {
				isBridgeContract = true
				break
			}
		}

		if !isBridgeContract || len(vlog.Topics) == 0 {
			continue
		}

		switch vlog.Topics[0].Hex() {
		case BridgeInitiatedSignature:
			// Decode BridgeInitiated event
			bridgeEvent, err := decodeBridgeInitiatedEvent(vlog)
			if err != nil {
				log.Error().Err(err).Msg("Failed to decode BridgeInitiated event")
				continue
			}

			if bridgeEvent.SourceChain != chainID {
				log.Warn().
					Str("bridgeId", bridgeEvent.BridgeID).
					Uint64("eventSourceChain", bridgeEvent.SourceChain).
					Uint64("expectedSourceChain", chainID).
					Msg("Source chain ID mismatch in BridgeInitiated event, skipping...")
				continue
			}

			log.Info().
				Str("bridgeId", bridgeEvent.BridgeID).
				Uint64("sourceChain", bridgeEvent.SourceChain).
				Uint64("destChain", bridgeEvent.DestChain).
				Str("token", bridgeEvent.Token).
				Uint64("amount", bridgeEvent.Amount).
				Str("sender", bridgeEvent.Sender).
				Str("recipient", bridgeEvent.Recipient).
				Msg("Received BridgeInitiated event from external chain")

			// Store bridge event in database
			err = storeBridgeEvent(dbtx, bridgeEvent)
			if err != nil {
				log.Error().Err(err).Msg("Failed to store bridge event")
				continue
			}

			// Generate ExternalTransaction to mint tokens on destination chain
			extTx, err := createMintTransaction(
				bridgeEvent.DestChain,
				bridgeEvent.BridgeID,
				bridgeEvent.SourceChain,
				bridgeEvent.Token,
				bridgeEvent.Amount,
				bridgeEvent.Recipient,
			)
			if err != nil {
				log.Error().Err(err).Msg("Failed to create mint transaction")
				continue
			}

			log.Info().
				Uint64("destChain", bridgeEvent.DestChain).
				Str("bridgeId", bridgeEvent.BridgeID).
				Msg("Generated ExternalTransaction for destination chain")

			externalTxs = append(externalTxs, extTx)

		case AssetClaimedSignature:
			// Decode AssetClaimed event - marks bridge as completed
			if len(vlog.Topics) < 2 {
				log.Error().Msg("AssetClaimed event missing bridgeId topic")
				continue
			}

			bridgeID := vlog.Topics[1].Hex()
			claimTxHash := vlog.TxHash.Hex()

			log.Info().
				Str("bridgeId", bridgeID).
				Str("claimTxHash", claimTxHash).
				Uint64("chainId", chainID).
				Msg("Received AssetClaimed event - marking bridge as completed")

			// Mark bridge as completed
			err := markBridgeCompleted(dbtx, bridgeID, claimTxHash)
			if err != nil {
				log.Error().Err(err).Str("bridgeId", bridgeID).Msg("Failed to mark bridge as completed")
				continue
			}

		default:
			log.Debug().Msgf("Unhandled event signature: %s", vlog.Topics[0].Hex())
		}
	}

	return externalTxs
}

// BridgeInitiatedEvent represents the decoded BridgeInitiated event
type BridgeInitiatedEvent struct {
	BridgeID    string
	SourceChain uint64
	DestChain   uint64
	Token       string
	Amount      uint64
	Sender      string
	Recipient   string
	TxHash      string // Source chain tx hash
}

// decodeBridgeInitiatedEvent decodes a BridgeInitiated event using ABI
func decodeBridgeInitiatedEvent(vlog *types.Log) (*BridgeInitiatedEvent, error) {
	// Parse the ABI
	parsedABI, err := abi.JSON(strings.NewReader(bridgeInitiatedEventABI))
	if err != nil {
		return nil, err
	}

	// Unpack the event data (non-indexed parameters)
	var eventData struct {
		Token     common.Address
		Amount    *big.Int
		Sender    common.Address
		Recipient common.Address
	}

	err = parsedABI.UnpackIntoInterface(&eventData, "BridgeInitiated", vlog.Data)
	if err != nil {
		return nil, err
	}

	if len(vlog.Topics) < 4 {
		return nil, Error("insufficient topics in BridgeInitiated event")
	}

	bridgeID := vlog.Topics[1].Hex()
	sourceChain := vlog.Topics[2].Big().Uint64()
	destChain := vlog.Topics[3].Big().Uint64()

	return &BridgeInitiatedEvent{
		BridgeID:    bridgeID,
		SourceChain: sourceChain,
		DestChain:   destChain,
		Token:       eventData.Token.Hex(),
		Amount:      eventData.Amount.Uint64(),
		Sender:      eventData.Sender.Hex(),
		Recipient:   eventData.Recipient.Hex(),
		TxHash:      vlog.TxHash.Hex(),
	}, nil
}

// createMintTransaction generates an ExternalTransaction for minting tokens on the destination chain
func createMintTransaction(
	destChainID uint64,
	bridgeID string,
	sourceChainID uint64,
	tokenAddress string,
	amount uint64,
	recipientAddress string,
) (apptypes.ExternalTransaction, error) {
	// Create the 160-byte payload for Bridge.executeTransaction()
	payload, err := createBridgePayload(bridgeID, sourceChainID, tokenAddress, amount, recipientAddress)
	if err != nil {
		return apptypes.ExternalTransaction{}, err
	}

	log.Info().
		Uint64("destChain", destChainID).
		Str("bridgeId", bridgeID).
		Uint64("sourceChain", sourceChainID).
		Str("token", tokenAddress).
		Uint64("amount", amount).
		Str("recipient", recipientAddress).
		Msg("Creating external transaction for bridge claim")

	// Use the SDK's ExternalTransaction builder
	// This payload will be sent to Pelagos.sol, which will call Bridge.executeTransaction(payload)
	extTx, err := external.NewExTxBuilder(payload, apptypes.ChainType(destChainID)).Build()
	if err != nil {
		log.Error().Err(err).
			Uint64("destChain", destChainID).
			Msg("Failed to build external transaction")
		return apptypes.ExternalTransaction{}, err
	}

	log.Info().
		Uint64("destChain", destChainID).
		Str("bridgeId", bridgeID).
		Int("payloadSize", len(payload)).
		Msg("Successfully created external transaction")

	return extTx, nil
}

// createBridgePayload creates the 160-byte payload for Bridge.executeTransaction()
// Payload structure:
// - Bytes 0-31:    bridgeId (bytes32)
// - Bytes 32-63:   sourceChainId (uint256)
// - Bytes 64-95:   token (address, LEFT-aligned: [20_bytes_address][12_bytes_zeros])
// - Bytes 96-127:  amount (uint256)
// - Bytes 128-159: recipient (address, LEFT-aligned: [20_bytes_address][12_bytes_zeros])
func createBridgePayload(
	bridgeID string,
	sourceChainID uint64,
	tokenAddress string,
	amount uint64,
	recipientAddress string,
) ([]byte, error) {
	payload := make([]byte, 160)

	// Parse bridgeId (bytes32)
	bridgeIDHash := common.HexToHash(bridgeID)
	copy(payload[0:32], bridgeIDHash[:])

	// Parse sourceChainId (uint256)
	sourceChainBig := new(big.Int).SetUint64(sourceChainID)
	sourceChainBytes := sourceChainBig.FillBytes(make([]byte, 32))
	copy(payload[32:64], sourceChainBytes)

	// Map token address for cross-chain compatibility
	// On Stavanger (50591822), POL is the native token (address(0))
	// On Sepolia (11155111), POL is an ERC20 token (0x6a7c...)
	mappedTokenAddr := mapTokenAddress(tokenAddress, sourceChainID)

	// Parse token address (LEFT-aligned)
	copy(payload[64:84], mappedTokenAddr[:]) // Copy 20 bytes starting at position 64
	// Bytes 84-95 remain zeros (LEFT-aligned padding)

	// Parse amount (uint256)
	amountBig := new(big.Int).SetUint64(amount)
	amountBytes := amountBig.FillBytes(make([]byte, 32))
	copy(payload[96:128], amountBytes)

	// Parse recipient address (LEFT-aligned)
	recipientAddr := common.HexToAddress(recipientAddress)
	copy(payload[128:148], recipientAddr[:]) // Copy 20 bytes starting at position 128
	// Bytes 148-159 remain zeros (LEFT-aligned padding)

	return payload, nil
}

// mapTokenAddress converts token addresses for cross-chain compatibility
// Returns the appropriate token address for the destination chain
func mapTokenAddress(tokenAddress string, sourceChainID uint64) common.Address {
	// POL token address on Sepolia
	const sepoliaPOL = "0x6a7c3f4b0651d6da389ad1d11d962ea458cdca70"

	// Normalize address for comparison
	normalizedToken := strings.ToLower(tokenAddress)
	normalizedSepoliaPOL := strings.ToLower(sepoliaPOL)
	zeroAddress := strings.ToLower("0x0000000000000000000000000000000000000000")

	// Sepolia → Stavanger: Convert Sepolia POL ERC20 to native POL
	if normalizedToken == normalizedSepoliaPOL && sourceChainID == uint64(gosdk.EthereumSepoliaChainID) {
		log.Info().
			Str("originalToken", tokenAddress).
			Str("mappedToken", "0x0000000000000000000000000000000000000000").
			Uint64("sourceChain", sourceChainID).
			Msg("Mapping Sepolia POL ERC20 to Stavanger native POL")
		return common.Address{} // address(0) for native token
	}

	// Stavanger → Sepolia: Convert native POL to Sepolia POL ERC20
	if normalizedToken == zeroAddress && sourceChainID == uint64(gosdk.StavangerTestnetChainID) {
		log.Info().
			Str("originalToken", tokenAddress).
			Str("mappedToken", sepoliaPOL).
			Uint64("sourceChain", sourceChainID).
			Msg("Mapping Stavanger native POL to Sepolia POL ERC20")
		return common.HexToAddress(sepoliaPOL)
	}

	// Default: return original address
	return common.HexToAddress(tokenAddress)
}

// storeBridgeEvent stores a bridge event in the database
func storeBridgeEvent(dbtx kv.RwTx, bridgeEvent *BridgeInitiatedEvent) error {
	bridgeKey := []byte(bridgeEvent.BridgeID)

	// Check if already exists
	existing, err := dbtx.GetOne(BridgeEventsBucket, bridgeKey)
	if err != nil {
		return err
	}

	if len(existing) > 0 {
		log.Info().Str("bridgeId", bridgeEvent.BridgeID).Msg("Bridge event already exists, skipping")
		return nil
	}

	// Create BridgeEvent
	event := BridgeEvent{
		BridgeID:     bridgeEvent.BridgeID,
		SourceChain:  bridgeEvent.SourceChain,
		DestChain:    bridgeEvent.DestChain,
		Token:        bridgeEvent.Token,
		Amount:       bridgeEvent.Amount,
		Sender:       bridgeEvent.Sender,
		Recipient:    bridgeEvent.Recipient,
		Status:       BridgeStatusConfirmed,
		SourceTxHash: bridgeEvent.TxHash,
	}

	// Store event
	eventData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	if err := dbtx.Put(BridgeEventsBucket, bridgeKey, eventData); err != nil {
		return err
	}

	// Add to pending list for destination chain
	pendingKey := []byte("chain_" + string(rune(event.DestChain)))
	var pendingBridges []string

	existingPending, err := dbtx.GetOne(BridgePendingBucket, pendingKey)
	if err != nil {
		return err
	}

	if len(existingPending) > 0 {
		if err := json.Unmarshal(existingPending, &pendingBridges); err != nil {
			return err
		}
	}

	pendingBridges = append(pendingBridges, event.BridgeID)
	pendingData, err := json.Marshal(pendingBridges)
	if err != nil {
		return err
	}

	if err := dbtx.Put(BridgePendingBucket, pendingKey, pendingData); err != nil {
		return err
	}

	log.Info().
		Str("bridgeId", event.BridgeID).
		Uint64("sourceChain", event.SourceChain).
		Uint64("destChain", event.DestChain).
		Msg("Stored bridge event")

	return nil
}

// markBridgeCompleted updates a bridge event status to Completed
// Called when AssetClaimed event is detected on the destination chain
func markBridgeCompleted(dbtx kv.RwTx, bridgeID, claimTxHash string) error {
	bridgeKey := []byte(bridgeID)

	// Get existing bridge event
	eventData, err := dbtx.GetOne(BridgeEventsBucket, bridgeKey)
	if err != nil {
		return err
	}

	if len(eventData) == 0 {
		log.Warn().Str("bridgeId", bridgeID).Msg("Bridge event not found for completion")
		return nil // Not an error - might be a bridge we didn't track
	}

	var event BridgeEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return err
	}

	// Update status to Completed and store claim tx hash
	event.Status = BridgeStatusCompleted
	event.ClaimTxHash = claimTxHash

	updatedData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	if err := dbtx.Put(BridgeEventsBucket, bridgeKey, updatedData); err != nil {
		return err
	}

	// Remove from pending bucket
	pendingKey := []byte("chain_" + string(rune(event.DestChain)))
	existingPending, err := dbtx.GetOne(BridgePendingBucket, pendingKey)
	if err != nil {
		return err
	}

	if len(existingPending) > 0 {
		var pendingBridges []string
		if err := json.Unmarshal(existingPending, &pendingBridges); err != nil {
			return err
		}

		// Remove this bridgeID from pending list
		var updatedPending []string
		for _, id := range pendingBridges {
			if id != bridgeID {
				updatedPending = append(updatedPending, id)
			}
		}

		pendingData, err := json.Marshal(updatedPending)
		if err != nil {
			return err
		}

		if err := dbtx.Put(BridgePendingBucket, pendingKey, pendingData); err != nil {
			return err
		}
	}

	// Add to completed bucket with timestamp
	timestamp := []byte(time.Now().Format(time.RFC3339))
	if err := dbtx.Put(BridgeCompletedBucket, bridgeKey, timestamp); err != nil {
		return err
	}

	log.Info().
		Str("bridgeId", bridgeID).
		Msg("Bridge marked as completed")

	return nil
}
