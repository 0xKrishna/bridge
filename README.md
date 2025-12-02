# Pelagos Bridge

> A production-ready cross-chain bridge enabling seamless token transfers between L1 and L2 networks using the Pelagos SDK.

[![Status](https://img.shields.io/badge/status-production%20ready-brightgreen)]()
[![License](https://img.shields.io/badge/license-MIT-blue)]()

## Overview

Pelagos Bridge is a cross-chain bridge that enables fast, secure token transfers between Ethereum L1 (Sepolia) and custom L2 networks (Stavanger). Built on the Pelagos SDK, it provides a simple, efficient bridging solution with validator-based consensus.

### Key Features

- ✅ **Bidirectional bridging** - L1 ↔ L2 token transfers
- ✅ **One-transaction bridging** - EIP-2612 permit support (gasless approval)
- ✅ **Automatic token mapping** - Seamless ERC20 ↔ native token conversion
- ✅ **Fast finality** - No ZK proof generation, validator-based consensus
- ✅ **Production frontend** - Modern UI with MetaMask integration
- ✅ **Event-driven architecture** - Real-time bridge transaction processing

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Bridge Appchain                          │
│  • Monitors external chain events (pelacli)                 │
│  • Processes BridgeInitiated events                         │
│  • Generates ExternalTransactions for destination           │
│  • Coordinates validator consensus                          │
└─────────────────┬───────────────────────────────────────────┘
                  │
    ┌─────────────┼─────────────┬
    │             │             │
┌───▼────┐  ┌────▼───┐  ┌──────▼──┐
│Sepolia │  │Stavanger│  │ Future  │
│ L1     │  │  L2    │  │ Chains  │
│Bridge  │  │ Bridge │  │ ...     │
└────────┘  └────────┘  └─────────┘
```

### How It Works

1. **User initiates bridge** - Calls `bridgeAsset()` on source chain
2. **Token locked** - Tokens locked in source Bridge contract
3. **Event emitted** - `BridgeInitiated` event with bridge details
4. **Appchain processes** - Detects event, maps token addresses
5. **ExternalTransaction created** - Generates mint transaction for destination
6. **Destination mints** - Bridge contract releases/mints tokens to recipient

## Deployed Contracts

| Network | Chain ID | Bridge Contract | Explorer |
|---------|----------|----------------|----------|
| Sepolia (L1) | 11155111 | `0x844E740Ea7F404c6208fd85Ee6114a14F8037df7` | [Etherscan](https://sepolia.etherscan.io/address/0x844E740Ea7F404c6208fd85Ee6114a14F8037df7) |
| Stavanger (L2) | 50591822 | `0x3C1c8351a09DB0300786148B56EcB7be2FaA322e` | [Blockscout](https://explorer.stavanger.gateway.fm/address/0x3C1c8351a09DB0300786148B56EcB7be2FaA322e) |

## Quick Start

### Prerequisites

- Go 1.21+
- Docker & Docker Compose
- Node.js 18+ (for frontend)

### Running the Bridge

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd bridge
   ```

2. **Configure environment**
   ```bash
   # Update config files with your RPC keys
   vim config/consensus_chains.json  # Add your RPC API keys
   vim config/ext_networks.json      # Add network credentials
   ```

3. **Start the bridge stack**
   ```bash
   docker compose up -d
   ```

4. **Start the frontend** (optional)
   ```bash
   cd frontend
   python3 -m http.server 8000
   # Open http://localhost:8000
   ```

### Testing the Bridge

#### Using the Frontend (Recommended)

1. Open http://localhost:8000
2. Connect MetaMask
3. Select source and destination networks
4. Enter amount and recipient
5. Click "Bridge with Permit" (one transaction) or "Bridge" (two transactions)

#### Using Direct Contract Calls

```javascript
// Bridge from Sepolia to Stavanger
const tx = await bridgeContract.bridgeAsset(
    token,         // Token address (or address(0) for native)
    amount,        // Amount in wei
    50591822,      // Destination chain ID (Stavanger)
    recipient,     // Recipient address
    permitData     // EIP-2612 permit data (or "0x" for pre-approved)
);
```

## Project Structure

```
bridge/
├── application/          # Core bridge logic
│   ├── state_transition.go   # Event processing & ExternalTransaction generation
│   ├── bridge_event.go       # Bridge event data types
│   ├── state.go              # Database query functions
│   └── api/                  # JSON-RPC API endpoints
├── cmd/                  # Application entry point
│   └── main.go
├── config/              # Configuration files
│   ├── chain_data.json
│   ├── consensus_chains.json
│   └── ext_networks.json
├── contracts/           # Solidity contracts
│   └── contracts/
│       └── Bridge.sol
└── frontend/            # Web UI
    ├── index.html
    ├── app.js
    └── styles.css
```

## Token Mapping

The bridge automatically maps token addresses between chains:

| Source Chain | Source Token | Destination Chain | Destination Token |
|--------------|--------------|-------------------|-------------------|
| Sepolia (L1) | POL ERC20 (`0x6a7c...`) | Stavanger (L2) | Native POL (`address(0)`) |
| Stavanger (L2) | Native POL (`address(0)`) | Sepolia (L1) | POL ERC20 (`0x6a7c...`) |

Token mapping logic is configurable in `application/state_transition.go:mapTokenAddress()`.

## Configuration

### Chain Data (`config/chain_data.json`)

Maps chain IDs to local MDBX database paths where external chain data is stored.

```json
{
  "11155111": "/multichain/sepolia",
  "26400": "/multichain/stavanger"
}
```

### Consensus Chains (`config/consensus_chains.json`)

Configures which chains pelacli monitors for bridge events.

```json
[
  {
    "ChainID": 11155111,
    "DBPath": "/multichain/sepolia",
    "APIKey": "YOUR_INFURA_KEY",
    "StartBlock": 9214937,
    "Contracts": {
      "Bridge": "0x844E740Ea7F404c6208fd85Ee6114a14F8037df7"
    }
  }
]
```

### External Networks (`config/ext_networks.json`)

Configures where pelacli sends ExternalTransactions.

```json
[
  {
    "chainId": 11155111,
    "rpcUrl": "https://sepolia.infura.io/v3/YOUR_KEY",
    "contractAddress": "0x844E740Ea7F404c6208fd85Ee6114a14F8037df7",
    "privateKey": "0x..."
  }
]
```

⚠️ **Security:** Never commit real private keys. Use environment variables or secret management.

## API Endpoints

The bridge exposes a JSON-RPC endpoint for querying bridge state:

### Bridge Methods
- `getBridgeStatus(bridgeId)` - Get bridge status and tx hashes

Example:
```bash
curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "getBridgeStatus",
    "params": [{"bridgeId": "0x123..."}],
    "id": 1
  }'
```

## Development

### Building

```bash
# Build the appchain
go build -o bridge ./cmd/main.go

# Run tests
go test ./...

# Run with race detection
go test -race ./...

# Lint code
golangci-lint run
```

### Adding New Chains

1. Deploy Bridge.sol to the new chain
2. Add chain config to `config/consensus_chains.json`
3. Add network config to `config/ext_networks.json`
4. Add chain data path to `config/chain_data.json`
5. Update token mappings in `state_transition.go` if needed
6. Restart the bridge appchain

### Adding New Tokens

1. Add token mapping logic in `application/state_transition.go:mapTokenAddress()`
2. Update frontend token selector (if using frontend)
3. Restart the bridge appchain

## Security Considerations

### Current Implementation

- ✅ **ReentrancyGuard** - Protects against reentrancy attacks
- ✅ **Pausable** - Emergency pause functionality
- ✅ **Access control** - Only Pelagos contract can call `executeTransaction()`
- ✅ **Balance checks** - Validates sufficient liquidity before claims
- ✅ **Event validation** - Verifies event signatures and contract addresses

### Production Recommendations

- 🔒 **Multi-sig validator** - Replace single-key validator with multi-sig
- 🔒 **Rate limiting** - Add per-address bridge limits
- 🔒 **Amount caps** - Set maximum bridge amounts
- 🔒 **Circuit breakers** - Automatic pause on anomalies
- 🔒 **Monitoring** - Set up alerts for large bridges
- 🔒 **Audit** - Get smart contracts audited before mainnet

## Monitoring & Logs

### Bridge Appchain Logs

```bash
docker compose logs -f appchain
```

Key log messages:
- `Received BridgeInitiated event from external chain` - Event detected
- `Mapping Sepolia POL ERC20 to Stavanger native POL` - Token mapping
- `Successfully created external transaction` - Ready to mint

### Pelacli Logs

```bash
docker compose logs -f pelacli
```

Key log messages:
- `Processing external block` - Monitoring external chains
- `Submitting external transaction` - Sending mint transaction

## Troubleshooting

### Bridge transaction not completing

1. Check pelacli is running: `docker compose ps`
2. Check RPC connectivity: Verify API keys in `config/consensus_chains.json`
3. Check logs: `docker compose logs pelacli | grep -i error`
4. Verify liquidity: Destination bridge must have sufficient tokens

### Event not detected

1. Verify event signature in `state_transition.go` matches contract
2. Check bridge contract address in `config/consensus_chains.json`
3. Verify pelacli is monitoring the correct chain
4. Check start block is before the bridge transaction

### Frontend connection issues

1. Verify MetaMask is installed
2. Check correct network is selected
3. Verify RPC URLs in frontend `app.js`
4. Check browser console for errors

## Documentation

- **[Pelagos SDK](https://github.com/0xAtelerix/sdk)** - Official Pelagos SDK documentation

## Future Enhancements

### Phase 2: TSS Integration
- Multi-party validator coordination
- Threshold signatures (e.g., 2/3 validators)
- No single point of failure

### Phase 3: Periodic Roots
- Merkle tree of bridge transactions
- Periodic root submissions for backup security
- Audit trail

### Phase 4: BridgeAndCall()
- Cross-chain execution with arbitrary calldata
- Bridge + swap/stake/mint in one transaction

### Phase 5: Advanced Features
- Native liquidity pools (USDC, USDT)
- Lock/mint for custom tokens
- Slashing and incentive mechanisms
- Circuit breakers and anomaly detection

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see LICENSE file for details

## Support

- **Issues:** [GitHub Issues](https://github.com/your-repo/issues)
- **Pelagos SDK:** [GitHub](https://github.com/0xAtelerix/sdk)

---

**Built with [Pelagos SDK](https://github.com/0xAtelerix/sdk) 🌊**
