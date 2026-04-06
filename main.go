package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env if present; ignore error if file doesn't exist.
	_ = godotenv.Load()

	mode    := flag.String("mode",         "buy",                   "search mode: 'buy' or 'rent'")
	minP    := flag.Int("min",             45000,                   "minimum price in USD (for rent, use monthly amount e.g. 400)")
	maxP    := flag.Int("max",             95000,                   "maximum price in USD (for rent, use monthly amount e.g. 900)")
	fsURL   := flag.String("flaresolverr", envOr("FLARESOLVERR_URL", "http://localhost:8191"), "FlareSolverr base URL")
	excl    := flag.String("exclude",      "",                      "comma-separated title keywords to exclude")
	only    := flag.String("only",         "Տիգրան Մեծ",           "comma-separated title keywords; only include matches")
	csvOut  := flag.String("csv",          "announcements.csv",     "CSV output path")
	htmlOut := flag.String("html",         "announcements.html",    "HTML output path")
	jsonOut := flag.String("json",         "data.json",             "JSON state file (used for new-listing detection)")

	// AI flags
	useAI    := flag.Bool("ai",        false,                           "scrape all pages then filter results with Claude AI")
	apiKey   := flag.String("api-key",  os.Getenv("ANTHROPIC_API_KEY"), "Anthropic API key (default: $ANTHROPIC_API_KEY)")
	aiModel  := flag.String("model",    defaultAIModel,                  "Claude model for AI filtering")
	aiPrompt := flag.String("prompt",   "",                              "natural-language filter sent to Claude after scraping (requires -ai)")
	flag.Parse()

	if *mode != "buy" && *mode != "rent" {
		log.Fatalf("-mode must be 'buy' or 'rent', got: %q", *mode)
	}
	if *useAI && *apiKey == "" {
		log.Fatal("-ai requires an Anthropic API key via -api-key or $ANTHROPIC_API_KEY")
	}
	if *useAI && *aiPrompt == "" {
		log.Fatal("-ai requires a -prompt describing what to find")
	}

	cfg := &Config{
		Mode:            *mode,
		MinPrice:        *minP,
		MaxPrice:        *maxP,
		FlareSolverrURL: *fsURL,
		ExcludeKeywords: splitCSV(*excl),
		OnlyKeywords:    splitCSV(*only),
	}
	logf("mode=%s  price=%d–%d  flaresolverr=%s", cfg.Mode, cfg.MinPrice, cfg.MaxPrice, cfg.FlareSolverrURL)

	anns, err := scrapeAll(cfg)
	if err != nil {
		log.Fatalf("scrape failed: %v", err)
	}
	sort.Slice(anns, func(i, j int) bool { return anns[i].SqPrice < anns[j].SqPrice })

	// AI filtering: send the full scraped dataset as JSON, Claude returns matching links.
	if *useAI {
		ai := newAIClient(*apiKey, *aiModel)
		logf("AI filter: model=%s  prompt=%q", ai.model, *aiPrompt)
		filtered, err := ai.filterListings(anns, *aiPrompt)
		if err != nil {
			log.Fatalf("AI filter failed: %v", err)
		}
		anns = filtered
	}

	seen, err := loadSeen(*jsonOut)
	if err != nil {
		logf("warn: cannot read %s: %v", *jsonOut, err)
		seen = map[string]bool{}
	}

	newCount := 0
	for _, a := range anns {
		if !seen[a.Hash()] {
			fmt.Printf("NEW  %-55s $%7d  $%5.0f/m²  %s\n", a.Title, a.Price, a.SqPrice, a.Link)
			newCount++
		}
	}
	fmt.Printf("\n%d new listing(s) found (total scraped: %d)\n\n", newCount, len(anns))

	outputs := []struct {
		name string
		fn   func() error
	}{
		{*jsonOut, func() error { return saveJSON(*jsonOut, anns) }},
		{*csvOut, func() error { return writeCSV(*csvOut, anns) }},
		{*htmlOut, func() error { return writeHTML(*htmlOut, anns, cfg.Mode) }},
	}
	for _, o := range outputs {
		if err := o.fn(); err != nil {
			log.Printf("error writing %s: %v", o.name, err)
		} else {
			logf("saved → %s", o.name)
		}
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if kw := strings.TrimSpace(p); kw != "" {
			out = append(out, kw)
		}
	}
	return out
}

// logf writes to stderr so it doesn't mix with the NEW-listing output on stdout.
func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[listamparser] "+format+"\n", args...)
}

// envOr returns the environment variable value or fallback if unset/empty.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
