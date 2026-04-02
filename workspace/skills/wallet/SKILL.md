---
name: wallet
description: "Crypto wallet operations: authenticate, fund, send USDC, trade tokens, x402 marketplace (search, pay, monetize). Use when the user needs wallet ops, payments, or agent-to-agent commerce on Base."
argument-hint: "[operation or question]"
---

# Wallet

Manage a Coinbase agentic wallet on Base. Send USDC, trade tokens, pay for x402 services, and monetize your own APIs.

All operations use the `wallet` tool. Authentication is required before send/trade/pay/fund.

## 1. Authenticate Wallet

Two-step email OTP flow. The email is configured in `tools.wallet.email` — do not ask the user for it.

### When to use
- First wallet interaction in a session
- Any action returns "Not authenticated"
- After extended idle period

### Flow

1. Call `wallet` with `action: "status"` to check if already authenticated
2. If not authenticated, call `wallet` with `action: "login"` — this sends an OTP to the configured email
3. The response contains a `flowId` — ask the user for the 6-digit code from their email
4. Call `wallet` with `action: "verify"`, `flow_id`, and `otp`
5. Call `action: "status"` again to confirm

### Error recovery
- If OTP expired: start over with `action: "login"`
- If wrong code: ask user to check and try `action: "verify"` again

## 2. Fund Wallet

Add money via Coinbase Onramp.

### When to use
- Wallet has insufficient balance for send or trade
- User explicitly wants to deposit funds
- After a failed transaction due to low balance

### Flow

1. Check balance with `action: "balance"`
2. Call `action: "fund"` — this opens the wallet companion UI
3. Instruct user to select amount and payment method in the UI
4. After funding, verify with `action: "balance"`

### Payment methods
- **Apple Pay** — instant
- **Coinbase** — transfer from existing account
- **Card** — debit card, instant
- **Bank** — ACH, 1-3 days

### Alternative
User can send USDC directly to the wallet address (get it with `action: "address"`).

## 3. Send USDC

Send USDC to an Ethereum address or ENS name.

### When to use
- User wants to send money, pay someone, transfer, tip, donate
- Paying for goods or services outside x402

### Prerequisites
- Must be authenticated
- Must have sufficient USDC balance

### Parameters
- `amount`: `"1.00"`, `"$5"`, or atomic units (1000000 = $1)
- `recipient`: `"0x1234..."` or `"vitalik.eth"`

### Chain options
- `base` (default) — mainnet
- `base-sepolia` — testnet

### Spending limit
The tool enforces `max_send_amount` from config. If exceeded, inform the user of the limit.

### Error recovery
| Error | Action |
|-------|--------|
| Not authenticated | Run auth flow (section 1) |
| Insufficient balance | Check balance, suggest fund (section 2) |
| Could not resolve ENS | Verify name, ask for 0x address |

## 4. Trade Tokens

Swap tokens on Base (mainnet only).

### When to use
- User wants to buy/sell/swap/exchange/convert tokens
- Converting between USDC, ETH, WETH

### Token aliases
| Alias | Token | Decimals |
|-------|-------|----------|
| usdc | USDC | 6 |
| eth | ETH | 18 |
| weth | WETH | 18 |

Can also use contract addresses directly.

### Parameters
- `amount`: same formats as send
- `from_token`: source token alias or address
- `to_token`: destination token alias or address
- `slippage`: basis points (100 = 1%), optional

### Spending limit
The tool enforces `max_trade_amount` from config.

### Error recovery
| Error | Action |
|-------|--------|
| Not authenticated | Run auth flow |
| TRANSFER_FROM_FAILED | Insufficient balance or approval issue |
| No liquidity | Try smaller amount or different pair |
| Too many decimals | Reduce decimal places |

## 5. Search for Service

Search the x402 bazaar marketplace for paid API services.

### When to use
- User needs an external API service
- Looking for what paid services are available
- Need a capability that requires an external service

### No authentication required

### Actions
- `action: "search"` with `query` — keyword search (e.g., "weather", "sentiment analysis")
- `action: "list"` — browse all available resources
- `action: "details"` with `url` — inspect endpoint price and schema before paying

