package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Offer is one product listing found at a store.
type Offer struct {
	StoreID     string   `json:"storeId"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Image       string   `json:"image,omitempty"`
	Price       float64  `json:"price"`
	Currency    string   `json:"currency"`
	InStock     *bool    `json:"inStock,omitempty"`
	StockQty    int      `json:"stockQty,omitempty"`
	MinQty      int      `json:"minQty,omitempty"`
	Brand       string   `json:"brand,omitempty"`
	Relevance   float64  `json:"relevance"`
	SellerName  string   `json:"sellerName,omitempty"`
	SellerPct   float64  `json:"sellerPct,omitempty"`   // marketplace seller positive feedback %
	SellerTrust *float64 `json:"sellerTrust,omitempty"` // 1–5 override of store trust for this seller
	Condition   string   `json:"condition,omitempty"`
}

// Source fetches live offers from one store.
type Source interface {
	StoreID() string
	// Ready reports whether the source can run (e.g. API keys present).
	Ready(s Settings) (bool, string)
	Search(ctx context.Context, q string, s Settings) ([]Offer, error)
}

var sources = []Source{
	&shopifySource{id: "thepihut", base: "https://thepihut.com", currency: "GBP"},
	&shopifySource{id: "pimoroni", base: "https://shop.pimoroni.com", currency: "GBP"},
	&shopifySource{id: "lilygo", base: "https://lilygo.cc", currency: "USD"},
	&shopifySource{id: "m5stack", base: "https://shop.m5stack.com", currency: "USD"},
	&shopifySource{id: "robotshop", base: "https://www.robotshop.com", currency: "USD"},
	&adafruitSource{},
	&lcscSource{},
	&mouserSource{},
	&digikeySource{},
	&ebaySource{},
}

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Timeout: 20 * time.Second, Jar: jar}
}

var httpClient = newClient()

func doJSON(ctx context.Context, c *http.Client, req *http.Request, out any) error {
	req = req.WithContext(ctx)
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", userAgent)
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	}
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		snippet := strings.TrimSpace(string(body))
		if len(snippet) > 160 {
			snippet = snippet[:160] + "…"
		}
		if strings.HasPrefix(snippet, "<") {
			snippet = "site returned an HTML page (likely bot protection)"
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, snippet)
	}
	if err := json.Unmarshal(body, out); err != nil {
		if len(body) > 0 && body[0] == '<' {
			return fmt.Errorf("site returned HTML instead of data (blocked or changed)")
		}
		return fmt.Errorf("unexpected response: %v", err)
	}
	return nil
}

func getJSON(ctx context.Context, c *http.Client, url string, out any) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	return doJSON(ctx, c, req, out)
}

// ---------- helpers ----------

var numRe = regexp.MustCompile(`[0-9][0-9.,]*`)

// parsePrice turns "$1,234.50", "1.234,50 €", "12.5" or a JSON number into a float.
func parsePrice(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	case string:
		m := numRe.FindString(t)
		if m == "" {
			return 0
		}
		lastDot, lastComma := strings.LastIndex(m, "."), strings.LastIndex(m, ",")
		switch {
		case lastDot >= 0 && lastComma >= 0:
			if lastComma > lastDot { // 1.234,56
				m = strings.ReplaceAll(m, ".", "")
				m = strings.Replace(m, ",", ".", 1)
			} else { // 1,234.56
				m = strings.ReplaceAll(m, ",", "")
			}
		case lastComma >= 0:
			// "12,50" → decimal comma; "1,234" → thousands
			if len(m)-lastComma-1 == 3 && strings.Count(m, ",") >= 1 && lastComma > 0 {
				m = strings.ReplaceAll(m, ",", "")
			} else {
				m = strings.ReplaceAll(m, ",", ".")
			}
		}
		m = strings.TrimRight(m, ".")
		f, _ := strconv.ParseFloat(m, 64)
		return f
	}
	return 0
}

func str(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case nil:
		return ""
	}
	return fmt.Sprint(v)
}

func boolPtr(b bool) *bool { return &b }

var tokenRe = regexp.MustCompile(`[a-z0-9]+`)

func normalize(s string) string {
	s = strings.ToLower(s)
	return strings.Join(tokenRe.FindAllString(s, -1), "")
}

// relevance scores how well a title matches the query (0–1).
// Tokens are matched against the title with spaces/punctuation removed, so
// "esp32 s3" matches "ESP32-S3" and "cc1101" matches "CC-1101".
func relevance(query, title string) float64 {
	toks := tokenRe.FindAllString(strings.ToLower(query), -1)
	if len(toks) == 0 {
		return 0
	}
	nt := normalize(title)
	whole := normalize(query)
	hit := 0
	for _, t := range toks {
		if strings.Contains(nt, t) {
			hit++
		}
	}
	score := float64(hit) / float64(len(toks))
	if strings.Contains(nt, whole) {
		score = 1
	}
	return score
}

func minRelevance(query string) float64 {
	n := len(tokenRe.FindAllString(strings.ToLower(query), -1))
	if n <= 2 {
		return 0.99
	}
	return 0.6
}
