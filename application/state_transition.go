package application

import (
	"context"
	"encoding/json"
	"math/big"
	"strings"

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
	// Bridge contract addresses
	BridgeContractAddressSepolia   = "0x844E740Ea7F404c6208fd85Ee6114a14F8037df7"
	BridgeContractAddressStavanger = "0x3C1c8351a09DB0300786148B56EcB7be2FaA322e"

	// Event signatures
	BridgeInitiatedSignature = "0xa43a2e0bb4454dc2f20f4a34be7549f0e1b00e4f5e88805c729900e40471a0cb"
	AssetClaimedSignature    = "0x260120404c049bd806f3d5d3444295a9eab7f94112fdec90a6072ad39acae708"
)

// Cached parsed ABI for BridgeInitiated event
//
//nolint:gochecknoglobals // Intentionally cached at package level for performance
var bridgeInitiatedABI abi.ABI

func init() {
	const eventABI = `[{"anonymous":false,"inputs":[` +
		`{"indexed":true,"internalType":"bytes32","name":"bridgeId","type":"bytes32"},` +
		`{"indexed":true,"internalType":"uint256","name":"sourceChainId","type":"uint256"},` +
		`{"indexed":true,"internalType":"uint256","name":"destChainId","type":"uint256"},` +
		`{"indexed":false,"internalType":"address","name":"token","type":"address"},` +
		`{"indexed":false,"internalType":"uint256","name":"amount","type":"uint256"},` +
		`{"indexed":false,"internalType":"address","name":"sender","type":"address"},` +
		`{"indexed":false,"internalType":"address","name":"recipient","type":"address"}],` +
		`"name":"BridgeInitiated","type":"event"}]`

	var err error

	bridgeInitiatedABI, err = abi.JSON(strings.NewReader(eventABI))
	if err != nil {
		panic("failed to parse BridgeInitiated ABI: " + err.Error())
	}
}

var (
	_ gosdk.StateTransitionSimplified                      = &StateTransition{}
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
			bridgeEvent, err := decodeBridgeInitiatedEvent(vlog)
			if err != nil {
				log.Error().Err(err).Msg("Failed to decode BridgeInitiated event")

				continue
			}

			// Validate source chain matches
			if bridgeEvent.SourceChain != chainID {
				log.Warn().
					Str("bridgeId", bridgeEvent.BridgeID).
					Msg("Source chain mismatch, skipping")

				continue
			}

			// Validate destination chain is supported
			if !isChainSupported(bridgeEvent.DestChain) {
				log.Warn().
					Str("bridgeId", bridgeEvent.BridgeID).
					Uint64("destChain", bridgeEvent.DestChain).
					Msg("Unsupported destination chain")

				continue
			}

			// Skip if already processed (handles restarts/reorgs)
			existing, err := dbtx.GetOne(BridgeEventsBucket, []byte(bridgeEvent.BridgeID))
			if err != nil {
				log.Error().Err(err).Msg("Failed to check existing bridge event")

				continue
			}

			if len(existing) > 0 {
				continue
			}

			if storeErr := storeBridgeEvent(dbtx, bridgeEvent); storeErr != nil {
				log.Error().Err(storeErr).Msg("Failed to store bridge event")

				continue
			}

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
				Str("bridgeId", bridgeEvent.BridgeID).
				Uint64("src", bridgeEvent.SourceChain).
				Uint64("dst", bridgeEvent.DestChain).
				Str("amount", bridgeEvent.Amount).
				Msg("Bridge event processed")

			externalTxs = append(externalTxs, extTx)

		case AssetClaimedSignature:
			if len(vlog.Topics) < 2 {
				continue
			}

			bridgeID := vlog.Topics[1].Hex()

			// Check if already completed
			existing, err := dbtx.GetOne(BridgeEventsBucket, []byte(bridgeID))
			if err != nil || len(existing) == 0 {
				continue
			}

			var event BridgeEvent
			if err := json.Unmarshal(existing, &event); err != nil {
				continue
			}

			if event.Status == BridgeStatusCompleted {
				continue
			}

			if err := markBridgeCompleted(dbtx, bridgeID, vlog.TxHash.Hex()); err != nil {
				log.Error().
					Err(err).
					Str("bridgeId", bridgeID).
					Msg("Failed to mark bridge completed")
			}

		default:
			// Ignore other events
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
	Amount      string // String to support amounts > uint64
	Sender      string
	Recipient   string
	TxHash      string
}

