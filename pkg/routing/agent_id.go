// Package routing provides message routing: agent ID normalization, session key
// construction, and multi-level priority routing.
package routing

import (
	"regexp"
	"strings"
)

const (
	DefaultAgentID   = "main"
	DefaultMainKey   = "main"
	DefaultAccountID = "default"
	MaxAgentIDLength = 64
)

var (
	validIDRe      = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	invalidCharsRe = regexp.MustCompile(`[^a-z0-9_-]+`)
	leadingDashRe  = regexp.MustCompile(`^-+`)
	trailingDashRe = regexp.MustCompile(`-+$`)
)

// NormalizeAgentID sanitizes an agent ID to [a-z0-9][a-z0-9_-]{0,63}.
// Invalid characters are collapsed to "-", leading/trailing dashes removed.
// Empty input returns DefaultAgentID.
func NormalizeAgentID(id string) string {
	return normalizeID(id, DefaultAgentID)
}

// NormalizeAccountID sanitizes an account ID. Empty input returns DefaultAccountID.
func NormalizeAccountID(id string) string {
	return normalizeID(id, DefaultAccountID)
}

func normalizeID(id, fallback string) string {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return fallback
	}
	lower := strings.ToLower(trimmed)
	if validIDRe.MatchString(lower) {
		return lower
	}
	result := invalidCharsRe.ReplaceAllString(lower, "-")
	result = leadingDashRe.ReplaceAllString(result, "")
	result = trailingDashRe.ReplaceAllString(result, "")
	if len(result) > MaxAgentIDLength {
		result = result[:MaxAgentIDLength]
	}
	if result == "" {
		return fallback
	}
	return result
}
