package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed web
var webFS embed.FS

var apiToken = func() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}()

type sourceResult struct {
	StoreID string  `json:"storeId"`
	Status  string  `json:"status"` // ok, error, skipped
	Message string  `json:"message,omitempty"`
	Offers  []Offer `json:"offers"`
	Ms      int64   `json:"ms"`
}

func startServer() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	sub, _ := fs.Sub(webFS, "web")
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/meta", guard(handleMeta))
	mux.HandleFunc("/api/settings", guard(handleSettings))
	mux.HandleFunc("/api/search", guard(handleSearch))
	mux.HandleFunc("/api/open", guard(handleOpen))
	mux.HandleFunc("/api/ping", guard(handlePing))
	go func() { _ = http.Serve(ln, noCache(mux)) }()
	return fmt.Sprintf("http://%s/?t=%s", ln.Addr().String(), apiToken), nil
}

func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}

// guard rejects requests that don't carry the per-launch token, so other
// websites open in a browser can't drive the local API.
func guard(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t := r.Header.Get("X-Token")
		if t == "" {
			t = r.URL.Query().Get("t")
		}
		if t != apiToken {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h(w, r)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func handleMeta(w http.ResponseWriter, r *http.Request) {
	s := currentSettings()
	status := map[string]string{}
	for _, src := range sources {
		if ok, msg := src.Ready(s); !ok {
			status[src.StoreID()] = msg
		}
	}
	fxMu.RLock()
	rates := map[string]float64{}
	for k, v := range fxRates {
		rates[k] = v
	}
	src := fxSrc
	fxMu.RUnlock()
	writeJSON(w, map[string]any{
		"stores": stores, "settings": s, "needsSetup": status,
		"fx": rates, "fxSource": src, "version": version,
	})
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, currentSettings())
		return
	}
	var s Settings
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	for _, p := range []*string{&s.MouserKey, &s.DigikeyID, &s.DigikeySecret, &s.EbayID, &s.EbaySecret} {
		*p = strings.TrimSpace(*p)
	}
	if err := saveSettings(s); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	handleMeta(w, r)
}

// handleSearch streams results per store as Server-Sent Events.
func handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	flusher, ok := w.(http.Flusher)
	if q == "" || !ok {
		http.Error(w, "missing query", 400)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")

	s := currentSettings()
	disabled := map[string]bool{}
	for _, d := range s.Disabled {
		disabled[d] = true
	}
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()

	results := make(chan sourceResult)
	var wg sync.WaitGroup
	for _, src := range sources {
		src := src
		id := src.StoreID()
		if disabled[id] {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			res := sourceResult{StoreID: id, Offers: []Offer{}}
			if ok, msg := src.Ready(s); !ok {
				res.Status, res.Message = "skipped", msg
			} else {
				sctx, scancel := context.WithTimeout(ctx, 25*time.Second)
				offers, err := safeSearch(sctx, src, q, s)
				scancel()
				if err != nil {
					res.Status, res.Message = "error", friendlyErr(err)
				} else {
					res.Status = "ok"
					res.Offers = rankOffers(q, offers)
				}
			}
			res.Ms = time.Since(start).Milliseconds()
			select {
			case results <- res:
			case <-ctx.Done():
			}
		}()
	}
	go func() { wg.Wait(); close(results) }()

	send := func(event string, v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		flusher.Flush()
	}
	send("start", map[string]any{"query": q})
	for res := range results {
		send("store", res)
	}
	send("done", map[string]any{})
}

func safeSearch(ctx context.Context, src Source, q string, s Settings) (offers []Offer, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("internal error: %v", r)
		}
	}()
	return src.Search(ctx, q, s)
}

func friendlyErr(err error) string {
	m := err.Error()
	switch {
	case strings.Contains(m, "context deadline exceeded") || strings.Contains(m, "Client.Timeout"):
		return "Timed out: the site took too long to respond"
	case strings.Contains(m, "no such host"):
		return "Couldn't reach the site (check your internet connection)"
	case strings.Contains(m, "HTTP 403") || strings.Contains(m, "HTTP 429"):
		return "The site blocked automated searching for now. Try the direct link"
	}
	if len(m) > 180 {
		m = m[:180] + "…"
	}
	return m
}

// rankOffers filters irrelevant results, adds USD price & relevance, keeps the best 12.
func rankOffers(q string, in []Offer) []Offer {
	minRel := minRelevance(q)
	out := make([]Offer, 0, len(in))
	for _, o := range in {
		o.Relevance = relevance(q, o.Title)
		if o.Relevance < minRel || o.Price <= 0 {
			continue
		}
		if o.Currency == "" {
			o.Currency = "USD"
		}
		out = append(out, o)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Relevance != out[j].Relevance {
			return out[i].Relevance > out[j].Relevance
		}
		return toUSD(out[i].Price, out[i].Currency) < toUSD(out[j].Price, out[j].Currency)
	})
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

func handleOpen(w http.ResponseWriter, r *http.Request) {
	u, err := url.Parse(r.URL.Query().Get("url"))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		http.Error(w, "bad url", 400)
		return
	}
	if err := openBrowser(u.String()); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func openBrowser(u string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
	case "darwin":
		return exec.Command("open", u).Start()
	default:
		return exec.Command("xdg-open", u).Start()
	}
}
