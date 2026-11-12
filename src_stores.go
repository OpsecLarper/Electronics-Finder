package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ---------- Shopify stores (public predictive-search endpoint) ----------

type shopifySource struct {
	id       string
	base     string
	currency string // fallback if cart.js doesn't report one

	once   sync.Once
	client *http.Client
	cur    string
}

func (s *shopifySource) StoreID() string               { return s.id }
func (s *shopifySource) Ready(Settings) (bool, string) { return true, "" }

func (s *shopifySource) init(ctx context.Context) {
	s.once.Do(func() {
		s.client = newClient()
		s.cur = s.currency
		// cart.js reports the currency the store is pricing this session in.
		var cart struct {
			Currency string `json:"currency"`
		}
		c, cancel := context.WithTimeout(ctx, 6*time.Second)
		defer cancel()
		if err := getJSON(c, s.client, s.base+"/cart.js", &cart); err == nil && len(cart.Currency) == 3 {
			s.cur = strings.ToUpper(cart.Currency)
		}
	})
}

func (s *shopifySource) Search(ctx context.Context, q string, _ Settings) ([]Offer, error) {
	s.init(ctx)
	u := fmt.Sprintf("%s/search/suggest.json?q=%s&resources[type]=product&resources[limit]=10&resources[options][unavailable_products]=last",
		s.base, url.QueryEscape(q))
	var resp struct {
		Resources struct {
			Results struct {
				Products []map[string]any `json:"products"`
			} `json:"results"`
		} `json:"resources"`
	}
	if err := getJSON(ctx, s.client, u, &resp); err != nil {
		return nil, err
	}
	var out []Offer
	for _, p := range resp.Resources.Results.Products {
		o := Offer{StoreID: s.id, Title: str(p["title"]), Currency: s.cur, Brand: str(p["vendor"])}
		o.Price = parsePrice(p["price"])
		if o.Price == 0 {
			o.Price = parsePrice(p["price_min"])
		}
		// Some themes return prices in cents as integers.
		if pv, ok := p["price"].(float64); ok && pv > 0 && pv == float64(int64(pv)) && pv >= 1000 {
			o.Price = pv / 100
		}
		link := str(p["url"])
		if i := strings.Index(link, "?"); i >= 0 {
			link = link[:i]
		}
		if strings.HasPrefix(link, "/") {
			link = s.base + link
		}
		o.URL = link
		switch img := p["image"].(type) {
		case string:
			o.Image = img
		case map[string]any:
			o.Image = str(img["url"])
		}
		if f, ok := p["featured_image"].(map[string]any); ok && o.Image == "" {
			o.Image = str(f["url"])
		}
		if strings.HasPrefix(o.Image, "//") {
			o.Image = "https:" + o.Image
		}
		if a, ok := p["available"].(bool); ok {
			o.InStock = boolPtr(a)
		}
		if o.Title != "" && o.URL != "" {
			out = append(out, o)
		}
	}
	return out, nil
}

// ---------- Adafruit (public product API, cached) ----------

type adafruitSource struct {
	mu       sync.Mutex
	products []map[string]any
	fetched  time.Time
}

func (a *adafruitSource) StoreID() string               { return "adafruit" }
func (a *adafruitSource) Ready(Settings) (bool, string) { return true, "" }

func (a *adafruitSource) catalog(ctx context.Context) ([]map[string]any, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.products != nil && time.Since(a.fetched) < 6*time.Hour {
		return a.products, nil
	}
	cache := filepath.Join(appDir(), "adafruit-products.json")
	if a.products == nil {
		if st, err := os.Stat(cache); err == nil && time.Since(st.ModTime()) < 12*time.Hour {
			if b, err := os.ReadFile(cache); err == nil {
				var p []map[string]any
				if json.Unmarshal(b, &p) == nil && len(p) > 0 {
					a.products, a.fetched = p, st.ModTime()
					return p, nil
				}
			}
		}
	}
	c, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var p []map[string]any
	client := &http.Client{Timeout: 45 * time.Second}
	if err := getJSON(c, client, "https://www.adafruit.com/api/products", &p); err != nil {
		if a.products != nil {
			return a.products, nil
		}
		return nil, err
	}
	a.products, a.fetched = p, time.Now()
	if b, err := json.Marshal(p); err == nil {
		_ = os.WriteFile(cache, b, 0o644)
	}
	return p, nil
}

