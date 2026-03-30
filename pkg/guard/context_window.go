// Package guard provides context window guard with intelligent degradation.
//
// Inspired by OpenClaw's context-window-guard:
// - Hard minimum detection
// - Warning threshold
// - Automatic degradation strategy
package guard

import (
	"fmt"
)

const (
	// HardMinTokens is the absolute minimum context window size
	HardMinTokens = 4000
	// WarnBelowTokens triggers warning when below this threshold
	WarnBelowTokens = 8000
)

// ContextWindowGuard manages context window with degradation.
type ContextWindowGuard struct {
	contextWindow int
	hardMin       int
	warnThreshold int
}

// NewContextWindowGuard creates a context window guard.
func NewContextWindowGuard(contextWindow int) *ContextWindowGuard {
	return &ContextWindowGuard{
		contextWindow: contextWindow,
		hardMin:       HardMinTokens,
		warnThreshold: WarnBelowTokens,
	}
}

// Evaluate checks context window and returns degradation strategy.
func (g *ContextWindowGuard) Evaluate(currentTokens int) *EvaluationResult {
	available := g.contextWindow - currentTokens

	result := &EvaluationResult{
		Available:     available,
		ContextWindow: g.contextWindow,
		CurrentTokens: currentTokens,
	}

	// Check hard minimum
	if available < g.hardMin {
		result.Level = LevelCritical
		result.Strategy = StrategyEmergencyCompress
		result.Message = fmt.Sprintf("Critical: only %d tokens available (min %d)", available, g.hardMin)
		return result
	}

	// Check warning threshold
	if available < g.warnThreshold {
		result.Level = LevelWarning
		result.Strategy = StrategyGracefulCompress
		result.Message = fmt.Sprintf("Warning: only %d tokens available (warn %d)", available, g.warnThreshold)
		return result
	}

	result.Level = LevelOK
	result.Strategy = StrategyNone
	return result
}

// EvaluationResult contains context window evaluation.
type EvaluationResult struct {
	Level         Level
	Strategy      Strategy
	Available     int
	ContextWindow int
	CurrentTokens int
	Message       string
}

// Level represents severity level.
type Level int

const (
	LevelOK Level = iota
	LevelWarning
	LevelCritical
)

// Strategy represents degradation strategy.
type Strategy int

const (
	StrategyNone Strategy = iota
	StrategyGracefulCompress // Use three-layer pruning
	StrategyEmergencyCompress // Force drop 50% messages
)
