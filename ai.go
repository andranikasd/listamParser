package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const defaultAIModel = "claude-opus-4-6"

const sysPrompt = `You are an apartment search assistant.
You receive a JSON array of scraped apartment listings and a user query.
Each listing has these fields:
  link, title, price (USD), region, rooms (count), size (m²),
  actual_floor, max_floor, sq_price (USD/m²), labels.

Return ONLY the "link" values of listings that match the user's query, as a JSON array of strings.
Example: ["https://list.am/item/12345", "https://list.am/item/67890"]

If nothing matches, return an empty array: []
Return ONLY valid JSON — no markdown, no explanation, nothing else.`

// listingForAI is a compact representation sent to Claude (avoids image URLs and noise).
type listingForAI struct {
	Link        string  `json:"link"`
	Title       string  `json:"title"`
	Price       int     `json:"price"`
	Region      string  `json:"region"`
	Rooms       int     `json:"rooms"`
	Size        int     `json:"size"`
	Floor       string  `json:"floor"`
	SqPrice     float64 `json:"sq_price"`
	Labels      string  `json:"labels,omitempty"`
}

type aiClient struct {
	c     *anthropic.Client
	model string
}

func newAIClient(apiKey, model string) *aiClient {
	if model == "" {
		model = defaultAIModel
	}
	c := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &aiClient{c: &c, model: model}
}

// filterListings sends the full scraped dataset as JSON to Claude and returns
// only the listings whose links Claude selected as matching the prompt.
func (ai *aiClient) filterListings(anns []*Announcement, userPrompt string) ([]*Announcement, error) {
	// Build compact listing structs for the AI.
	compact := make([]listingForAI, len(anns))
	for i, a := range anns {
		compact[i] = listingForAI{
			Link:    a.Link,
			Title:   a.Title,
			Price:   a.Price,
			Region:  a.Region,
			Rooms:   a.Rooms,
			Size:    a.Size,
			Floor:   a.ActualFloor + "/" + a.MaxFloor,
			SqPrice: a.SqPrice,
			Labels:  a.Labels,
		}
	}

	data, err := json.Marshal(compact)
	if err != nil {
		return nil, fmt.Errorf("marshal listings: %w", err)
	}
	logf("AI: sending %d listings (%d bytes) to Claude (%s)", len(anns), len(data), ai.model)

	msg, err := ai.c.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     anthropic.Model(ai.model),
		MaxTokens: 4096,
		System: []anthropic.TextBlockParam{
			{Text: sysPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(
				userPrompt + "\n\nListings:\n" + string(data),
			)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("claude API: %w", err)
	}

	raw := ""
	for _, block := range msg.Content {
		if block.Type == "text" {
			raw = block.Text
			break
		}
	}
	if raw == "" {
		return nil, fmt.Errorf("claude returned empty response")
	}

	// Strip optional markdown fences.
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		if i := strings.Index(raw, "\n"); i != -1 {
			raw = raw[i+1:]
		}
		if i := strings.LastIndex(raw, "```"); i != -1 {
			raw = raw[:i]
		}
		raw = strings.TrimSpace(raw)
	}

	var links []string
	if err := json.Unmarshal([]byte(raw), &links); err != nil {
		preview := raw
		if len(preview) > 400 {
			preview = preview[:400] + "..."
		}
		return nil, fmt.Errorf("parse AI response: %w\npreview: %s", err, preview)
	}

	// Build a set of matching links for O(1) lookup.
	matched := make(map[string]bool, len(links))
	for _, l := range links {
		matched[l] = true
	}

	var result []*Announcement
	for _, a := range anns {
		if matched[a.Link] {
			result = append(result, a)
		}
	}
	logf("AI: %d / %d listings matched", len(result), len(anns))
	return result, nil
}
