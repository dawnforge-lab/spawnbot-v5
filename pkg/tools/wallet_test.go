package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
)

func newTestWalletTool(t *testing.T) *WalletTool {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Tools.Wallet.Enabled = true
	cfg.Tools.Wallet.Email = "test@example.com"
	cfg.Tools.Wallet.Chain = "base-sepolia"
	cfg.Tools.Wallet.MaxSendAmount = 10.0
	cfg.Tools.Wallet.MaxTradeAmount = 5.0
	cfg.Tools.Wallet.MaxPayAmount = 0.50
	return NewWalletTool(&cfg.Tools.Wallet)
}

func TestWalletTool_Name(t *testing.T) {
	tool := newTestWalletTool(t)
	if got := tool.Name(); got != "wallet" {
		t.Errorf("Name() = %q, want %q", got, "wallet")
	}
}

func TestWalletTool_StatusBuildCommand(t *testing.T) {
	tool := newTestWalletTool(t)
	cmd := tool.buildCommand("status", nil)
	want := "npx awal@latest status --json"
	if cmd != want {
		t.Errorf("buildCommand(status) = %q, want %q", cmd, want)
	}
}

func TestWalletTool_LoginBuildCommand(t *testing.T) {
	tool := newTestWalletTool(t)
	cmd := tool.buildCommand("login", nil)
	if !strings.Contains(cmd, "auth login") {
		t.Errorf("expected 'auth login' in command, got: %s", cmd)
	}
	if !strings.Contains(cmd, "test@example.com") {
		t.Errorf("expected email in command, got: %s", cmd)
	}
}

func TestWalletTool_VerifyBuildCommand(t *testing.T) {
	tool := newTestWalletTool(t)
	args := map[string]any{"flow_id": "flow-123", "otp": "456789"}
	cmd := tool.buildCommand("verify", args)
	if !strings.Contains(cmd, "auth verify") {
		t.Errorf("expected 'auth verify' in command, got: %s", cmd)
	}
	if !strings.Contains(cmd, "flow-123") {
		t.Errorf("expected flow_id in command, got: %s", cmd)
	}
	if !strings.Contains(cmd, "456789") {
		t.Errorf("expected otp in command, got: %s", cmd)
	}
}

func TestWalletTool_VerifyMissingFlowID(t *testing.T) {
	tool := newTestWalletTool(t)
	ctx := WithToolContext(context.Background(), "pico", "chat-1")
	result := tool.Execute(ctx, map[string]any{"action": "verify", "otp": "123456"})
	if !result.IsError {
		t.Fatal("expected error when flow_id is missing")
	}
	if !strings.Contains(result.ForLLM, "flow_id is required") {
		t.Errorf("expected flow_id error, got: %s", result.ForLLM)
	}
}

func TestWalletTool_VerifyMissingOTP(t *testing.T) {
	tool := newTestWalletTool(t)
	ctx := WithToolContext(context.Background(), "pico", "chat-1")
	result := tool.Execute(ctx, map[string]any{"action": "verify", "flow_id": "flow-123"})
	if !result.IsError {
		t.Fatal("expected error when otp is missing")
	}
	if !strings.Contains(result.ForLLM, "otp is required") {
		t.Errorf("expected otp error, got: %s", result.ForLLM)
	}
}

func TestWalletTool_BalanceBuildCommand(t *testing.T) {
	tool := newTestWalletTool(t)
	cmd := tool.buildCommand("balance", nil)
	if !strings.Contains(cmd, "balance") {
		t.Errorf("expected 'balance' in command, got: %s", cmd)
	}
	if !strings.Contains(cmd, "base-sepolia") {
		t.Errorf("expected chain in command, got: %s", cmd)
	}
}

func TestWalletTool_AddressBuildCommand(t *testing.T) {
	tool := newTestWalletTool(t)
	cmd := tool.buildCommand("address", nil)
	want := "npx awal@latest address --json"
	if cmd != want {
		t.Errorf("buildCommand(address) = %q, want %q", cmd, want)
	}
}

