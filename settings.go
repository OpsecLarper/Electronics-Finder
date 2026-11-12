package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Settings struct {
	Currency      string   `json:"currency"`
	Region        string   `json:"region"` // NA, EU, UK, ASIA, OTHER
	Disabled      []string `json:"disabled"`
	MouserKey     string   `json:"mouserKey"`
	DigikeyID     string   `json:"digikeyId"`
	DigikeySecret string   `json:"digikeySecret"`
	EbayID        string   `json:"ebayId"`
	EbaySecret    string   `json:"ebaySecret"`
}

var (
	settingsMu sync.RWMutex
	settings   = Settings{Currency: "USD", Region: "NA"}
)

func appDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	d := filepath.Join(base, "ElectronicsFinder")
	_ = os.MkdirAll(d, 0o755)
	return d
}

func loadSettings() {
	b, err := os.ReadFile(filepath.Join(appDir(), "settings.json"))
	if err != nil {
		return
	}
	settingsMu.Lock()
	defer settingsMu.Unlock()
	_ = json.Unmarshal(b, &settings)
	if settings.Currency == "" {
		settings.Currency = "USD"
	}
	if settings.Region == "" {
		settings.Region = "NA"
	}
}

func saveSettings(s Settings) error {
	settingsMu.Lock()
	settings = s
	settingsMu.Unlock()
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(filepath.Join(appDir(), "settings.json"), b, 0o600)
}

func currentSettings() Settings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return settings
}

// ---------- currency rates (USD per 1 unit of currency) ----------

var (
	fxMu  sync.RWMutex
	fxSrc = "built-in estimates"
	// Fallback rates expressed as units per 1 USD.
	fxRates = map[string]float64{
		"USD": 1, "EUR": 0.93, "GBP": 0.79, "CAD": 1.39, "AUD": 1.53, "JPY": 152,
		"CNY": 7.2, "INR": 84.5, "CHF": 0.88, "SEK": 10.7, "NOK": 11.0, "DKK": 6.95,
		"PLN": 4.0, "NZD": 1.69, "SGD": 1.34, "HKD": 7.8, "MXN": 20.0, "BRL": 5.6,
	}
)

func refreshFX() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var r struct {
		Result string             `json:"result"`
		Rates  map[string]float64 `json:"rates"`
	}
	if err := getJSON(ctx, httpClient, "https://open.er-api.com/v6/latest/USD", &r); err != nil || len(r.Rates) < 10 {
		return
	}
	fxMu.Lock()
	for k, v := range r.Rates {
		if v > 0 {
			fxRates[k] = v
		}
	}
	fxSrc = "live (open.er-api.com)"
	fxMu.Unlock()
}

func toUSD(amount float64, cur string) float64 {
	fxMu.RLock()
	defer fxMu.RUnlock()
	r, ok := fxRates[cur]
	if !ok || r == 0 {
		return amount
	}
	return amount / r
}
