package types

// ProviderSpec describes an LLM provider's metadata.
type ProviderSpec struct {
	Name           string
	Keywords       []string
	IsGateway      bool
	DefaultAPIBase string
	EnvKey         string
}

// FailoverReason classifies why a provider call failed.
type FailoverReason string

const (
	FailoverAuth       FailoverReason = "auth"
	FailoverRateLimit  FailoverReason = "rate_limit"
	FailoverBilling    FailoverReason = "billing"
	FailoverTimeout    FailoverReason = "timeout"
	FailoverFormat     FailoverReason = "format"
	FailoverOverloaded FailoverReason = "overloaded"
	FailoverUnknown    FailoverReason = "unknown"
)
