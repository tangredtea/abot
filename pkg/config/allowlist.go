package config

import (
	"encoding/json"
	"os"

	"abot/pkg/types"
)

// LoadAllowlistFromFile loads sender allowlist from JSON file (CLI mode).
func LoadAllowlistFromFile(path string) (*types.SenderAllowlist, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultAllowlist(), nil
		}
		return nil, err
	}

	var cfg struct {
		Default types.AllowlistEntry            `json:"default"`
		Chats   map[string]types.AllowlistEntry `json:"chats"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &types.SenderAllowlist{
		Default: cfg.Default,
		Chats:   cfg.Chats,
	}, nil
}

func defaultAllowlist() *types.SenderAllowlist {
	return &types.SenderAllowlist{
		Default: types.AllowlistEntry{
			Allow: []string{"*"},
			Mode:  types.AllowlistModeTrigger,
		},
		Chats: make(map[string]types.AllowlistEntry),
	}
}
