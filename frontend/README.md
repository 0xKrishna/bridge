# Pelagos Bridge Frontend

Modern, production-ready frontend for the Pelagos cross-chain bridge with EIP-2612 permit support.

## Features

✅ **One-Transaction Bridging** - Sign permit + bridge in one transaction (like AggLayer!)
✅ **Modern UI/UX** - Clean, responsive design
✅ **Network Switching** - Automatic network detection and switching
✅ **Real-time Balances** - Live balance updates
✅ **Transaction History** - Local storage of bridge transactions
✅ **Mobile Responsive** - Works on all devices
✅ **FAQ Section** - Built-in help for users

## Quick Start

### 1. Configure Bridge Addresses

Edit `app.js` and update the configuration:

```javascript
const CONFIG = {
    BRIDGE_SEPOLIA: '0x...', // Your deployed Sepolia bridge address
    BRIDGE_STAVANGER: '0x...', // Your deployed Stavanger bridge address
    POL_TOKEN_SEPOLIA: '0x6a7c3f4b0651d6da389ad1d11d962ea458cdca70',
    SEPOLIA_CHAIN_ID: 11155111,
    STAVANGER_CHAIN_ID: 50591822,
    SEPOLIA_RPC: 'https://eth-sepolia.g.alchemy.com/v2/YOUR_KEY',
    STAVANGER_RPC: 'https://stavanger-rpc.eu-north-2.gateway.fm',
};
```

### 2. Deploy

#### Option A: Simple HTTP Server (Development)

```bash
# Using Python
python3 -m http.server 8000

# Using Node.js
npx http-server -p 8000

# Using PHP
php -S localhost:8000
```

Then open http://localhost:8000

#### Option B: Nginx (Production)

```nginx
server {
    listen 80;
    server_name bridge.yourdomain.com;

    root /path/to/bridge/frontend;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    # Enable gzip
    gzip on;
    gzip_types text/css application/javascript application/json;

    # Cache static assets
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
```

#### Option C: Vercel/Netlify (Easy Deploy)

1. Push to GitHub
2. Connect to Vercel/Netlify
3. Deploy!

**Vercel:**
```bash
vercel --prod
```

**Netlify:**
```bash
netlify deploy --prod --dir=.
```

### 3. Test

Open the frontend and test:

1. **Connect Wallet** - Should show MetaMask popup
2. **Select Networks** - Choose Sepolia → Stavanger
3. **Enter Amount** - Try bridging 1 POL
4. **Sign Permit** - MetaMask shows signature request (free!)
5. **Bridge** - One transaction submitted
6. **Check History** - Transaction appears in History tab

## File Structure

```
frontend/
├── index.html          # Main HTML
├── styles.css          # All styles
├── app.js              # Bridge logic + permit signatures
└── README.md           # This file
```

## User Flow

### Permit-Based Bridge (Recommended)

1. User connects wallet (MetaMask/WalletConnect)
2. User selects source and destination networks
3. User enters amount and recipient
4. User clicks "Bridge with Permit"
5. **MetaMask pops up with "Sign Permit" message** ← Free, no gas!
6. User signs the permit
7. **MetaMask pops up with transaction** ← One transaction
8. User confirms transaction
9. Bridge completes in 2-5 minutes
10. Done! ✅

### Traditional Bridge (Fallback)

