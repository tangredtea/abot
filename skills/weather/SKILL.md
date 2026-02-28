---
name: weather
description: Get current weather and forecasts using free services (no API key required). Use when the user asks about weather conditions or forecasts.
---

# Weather

Two free services, no API keys needed.

## wttr.in (primary)

Quick one-liner via `exec` tool:
```bash
curl -s "wttr.in/London?format=3"
# Output: London: ⛅️ +8°C
```

Compact format:
```bash
curl -s "wttr.in/London?format=%l:+%c+%t+%h+%w"
```

Full forecast:
```bash
curl -s "wttr.in/London?T"
```

Tips:
- URL-encode spaces: `wttr.in/New+York`
- Airport codes: `wttr.in/JFK`
- Units: `?m` (metric) `?u` (USCS)
- Today only: `?1` · Current only: `?0`

## Open-Meteo (fallback, JSON)

Free, no key, good for programmatic use:
```bash
curl -s "https://api.open-meteo.com/v1/forecast?latitude=51.5&longitude=-0.12&current_weather=true"
```

Docs: https://open-meteo.com/en/docs
