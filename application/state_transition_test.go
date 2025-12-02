package application

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPayloadEncoding tests the critical binary payload encoding for bridge claims
func TestPayloadEncoding(t *testing.T) {
	tests := []struct {
		name        string
		bridgeID    string
		sourceChain uint64
		token       string
		amount      string
		recipient   string
	}{
		{
			name:        "Native POL - Stavanger to Sepolia",
			bridgeID:    "0x1234567890123456789012345678901234567890123456789012345678901234",
			sourceChain: 50591822,
			token:       "0x0000000000000000000000000000000000000000",
			amount:      "1000000000000000000", // 1 POL
			recipient:   "0xAbcdEF1234567890AbcdEF1234567890AbcdEF12",
		},
		{
			name:        "ERC20 POL - Sepolia to Stavanger",
			bridgeID:    "0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			sourceChain: 11155111,
			token:       "0x6a7c3f4b0651d6da389ad1d11d962ea458cdca70",
			amount:      "5000000000000000000", // 5 POL
			recipient:   "0x1234567890123456789012345678901234567890",
		},
		{
			name:        "Large amount (exceeds uint64)",
			bridgeID:    "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			sourceChain: 11155111,
			token:       "0x6a7c3f4b0651d6da389ad1d11d962ea458cdca70",
			amount:      "100000000000000000000000000", // 100M tokens - exceeds uint64
			recipient:   "0x1234567890123456789012345678901234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := createBridgePayload(tt.bridgeID, tt.sourceChain, tt.token, tt.amount, tt.recipient)
			require.NoError(t, err)
			require.Equal(t, 160, len(payload), "Payload must be exactly 160 bytes")

			// Verify bridgeID
			expectedBridgeID := common.HexToHash(tt.bridgeID)
			assert.Equal(t, expectedBridgeID.Bytes(), payload[0:32], "BridgeID mismatch")

			// Verify sourceChainID
			sourceChainBig := new(big.Int).SetBytes(payload[32:64])
			assert.Equal(t, tt.sourceChain, sourceChainBig.Uint64(), "SourceChainID mismatch")

			// Verify token (LEFT-aligned)
			mappedToken := mapTokenAddress(tt.token, tt.sourceChain)
			assert.Equal(t, mappedToken.Bytes(), payload[64:84], "Token address mismatch")
			assert.Equal(t, make([]byte, 12), payload[84:96], "Token padding should be zeros")

			// Verify amount
			expectedAmount, _ := new(big.Int).SetString(tt.amount, 10)
			actualAmount := new(big.Int).SetBytes(payload[96:128])
			assert.Equal(t, expectedAmount.String(), actualAmount.String(), "Amount mismatch")

			// Verify recipient (LEFT-aligned)
			expectedRecipient := common.HexToAddress(tt.recipient)
			assert.Equal(t, expectedRecipient.Bytes(), payload[128:148], "Recipient mismatch")
			assert.Equal(t, make([]byte, 12), payload[148:160], "Recipient padding should be zeros")

			t.Logf("Payload (hex): %s", hex.EncodeToString(payload))
		})
	}
}

// TestAddressEncodingCritical tests the critical LEFT-aligned address encoding
func TestAddressEncodingCritical(t *testing.T) {
	testAddr := "0xAbcdEF1234567890AbcdEF1234567890AbcdEF12"
	addr := common.HexToAddress(testAddr)

	t.Run("Correct LEFT-aligned encoding", func(t *testing.T) {
		encoded := make([]byte, 32)
		copy(encoded[0:20], addr.Bytes())

		assert.Equal(t, addr.Bytes(), encoded[0:20], "Address should be in first 20 bytes")

		zeros := make([]byte, 12)
		assert.Equal(t, zeros, encoded[20:32], "Last 12 bytes should be zeros")

		// Simulate Solidity shr(96, x)
		shifted := new(big.Int).SetBytes(encoded)
		shifted.Rsh(shifted, 96)

		extractedAddr := common.BytesToAddress(shifted.Bytes())
		assert.Equal(t, addr, extractedAddr, "Address extraction via shr(96) MUST work")
	})

	t.Run("Wrong RIGHT-aligned encoding (the bug we fixed)", func(t *testing.T) {
		encoded := common.LeftPadBytes(addr.Bytes(), 32)

		zeros := make([]byte, 12)
		assert.Equal(t, zeros, encoded[0:12], "First 12 bytes are zeros (wrong!)")

		shifted := new(big.Int).SetBytes(encoded)
		shifted.Rsh(shifted, 96)

		extractedAddr := common.BytesToAddress(shifted.Bytes())
		assert.NotEqual(t, addr, extractedAddr, "This proves the old encoding was WRONG")
		t.Logf("Original: %s, Extracted with wrong encoding: %s", addr.Hex(), extractedAddr.Hex())
	})
}

// TestTokenMapping tests the token address mapping between chains
func TestTokenMapping(t *testing.T) {
	t.Run("Sepolia POL ERC20 -> Stavanger native", func(t *testing.T) {
		sepoliaPOL := "0x6a7c3f4b0651d6da389ad1d11d962ea458cdca70"
		mapped := mapTokenAddress(sepoliaPOL, 11155111) // Sepolia
		assert.Equal(t, common.Address{}, mapped, "Should map to native token (address(0))")
	})

	t.Run("Stavanger native -> Sepolia POL ERC20", func(t *testing.T) {
		nativeToken := "0x0000000000000000000000000000000000000000"
		mapped := mapTokenAddress(nativeToken, 50591822) // Stavanger
		expectedPOL := common.HexToAddress("0x6a7c3f4b0651d6da389ad1d11d962ea458cdca70")
		assert.Equal(t, expectedPOL, mapped, "Should map to Sepolia POL ERC20")
	})

	t.Run("Unknown token passes through", func(t *testing.T) {
		unknownToken := "0x1234567890123456789012345678901234567890"
		mapped := mapTokenAddress(unknownToken, 11155111)
		expected := common.HexToAddress(unknownToken)
		assert.Equal(t, expected, mapped, "Unknown token should pass through unchanged")
	})
}

// TestBridgeEventStatus tests the status constants
func TestBridgeEventStatus(t *testing.T) {
	assert.Equal(t, "Confirmed", BridgeStatusConfirmed)
	assert.Equal(t, "Completed", BridgeStatusCompleted)
}