func TestWalletTool_FundBuildCommand(t *testing.T) {
	tool := newTestWalletTool(t)
	cmd := tool.buildCommand("fund", nil)
	want := "npx awal@latest show"
	if cmd != want {
		t.Errorf("buildCommand(fund) = %q, want %q", cmd, want)
	}
}

func TestWalletTool_UnknownAction(t *testing.T) {
	tool := newTestWalletTool(t)
	ctx := WithToolContext(context.Background(), "pico", "chat-1")
	result := tool.Execute(ctx, map[string]any{"action": "explode"})
	if !result.IsError {
		t.Fatal("expected error for unknown action")
	}
	if !strings.Contains(result.ForLLM, "unknown wallet action") {
		t.Errorf("expected unknown action error, got: %s", result.ForLLM)
	}
}

func TestWalletTool_MissingAction(t *testing.T) {
	tool := newTestWalletTool(t)
	ctx := WithToolContext(context.Background(), "pico", "chat-1")
	result := tool.Execute(ctx, map[string]any{})
	if !result.IsError {
		t.Fatal("expected error when action is missing")
	}
	if !strings.Contains(result.ForLLM, "action is required") {
		t.Errorf("expected action required error, got: %s", result.ForLLM)
	}
}

func TestWalletTool_EmptyEmail(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Tools.Wallet.Email = ""
	tool := NewWalletTool(&cfg.Tools.Wallet)

	ctx := WithToolContext(context.Background(), "pico", "chat-1")
	result := tool.Execute(ctx, map[string]any{"action": "login"})
	if !result.IsError {
		t.Fatal("expected error when email is not configured")
	}
	if !strings.Contains(result.ForLLM, "email not configured") {
		t.Errorf("expected email error, got: %s", result.ForLLM)
	}
}

func TestWalletTool_SendAmountExceedsLimit(t *testing.T) {
	tool := newTestWalletTool(t) // maxSendAmount = 10.0
	ctx := WithToolContext(context.Background(), "pico", "chat-1")
	result := tool.Execute(ctx, map[string]any{
		"action":    "send",
		"amount":    "15.00",
		"recipient": "0xabc",
	})
	if !result.IsError {
		t.Fatal("expected error when send amount exceeds limit")
	}
	if !strings.Contains(result.ForLLM, "exceeds configured limit") {
		t.Errorf("expected exceeds limit error, got: %s", result.ForLLM)
	}
}

func TestWalletTool_SendAmountWithinLimit(t *testing.T) {
	tool := newTestWalletTool(t)
	cmd := tool.buildCommand("send", map[string]any{
		"amount":    "5.00",
		"recipient": "0xabc",
	})
	if !strings.Contains(cmd, "send") {
		t.Errorf("expected 'send' in command, got: %s", cmd)
	}
	if !strings.Contains(cmd, "5.00") {
		t.Errorf("expected amount in command, got: %s", cmd)
	}
	if !strings.Contains(cmd, "0xabc") {
		t.Errorf("expected recipient in command, got: %s", cmd)
	}
}

func TestWalletTool_SendDollarPrefixAmount(t *testing.T) {
	tool := newTestWalletTool(t) // maxSendAmount = 10.0
	ctx := WithToolContext(context.Background(), "pico", "chat-1")
	result := tool.Execute(ctx, map[string]any{
		"action":    "send",
		"amount":    "$15",
		"recipient": "0xabc",
	})
	if !result.IsError {
		t.Fatal("expected error when $15 exceeds $10 limit")
	}
	if !strings.Contains(result.ForLLM, "exceeds configured limit") {
		t.Errorf("expected exceeds limit error, got: %s", result.ForLLM)
	}
}

func TestWalletTool_TradeAmountExceedsLimit(t *testing.T) {
	tool := newTestWalletTool(t) // maxTradeAmount = 5.0
	ctx := WithToolContext(context.Background(), "pico", "chat-1")
	result := tool.Execute(ctx, map[string]any{
		"action":     "trade",
		"amount":     "10.00",
		"from_token": "USDC",
		"to_token":   "ETH",
	})
	if !result.IsError {
		t.Fatal("expected error when trade amount exceeds limit")
	}
	if !strings.Contains(result.ForLLM, "exceeds configured limit") {
		t.Errorf("expected exceeds limit error, got: %s", result.ForLLM)
	}
}

