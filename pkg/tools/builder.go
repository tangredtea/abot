package tools

import (
	"log/slog"

	"google.golang.org/adk/tool"
)

// BuildAllTools constructs all built-in tools and returns them as ADK-Go native tool.Tool slice.
// Called by bootstrap, results injected into llmagent.Config.Tools.
func BuildAllTools(deps *Deps) ([]tool.Tool, error) {
	type named struct {
		name string
		t    tool.Tool
	}
	candidates := []named{
		{"read_file", newReadFile(deps)},
		{"write_file", newWriteFile(deps)},
		{"edit_file", newEditFile(deps)},
		{"append_file", newAppendFile(deps)},
		{"list_dir", newListDir(deps)},
		{"exec", newExec(deps)},
		{"web_search", newWebSearch(deps)},
		{"web_fetch", newWebFetch(deps)},
		{"message", newMessage(deps)},
		{"spawn", newSpawn(deps)},
		{"cron", newCron(deps)},
		{"find_skills", newFindSkills(deps)},
		{"install_skill", newInstallSkill(deps)},
		{"create_skill", newCreateSkill(deps)},
		{"promote_skill", newPromoteSkill(deps)},
		{"save_memory", newSaveMemory(deps)},
		{"search_memory", newSearchMemory(deps)},
		{"update_doc", newUpdateDoc(deps)},
	}

	if deps.Subagent != nil {
		candidates = append(candidates,
			named{"subagent", newSubagent(deps)},
			named{"list_tasks", newListTasks(deps)},
		)
	}

	var tools []tool.Tool
	for _, c := range candidates {
		if c.t == nil {
			slog.Warn("tools: returned nil, skipped", "tool", c.name)
			continue
		}
		tools = append(tools, wrapGuard(c.name, c.t, deps))
	}
	return tools, nil
}
