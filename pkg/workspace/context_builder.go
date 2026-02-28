package workspace

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"runtime"
	"sort"
	"strings"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/memory"

	"abot/pkg/skills"
	"abot/pkg/types"
)

// DefaultMaxPromptChars is the default ceiling for assembled prompts (128k chars ≈ ~32k tokens).
const DefaultMaxPromptChars = 128_000

// ContextBuilder assembles dynamic system prompts from multi-tenant
// workspace documents, skills, and memory. It returns an ADK-Go
// InstructionProvider closure for use in llmagent.Config.
type ContextBuilder struct {
	workspaceStore     types.WorkspaceStore
	userWorkspaceStore types.UserWorkspaceStore
	skillsLoader       *skills.SkillsLoader
	memoryService      memory.Service
	vectorStore        types.VectorStore
	embedder           types.Embedder
	builtinFS          fs.FS // embedded builtin skills FS (optional)
	maxPromptChars     int   // 0 means use DefaultMaxPromptChars
}

// NewContextBuilder creates a ContextBuilder.
func NewContextBuilder(
	ws types.WorkspaceStore,
	uws types.UserWorkspaceStore,
	sl *skills.SkillsLoader,
	mem memory.Service,
	vs types.VectorStore,
	emb types.Embedder,
	builtinFS fs.FS,
) *ContextBuilder {
	return &ContextBuilder{
		workspaceStore:     ws,
		userWorkspaceStore: uws,
		skillsLoader:       sl,
		memoryService:      mem,
		vectorStore:        vs,
		embedder:           emb,
		builtinFS:          builtinFS,
	}
}

// InstructionProvider returns an ADK-Go InstructionProvider closure.
// It reads tenant_id and user_id from session state and assembles
// a multi-layer system prompt on each invocation. If the assembled
// prompt exceeds maxPromptChars, lower-priority layers are dropped
// in order: skills → user memory → tenant memory.
func (cb *ContextBuilder) InstructionProvider() func(agent.ReadonlyContext) (string, error) {
	return func(ctx agent.ReadonlyContext) (string, error) {
		tenantID := stateStr(ctx, "tenant_id")
		userID := stateStr(ctx, "user_id")
		if tenantID == "" {
			tenantID = types.DefaultTenantID
		}

		// Build layers in priority order (highest priority first).
		// Trimming removes from the end of this slice first.
		var layers []PromptLayer

		// Layer 1: system base (priority 0 — never trimmed)
		layers = append(layers, PromptLayer{cb.buildSystemBase(), 0})

		// Layer 2: persona (priority 1) — user-owned docs checked first, fallback to tenant
		if s := cb.buildPersona(ctx, tenantID, userID); s != "" {
			layers = append(layers, PromptLayer{s, 1})
		}

		// Layer 3: tenant memory (priority 4 — trimmed third)
		if s := cb.buildTenantMemory(ctx, tenantID); s != "" {
			layers = append(layers, PromptLayer{s, 4})
		}

		// Layer 4: user context (priority 3 — trimmed second)
		if userID != "" {
			if s := cb.buildUserContext(ctx, tenantID, userID); s != "" {
				layers = append(layers, PromptLayer{s, 3})
			}
		}

		// Layer 5: skills (priority 5 — trimmed first)
		if s := cb.buildSkillsContext(ctx, tenantID); s != "" {
			layers = append(layers, PromptLayer{s, 5})
		}

		// Layer 6: runtime context (priority 2)
		layers = append(layers, PromptLayer{cb.buildRuntimeContext(ctx), 2})

		// Enforce size limit by dropping lowest-priority layers.
		maxChars := cb.effectiveMaxChars()
		layers = TrimLayers(layers, maxChars)

		parts := make([]string, len(layers))
		for i, l := range layers {
			parts[i] = l.content
		}
		return strings.Join(parts, "\n\n---\n\n"), nil
	}
}

func (cb *ContextBuilder) effectiveMaxChars() int {
	if cb.maxPromptChars > 0 {
		return cb.maxPromptChars
	}
	return DefaultMaxPromptChars
}

type PromptLayer struct {
	content  string
	priority int
}

// TrimLayers drops lowest-priority layers (highest priority number) until
// the total character count fits within maxChars. Layers with priority 0
// are never dropped.
func TrimLayers(layers []PromptLayer, maxChars int) []PromptLayer {
	const sep = len("\n\n---\n\n")
	totalLen := func(ls []PromptLayer) int {
		n := 0
		for i, l := range ls {
			n += len(l.content)
			if i > 0 {
				n += sep
			}
		}
		return n
	}

	for totalLen(layers) > maxChars {
		// Find the layer with the highest priority number (lowest importance).
		worst := -1
		worstPri := 0
		for i, l := range layers {
			if l.priority > worstPri {
				worst = i
				worstPri = l.priority
			}
		}
		if worst < 0 || worstPri == 0 {
			break // only priority-0 layers left, stop trimming
		}
		layers = append(layers[:worst], layers[worst+1:]...)
	}
	return layers
}