func TestWalletTool_SendMissingRecipient(t *testing.T) {
	tool := newTestWalletTool(t)
	ctx := WithToolContext(context.Background(), "pico", "chat-1")
	result := tool.Execute(ctx, map[string]any{
		"action": "send",
		"amount": "1.00",
	})
	if !result.IsError {
		t.Fatal("expected error when recipient is missing")
	}
	if !strings.Contains(result.ForLLM, "recipient is required") {
		t.Errorf("expected recipient error, got: %s", result.ForLLM)
	}
}

func TestWalletTool_TradeMissingTokens(t *testing.T) {
	tool := newTestWalletTool(t)
	ctx := WithToolContext(context.Background(), "pico", "chat-1")
	result := tool.Execute(ctx, map[string]any{
		"action":   "trade",
		"amount":   "1.00",
		"to_token": "ETH",
	})
	if !result.IsError {
		t.Fatal("expected error when from_token is missing")
	}
	if !strings.Contains(result.ForLLM, "from_token is required") {
		t.Errorf("expected from_token error, got: %s", result.ForLLM)
	}
}

func TestWalletTool_ParseAmount(t *testing.T) {
	tests := []struct {
		input   string
		want    float64
		wantErr bool
	}{
		{"1.00", 1.0, false},
		{"$5.00", 5.0, false},
		{"0.50", 0.5, false},
		{"$0.01", 0.01, false},
		{"100", 100.0, false},
		{"garbage", 0.0, true},
	}
	for _, tc := range tests {
		got, err := parseAmount(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseAmount(%q): expected error, got nil (value %v)", tc.input, got)
			}
		} else {
			if err != nil {
				t.Errorf("parseAmount(%q): unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("parseAmount(%q) = %v, want %v", tc.input, got, tc.want)
			}
		}
	}
}

func TestWalletTool_SearchBuildCommand(t *testing.T) {
	tool := newTestWalletTool(t)
	args := map[string]any{"query": "weather API"}
	cmd := tool.buildCommand("search", args)
	if !strings.Contains(cmd, "x402 bazaar search") {
		t.Errorf("expected 'x402 bazaar search' in command, got: %s", cmd)
	}
	if !strings.Contains(cmd, "weather API") {
		t.Errorf("expected query in command, got: %s", cmd)
	}
}

func TestWalletTool_ListBuildCommand(t *testing.T) {
	tool := newTestWalletTool(t)
	cmd := tool.buildCommand("list", nil)
	want := "npx awal@latest x402 bazaar list --json"
	if cmd != want {
		t.Errorf("buildCommand(list) = %q, want %q", cmd, want)
	}
}

func TestWalletTool_DetailsBuildCommand(t *testing.T) {
	tool := newTestWalletTool(t)
	args := map[string]any{"url": "https://example.com/api"}
	cmd := tool.buildCommand("details", args)
	if !strings.Contains(cmd, "x402 details") {
		t.Errorf("expected 'x402 details' in command, got: %s", cmd)
	}
	if !strings.Contains(cmd, "https://example.com/api") {
		t.Errorf("expected url in command, got: %s", cmd)
	}
}

func TestWalletTool_PayBuildCommand(t *testing.T) {
	tool := newTestWalletTool(t)
	args := map[string]any{
		"url":    "https://example.com/api",
		"method": "POST",
		"data":   `{"key":"value"}`,
	}
	cmd := tool.buildCommand("pay", args)
	if !strings.Contains(cmd, "x402 pay") {
		t.Errorf("expected 'x402 pay' in command, got: %s", cmd)
	}
	if !strings.Contains(cmd, "-X") {
		t.Errorf("expected -X flag in command, got: %s", cmd)
	}
	if !strings.Contains(cmd, "POST") {
		t.Errorf("expected method in command, got: %s", cmd)
	}
	if !strings.Contains(cmd, "-d") {
		t.Errorf("expected -d flag in command, got: %s", cmd)
	}
}