### Flow
1. Search with a keyword to find relevant services
2. Use details to check price and input/output schema
3. Proceed to pay-for-service (section 6) when ready

## 6. Pay for Service

Make a paid API request to an x402 endpoint.

### When to use
- Calling a paid API found via search
- Making an x402 payment request
- Using any service that returns HTTP 402

### Prerequisites
- Must be authenticated
- Must have sufficient USDC balance

### Parameters
- `url`: the x402 endpoint URL
- `method`: HTTP method (default: GET)
- `data`: JSON request body for POST requests
- `max_amount`: cap payment in USDC atomic units (1000000 = $1.00)

### Spending limit
The tool enforces `max_pay_amount` from config. If `max_amount` is not specified, the tool automatically injects the config limit.

### USDC atomic units
| Atomic Units | USD |
|---|---|
| 1000000 | $1.00 |
| 100000 | $0.10 |
| 50000 | $0.05 |
| 10000 | $0.01 |

### Error recovery
| Error | Action |
|-------|--------|
| Not authenticated | Run auth flow |
| No X402 payment requirements | URL is not x402 — use search to find valid endpoints |
| Insufficient balance | Fund wallet |

## 7. Monetize Service

Build and deploy a paid API that other agents can consume via x402.

### When to use
- User wants to offer a paid API
- Building agent-to-agent commerce
- Monetizing a data source or AI model

### This is a guided workflow, not a single tool action

### Steps

1. **Get wallet address** — call `action: "address"` to get the payment receiving address

2. **Scaffold the project**
```bash
mkdir x402-server && cd x402-server
npm init -y
npm install express x402-express
```

3. **Create the server** — `index.js`:
```javascript
const express = require("express");
const { paymentMiddleware } = require("x402-express");

const app = express();
app.use(express.json());

const PAY_TO = "<address from step 1>";

const payment = paymentMiddleware(PAY_TO, {
  "GET /api/example": {
    price: "$0.01",
    network: "base",
    config: {
      description: "What this endpoint returns",
    },
  },
});

// Health check (free, before payment middleware)
app.get("/health", (req, res) => res.json({ status: "ok" }));

// Protected endpoint
app.get("/api/example", payment, (req, res) => {
  res.json({ data: "This costs $0.01 per request" });
});

app.listen(3000, () => console.log("Server running on port 3000"));
```

4. **Test** — should get 402 without payment, 200 with:
```bash
curl -i http://localhost:3000/api/example
npx awal@latest x402 pay http://localhost:3000/api/example
```

### Route config options
- `price`: USDC price string (e.g., "$0.01", "$1.00")
- `network`: "base" or "base-sepolia"
- `config.description`: what the endpoint does
- `config.inputSchema`: expected request body schema
- `config.outputSchema`: response body schema

### Multiple endpoints with different prices

```javascript
const payment = paymentMiddleware(PAY_TO, {
  "GET /api/cheap": { price: "$0.001", network: "base" },
  "GET /api/expensive": { price: "$1.00", network: "base" },
  "POST /api/query": { price: "$0.25", network: "base" },
});
```

### POST with body schema

```javascript
const payment = paymentMiddleware(PAY_TO, {
  "POST /api/analyze": {
    price: "$0.10",
    network: "base",
    config: {
      description: "Analyze text sentiment",
      inputSchema: {
        bodyType: "json",
        bodyFields: {
          text: { type: "string", description: "Text to analyze" },
        },
      },
      outputSchema: {
        type: "object",
        properties: {
          sentiment: { type: "string" },
          score: { type: "number" },
        },
      },
    },
  },
});
```

### Pricing guidelines
| Use Case | Suggested Price |
|---|---|
| Simple data lookup | $0.001 - $0.01 |
| API proxy / enrichment | $0.01 - $0.10 |
| Compute-heavy query | $0.10 - $0.50 |
| AI inference | $0.05 - $1.00 |

### Production deployment
For production, use the Coinbase facilitator:
```bash
npm install @coinbase/x402
```
Requires `CDP_API_KEY_ID` and `CDP_API_KEY_SECRET` environment variables from the CDP Portal.