// decodeBridgeInitiatedEvent decodes a BridgeInitiated event
func decodeBridgeInitiatedEvent(vlog *types.Log) (*BridgeInitiatedEvent, error) {
	if len(vlog.Topics) < 4 {
		return nil, Error("insufficient topics in BridgeInitiated event")
	}

	var eventData struct {
		Token     common.Address
		Amount    *big.Int
		Sender    common.Address
		Recipient common.Address
	}

	if err := bridgeInitiatedABI.UnpackIntoInterface(&eventData, "BridgeInitiated", vlog.Data); err != nil {
		return nil, err
	}

	return &BridgeInitiatedEvent{
		BridgeID:    vlog.Topics[1].Hex(),
		SourceChain: vlog.Topics[2].Big().Uint64(),
		DestChain:   vlog.Topics[3].Big().Uint64(),
		Token:       eventData.Token.Hex(),
		Amount:      eventData.Amount.String(),
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
	amount string,
	recipientAddress string,
) (apptypes.ExternalTransaction, error) {
	payload, err := createBridgePayload(
		bridgeID,
		sourceChainID,
		tokenAddress,
		amount,
		recipientAddress,
	)
	if err != nil {
		return apptypes.ExternalTransaction{}, err
	}

	extTx, err := external.NewExTxBuilder(payload, apptypes.ChainType(destChainID)).Build()
	if err != nil {
		log.Error().
			Err(err).
			Uint64("destChain", destChainID).
			Msg("Failed to build external transaction")

		return apptypes.ExternalTransaction{}, err
	}

	return extTx, nil
}

// createBridgePayload creates the 160-byte payload for Bridge.executeTransaction()
// Layout: bridgeId(32) | sourceChainId(32) | token(32, left-aligned) | amount(32) | recipient(32, left-aligned)
func createBridgePayload(
	bridgeID string,
	sourceChainID uint64,
	tokenAddress string,
	amount string,
	recipientAddress string,
) ([]byte, error) {
	payload := make([]byte, 160)

	// bridgeId (bytes32)
	bridgeHash := common.HexToHash(bridgeID)
	copy(payload[0:32], bridgeHash[:])

	// sourceChainId (uint256)
	copy(payload[32:64], new(big.Int).SetUint64(sourceChainID).FillBytes(make([]byte, 32)))

	// token (address, LEFT-aligned)
	mappedToken := mapTokenAddress(tokenAddress, sourceChainID)
	copy(payload[64:84], mappedToken[:])

	// amount (uint256) - parse from string to support large values
	amountBig, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return nil, Error("invalid amount: " + amount)
	}

	copy(payload[96:128], amountBig.FillBytes(make([]byte, 32)))

	// recipient (address, LEFT-aligned)
	recipient := common.HexToAddress(recipientAddress)
	copy(payload[128:148], recipient[:])

	return payload, nil
}

// isChainSupported checks if a chain ID is supported by the bridge
func isChainSupported(chainID uint64) bool {
	return chainID == uint64(gosdk.EthereumSepoliaChainID) ||
		chainID == uint64(gosdk.StavangerTestnetChainID)
}

// mapTokenAddress converts token addresses for cross-chain compatibility
func mapTokenAddress(tokenAddress string, sourceChainID uint64) common.Address {
	const sepoliaPOL = "0x6a7c3f4b0651d6da389ad1d11d962ea458cdca70"

	normalizedToken := strings.ToLower(tokenAddress)

	// Sepolia → Stavanger: POL ERC20 to native
	if normalizedToken == strings.ToLower(sepoliaPOL) &&
		sourceChainID == uint64(gosdk.EthereumSepoliaChainID) {
		return common.Address{}
	}

	// Stavanger → Sepolia: native to POL ERC20
	if normalizedToken == "0x0000000000000000000000000000000000000000" &&
		sourceChainID == uint64(gosdk.StavangerTestnetChainID) {
		return common.HexToAddress(sepoliaPOL)
	}

	return common.HexToAddress(tokenAddress)
}

// storeBridgeEvent stores a bridge event in the database
func storeBridgeEvent(dbtx kv.RwTx, bridgeEvent *BridgeInitiatedEvent) error {
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

	eventData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return dbtx.Put(BridgeEventsBucket, []byte(bridgeEvent.BridgeID), eventData)
}

// markBridgeCompleted updates a bridge event status to Completed
func markBridgeCompleted(dbtx kv.RwTx, bridgeID, claimTxHash string) error {
	bridgeKey := []byte(bridgeID)

	eventData, err := dbtx.GetOne(BridgeEventsBucket, bridgeKey)
	if err != nil {
		return err
	}

	if len(eventData) == 0 {
		log.Warn().Str("bridgeId", bridgeID).Msg("Bridge event not found for completion")

		return nil
	}

	var event BridgeEvent

	if unmarshalErr := json.Unmarshal(eventData, &event); unmarshalErr != nil {
		return unmarshalErr
	}

	event.Status = BridgeStatusCompleted
	event.ClaimTxHash = claimTxHash

	updatedData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	if err := dbtx.Put(BridgeEventsBucket, bridgeKey, updatedData); err != nil {
		return err
	}

	log.Info().Str("bridgeId", bridgeID).Msg("Bridge marked as completed")

	return nil
}