func (a *adafruitSource) Search(ctx context.Context, q string, _ Settings) ([]Offer, error) {
	cat, err := a.catalog(ctx)
	if err != nil {
		return nil, err
	}
	minRel := minRelevance(q)
	var out []Offer
	for _, p := range cat {
		name := str(p["product_name"])
		if name == "" || relevance(q, name) < minRel {
			continue
		}
		if d := str(p["discontinue_status"]); d != "" && d != "0" && !strings.EqualFold(d, "false") {
			continue
		}
		id := str(p["product_id"])
		o := Offer{StoreID: "adafruit", Title: name, Currency: "USD", Brand: str(p["product_manufacturer"])}
		o.Price = parsePrice(p["product_price"])
		o.URL = str(p["product_url"])
		if o.URL == "" && id != "" {
			o.URL = "https://www.adafruit.com/product/" + id
		}
		if img := str(p["product_image"]); img != "" {
			o.Image = img
		} else if id != "" {
			o.Image = "https://cdn-shop.adafruit.com/310x233/" + id + "-00.jpg"
		}
		if st, ok := p["product_stock"]; ok && st != nil {
			n := int(parsePrice(st))
			o.StockQty = n
			o.InStock = boolPtr(n > 0 || strings.EqualFold(str(st), "true"))
		}
		out = append(out, o)
	}
	return out, nil
}

// ---------- LCSC (unofficial JSON search, best-effort) ----------

type lcscSource struct{}

func (lcscSource) StoreID() string               { return "lcsc" }
func (lcscSource) Ready(Settings) (bool, string) { return true, "" }

func (lcscSource) Search(ctx context.Context, q string, _ Settings) ([]Offer, error) {
	var resp struct {
		Code   int            `json:"code"`
		Result map[string]any `json:"result"`
	}
	u := "https://wmsc.lcsc.com/ftps/wm/search/global?keyword=" + url.QueryEscape(q)
	if err := getJSON(ctx, httpClient, u, &resp); err != nil {
		return nil, err
	}
	var list []any
	if vo, ok := resp.Result["productSearchResultVO"].(map[string]any); ok {
		list, _ = vo["productList"].([]any)
	}
	if list == nil {
		list, _ = resp.Result["productList"].([]any)
	}
	var out []Offer
	for _, it := range list {
		p, ok := it.(map[string]any)
		if !ok {
			continue
		}
		code := str(p["productCode"])
		title := strings.TrimSpace(str(p["productModel"]) + " — " + str(p["productIntroEn"]))
		o := Offer{StoreID: "lcsc", Title: title, Currency: "USD", Brand: str(p["brandNameEn"]),
			URL: "https://www.lcsc.com/product-detail/" + code + ".html"}
		if prices, ok := p["productPriceList"].([]any); ok && len(prices) > 0 {
			if pb, ok := prices[0].(map[string]any); ok {
				o.Price = parsePrice(pb["usdPrice"])
				if o.Price == 0 {
					o.Price = parsePrice(pb["productPrice"])
				}
				o.MinQty = int(parsePrice(pb["ladder"]))
			}
		}
		if imgs, ok := p["productImages"].([]any); ok && len(imgs) > 0 {
			o.Image = str(imgs[0])
		}
		n := int(parsePrice(p["stockNumber"]))
		o.StockQty, o.InStock = n, boolPtr(n > 0)
		if code != "" && o.Price > 0 {
			out = append(out, o)
		}
	}
	return out, nil
}

// ---------- Mouser (official API, free key) ----------

type mouserSource struct{}

func (mouserSource) StoreID() string { return "mouser" }
func (mouserSource) Ready(s Settings) (bool, string) {
	if s.MouserKey == "" {
		return false, "Add a free Mouser Search API key in Settings to enable live results"
	}
	return true, ""
}

