package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// fetchHTML GETs a page with full browser-like headers.
func fetchHTML(ctx context.Context, c *http.Client, u string, extra map[string]string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return "", err
	}
	h := req.Header
	h.Set("User-Agent", userAgent)
	h.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	h.Set("Accept-Language", "en-US,en;q=0.9")
	h.Set("Upgrade-Insecure-Requests", "1")
	h.Set("Sec-Fetch-Dest", "document")
	h.Set("Sec-Fetch-Mode", "navigate")
	h.Set("Sec-Fetch-Site", "none")
	h.Set("Sec-Fetch-User", "?1")
	h.Set("Sec-Ch-Ua", `"Chromium";v="131", "Not_A Brand";v="24", "Google Chrome";v="131"`)
	h.Set("Sec-Ch-Ua-Mobile", "?0")
	h.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	for k, v := range extra {
		h.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return string(b), fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return string(b), nil
}

var tagRe = regexp.MustCompile(`<[^>]+>`)

func cleanText(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}

// ratingTrust converts a product's star rating into a 1–5 trust value for
// marketplace listings, blending it with the marketplace's base trust.
func ratingTrust(base, stars float64, reviews int) *float64 {
	if stars <= 0 || reviews < 10 {
		return nil
	}
	t := base + (stars-4.2)*1.6
	switch {
	case reviews >= 1000:
		t += 0.4
	case reviews >= 100:
		t += 0.2
	}
	if t < 1 {
		t = 1
	}
	if t > 4.6 { // never rate a marketplace listing as high as an authorized distributor
		t = 4.6
	}
	return &t
}

// ---------- Amazon ----------

type amazonSource struct{}

func (amazonSource) StoreID() string               { return "amazon" }
func (amazonSource) Ready(Settings) (bool, string) { return true, "" }

var amazonSites = map[string][2]string{
	"NA": {"www.amazon.com", "USD"}, "UK": {"www.amazon.co.uk", "GBP"}, "EU": {"www.amazon.de", "EUR"},
	"ASIA": {"www.amazon.com", "USD"}, "OTHER": {"www.amazon.com", "USD"},
}

func (amazonSource) Search(ctx context.Context, q string, s Settings) ([]Offer, error) {
	site, ok := amazonSites[s.Region]
	if !ok {
		site = amazonSites["NA"]
	}
	c := newClient()
	page, err := fetchHTML(ctx, c, "https://"+site[0]+"/s?k="+url.QueryEscape(q), nil)
	if isAmazonBlock(page) {
		return nil, errors.New("Amazon showed a CAPTCHA (bot check). Use the direct link")
	}
	if err != nil {
		return nil, err
	}
	offers := parseAmazon(page, site[0], site[1])
	if len(offers) == 0 && !strings.Contains(page, "s-search-result") {
		return nil, errors.New("Amazon returned a page we couldn't read (layout changed or blocked)")
	}
	return offers, nil
}

func isAmazonBlock(page string) bool {
	return strings.Contains(page, "/errors/validateCaptcha") || strings.Contains(page, "api-services-support@amazon.com") ||
		strings.Contains(page, "Type the characters you see")
}

var (
	amzASIN    = regexp.MustCompile(`data-asin="([A-Z0-9]{10})"`)
	amzH2      = regexp.MustCompile(`(?s)<h2[^>]*>(.*?)</h2>`)
	amzH2Aria  = regexp.MustCompile(`<h2[^>]*aria-label="([^"]+)"`)
	amzPrice   = regexp.MustCompile(`<span class="a-offscreen">([^<]+)</span>`)
	amzImg     = regexp.MustCompile(`class="s-image"[^>]*?src="([^"]+)"|src="([^"]+)"[^>]*class="s-image"`)
	amzStars   = regexp.MustCompile(`([0-9][.,][0-9]) (?:out of|von) 5`)
	amzReviews = regexp.MustCompile(`aria-label="([0-9][0-9.,]*[Kk]?) (?:ratings?|Bewertungen)"|<span class="a-size-base s-underline-text">\(?([0-9][0-9.,]*[Kk]?)\)?</span>`)
)

