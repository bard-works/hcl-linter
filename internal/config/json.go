package config

import (
	"encoding/json"
	"fmt"
)

func loadJSONConfig(data []byte, path string) (*Rules, error) {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config %s: %w", path, err)
	}
	return &cfg.Rules, nil
}
