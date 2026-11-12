package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"
)

// Demo mode (EF_DEMO=1) returns synthetic offers so the UI can be previewed offline.
type demoSource struct {
	id, cur string
	base    float64
}

func (d demoSource) StoreID() string               { return d.id }
func (d demoSource) Ready(Settings) (bool, string) { return true, "" }
func (d demoSource) Search(ctx context.Context, q string, _ Settings) ([]Offer, error) {
	time.Sleep(time.Duration(300+rand.Intn(1500)) * time.Millisecond)
	if d.id == "lcsc" {
		return nil, fmt.Errorf("HTTP 403: blocked")
	}
	var out []Offer
	for i := 0; i < 1+rand.Intn(4); i++ {
		in := rand.Intn(5) != 0
		out = append(out, Offer{StoreID: d.id, Title: fmt.Sprintf("%s Development Board v%d — WiFi + Bluetooth", q, i+1),
			URL: "https://example.com/" + d.id, Price: d.base * (0.8 + rand.Float64()*0.8), Currency: d.cur, InStock: &in})
	}
	return out, nil
}

func init() {
	if os.Getenv("EF_DEMO") == "" {
		return
	}
	sources = []Source{
		demoSource{"adafruit", "USD", 12}, demoSource{"thepihut", "GBP", 9}, demoSource{"pimoroni", "GBP", 10},
		demoSource{"lilygo", "USD", 8}, demoSource{"m5stack", "USD", 11}, demoSource{"robotshop", "USD", 14},
		demoSource{"mouser", "USD", 10}, demoSource{"digikey", "USD", 10.5}, demoSource{"lcsc", "USD", 3},
		&ebaySource{},
	}
}