func TestWalletTool_PayWithMaxAmount(t *testing.T) {
	tool := newTestWalletTool(t)
	args := map[string]any{
		"url":        "https://example.com/api",
		"max_amount": "0.25",
	}
	cmd := tool.buildCommand("pay", args)
	if !strings.Contains(cmd, "--max-amount") {
		t.Errorf("expected --max-amount flag in command, got: %s", cmd)
	}
	if !strings.Contains(cmd, "0.25") {
		t.Errorf("expected max amount value in command, got: %s", cmd)
	}
}

func TestWalletTool_TradeBuildCommandWithSlippage(t *testing.T) {
	tool := newTestWalletTool(t)
	args := map[string]any{
		"amount":     "1.00",
		"from_token": "USDC",
		"to_token":   "ETH",
		"slippage":   float64(50),
	}
	cmd := tool.buildCommand("trade", args)
	if !strings.Contains(cmd, "--slippage 50") {
		t.Errorf("expected '--slippage 50' in command, got: %s", cmd)
	}
}

func TestWalletTool_SearchMissingQuery(t *testing.T) {
	tool := newTestWalletTool(t)
	ctx := WithToolContext(context.Background(), "pico", "chat-1")
	result := tool.Execute(ctx, map[string]any{"action": "search"})
	if !result.IsError {
		t.Fatal("expected error when query is missing")
	}
	if !strings.Contains(result.ForLLM, "query is required") {
		t.Errorf("expected query error, got: %s", result.ForLLM)
	}
}

func TestWalletTool_PayMissingURL(t *testing.T) {
	tool := newTestWalletTool(t)
	ctx := WithToolContext(context.Background(), "pico", "chat-1")
	result := tool.Execute(ctx, map[string]any{"action": "pay"})
	if !result.IsError {
		t.Fatal("expected error when url is missing")
	}
	if !strings.Contains(result.ForLLM, "url is required") {
		t.Errorf("expected url error, got: %s", result.ForLLM)
	}
}

func TestWalletTool_PayInjectsConfigLimitWhenMaxAmountOmitted(t *testing.T) {
	tool := newTestWalletTool(t) // maxPayAmount = 0.50 → 500000 atomic units
	args := map[string]any{"url": "https://example.com/api"}
	cmd := tool.buildCommand("pay", args)
	if !strings.Contains(cmd, "--max-amount 500000") {
		t.Errorf("expected '--max-amount 500000' injected from config, got: %s", cmd)
	}
}

func TestWalletTool_PayMaxAmountExceedsConfigLimit(t *testing.T) {
	tool := newTestWalletTool(t) // maxPayAmount = 0.50
	ctx := WithToolContext(context.Background(), "pico", "chat-1")
	result := tool.Execute(ctx, map[string]any{
		"action":     "pay",
		"url":        "https://example.com/api",
		"max_amount": "1.00",
	})
	if !result.IsError {
		t.Fatal("expected error when pay max_amount exceeds config limit")
	}
	if !strings.Contains(result.ForLLM, "exceeds configured limit") {
		t.Errorf("expected exceeds configured limit error, got: %s", result.ForLLM)
	}
}

func TestWalletTool_DetailsMissingURL(t *testing.T) {
	tool := newTestWalletTool(t)
	ctx := WithToolContext(context.Background(), "pico", "chat-1")
	result := tool.Execute(ctx, map[string]any{"action": "details"})
	if !result.IsError {
		t.Fatal("expected error when url is missing for details action")
	}
	if !strings.Contains(result.ForLLM, "url is required") {
		t.Errorf("expected url error, got: %s", result.ForLLM)
	}
}

func TestWalletTool_Shellescape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "'hello'"},
		{"it's", "'it'\"'\"'s'"},
		{"normal query", "'normal query'"},
	}
	for _, tc := range tests {
		got := shellescape(tc.input)
		if got != tc.want {
			t.Errorf("shellescape(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
