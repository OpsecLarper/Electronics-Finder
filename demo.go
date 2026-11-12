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
		o := Offer{StoreID: d.id, Title: fmt.Sprintf("%s Development Board v%d — WiFi + Bluetooth", q, i+1),
			URL: "https://example.com/" + d.id, Price: d.base * (0.8 + rand.Float64()*0.8), Currency: d.cur, InStock: &in}
		if d.id == "amazon" || d.id == "aliexpress" {
			o.Rating, o.Reviews = 3.9+rand.Float64()*0.9, 50+rand.Intn(3000)
			if d.id == "aliexpress" {
				o.Sold = fmt.Sprintf("%d+ sold", o.Reviews)
			}
			o.SellerTrust = ratingTrust(map[string]float64{"amazon": 3.5, "aliexpress": 2.5}[d.id], o.Rating, o.Reviews)
		}
		out = append(out, o)
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
		demoSource{"amazon", "USD", 9}, demoSource{"aliexpress", "USD", 4}, &ebaySource{},
	}
}
