package config

// WalletConfig holds configuration for the agentic wallet tool (awal CLI).
type WalletConfig struct {
	ToolConfig     `envPrefix:"SPAWNBOT_TOOLS_WALLET_"`
	Email          string  `json:"email"            env:"SPAWNBOT_TOOLS_WALLET_EMAIL"`
	Chain          string  `json:"chain"            env:"SPAWNBOT_TOOLS_WALLET_CHAIN"`
	MaxSendAmount  float64 `json:"max_send_amount"  env:"SPAWNBOT_TOOLS_WALLET_MAX_SEND_AMOUNT"`
	MaxTradeAmount float64 `json:"max_trade_amount" env:"SPAWNBOT_TOOLS_WALLET_MAX_TRADE_AMOUNT"`
	MaxPayAmount   float64 `json:"max_pay_amount"   env:"SPAWNBOT_TOOLS_WALLET_MAX_PAY_AMOUNT"`
}