func (mouserSource) Search(ctx context.Context, q string, s Settings) ([]Offer, error) {
	body, _ := json.Marshal(map[string]any{"SearchByKeywordRequest": map[string]any{
		"keyword": q, "records": 25, "startingRecord": 0, "searchOptions": "", "searchWithYourSignUpLanguage": "false",
	}})
	req, _ := http.NewRequest("POST", "https://api.mouser.com/api/v1/search/keyword?apiKey="+url.QueryEscape(s.MouserKey), strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	var resp struct {
		Errors []struct {
			Message string `json:"Message"`
		} `json:"Errors"`
		SearchResults struct {
			Parts []map[string]any `json:"Parts"`
		} `json:"SearchResults"`
	}
	if err := doJSON(ctx, httpClient, req, &resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, errors.New(resp.Errors[0].Message)
	}
	var out []Offer
	for _, p := range resp.SearchResults.Parts {
		o := Offer{StoreID: "mouser", Brand: str(p["Manufacturer"]), URL: str(p["ProductDetailUrl"]), Image: str(p["ImagePath"]), Currency: "USD"}
		o.Title = strings.TrimSpace(str(p["ManufacturerPartNumber"]) + " — " + str(p["Description"]))
		if pbs, ok := p["PriceBreaks"].([]any); ok && len(pbs) > 0 {
			if pb, ok := pbs[0].(map[string]any); ok {
				o.Price = parsePrice(pb["Price"])
				if c := str(pb["Currency"]); c != "" {
					o.Currency = c
				}
				o.MinQty = int(parsePrice(pb["Quantity"]))
			}
		}
		n := int(parsePrice(p["AvailabilityInStock"]))
		o.StockQty, o.InStock = n, boolPtr(n > 0)
		if o.Price > 0 {
			out = append(out, o)
		}
	}
	return out, nil
}

// ---------- DigiKey (official API v4, OAuth client credentials) ----------

type digikeySource struct {
	mu     sync.Mutex
	token  string
	expiry time.Time
	keyFor string
}

func (*digikeySource) StoreID() string { return "digikey" }
func (*digikeySource) Ready(s Settings) (bool, string) {
	if s.DigikeyID == "" || s.DigikeySecret == "" {
		return false, "Add a DigiKey API Client ID/Secret in Settings to enable live results"
	}
	return true, ""
}

func (d *digikeySource) auth(ctx context.Context, s Settings) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.token != "" && time.Now().Before(d.expiry) && d.keyFor == s.DigikeyID {
		return d.token, nil
	}
	form := url.Values{"client_id": {s.DigikeyID}, "client_secret": {s.DigikeySecret}, "grant_type": {"client_credentials"}}
	req, _ := http.NewRequest("POST", "https://api.digikey.com/v1/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := doJSON(ctx, httpClient, req, &tok); err != nil {
		return "", fmt.Errorf("auth failed: %v", err)
	}
	d.token, d.keyFor = tok.AccessToken, s.DigikeyID
	d.expiry = time.Now().Add(time.Duration(tok.ExpiresIn-60) * time.Second)
	return d.token, nil
}

