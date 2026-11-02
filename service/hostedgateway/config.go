package hostedgateway

import (
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const defaultTimeoutMS = 45000

// Config is the GeeLark gateway outbound settings (env only).
type Config struct {
	BaseURL    string
	APIKey     string
	TimeoutMS  int
}

func LoadConfig() Config {
	timeout := common.GetEnvOrDefault("HOSTED_GATEWAY_TIMEOUT_MS", defaultTimeoutMS)
	if timeout <= 0 {
		timeout = defaultTimeoutMS
	}
	return Config{
		BaseURL:   strings.TrimRight(strings.TrimSpace(os.Getenv("HOSTED_GATEWAY_URL")), "/"),
		APIKey:    strings.TrimSpace(os.Getenv("HOSTED_GATEWAY_API_KEY")),
		TimeoutMS: timeout,
	}
}

func (c Config) Ready() bool {
	return c.BaseURL != "" && c.APIKey != ""
}
