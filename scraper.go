package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	listBase = "https://www.list.am"
	catBuy   = 60
	catRent  = 56
)

// Config holds all runtime search parameters.
type Config struct {
	Mode            string // "buy" or "rent"
	MinPrice        int
	MaxPrice        int
	FlareSolverrURL string
	ExcludeKeywords []string
	OnlyKeywords    []string
}

// Announcement represents a single listing scraped from list.am.
type Announcement struct {
	Link        string  `json:"link"`
	Title       string  `json:"title"`
	Price       int     `json:"price"`
	ImageURL    string  `json:"image_url"`
	Region      string  `json:"region"`
	Labels      string  `json:"labels"`
	ActualFloor string  `json:"actual_floor"`
	MaxFloor    string  `json:"max_floor"`
	Size        int     `json:"size"`
	Rooms       int     `json:"rooms"`
	SqPrice     float64 `json:"sq_price"`
}

// Hash returns an MD5 fingerprint of (link+price) used for change detection.
func (a *Announcement) Hash() string {
	h := md5.Sum([]byte(a.Link + strconv.Itoa(a.Price)))
	return fmt.Sprintf("%x", h)
}

var (
	// floorRe matches "3/9" style floor notation in the attributes string.
	floorRe  = regexp.MustCompile(`(\d+)/(\d+)`)
	numberRe = regexp.MustCompile(`\d+`)
)

func pageURL(cat, page, minP, maxP int, mode string) string {
	u := fmt.Sprintf("%s/category/%d/%d?n=1&price1=%d&price2=%d", listBase, cat, page, minP, maxP)
	if mode == "buy" {
		// Extra filters: new buildings (_a39=2) with renovation (_a11_1=4)
		u += "&_a39=2&_a11_1=4"
	}
	return u
}

func fetchPage(fsURL, target string) (string, error) {
	payload, _ := json.Marshal(map[string]interface{}{
		"cmd":        "request.get",
		"url":        target,
		"maxTimeout": 60000,
	})
	resp, err := http.Post(fsURL+"/v1", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	sol, ok := result["solution"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("FlareSolverr: missing solution in response")
	}
	html, ok := sol["response"].(string)
	if !ok {
		return "", fmt.Errorf("FlareSolverr: missing html in solution")
	}
	return html, nil
}

func parseItem(sel *goquery.Selection, cfg *Config) (*Announcement, bool) {
	link, _ := sel.Attr("href")
	if link == "" {
		return nil, false
	}

	// Price: strip $, €, commas and non-breaking spaces.
	price := 9_999_999
	if raw := strings.TrimSpace(sel.Find("div.p").First().Text()); raw != "" {
		clean := strings.NewReplacer("$", "", "€", "", ",", "", "\u00a0", "", " ", "").Replace(raw)
		if p, err := strconv.Atoi(clean); err == nil {
			price = p
		}
	}

	labels := strings.TrimSpace(sel.Find("div.clabel").First().Text())
	title := strings.TrimSpace(sel.Find("div.l").First().Text())

	for _, kw := range cfg.ExcludeKeywords {
		if strings.Contains(title, kw) {
			return nil, false
		}
	}
	if len(cfg.OnlyKeywords) > 0 {
		matched := false
		for _, kw := range cfg.OnlyKeywords {
			if strings.Contains(title, kw) {
				matched = true
				break
			}
		}
		if !matched {
			return nil, false
		}
	}

	imgURL, _ := sel.Find("img").First().Attr("src")

	// Attributes are comma-separated: region, rooms, size, floor
	parts := strings.Split(strings.TrimSpace(sel.Find("div.at").First().Text()), ",")

	region := ""
	if len(parts) > 0 {
		region = strings.TrimSpace(parts[0])
	}
	rooms := 0
	if len(parts) > 1 {
		if m := numberRe.FindString(parts[1]); m != "" {
			rooms, _ = strconv.Atoi(m)
		}
	}
	size := 1
	if len(parts) > 2 {
		if m := numberRe.FindString(parts[2]); m != "" {
			if v, _ := strconv.Atoi(m); v > 0 {
				size = v
			}
		}
	}
	actualFloor, maxFloor := "?", "?"
	if len(parts) > 3 {
		if m := floorRe.FindStringSubmatch(parts[3]); len(m) == 3 {
			actualFloor, maxFloor = m[1], m[2]
		}
	}

	return &Announcement{
		Link:        listBase + link,
		Title:       title,
		Price:       price,
		ImageURL:    imgURL,
		Region:      region,
		Labels:      labels,
		ActualFloor: actualFloor,
		MaxFloor:    maxFloor,
		Size:        size,
		Rooms:       rooms,
		SqPrice:     math.Round(float64(price)/float64(size)*100) / 100,
	}, true
}

func scrapeAll(cfg *Config) ([]*Announcement, error) {
	cat := catBuy
	if cfg.Mode == "rent" {
		cat = catRent
	}

	var all []*Announcement
	for page := 1; ; page++ {
		url := pageURL(cat, page, cfg.MinPrice, cfg.MaxPrice, cfg.Mode)
		logf("page %d → %s", page, url)

		html, err := fetchPage(cfg.FlareSolverrURL, url)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", page, err)
		}

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
		if err != nil {
			return nil, fmt.Errorf("parse page %d: %w", page, err)
		}

		items := doc.Find("div.dl div.gl a")
		n := items.Length()
		logf("page %d: %d items", page, n)
		if n == 0 {
			break
		}

		items.Each(func(_ int, s *goquery.Selection) {
			if a, ok := parseItem(s, cfg); ok {
				all = append(all, a)
			}
		})

		if n < 28 {
			break // last page
		}
		time.Sleep(time.Second)
	}
	return all, nil
}