func (d *digikeySource) Search(ctx context.Context, q string, s Settings) ([]Offer, error) {
	tok, err := d.auth(ctx, s)
	if err != nil {
		return nil, err
	}
	body, _ := json.Marshal(map[string]any{"Keywords": q, "Limit": 25, "Offset": 0})
	req, _ := http.NewRequest("POST", "https://api.digikey.com/products/v4/search/keyword", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-DIGIKEY-Client-Id", s.DigikeyID)
	req.Header.Set("X-DIGIKEY-Locale-Currency", "USD")
	var resp struct {
		Products []map[string]any `json:"Products"`
	}
	if err := doJSON(ctx, httpClient, req, &resp); err != nil {
		return nil, err
	}
	var out []Offer
	for _, p := range resp.Products {
		o := Offer{StoreID: "digikey", Currency: "USD", URL: str(p["ProductUrl"]), Image: str(p["PhotoUrl"])}
		desc := ""
		if dm, ok := p["Description"].(map[string]any); ok {
			desc = str(dm["ProductDescription"])
		}
		if m, ok := p["Manufacturer"].(map[string]any); ok {
			o.Brand = str(m["Name"])
		}
		o.Title = strings.TrimSpace(str(p["ManufacturerProductNumber"]) + " — " + desc)
		o.Price = parsePrice(p["UnitPrice"])
		n := int(parsePrice(p["QuantityAvailable"]))
		o.StockQty, o.InStock = n, boolPtr(n > 0)
		if o.Price > 0 {
			out = append(out, o)
		}
	}
	return out, nil
}

// ---------- eBay (official Browse API, OAuth client credentials) ----------

type ebaySource struct {
	mu     sync.Mutex
	token  string
	expiry time.Time
	keyFor string
}

func (*ebaySource) StoreID() string { return "ebay" }
func (*ebaySource) Ready(s Settings) (bool, string) {
	if s.EbayID == "" || s.EbaySecret == "" {
		return false, "Add an eBay App ID/Cert ID in Settings to enable live results"
	}
	return true, ""
}

func (e *ebaySource) auth(ctx context.Context, s Settings) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.token != "" && time.Now().Before(e.expiry) && e.keyFor == s.EbayID {
		return e.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "scope": {"https://api.ebay.com/oauth/api_scope"}}
	req, _ := http.NewRequest("POST", "https://api.ebay.com/identity/v1/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(s.EbayID, s.EbaySecret)
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := doJSON(ctx, httpClient, req, &tok); err != nil {
		return "", fmt.Errorf("auth failed: %v", err)
	}
	e.token, e.keyFor = tok.AccessToken, s.EbayID
	e.expiry = time.Now().Add(time.Duration(tok.ExpiresIn-60) * time.Second)
	return e.token, nil
}

var ebayMarket = map[string]string{"NA": "EBAY_US", "UK": "EBAY_GB", "EU": "EBAY_DE", "ASIA": "EBAY_US", "OTHER": "EBAY_US"}

func (e *ebaySource) Search(ctx context.Context, q string, s Settings) ([]Offer, error) {
	tok, err := e.auth(ctx, s)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest("GET", "https://api.ebay.com/buy/browse/v1/item_summary/search?limit=30&q="+url.QueryEscape(q), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	m := ebayMarket[s.Region]
	if m == "" {
		m = "EBAY_US"
	}
	req.Header.Set("X-EBAY-C-MARKETPLACE-ID", m)
	var resp struct {
		ItemSummaries []struct {
			Title      string `json:"title"`
			ItemWebURL string `json:"itemWebUrl"`
			Condition  string `json:"condition"`
			Price      struct {
				Value    string `json:"value"`
				Currency string `json:"currency"`
			} `json:"price"`
			Image struct {
				ImageURL string `json:"imageUrl"`
			} `json:"image"`
			Seller struct {
				Username           string `json:"username"`
				FeedbackPercentage string `json:"feedbackPercentage"`
				FeedbackScore      int    `json:"feedbackScore"`
			} `json:"seller"`
		} `json:"itemSummaries"`
	}
	if err := doJSON(ctx, httpClient, req, &resp); err != nil {
		return nil, err
	}
	var out []Offer
	for _, it := range resp.ItemSummaries {
		o := Offer{StoreID: "ebay", Title: it.Title, URL: it.ItemWebURL, Image: it.Image.ImageURL,
			Price: parsePrice(it.Price.Value), Currency: it.Price.Currency, InStock: boolPtr(true),
			SellerName: it.Seller.Username, Condition: it.Condition}
		pct := parsePrice(it.Seller.FeedbackPercentage)
		o.SellerPct = pct
		if pct > 0 {
			// Map feedback % and volume to a 1–5 trust rating.
			t := 1 + (pct-90)/10*3 // 90% → 1, 100% → 4
			if t < 1 {
				t = 1
			}
			switch {
			case it.Seller.FeedbackScore >= 5000:
				t += 1
			case it.Seller.FeedbackScore >= 500:
				t += 0.6
			case it.Seller.FeedbackScore >= 50:
				t += 0.3
			}
			if t > 5 {
				t = 5
			}
			o.SellerTrust = &t
		}
		if o.Price > 0 {
			out = append(out, o)
		}
	}
	return out, nil
}
