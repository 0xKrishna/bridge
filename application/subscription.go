package application

import (
	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/library"
	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog/log"
)

// Event signatures for subscription
const (
	BridgeInitiatedSignature = "BridgeInitiated(bytes32,uint256,uint256,address,uint256,address,address)"
	AssetClaimedSignature    = "AssetClaimed(bytes32,address,address,uint256)"
)

// SubscribeBridgeContracts registers the bridge contracts and events with the subscriber
// so that ProcessBlock receives blocks containing events from these contracts.
func SubscribeBridgeContracts(subscriber *gosdk.Subscriber) {
	sepoliaContract := common.HexToAddress(BridgeContractAddressSepolia)
	stavangerContract := common.HexToAddress(BridgeContractAddressStavanger)

	// Subscribe to bridge events on Sepolia
	subscriber.SubscribeEthContract(
		library.EthereumSepoliaChainID,
		library.EthereumAddress(sepoliaContract),
		nil,
		library.EventTopic(BridgeInitiatedSignature),
		library.EventTopic(AssetClaimedSignature),
	)

	// Subscribe to bridge events on Stavanger
	subscriber.SubscribeEthContract(
		library.StavangerTestnetChainID,
		library.EthereumAddress(stavangerContract),
		nil,
		library.EventTopic(BridgeInitiatedSignature),
		library.EventTopic(AssetClaimedSignature),
	)

	log.Info().
		Str("sepolia", BridgeContractAddressSepolia).
		Str("stavanger", BridgeContractAddressStavanger).
		Msg("Subscribed to bridge contracts and events")
}
