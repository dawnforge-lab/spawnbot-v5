> ## Documentation Index
> Fetch the complete documentation index at: https://docs.cdp.coinbase.com/llms.txt
> Use this file to discover all available pages before exploring further.

# Agentic Wallet

Give your AI agent a wallet. Pay for APIs, send money, and trade tokens safely, with built-in spending limits.

## What is Agentic Wallet?

Agentic Wallet gives any AI agent a standalone wallet to hold and spend stablecoins, or trade for other tokens on Base.

Built on Coinbase Developer Platform (CDP) infrastructure, agents can authenticate via email OTP, hold USDC, and send, trade, or pay for services without ever accessing private keys.

<Accordion title="Comparing AgentKit vs Agentic Wallet">
  |                 | [AgentKit](/agent-kit/welcome)                     | Agentic Wallet                                    |
  | --------------- | -------------------------------------------------- | ------------------------------------------------- |
  | **What it is**  | SDK/toolkit for onchain actions                    | Standalone wallet via CLI/MCP                     |
  | **Integration** | Import into your agent code                        | Can call CLI or MCP tools (e.g., `npx awal send`) |
  | **Scope**       | Full onchain capabilities (deploy contracts, etc.) | Wallet ops: send, trade, x402                     |
  | **Networks**    | Multi-network (EVM + Solana)                       | Base                                              |
</Accordion>

## Use cases

<CardGroup cols={2}>
  <Card title="Pay-per-call APIs" icon="bolt">
    Agents pay for external services via [x402](/x402/core-concepts/how-it-works)
  </Card>

  <Card title="Gasless autonomy" icon="paper-plane">
    Send payments, tip creators, split bills without paying gas fees
  </Card>

  <Card title="Agent-to-agent commerce" icon="store">
    Build paid APIs that other agents can consume
  </Card>

  <Card title="Budget-constrained agents" icon="shield">
    Give agents spending power with per-session limits
  </Card>
</CardGroup>

## Capabilities

| Feature                                               | Description                                                                                         |
| ----------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| **Wallet identity**                                   | Self-custody wallet controlled by the agent                                                         |
| **Spending limits**                                   | Configurable caps per session and per transaction                                                   |
| **Gasless trading**                                   | Token swaps on Base without requiring gas                                                           |
| **Skill extensibility**                               | Add new capabilities via [agentic-wallet-skills](https://github.com/coinbase/agentic-wallet-skills) |
| **[x402](/x402/core-concepts/how-it-works) payments** | Machine-to-machine paid API requests                                                                |

### Security

* **Key isolation**: Private keys stay in Coinbase infrastructure
* **Spending guardrails**: Enforce limits before any transaction
* **KYT screening**: Block high-risk interactions automatically
* **Built-in OFAC compliance**: All transfers are automatically screened against OFAC sanctions lists and blocked before submission onchain, helping agents operate within regulatory bounds on Coinbase's trusted, compliant crypto infrastructure

## Components

### awal CLI

Command-line tool for wallet operations. Use it directly for testing, or let agents invoke it via skills.

```bash  theme={null}
npx awal status      # Check auth status
npx awal send 1 vitalik.eth   # Send USDC
npx awal trade 5 usdc eth     # Swap tokens
```

### Agent Skills

Instead of manually wiring wallet operations into your agent, install skills and let the agent handle it.

```bash  theme={null}
npx skills add coinbase/agentic-wallet-skills
```

Skills include: authenticate, fund, send, trade, search-for-service, pay-for-service, monetize-service.

### x402 Integration

Protocol for machine-to-machine payments. Agents can both consume and provide paid APIs with [x402](/x402/core-concepts/how-it-works), enabling agent-to-agent commerce.

## What to read next

<CardGroup cols={2}>
  <Card title="Quickstart" icon="rocket" href="/agentic-wallet/quickstart">
    First wallet in 2 minutes
  </Card>

  <Card title="Skills Reference" icon="book" href="/agentic-wallet/skills/overview">
    All agent capabilities
  </Card>
</CardGroup>
