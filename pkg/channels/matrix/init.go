package matrix

import (
	"path/filepath"

	"github.com/dawnforge-lab/spawnbot-v5/pkg/bus"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/channels"
	"github.com/dawnforge-lab/spawnbot-v5/pkg/config"
)

func init() {
	channels.RegisterFactory("matrix", func(cfg *config.Config, b *bus.MessageBus) (channels.Channel, error) {
		matrixCfg := cfg.Channels.Matrix
		cryptoDatabasePath := matrixCfg.CryptoDatabasePath
		if cryptoDatabasePath == "" {
			cryptoDatabasePath = filepath.Join(cfg.WorkspacePath(), "matrix")
		}
		return NewMatrixChannel(matrixCfg, b, cryptoDatabasePath)
	})
}
