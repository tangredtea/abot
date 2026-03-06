package tools

import (
	"fmt"
	"time"

	"abot/pkg/types"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// --- browse_events ---

type browseEventsArgs struct {
	DateFrom string `json:"date_from,omitempty" jsonschema:"Start date (RFC3339 or YYYY-MM-DD). Defaults to 7 days ago."`
	DateTo   string `json:"date_to,omitempty" jsonschema:"End date (RFC3339 or YYYY-MM-DD). Defaults to now."`
	Limit    int    `json:"limit,omitempty" jsonschema:"Max events to return (default 30, max 100)"`
}

type browseEventsResult struct {
	Events []eventHit `json:"events,omitempty"`
	Error  string     `json:"error,omitempty"`
}

type eventHit struct {
	Category  string `json:"category"`
	Summary   string `json:"summary"`
	CreatedAt string `json:"created_at"`
}

func newBrowseEvents(deps *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "browse_events",
		Description: "Browse episodic event log by time range. Use to recall what happened on a specific day or period.",
	}, func(ctx tool.Context, args browseEventsArgs) (browseEventsResult, error) {
		if deps.MemoryEventStore == nil {
			return browseEventsResult{Error: "event store not configured"}, nil
		}

		tenantID := stateStr(ctx, "tenant_id")
		if tenantID == "" {
			tenantID = types.DefaultTenantID
		}
		userID := stateStr(ctx, "user_id")

		from, to := parseDateBounds(args.DateFrom, args.DateTo)
		if from.IsZero() {
			from = time.Now().AddDate(0, 0, -7)
		}

		limit := args.Limit
		if limit <= 0 {
			limit = 30
		}
		if limit > 100 {
			limit = 100
		}

		events, err := deps.MemoryEventStore.List(ctx, tenantID, userID, from, to, limit)
		if err != nil {
			return browseEventsResult{Error: fmt.Sprintf("list events: %v", err)}, nil
		}

		hits := make([]eventHit, len(events))
		for i, e := range events {
			hits[i] = eventHit{
				Category:  e.Category,
				Summary:   e.Summary,
				CreatedAt: e.CreatedAt.Format(time.RFC3339),
			}
		}
		return browseEventsResult{Events: hits}, nil
	})
	return t
}
