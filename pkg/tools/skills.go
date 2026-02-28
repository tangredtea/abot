package tools

import (
	"fmt"
	"strings"
	"time"

	"abot/pkg/types"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// --- find_skills ---

type findSkillsArgs struct {
	Query string `json:"query" jsonschema:"Search query for skills"`
	Limit int    `json:"limit,omitempty" jsonschema:"Max results (1-20, default 5)"`
}

type findSkillsHit struct {
	Slug        string  `json:"slug"`
	DisplayName string  `json:"display_name"`
	Summary     string  `json:"summary"`
	Version     string  `json:"version"`
	Registry    string  `json:"registry"`
	Score       float64 `json:"score"`
}

type findSkillsResult struct {
	Results []findSkillsHit `json:"results,omitempty"`
	Error   string          `json:"error,omitempty"`
}

func newFindSkills(deps *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "find_skills",
		Description: "Search remote skill registries for available skills.",
	}, func(ctx tool.Context, args findSkillsArgs) (findSkillsResult, error) {
		if deps.SkillSearcher == nil {
			return findSkillsResult{Error: "skill search not configured"}, nil
		}
		limit := args.Limit
		if limit <= 0 || limit > 20 {
			limit = 5
		}
		hits, err := deps.SkillSearcher.SearchAll(ctx, args.Query, limit)
		if err != nil {
			return findSkillsResult{Error: fmt.Sprintf("search failed: %v", err)}, nil
		}
		results := make([]findSkillsHit, 0, len(hits))
		for _, h := range hits {
			results = append(results, findSkillsHit{
				Slug:        h.Slug,
				DisplayName: h.DisplayName,
				Summary:     h.Summary,
				Version:     h.Version,
				Registry:    h.RegistryName,
				Score:       h.Score,
			})
		}
		return findSkillsResult{Results: results}, nil
	})
	return t
}

// --- install_skill ---

type installSkillArgs struct {
	Slug     string `json:"slug" jsonschema:"Skill slug identifier"`
	Version  string `json:"version,omitempty" jsonschema:"Specific version (default latest)"`
	Registry string `json:"registry,omitempty" jsonschema:"Registry name (if multiple)"`
}

type installSkillResult struct {
	Result  string `json:"result,omitempty"`
	Warning string `json:"warning,omitempty"`
	Error   string `json:"error,omitempty"`
}

func newInstallSkill(deps *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "install_skill",
		Description: "Download a skill from a remote registry and install it for the current tenant.",
	}, func(ctx tool.Context, args installSkillArgs) (installSkillResult, error) {
		if deps.SkillInstaller == nil || deps.ObjectStore == nil {
			return installSkillResult{Error: "skill installation not configured"}, nil
		}
		if args.Slug == "" {
			return installSkillResult{Error: "slug is required"}, nil
		}

		tenantID := stateStr(ctx, "tenant_id")

		version := args.Version
		if version == "" {
			version = "latest"
		}
		objectPath := fmt.Sprintf("skills/%s/%s.tar.gz", args.Slug, version)

		res, err := deps.SkillInstaller.Install(
			ctx, args.Slug, version, args.Registry,
			deps.ObjectStore, objectPath,
		)
		if err != nil {
			return installSkillResult{Error: fmt.Sprintf("install failed: %v", err)}, nil
		}
		if res.IsMalwareBlocked {
			return installSkillResult{Error: "skill blocked: malware detected"}, nil
		}

		// Register in global registry
		record := &types.SkillRecord{
			Name:       args.Slug,
			Version:    res.Version,
			ObjectPath: objectPath,
			Tier:       types.SkillTierGlobal,
			Status:     types.StatusPublished,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		if err := deps.SkillRegistryStore.Put(ctx, record); err != nil {
			return installSkillResult{Error: fmt.Sprintf("registry save failed: %v", err)}, nil
		}

		// Install for tenant
		if tenantID != "" {
			// Re-fetch to get the auto-generated ID
			saved, _ := deps.SkillRegistryStore.Get(ctx, args.Slug)
			if saved != nil {
				ts := &types.TenantSkill{
					TenantID:    tenantID,
					SkillID:     saved.ID,
					InstalledAt: time.Now(),
				}
				_ = deps.TenantSkillStore.Install(ctx, ts)
			}
		}

		out := installSkillResult{Result: fmt.Sprintf("installed %s@%s", args.Slug, res.Version)}
		if res.IsSuspicious {
			out.Warning = "skill flagged as suspicious: " + res.Summary
		}
		return out, nil
	})
	return t
}

