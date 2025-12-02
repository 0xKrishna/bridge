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
		amount      uint64
		recipient   string
	}{
		{
			name:        "Native POL - Stavanger to Sepolia",
			bridgeID:    "0x1234567890123456789012345678901234567890123456789012345678901234",
			sourceChain: 50591822, // Stavanger
			token:       "0x0000000000000000000000000000000000000000",
			amount:      1000000000000000000, // 1 POL
			recipient:   "0xAbcdEF1234567890AbcdEF1234567890AbcdEF12",
		},
		{
			name:        "ERC20 POL - Sepolia to Stavanger",
			bridgeID:    "0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			sourceChain: 11155111, // Sepolia
			token:       "0x6a7c3f4b0651d6da389ad1d11d962ea458cdca70",
			amount:      5000000000000000000, // 5 POL
			recipient:   "0x1234567890123456789012345678901234567890",
		},
		{
			name:        "Zero address recipient should still encode",
			bridgeID:    "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			sourceChain: 1,
			token:       "0x0000000000000000000000000000000000000000",
			amount:      1,
			recipient:   "0x0000000000000000000000000000000000000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate payload using the actual function
			payload, err := createBridgePayload(tt.bridgeID, tt.sourceChain, tt.token, tt.amount, tt.recipient)
			require.NoError(t, err)

			// Verify payload length
			require.Equal(t, 160, len(payload), "Payload must be exactly 160 bytes")

			// Decode and verify each field
			t.Run("BridgeID", func(t *testing.T) {
				bridgeIDBytes := payload[0:32]
				expectedBridgeID := common.HexToHash(tt.bridgeID)
				assert.Equal(t, expectedBridgeID.Bytes(), bridgeIDBytes, "BridgeID mismatch")
			})

			t.Run("SourceChainID", func(t *testing.T) {
				sourceChainBytes := payload[32:64]
				sourceChainBig := new(big.Int).SetBytes(sourceChainBytes)
				assert.Equal(t, tt.sourceChain, sourceChainBig.Uint64(), "SourceChainID mismatch")
			})

			t.Run("Token - LEFT aligned", func(t *testing.T) {
				tokenBytes := payload[64:96]

				// Token is mapped, so get the mapped token for verification
				mappedToken := mapTokenAddress(tt.token, tt.sourceChain)

				// First 20 bytes should be the address
				actualTokenBytes := tokenBytes[0:20]
				assert.Equal(t, mappedToken.Bytes(), actualTokenBytes, "Token address mismatch")

				// Last 12 bytes should be zeros
				zeros := make([]byte, 12)
				assert.Equal(t, zeros, tokenBytes[20:32], "Token padding should be zeros")

				// Verify Solidity shr(96,...) extraction would work
				shifted := new(big.Int).SetBytes(tokenBytes)
				shifted.Rsh(shifted, 96)
				extractedAddr := common.BytesToAddress(shifted.Bytes())
				assert.Equal(t, mappedToken, extractedAddr, "Solidity shr(96) extraction would fail!")
			})

			t.Run("Amount", func(t *testing.T) {
				amountBytes := payload[96:128]
				amountBig := new(big.Int).SetBytes(amountBytes)
				assert.Equal(t, tt.amount, amountBig.Uint64(), "Amount mismatch")
			})

			t.Run("Recipient - LEFT aligned", func(t *testing.T) {
				recipientBytes := payload[128:160]
				expectedRecipient := common.HexToAddress(tt.recipient)

				// First 20 bytes should be the address
				actualRecipientBytes := recipientBytes[0:20]
				assert.Equal(t, expectedRecipient.Bytes(), actualRecipientBytes, "Recipient address mismatch")

				// Last 12 bytes should be zeros
				zeros := make([]byte, 12)
				assert.Equal(t, zeros, recipientBytes[20:32], "Recipient padding should be zeros")

				// Verify Solidity shr(96,...) extraction would work
				shifted := new(big.Int).SetBytes(recipientBytes)
				shifted.Rsh(shifted, 96)
				extractedAddr := common.BytesToAddress(shifted.Bytes())
				assert.Equal(t, expectedRecipient, extractedAddr, "Solidity shr(96) extraction would fail!")
			})

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
		assert.Equal(t, common.Address{}, extractedAddr, "Would extract zero address!")
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