func parseAmazon(page, host, cur string) []Offer {
	const marker = `data-component-type="s-search-result"`
	locs := regexp.MustCompile(regexp.QuoteMeta(marker)).FindAllStringIndex(page, -1)
	var out []Offer
	seen := map[string]bool{}
	for i, loc := range locs {
		end := len(page)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		p := page[loc[1]:end]
		if len(p) > 60000 {
			p = p[:60000]
		}
		// data-asin sits in the same tag, usually just before the marker.
		back := page[max(0, loc[0]-1500):loc[0]]
		asin := ""
		if ms := amzASIN.FindAllStringSubmatch(back, -1); ms != nil {
			asin = ms[len(ms)-1][1]
		} else if m := amzASIN.FindStringSubmatch(p[:min(len(p), 1500)]); m != nil {
			asin = m[1]
		}
		if asin == "" || seen[asin] {
			continue
		}
		title := ""
		if m := amzH2Aria.FindStringSubmatch(p); m != nil {
			title = cleanText(m[1])
		}
		if m := amzH2.FindStringSubmatch(p); m != nil && title == "" {
			title = cleanText(m[1])
		}
		title = strings.TrimPrefix(title, "Sponsored Ad - ")
		price := 0.0
		if m := amzPrice.FindStringSubmatch(p); m != nil {
			price = parsePrice(html.UnescapeString(m[1]))
		}
		if title == "" || price <= 0 {
			continue
		}
		seen[asin] = true
		o := Offer{StoreID: "amazon", Title: title, Price: price, Currency: cur,
			URL: "https://" + host + "/dp/" + asin, InStock: boolPtr(true)}
		if m := amzImg.FindStringSubmatch(p); m != nil {
			o.Image = m[1] + m[2]
		}
		if m := amzStars.FindStringSubmatch(p); m != nil {
			o.Rating = parsePrice(strings.Replace(m[1], ",", ".", 1))
		}
		if m := amzReviews.FindStringSubmatch(p); m != nil {
			o.Reviews = parseCount(m[1] + m[2])
		}
		o.SellerTrust = ratingTrust(3.5, o.Rating, o.Reviews)
		out = append(out, o)
	}
	return out
}

// parseCount handles "1,234", "1.234" and "2.5K".
func parseCount(s string) int {
	s = strings.TrimSpace(s)
	mult := 1.0
	if strings.HasSuffix(s, "K") || strings.HasSuffix(s, "k") {
		mult, s = 1000, s[:len(s)-1]
		return int(parsePrice(strings.Replace(s, ",", ".", 1)) * mult)
	}
	s = strings.NewReplacer(",", "", ".", "").Replace(s)
	return int(parsePrice(s) * mult)
}

// ---------- AliExpress ----------

type aliSource struct{}

func (aliSource) StoreID() string               { return "aliexpress" }
func (aliSource) Ready(Settings) (bool, string) { return true, "" }

var aliCurrency = map[string]string{"NA": "USD", "UK": "GBP", "EU": "EUR", "ASIA": "USD", "OTHER": "USD"}
var aliCountry = map[string]string{"NA": "US", "UK": "UK", "EU": "DE", "ASIA": "SG", "OTHER": "US"}

func (aliSource) Search(ctx context.Context, q string, s Settings) ([]Offer, error) {
	cur, ok := aliCurrency[s.Region]
	if !ok {
		cur = "USD"
	}
	slug := strings.Trim(regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(q, "-"), "-")
	u := "https://www.aliexpress.com/w/wholesale-" + url.PathEscape(slug) + ".html?SearchText=" + url.QueryEscape(q) + "&g=y"
	cookie := fmt.Sprintf("aep_usuc_f=site=glo&c_tp=%s&region=%s&b_locale=en_US; intl_locale=en_US; xman_us_f=x_locale=en_US", cur, aliCountry[s.Region])
	page, err := fetchHTML(ctx, newClient(), u, map[string]string{"Cookie": cookie, "Referer": "https://www.aliexpress.com/"})
	if strings.Contains(page, "punish") || strings.Contains(page, "x5secdata") || strings.Contains(page, "baxia") {
		return nil, errors.New("AliExpress showed a slider CAPTCHA (bot check). Use the direct link")
	}
	if err != nil {
		return nil, err
	}
	offers := parseAliExpress(page, cur)
	if len(offers) == 0 {
		return nil, errors.New("AliExpress returned a page we couldn't read (layout changed or blocked)")
	}
	return offers, nil
}

// parseAliExpress pulls the product list out of the JSON state that
// AliExpress embeds in its search page. Rather than depend on an exact
// path, it walks every embedded object looking for product-shaped records.
func parseAliExpress(page, cur string) []Offer {
	var out []Offer
	seen := map[string]bool{}
	for _, marker := range []string{"_init_data_", "window.runParams", "__INITIAL_STATE__", "_dida_config_"} {
		idx := 0
		for {
			i := strings.Index(page[idx:], marker)
			if i < 0 {
				break
			}
			start := idx + i
			idx = start + len(marker)
			blob := extractJSONObject(page[start:])
			if blob == "" {
				continue
			}
			if v, ok := decodeLoose(blob); ok {
				walkAli(v, cur, seen, &out, 0)
			}
		}
		if len(out) > 0 {
			break
		}
	}
	return out
}