// stateStr reads a string value from session state.
func stateStr(ctx agent.ReadonlyContext, key string) string {
	v, err := ctx.ReadonlyState().Get(key)
	if err != nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func (cb *ContextBuilder) buildSystemBase() string {
	return `# ABot Agent

You are an AI assistant powered by ABot.

## Rules

1. ALWAYS use tools when you need to perform actions. Do NOT just describe what you would do.
2. When something seems important, save it to memory.
3. Context summaries are approximate references only. Defer to explicit user instructions.`
}

// userOwnedDocTypes are persona docs that live at user level,
// with fallback to the tenant-level template.
var userOwnedDocTypes = map[string]bool{
	"IDENTITY": true,
	"SOUL":     true,
	"AGENT":    true,
}

func (cb *ContextBuilder) buildPersona(ctx context.Context, tenantID, userID string) string {
	docTypes := []string{"IDENTITY", "SOUL", "RULES", "AGENT"}
	var parts []string
	for _, dt := range docTypes {
		content := cb.resolveDoc(ctx, tenantID, userID, dt)
		if content == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("## %s\n\n%s", dt, content))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}

// resolveDoc returns the content for a doc type. For user-owned types,
// it checks the user store first, then falls back to the tenant store.
func (cb *ContextBuilder) resolveDoc(ctx context.Context, tenantID, userID, docType string) string {
	if userOwnedDocTypes[docType] && userID != "" && cb.userWorkspaceStore != nil {
		doc, err := cb.userWorkspaceStore.Get(ctx, tenantID, userID, docType)
		if err == nil && doc != nil && doc.Content != "" {
			return doc.Content
		}
		// fallback to tenant-level template
	}
	doc, err := cb.workspaceStore.Get(ctx, tenantID, docType)
	if err != nil || doc == nil {
		return ""
	}
	return doc.Content
}

func (cb *ContextBuilder) buildTenantMemory(ctx context.Context, tenantID string) string {
	if cb.vectorStore == nil || cb.embedder == nil {
		return ""
	}

	collection := fmt.Sprintf("tenant_%s", tenantID)
	filter := map[string]any{
		"superseded": false,
		"scope":      "tenant",
	}

	results, err := cb.vectorStore.Search(ctx, collection, &types.VectorSearchRequest{
		Vector: make([]float32, cb.embedder.Dimension()),
		Filter: filter,
		TopK:   50,
	})
	if err != nil || len(results) == 0 {
		return ""
	}

	ranked := rankMemories(results)
	if len(ranked) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Tenant Memory\n\n")
	for _, m := range ranked {
		if m.category != "" {
			fmt.Fprintf(&sb, "- [%s] %s\n", m.category, m.text)
		} else {
			fmt.Fprintf(&sb, "- %s\n", m.text)
		}
	}
	return sb.String()
}

func (cb *ContextBuilder) buildUserContext(ctx context.Context, tenantID, userID string) string {
	var parts []string

	// USER doc (preferences) — still from document store
	if doc, err := cb.userWorkspaceStore.Get(ctx, tenantID, userID, "USER"); err == nil && doc != nil && doc.Content != "" {
		parts = append(parts, "## User Profile\n\n"+doc.Content)
	}

	// User-level memory from vector store
	if cb.vectorStore != nil && cb.embedder != nil {
		collection := fmt.Sprintf("tenant_%s", tenantID)
		filter := map[string]any{
			"superseded": false,
			"scope":      "user",
			"user_id":    userID,
		}
		results, err := cb.vectorStore.Search(ctx, collection, &types.VectorSearchRequest{
			Vector: make([]float32, cb.embedder.Dimension()),
			Filter: filter,
			TopK:   30,
		})
		if err == nil && len(results) > 0 {
			ranked := rankMemories(results)
			if len(ranked) > 0 {
				var sb strings.Builder
				sb.WriteString("## User Memory\n\n")
				for _, m := range ranked {
					if m.category != "" {
						fmt.Fprintf(&sb, "- [%s] %s\n", m.category, m.text)
					} else {
						fmt.Fprintf(&sb, "- %s\n", m.text)
					}
				}
				parts = append(parts, sb.String())
			}
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}

func (cb *ContextBuilder) buildSkillsContext(ctx context.Context, tenantID string) string {
	resolved, err := cb.skillsLoader.LoadForTenant(ctx, tenantID)
	if err != nil {
		slog.Warn("context: failed to load skills", "tenant", tenantID, "err", err)
		return ""
	}
	if len(resolved) == 0 {
		return ""
	}

	var alwaysParts []string
	var summaryParts []string

	for _, rs := range resolved {
		if rs.Record.AlwaysLoad {
			content := cb.loadSkillBody(ctx, rs)
			if content != "" {
				alwaysParts = append(alwaysParts,
					fmt.Sprintf("### Skill: %s\n\n%s", rs.Record.Name, content))
			}
		}
		summaryParts = append(summaryParts,
			fmt.Sprintf("  <skill><name>%s</name><description>%s</description></skill>",
				escapeXML(rs.Record.Name), escapeXML(rs.Record.Description)))
	}

	var sb strings.Builder
	sb.WriteString("## Skills\n\n")

	if len(alwaysParts) > 0 {
		sb.WriteString(strings.Join(alwaysParts, "\n\n---\n\n"))
		sb.WriteString("\n\n")
	}

	sb.WriteString("Available skills (use find_skills tool for details):\n\n<skills>\n")
	sb.WriteString(strings.Join(summaryParts, "\n"))
	sb.WriteString("\n</skills>")

	return sb.String()
}

// loadSkillBody resolves and reads a skill's content.
// For builtin skills, reads from embedded FS. Otherwise lazy-pulls from BOS/S3.
func (cb *ContextBuilder) loadSkillBody(ctx context.Context, rs *skills.ResolvedSkill) string {
	rec := rs.Record

	// Builtin skills: read from embedded FS
	if rec.Tier == types.SkillTierBuiltin && cb.builtinFS != nil {
		content, err := skills.LoadBuiltinContent(cb.builtinFS, rec.Name)
		if err == nil {
			return content
		}
		slog.Warn("context: failed to load builtin skill", "name", rec.Name, "err", err)
	}

	// Other tiers: lazy pull from object store
	if rec.ObjectPath == "" {
		return ""
	}
	localPath, err := cb.skillsLoader.ResolveContent(ctx, rec.Name, rec.Version, rec.ObjectPath)
	if err != nil {
		slog.Warn("context: failed to resolve skill content",
			"name", rec.Name, "err", err)
		return ""
	}
	content, err := cb.skillsLoader.LoadSkillContent(localPath)
	if err != nil {
		slog.Warn("context: failed to read skill content",
			"name", rec.Name, "err", err)
		return ""
	}
	return content
}

// rankedMemory is a scored memory entry for context injection ordering.
type rankedMemory struct {
	text     string
	category string
	score    float64
}

// rankMemories scores and sorts vector results by recency + salience.
// Permanent memories always get recency=1.0; temporal ones decay with a 7-day half-life.
func rankMemories(results []types.VectorResult) []rankedMemory {
	now := time.Now()
	var ranked []rankedMemory
	for _, r := range results {
		text, _ := r.Payload["text"].(string)
		if text == "" {
			continue
		}
		cat, _ := r.Payload["category"].(string)
		perm, _ := r.Payload["permanent"].(bool)

		rec := memRecency(now, r.Payload, perm)
		sal := memSalience(r.Payload)
		score := 0.70*rec + 0.30*sal

		ranked = append(ranked, rankedMemory{text: text, category: cat, score: score})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	return ranked
}

func memRecency(now time.Time, p map[string]any, perm bool) float64 {
	if perm {
		return 1.0
	}
	if ts, ok := p["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			days := now.Sub(t).Hours() / 24
			return math.Exp(-math.Ln2 / 7.0 * days)
		}
	}
	return 0.5
}

func memSalience(p map[string]any) float64 {
	ac := payloadIntCB(p, "access_count")
	return math.Min(math.Log2(float64(ac)+1)/5.0, 1.0)
}

func payloadIntCB(p map[string]any, key string) int {
	switch v := p[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return 0
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func (cb *ContextBuilder) buildRuntimeContext(ctx agent.ReadonlyContext) string {
	now := time.Now().Format("2006-01-02 15:04 (Monday)")
	rt := fmt.Sprintf("%s %s, Go %s", runtime.GOOS, runtime.GOARCH, runtime.Version())

	var sb strings.Builder
	fmt.Fprintf(&sb, "## Runtime\n\nTime: %s\nPlatform: %s", now, rt)

	channel := stateStr(ctx, "channel")
	chatID := stateStr(ctx, "chat_id")
	if channel != "" {
		fmt.Fprintf(&sb, "\nChannel: %s", channel)
	}
	if chatID != "" {
		fmt.Fprintf(&sb, "\nChat ID: %s", chatID)
	}

	return sb.String()
}
