package aiconfig

import (
	"testing"

	"ai-workbench-api/internal/model"

	"github.com/spf13/viper"
)

func TestResolveDefaultModelUsesSavedProviders(t *testing.T) {
	viper.Reset()
	viper.Set("ai_providers", []model.AIProvider{{ID: "p1", Name: "saved", Models: []string{"gpt-5.5"}, Default: true}})

	if got := ResolveDefaultModel(); got != "gpt-5.5" {
		t.Fatalf("expected saved provider model, got %q", got)
	}
}
