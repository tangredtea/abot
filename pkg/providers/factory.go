// Package providers implements the provider registry and LLM factory.
package providers

import (
	"fmt"
	"strings"

	"google.golang.org/adk/model"

	"abot/pkg/providers/anthropic"
	"abot/pkg/providers/openaicompat"
)

// ExtractProtocol splits a model string into protocol prefix and model ID.
// Defaults to "openai" when no prefix is present.
//
// Examples:
//   - "openai/gpt-4o"              -> ("openai", "gpt-4o")
//   - "anthropic/claude-sonnet-4.6" -> ("anthropic", "claude-sonnet-4.6")
//   - "gpt-4o"                      -> ("openai", "gpt-4o")
func ExtractProtocol(modelStr string) (protocol, modelID string) {
	modelStr = strings.TrimSpace(modelStr)
	protocol, modelID, found := strings.Cut(modelStr, "/")
	if !found {
		return "openai", modelStr
	}
	return protocol, modelID
}

// ModelFactoryConfig holds the configuration needed to create a model.LLM instance.
type ModelFactoryConfig struct {
	Model        string // Protocol/model identifier, e.g. "anthropic/claude-sonnet-4.6".
	APIKey       string
	APIBase      string
	Proxy        string // Reserved for future use.
	PromptCaching bool  // Enable Anthropic prompt caching.
}

// CreateModelFromConfig creates a model.LLM based on the protocol prefix in the Model field.
// Supported protocols: openai (default), anthropic, and all OpenAI-compatible third-party services.
// Returns the LLM instance, the bare model ID (without protocol prefix), and any error.
func CreateModelFromConfig(cfg ModelFactoryConfig) (model.LLM, string, error) {
	if cfg.Model == "" {
		return nil, "", fmt.Errorf("model is required")
	}

	protocol, modelID := ExtractProtocol(cfg.Model)

	switch protocol {
	case "anthropic":
		return createAnthropicModel(cfg, modelID)

	case "openai", "openrouter", "groq", "zhipu", "gemini", "nvidia",
		"ollama", "moonshot", "deepseek", "cerebras", "volcengine",
		"vllm", "qwen", "mistral", "shengsuanyun":
		return createOpenAICompatModel(cfg, protocol, modelID)

	default:
		// Unknown protocols fall back to OpenAI-compatible
		return createOpenAICompatModel(cfg, protocol, modelID)
	}
}

func createAnthropicModel(cfg ModelFactoryConfig, modelID string) (model.LLM, string, error) {
	apiBase := cfg.APIBase
	if apiBase == "" {
		apiBase = "https://api.anthropic.com"
	}
	if cfg.APIKey == "" {
		return nil, "", fmt.Errorf("api_key is required for anthropic (model: %s)", cfg.Model)
	}
	m := anthropic.NewModel(anthropic.Config{
		Name:          modelID,
		APIKey:        cfg.APIKey,
		APIBase:       apiBase,
		PromptCaching: cfg.PromptCaching,
	})
	return m, modelID, nil
}

func createOpenAICompatModel(cfg ModelFactoryConfig, protocol, modelID string) (model.LLM, string, error) {
	apiBase := cfg.APIBase
	if apiBase == "" {
		apiBase = GetDefaultAPIBase(protocol)
	}
	if apiBase == "" {
		return nil, "", fmt.Errorf("api_base is required for protocol %q (model: %s)", protocol, cfg.Model)
	}
	m := openaicompat.NewModel(openaicompat.Config{
		Name:    modelID,
		APIKey:  cfg.APIKey,
		APIBase: apiBase,
	})
	return m, modelID, nil
}

// GetDefaultAPIBase returns the default API base URL for the given protocol.
// Returns an empty string if the protocol has no known default.
func GetDefaultAPIBase(protocol string) string {
	switch protocol {
	case "openai":
		return "https://api.openai.com/v1"
	case "openrouter":
		return "https://openrouter.ai/api/v1"
	case "groq":
		return "https://api.groq.com/openai/v1"
	case "zhipu":
		return "https://open.bigmodel.cn/api/paas/v4"
	case "gemini":
		return "https://generativelanguage.googleapis.com/v1beta"
	case "nvidia":
		return "https://integrate.api.nvidia.com/v1"
	case "ollama":
		return "http://localhost:11434/v1"
	case "moonshot":
		return "https://api.moonshot.cn/v1"
	case "shengsuanyun":
		return "https://router.shengsuanyun.com/api/v1"
	case "deepseek":
		return "https://api.deepseek.com/v1"
	case "cerebras":
		return "https://api.cerebras.ai/v1"
	case "volcengine":
		return "https://ark.cn-beijing.volces.com/api/v3"
	case "qwen":
		return "https://dashscope.aliyuncs.com/compatible-mode/v1"
	case "vllm":
		return "http://localhost:8000/v1"
	case "mistral":
		return "https://api.mistral.ai/v1"
	default:
		return ""
	}
}
