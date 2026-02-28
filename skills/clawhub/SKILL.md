---
name: clawhub
description: Search and install agent skills from ClawHub, the public skill registry. Use when the user asks to find, install, or update skills.
---

# ClawHub

Public skill registry for AI agents. Search by natural language (vector search).

## When to Use

- "find a skill for …"
- "search for skills"
- "install a skill"
- "what skills are available?"

## Search

Use the `find_skills` tool with a natural language query:
```
find_skills query="web scraping" limit=5
```

## Install

Use the `install_skill` tool with the slug from search results:
```
install_skill slug="rotate-pdf" registry="clawhub"
```

The skill is downloaded, scanned, and stored in the object store. It becomes available after the next session.

## Notes

- No API key needed for search and install
- Skills are moderated — malware-flagged packages are blocked
- After install, remind the user to start a new session to load the skill
