package aiconfig

import (
	"strings"

	"github.com/spf13/viper"
)

type providerConfig struct {
	Models       []string `mapstructure:"models" json:"models"`
	Default      bool     `mapstructure:"default" json:"default"`
	DefaultModel string   `mapstructure:"default_model" json:"default_model"`
}

func ResolveDefaultModel() string {
	var providers []providerConfig
	_ = viper.UnmarshalKey("ai_providers", &providers)
	for _, provider := range providers {
		if !provider.Default {
			continue
		}
		if model := providerModel(provider); model != "" {
			return model
		}
	}
	for _, provider := range providers {
		if model := providerModel(provider); model != "" {
			return model
		}
	}
	return ""
}

func providerModel(provider providerConfig) string {
	if model := strings.TrimSpace(provider.DefaultModel); model != "" {
		return model
	}
	for _, model := range provider.Models {
		if model = strings.TrimSpace(model); model != "" {
			return model
		}
	}
	return ""
}
