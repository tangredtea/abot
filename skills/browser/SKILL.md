---
name: browser
description: "Browser automation using Playwright. Navigate pages, interact with elements, extract data, and automate web workflows."
---

# Browser Automation

Use Playwright for browser automation. Requires `playwright` and `playwright-cli` installed.

## Installation

```bash
npm install -g playwright playwright-cli
playwright install chromium
```

## Basic Navigation

```bash
# Open a URL
playwright open https://example.com

# Take screenshot
playwright screenshot https://example.com output.png

# Generate PDF
playwright pdf https://example.com output.pdf
```

## Interactive Mode

```bash
# Launch browser with inspector
playwright codegen https://example.com
```

## Scripting with Node.js

Create `script.js`:
```javascript
const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage();

  await page.goto('https://example.com');
  await page.screenshot({ path: 'screenshot.png' });

  await browser.close();
})();
```

Run: `node script.js`

## Common Patterns

### Login Flow
```javascript
await page.goto('https://example.com/login');
await page.fill('input[name="username"]', 'user');
await page.fill('input[name="password"]', 'pass');
await page.click('button[type="submit"]');
await page.waitForNavigation();
```

### Extract Data
```javascript
const title = await page.textContent('h1');
const links = await page.$$eval('a', els => els.map(e => e.href));
```

### Wait for Elements
```javascript
await page.waitForSelector('.content');
await page.waitForLoadState('networkidle');
```

## Tips

- Use `page.pause()` for debugging
- Save cookies with `context.storageState()` for session reuse
- Use headless mode for production: `chromium.launch({ headless: true })`