// decodeLoose parses blob as JSON. If that fails (e.g. JS object syntax like
// "{ data: {...} }"), it tries the first few nested JSON objects inside it.
func decodeLoose(blob string) (any, bool) {
	var v any
	if json.Unmarshal([]byte(blob), &v) == nil {
		return v, true
	}
	off := 1
	for tries := 0; tries < 8 && off < len(blob); tries++ {
		i := strings.Index(blob[off:], `{"`)
		if i < 0 {
			break
		}
		inner := extractJSONObject(blob[off+i:])
		if inner != "" && json.Unmarshal([]byte(inner), &v) == nil {
			return v, true
		}
		off += i + 1
	}
	return nil, false
}

// extractJSONObject returns the first balanced {...} in s (string-aware).
func extractJSONObject(s string) string {
	start := strings.IndexByte(s, '{')
	if start < 0 || start > 200 {
		return ""
	}
	depth, inStr, esc := 0, false, false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case ch == '\\':
				esc = true
			case ch == '"':
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

func walkAli(v any, cur string, seen map[string]bool, out *[]Offer, depth int) {
	if depth > 40 {
		return
	}
	switch t := v.(type) {
	case []any:
		for _, x := range t {
			walkAli(x, cur, seen, out, depth+1)
		}
	case map[string]any:
		if o, ok := aliOffer(t, cur); ok {
			id := o.URL
			if !seen[id] {
				seen[id] = true
				*out = append(*out, o)
			}
			return
		}
		for _, x := range t {
			walkAli(x, cur, seen, out, depth+1)
		}
	}
}

func aliOffer(m map[string]any, cur string) (Offer, bool) {
	id := str(m["productId"])
	if id == "" {
		id = str(m["itemId"])
	}
	if id == "" || len(id) < 8 {
		return Offer{}, false
	}
	title := ""
	switch t := m["title"].(type) {
	case string:
		title = t
	case map[string]any:
		title = str(t["displayTitle"])
		if title == "" {
			title = str(t["seoTitle"])
		}
	}
	if title == "" {
		title = str(m["productTitle"])
	}
	price, pcur := 0.0, ""
	if pr, ok := m["prices"].(map[string]any); ok {
		for _, k := range []string{"salePrice", "originalPrice"} {
			if sp, ok := pr[k].(map[string]any); ok {
				price = parsePrice(sp["minPrice"])
				if price == 0 {
					price = parsePrice(str(sp["formattedPrice"]))
				}
				pcur = str(sp["currencyCode"])
				if price > 0 {
					break
				}
			}
		}
	}
	if price == 0 {
		for _, k := range []string{"salePrice", "price", "minPrice"} {
			switch p := m[k].(type) {
			case map[string]any:
				price = parsePrice(p["minPrice"])
				if price == 0 {
					price = parsePrice(str(p["formattedPrice"]))
				}
			case string, float64:
				price = parsePrice(p)
			}
			if price > 0 {
				break
			}
		}
	}
	if title == "" || price <= 0 {
		return Offer{}, false
	}
	o := Offer{StoreID: "aliexpress", Title: cleanText(title), Price: price, Currency: cur,
		URL: "https://www.aliexpress.com/item/" + id + ".html", InStock: boolPtr(true)}
	if len(pcur) == 3 {
		o.Currency = pcur
	}
	if img, ok := m["image"].(map[string]any); ok {
		o.Image = str(img["imgUrl"])
	} else if s := str(m["imageUrl"]); s != "" {
		o.Image = s
	}
	if strings.HasPrefix(o.Image, "//") {
		o.Image = "https:" + o.Image
	}
	if ev, ok := m["evaluation"].(map[string]any); ok {
		o.Rating = parsePrice(ev["starRating"])
	}
	if tr, ok := m["trade"].(map[string]any); ok {
		o.Sold = str(tr["tradeDesc"])
		o.Reviews = parseCount(strings.TrimSuffix(strings.Fields(o.Sold + " 0")[0], "+"))
	}
	if st, ok := m["store"].(map[string]any); ok {
		o.SellerName = str(st["storeName"])
	}
	o.SellerTrust = ratingTrust(2.5, o.Rating, o.Reviews)
	return o, true
}
