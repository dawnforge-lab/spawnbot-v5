> ## Documentation Index
> Fetch the complete documentation index at: https://docs.cdp.coinbase.com/llms.txt
> Use this file to discover all available pages before exploring further.

# Agent Skills

Agent Skills are pre-built capabilities that enable AI agents to perform wallet operations using the `awal` CLI.

## Available skills

| Skill                                                           | Description                                                    |
| --------------------------------------------------------------- | -------------------------------------------------------------- |
| [authenticate-wallet](/agentic-wallet/skills/authenticate)      | Sign in to the wallet via email OTP                            |
| [fund](/agentic-wallet/skills/fund)                             | Add money to the wallet via Coinbase Onramp                    |
| [send-usdc](/agentic-wallet/skills/send)                        | Send USDC to Ethereum addresses or ENS names                   |
| [trade](/agentic-wallet/skills/trade)                           | Trade tokens on Base                                           |
| [search-for-service](/agentic-wallet/skills/search-for-service) | Search the x402 bazaar for paid API services                   |
| [pay-for-service](/agentic-wallet/skills/pay-for-service)       | Make paid API requests via x402                                |
| [monetize-service](/agentic-wallet/skills/monetize-service)     | Build and deploy a paid API that other agents can use via x402 |

## Installation

Install skills using Vercel's Skills CLI:

```bash  theme={null}
npx skills add coinbase/agentic-wallet-skills
```

Once installed, your agent gains wallet capabilities with no additional wiring required.

## Example prompts

Once skills are installed, agents respond to wallet-related requests:

```
"Sign in to my wallet with agent@company.com"
→ Uses authenticate-wallet skill

"Send 10 USDC to vitalik.eth"
→ Uses send-usdc skill

"Buy $5 of ETH"
→ Uses trade skill

"Find APIs for sentiment analysis"
→ Uses search-for-service skill

"Call that weather API"
→ Uses pay-for-service skill

"Set up a paid endpoint for my data"
→ Uses monetize-service skill
```

## Skill structure

Each skill includes:

* **Name**: Unique identifier
* **Description**: Trigger phrases and when to use
* **Instructions**: Step-by-step guidance
* **CLI Commands**: Specific `awal` commands
* **Allowed Tools**: Commands the agent can run without prompting