If permit fails (token doesn't support EIP-2612):

1. User clicks "Bridge"
2. **MetaMask pops up with "Approve" transaction** ← First tx
3. User confirms approval
4. **MetaMask pops up with "Bridge" transaction** ← Second tx
5. User confirms bridge
6. Bridge completes
7. Done (but took 2x longer)

## Key Features Explained

### EIP-2612 Permit Integration

The frontend implements gasless approval via permit signatures:

```javascript
// 1. Get permit signature (off-chain, no gas!)
const { v, r, s } = await getPermitSignature(
    tokenAddress,
    bridgeAddress,
    amount,
    deadline
);

// 2. Encode permit data
const permitData = ethers.utils.defaultAbiCoder.encode(
    ['address', 'address', 'uint256', 'uint256', 'uint8', 'bytes32', 'bytes32'],
    [owner, spender, value, deadline, v, r, s]
);

// 3. Submit bridge transaction (includes permit!)
await bridge.bridgeAsset(token, amount, destChain, recipient, permitData);
```

### Network Auto-Switching

Automatically switches to the correct network:

```javascript
await window.ethereum.request({
    method: 'wallet_switchEthereumChain',
    params: [{ chainId: '0x' + chainId.toString(16) }],
});
```

If Stavanger isn't added, it adds it automatically:

```javascript
await window.ethereum.request({
    method: 'wallet_addEthereumChain',
    params: [{
        chainId: '0x303552e',
        chainName: 'Stavanger',
        nativeCurrency: { name: 'POL', symbol: 'POL', decimals: 18 },
        rpcUrls: ['https://stavanger-rpc.eu-north-2.gateway.fm'],
    }]
});
```

### Real-time Balance Updates

Fetches balances from both networks:

```javascript
async function updateBalances() {
    // From network
    const fromProvider = new ethers.providers.JsonRpcProvider(fromNetwork.rpcUrl);
    const fromBalance = await fromProvider.getBalance(account);

    // To network
    const toProvider = new ethers.providers.JsonRpcProvider(toNetwork.rpcUrl);
    const toBalance = await toProvider.getBalance(account);

    // Update UI
    document.getElementById('fromBalance').textContent =
        `Balance: ${ethers.utils.formatEther(fromBalance)} POL`;
}
```

### Transaction History

Stores transactions in localStorage:

```javascript
function saveTransaction(tx) {
    const history = JSON.parse(localStorage.getItem('bridgeHistory') || '[]');
    history.unshift(tx);
    localStorage.setItem('bridgeHistory', JSON.stringify(history.slice(0, 50)));
}
```

## Customization

### Change Colors

Edit `styles.css`:

```css
:root {
    --primary: #8247e5;          /* Main brand color */
    --primary-dark: #6b3ac4;     /* Darker variant */
    --success: #10b981;           /* Success green */
    --error: #ef4444;             /* Error red */
}
```

### Change Network Icons

Update `NETWORKS` in `app.js`:

```javascript
const NETWORKS = {
    sepolia: {
        name: 'Sepolia',
        icon: 'https://your-cdn.com/sepolia-icon.svg',
        // ...
    }
};
```

### Add More Networks

Extend `NETWORKS` object:

```javascript
const NETWORKS = {
    sepolia: { /* ... */ },
    stavanger: { /* ... */ },
    polygon: {
        name: 'Polygon',
        chainId: 137,
        icon: 'https://your-cdn.com/polygon-icon.svg',
        rpcUrl: 'https://polygon-rpc.com',
        bridgeAddress: '0x...',
        tokenAddress: '0x...',
        tokenType: 'ERC20'
    }
};
```

### Add More Tokens

Currently only POL is supported. To add more:

1. Update token selector UI in `index.html`
2. Add token config to `app.js`
3. Update balance fetching logic
4. Update bridge transaction logic

## Browser Support

- ✅ Chrome/Edge (recommended)
- ✅ Firefox
- ✅ Safari
- ✅ Brave
- ✅ Opera

Requires:
- MetaMask or compatible Web3 wallet
- Modern browser with ES6 support

## Security Considerations

### Client-Side Only

This frontend is completely client-side:
- ✅ No backend required
- ✅ No user data stored on server
- ✅ No API keys exposed (except public RPCs)
- ✅ All signing happens in user's wallet

### Private Key Safety

- ✅ Private keys never leave the wallet
- ✅ All transactions signed by user
- ✅ Permit signatures are EIP-712 typed data (safe)
- ✅ No phishing risk (all contract addresses in code)

### Network Validation

Always validates:
- ✅ Contract addresses match config
- ✅ Chain IDs are correct
- ✅ Token addresses are correct
- ✅ Amounts are valid
- ✅ Recipients are valid addresses

## Troubleshooting

### "Connect Wallet" not working

- Check if MetaMask is installed
- Try refreshing the page
- Check browser console for errors

### Permit signature fails

- Token might not support EIP-2612
- Frontend will automatically fall back to traditional approval
- This is expected for some tokens (like USDT)

### Network switching fails

- User might have rejected the request
- Try manually switching in MetaMask
- Stavanger might not be added - use "Add Network" in MetaMask

### Transaction fails

- Check gas balance on source network
- Check token balance
- Check if bridge has enough liquidity
- Check if recipient address is valid

### Balances not updating

- Check RPC URLs in config
- Try refreshing the page
- Check network connection
- RPC might be rate-limited

## Performance Optimization

### Already Implemented

- ✅ Minimal dependencies (only ethers.js)
- ✅ Lightweight CSS (no frameworks)
- ✅ Lazy loading of balances
- ✅ Local transaction history (no backend)
- ✅ Efficient rendering

### Optional Improvements

**Add service worker for offline support:**

```javascript
// sw.js
self.addEventListener('install', (event) => {
    event.waitUntil(
        caches.open('bridge-v1').then((cache) => {
            return cache.addAll([
                '/',
                '/index.html',
                '/styles.css',
                '/app.js'
            ]);
        })
    );
});
```

**Add analytics:**

```javascript
// Google Analytics
gtag('event', 'bridge_initiated', {
    from_network: fromNetwork.name,
    to_network: toNetwork.name,
    amount: amount
});
```

## Future Enhancements

Possible improvements:

- [ ] WalletConnect support (in addition to MetaMask)
- [ ] Dark mode toggle
- [ ] Multi-token support (not just POL)
- [ ] Gas estimation display
- [ ] Bridge fee calculator
- [ ] Real-time bridge status tracking
- [ ] Email notifications
- [ ] Mobile app (React Native)
- [ ] Advanced mode (custom gas, slippage)

## Support

- GitHub Issues: [Your repo]/issues
- Documentation: [Your docs site]
- Discord: [Your Discord server]

## License

MIT