// --- create_skill ---

type createSkillArgs struct {
	Name        string `json:"name" jsonschema:"Skill name"`
	Description string `json:"description" jsonschema:"What this skill does"`
	Content     string `json:"content" jsonschema:"SKILL.md content"`
}

type createSkillResult struct {
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

func newCreateSkill(deps *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:        "create_skill",
		Description: "Create a new skill and submit it for review. Uploads content to object storage and creates a proposal.",
	}, func(ctx tool.Context, args createSkillArgs) (createSkillResult, error) {
		if args.Name == "" || args.Content == "" {
			return createSkillResult{Error: "name and content are required"}, nil
		}
		if deps.ObjectStore == nil || deps.ProposalStore == nil {
			return createSkillResult{Error: "skill creation not configured"}, nil
		}

		proposedBy := stateStr(ctx, "tenant_id")

		objectPath := fmt.Sprintf("proposals/%s/%d/SKILL.md", args.Name, time.Now().UnixNano())
		if err := deps.ObjectStore.Put(ctx, objectPath, strings.NewReader(args.Content)); err != nil {
			return createSkillResult{Error: fmt.Sprintf("upload failed: %v", err)}, nil
		}

		proposal := &types.SkillProposal{
			SkillName:  args.Name,
			ProposedBy: proposedBy,
			ObjectPath: objectPath,
			Status:     "pending",
			CreatedAt:  time.Now(),
		}
		if err := deps.ProposalStore.Create(ctx, proposal); err != nil {
			return createSkillResult{Error: fmt.Sprintf("proposal failed: %v", err)}, nil
		}
		return createSkillResult{Result: fmt.Sprintf("skill %q submitted for review", args.Name)}, nil
	})
	return t
}

// --- promote_skill ---

type promoteSkillArgs struct {
	ProposalID int64 `json:"proposal_id" jsonschema:"Proposal ID to approve and promote to global registry"`
}

type promoteSkillResult struct {
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

func newPromoteSkill(deps *Deps) tool.Tool {
	t, _ := functiontool.New(functiontool.Config{
		Name:                "promote_skill",
		Description:         "Approve a skill proposal and register it in the global skill registry. Requires confirmation.",
		RequireConfirmation: true,
	}, func(ctx tool.Context, args promoteSkillArgs) (promoteSkillResult, error) {
		if deps.ProposalStore == nil {
			return promoteSkillResult{Error: "proposal store not configured"}, nil
		}

		// Only the proposal's originating tenant can promote it.
		callerTenant := stateStr(ctx, "tenant_id")

		proposal, err := deps.ProposalStore.Get(ctx, args.ProposalID)
		if err != nil {
			return promoteSkillResult{Error: fmt.Sprintf("proposal not found: %v", err)}, nil
		}
		if proposal.Status != "pending" {
			return promoteSkillResult{Error: fmt.Sprintf("proposal status is %q, not pending", proposal.Status)}, nil
		}
		if proposal.ProposedBy != callerTenant {
			return promoteSkillResult{Error: "permission denied: proposal belongs to another tenant"}, nil
		}

		// Determine reviewer
		reviewedBy := stateStr(ctx, "user_id")

		// Register in global registry
		record := &types.SkillRecord{
			Name:       proposal.SkillName,
			ObjectPath: proposal.ObjectPath,
			Tier:       types.SkillTierGlobal,
			Status:     types.StatusPublished,
			Version:    "1.0.0",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		if err := deps.SkillRegistryStore.Put(ctx, record); err != nil {
			return promoteSkillResult{Error: fmt.Sprintf("registry save failed: %v", err)}, nil
		}

		// Update proposal status
		_ = deps.ProposalStore.UpdateStatus(ctx, args.ProposalID, "approved", reviewedBy)

		return promoteSkillResult{Result: fmt.Sprintf("skill %q promoted to global registry", proposal.SkillName)}, nil
	})
	return t
}
