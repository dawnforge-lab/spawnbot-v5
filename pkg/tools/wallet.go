package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
)

// WalletTool wraps the awal CLI (npx awal@latest) for crypto wallet operations.
type WalletTool struct {
	email          string
	chain          string
	maxSendAmount  float64
	maxTradeAmount float64
	maxPayAmount   float64
}

// NewWalletTool creates a WalletTool from the wallet section of config.
func NewWalletTool(cfg *config.WalletConfig) *WalletTool {
	chain := cfg.Chain
	if chain == "" {
		chain = "base"
	}
	return &WalletTool{
		email:          cfg.Email,
		chain:          chain,
		maxSendAmount:  cfg.MaxSendAmount,
		maxTradeAmount: cfg.MaxTradeAmount,
		maxPayAmount:   cfg.MaxPayAmount,
	}
}

func (t *WalletTool) Name() string { return "wallet" }

func (t *WalletTool) Description() string {
	return "Crypto wallet operations: check status, authenticate, check balance, send USDC, trade tokens, search/pay x402 services. Uses the Coinbase agentic wallet (awal CLI)."
}

func (t *WalletTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"status", "login", "verify", "balance", "address", "fund", "send", "trade", "search", "list", "details", "pay"},
				"description": "Wallet action to perform.",
			},
			"otp": map[string]any{
				"type":        "string",
				"description": "One-time password for verify action.",
			},
			"flow_id": map[string]any{
				"type":        "string",
				"description": "Flow ID for verify action.",
			},
			"amount": map[string]any{
				"type":        "string",
				"description": "Amount for send/trade actions (e.g. '1.00' or '$5.00').",
			},
			"recipient": map[string]any{
				"type":        "string",
				"description": "Recipient address for send action.",
			},
			"from_token": map[string]any{
				"type":        "string",
				"description": "Source token for trade action (e.g. 'USDC').",
			},
			"to_token": map[string]any{
				"type":        "string",
				"description": "Destination token for trade action (e.g. 'ETH').",
			},
			"slippage": map[string]any{
				"type":        "integer",
				"description": "Slippage tolerance for trade action (basis points).",
			},
			"query": map[string]any{
				"type":        "string",
				"description": "Search query for x402 bazaar search.",
			},
			"url": map[string]any{
				"type":        "string",
				"description": "URL for x402 details/pay actions.",
			},
			"method": map[string]any{
				"type":        "string",
				"description": "HTTP method for pay action (e.g. 'POST').",
			},
			"data": map[string]any{
				"type":        "string",
				"description": "Request body data for pay action.",
			},
			"max_amount": map[string]any{
				"type":        "string",
				"description": "Maximum amount willing to pay for pay action.",
			},
		},
		"required": []string{"action"},
	}
}

func (t *WalletTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, ok := args["action"].(string)
	if !ok || action == "" {
		return ErrorResult("action is required")
	}

	switch action {
	case "status", "balance", "address", "fund", "list":
		// No extra validation needed
	case "login":
		if t.email == "" {
			return ErrorResult("wallet email not configured; set tools.wallet.email in config")
		}
	case "verify":
		flowID, _ := args["flow_id"].(string)
		otp, _ := args["otp"].(string)
		if flowID == "" {
			return ErrorResult("flow_id is required for verify action")
		}
		if otp == "" {
			return ErrorResult("otp is required for verify action")
		}
	case "send":
		amount, _ := args["amount"].(string)
		recipient, _ := args["recipient"].(string)
		if amount == "" {
			return ErrorResult("amount is required for send action")
		}
		if recipient == "" {
			return ErrorResult("recipient is required for send action")
		}
		if t.maxSendAmount > 0 && parseAmount(amount) > t.maxSendAmount {
			return ErrorResult(fmt.Sprintf("send amount %.2f exceeds limit of %.2f", parseAmount(amount), t.maxSendAmount))
		}
	case "trade":
		amount, _ := args["amount"].(string)
		fromToken, _ := args["from_token"].(string)
		toToken, _ := args["to_token"].(string)
		if amount == "" {
			return ErrorResult("amount is required for trade action")
		}
		if fromToken == "" {
			return ErrorResult("from_token is required for trade action")
		}
		if toToken == "" {
			return ErrorResult("to_token is required for trade action")
		}
		if t.maxTradeAmount > 0 && parseAmount(amount) > t.maxTradeAmount {
			return ErrorResult(fmt.Sprintf("trade amount %.2f exceeds limit of %.2f", parseAmount(amount), t.maxTradeAmount))
		}
	case "search":
		query, _ := args["query"].(string)
		if query == "" {
			return ErrorResult("query is required for search action")
		}
	case "details":
		url, _ := args["url"].(string)
		if url == "" {
			return ErrorResult("url is required for details action")
		}
	case "pay":
		url, _ := args["url"].(string)
		if url == "" {
			return ErrorResult("url is required for pay action")
		}
		maxAmount, _ := args["max_amount"].(string)
		if maxAmount != "" && t.maxPayAmount > 0 && parseAmount(maxAmount) > t.maxPayAmount {
			return ErrorResult(fmt.Sprintf("pay max_amount %.2f exceeds limit of %.2f", parseAmount(maxAmount), t.maxPayAmount))
		}
	default:
		return ErrorResult(fmt.Sprintf("unknown wallet action: %s", action))
	}

	cmd := t.buildCommand(action, args)
	output, err := runAwal(ctx, cmd)
	if err != nil {
		return ErrorResult(fmt.Sprintf("awal command failed: %v\nOutput: %s", err, output))
	}
	return NewToolResult(output)
}

