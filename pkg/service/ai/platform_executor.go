package ai

import (
	"scheduler0-private/pkg/config"
	"scheduler0-private/pkg/constants"
	"strings"

	awsbedrockruntime "github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

type PlatformBillable interface {
	IsPlatform() bool
}

type platformModelExecutor struct {
	ModelExecutor
}

func (platformModelExecutor) IsPlatform() bool { return true }

func PlatformAIEnabled(cfg *config.Scheduler0Configurations, bedrockClient *awsbedrockruntime.Client) bool {
	if cfg == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(cfg.PlatformAIProvider)) {
	case "", "none", "off", "disabled":
		return false
	case constants.AIProviderBedrock:
		return bedrockClient != nil
	default:
		return false
	}
}

func PlatformModelID(cfg *config.Scheduler0Configurations) string {
	if cfg != nil {
		if m := strings.TrimSpace(cfg.PlatformAIModel); m != "" {
			return m
		}
	}
	return constants.AIDefaultPlatformModel
}
