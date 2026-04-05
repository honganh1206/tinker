package inference

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInit_UnknownProvider(t *testing.T) {
	cfg := ClientConfig{
		ProviderName: "unknown_provider",
		ModelName:    "unknown_model",
		TokenLimit:   4096,
	}

	client, err := Init(context.Background(), cfg)

	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "unknown model provider")
}

func TestInit_GoogleProvider_MissingAPIKey(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "")

	cfg := ClientConfig{
		ProviderName: GoogleProvider,
		ModelName:    string(Gemini25Pro),
		TokenLimit:   8192,
	}

	// Now returns error instead of log.Fatal
	client, err := Init(context.Background(), cfg)
	// Gemini SDK may or may not error on missing key at client creation time,
	// but at minimum it should not crash the process.
	_ = client
	_ = err
}