// buildCommand constructs the awal CLI command string for the given action.
func (t *WalletTool) buildCommand(action string, args map[string]any) string {
	switch action {
	case "status":
		return "npx awal@latest status --json"
	case "login":
		return fmt.Sprintf("npx awal@latest auth login %s --json", shellescape(t.email))
	case "verify":
		flowID, _ := args["flow_id"].(string)
		otp, _ := args["otp"].(string)
		return fmt.Sprintf("npx awal@latest auth verify %s %s --json", shellescape(flowID), shellescape(otp))
	case "balance":
		return fmt.Sprintf("npx awal@latest balance --chain %s --json", shellescape(t.chain))
	case "address":
		return "npx awal@latest address --json"
	case "fund":
		return "npx awal@latest show"
	case "send":
		amount, _ := args["amount"].(string)
		recipient, _ := args["recipient"].(string)
		return fmt.Sprintf("npx awal@latest send %s %s --chain %s --json",
			shellescape(amount), shellescape(recipient), shellescape(t.chain))
	case "trade":
		amount, _ := args["amount"].(string)
		fromToken, _ := args["from_token"].(string)
		toToken, _ := args["to_token"].(string)
		cmd := fmt.Sprintf("npx awal@latest trade %s %s %s",
			shellescape(amount), shellescape(fromToken), shellescape(toToken))
		if slippage, ok := args["slippage"].(float64); ok && slippage > 0 {
			cmd += fmt.Sprintf(" --slippage %d", int(slippage))
		}
		cmd += " --json"
		return cmd
	case "search":
		query, _ := args["query"].(string)
		return fmt.Sprintf("npx awal@latest x402 bazaar search %s --json", shellescape(query))
	case "list":
		return "npx awal@latest x402 bazaar list --json"
	case "details":
		url, _ := args["url"].(string)
		return fmt.Sprintf("npx awal@latest x402 details %s --json", shellescape(url))
	case "pay":
		url, _ := args["url"].(string)
		cmd := fmt.Sprintf("npx awal@latest x402 pay %s", shellescape(url))
		if method, ok := args["method"].(string); ok && method != "" {
			cmd += fmt.Sprintf(" -X %s", shellescape(method))
		}
		if data, ok := args["data"].(string); ok && data != "" {
			cmd += fmt.Sprintf(" -d %s", shellescape(data))
		}
		if maxAmount, ok := args["max_amount"].(string); ok && maxAmount != "" {
			cmd += fmt.Sprintf(" --max-amount %s", shellescape(maxAmount))
		}
		cmd += " --json"
		return cmd
	default:
		return ""
	}
}

// parseAmount strips a leading '$' and parses the string as a float64.
// Returns 0.0 if the string cannot be parsed.
func parseAmount(s string) float64 {
	s = strings.TrimPrefix(s, "$")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return v
}

// shellescape wraps a string in single quotes, escaping any embedded single quotes.
func shellescape(s string) string {
	escaped := strings.ReplaceAll(s, "'", "'\"'\"'")
	return "'" + escaped + "'"
}

// runAwal executes a shell command with a 30-second timeout and returns combined output.
func runAwal(ctx context.Context, command string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
