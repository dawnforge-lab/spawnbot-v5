package gateway

import (
	"testing"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestGateway_AutonomyConfig(t *testing.T) {
	cfg := config.DefaultConfig()

	// Verify autonomy config exists and has correct defaults
	assert.False(t, cfg.Autonomy.IdleTrigger.Enabled)
	assert.Equal(t, 8, cfg.Autonomy.IdleTrigger.ThresholdHours)
	assert.Nil(t, cfg.Autonomy.Feeds)
}
