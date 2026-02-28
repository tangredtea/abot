---
name: skill-creator
description: Create or update skills for the agent system. Use when designing, structuring, or packaging skills with scripts, references, and assets. Guides the agent through skill creation workflow.
---

# Skill Creator

Guide for creating effective skills that extend agent capabilities.

## Skill Structure

```
skill-name/
├── SKILL.md          (required — frontmatter + instructions)
├── scripts/          (optional — executable code)
├── references/       (optional — documentation for context)
└── assets/           (optional — templates, images, fonts)
```

## SKILL.md Format

```yaml
---
name: my-skill
description: What this skill does and when to use it.
---
```

Body contains markdown instructions loaded after the skill triggers.

## Creation Workflow

1. Understand the skill's use cases with concrete examples
2. Plan reusable contents (scripts, references, assets)
3. Create the skill directory and SKILL.md
4. Implement resources and write instructions
5. Upload to object store via `create_skill` tool
6. Submit for review via `promote_skill` tool
7. Iterate based on real usage

## Key Principles

- **Concise**: The context window is shared. Only include what the agent doesn't already know.
- **Progressive disclosure**: Metadata always loaded (~100 words), body on trigger (<5k words), resources on demand.
- **Match freedom to fragility**: Narrow bridge needs guardrails (scripts), open field allows many routes (text guidance).

## Naming

- Lowercase letters, digits, hyphens only
- Under 64 characters
- Prefer short, verb-led phrases (e.g., `rotate-pdf`, `gh-address-comments`)
