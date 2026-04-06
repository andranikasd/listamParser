# listamParser

Scrapes apartment listings from [list.am](https://list.am) and outputs a CSV, searchable HTML table, and JSON state file. Optionally uses Claude AI to filter results with natural language.

## Requirements

- Go 1.22+
- [FlareSolverr](https://github.com/FlareSolverr/FlareSolverr) running locally (bypasses Cloudflare)

## Setup

**1. Start FlareSolverr**
```bash
docker compose up -d
```

**2. Configure environment**
```bash
cp .env.example .env
# Edit .env and set ANTHROPIC_API_KEY if you want AI filtering
```

**3. Build**
```bash
go build -o listamparser .
```

## Usage

**Basic — scrape and filter by keyword**
```bash
./listamparser
# defaults: buy mode, $45k–$95k, only titles containing "Տիգրան Մեծ"
```

**Rent mode**
```bash
./listamparser -mode rent -min 400 -max 900
```

**Custom keyword filter**
```bash
./listamparser -only "Տիգրան Մեծ" -exclude "Նորաշեն"
```

**AI filtering — natural language query applied after scraping**
```bash
./listamparser -ai -prompt "Apartments near Komitas with exactly 2 rooms, below 5th floor"
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `-mode` | `buy` | `buy` or `rent` |
| `-min` | `45000` | Minimum price (USD; use monthly amount for rent) |
| `-max` | `95000` | Maximum price |
| `-only` | `Տиgrran Мets` | Comma-separated title keywords to include |
| `-exclude` | — | Comma-separated title keywords to exclude |
| `-ai` | `false` | Filter results with Claude AI after scraping |
| `-prompt` | — | Natural-language query for AI filtering (required with `-ai`) |
| `-model` | `claude-opus-4-6` | Claude model to use |
| `-api-key` | `$ANTHROPIC_API_KEY` | Anthropic API key |
| `-flaresolverr` | `$FLARESOLVERR_URL` or `http://localhost:8191` | FlareSolverr URL |
| `-csv` | `announcements.csv` | CSV output path |
| `-html` | `announcements.html` | HTML output path |
| `-json` | `data.json` | State file (tracks seen listings between runs) |

## Output

| File | Description |
|---|---|
| `announcements.html` | Searchable, sortable table — open in any browser |
| `announcements.csv` | Spreadsheet-friendly export |
| `data.json` | State file; new listings since last run are printed to stdout |

## How AI filtering works

Without `-ai`: CSS selectors extract listings, `-only`/`-exclude` keyword flags filter them.

With `-ai`: all pages are scraped first, then the full dataset is sent to Claude as JSON in a single API call. Claude returns the links of matching listings based on your prompt. More flexible than keyword matching — understands context, proximity, floor preferences, etc.
