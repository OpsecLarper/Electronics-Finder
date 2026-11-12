package main

import "testing"

func TestParsePrice(t *testing.T) {
	cases := map[string]float64{"$1,234.50": 1234.5, "1.234,50 €": 1234.5, "12,50 €": 12.5, "£8.22": 8.22, "1,234": 1234, "12": 12, "": 0}
	for in, want := range cases {
		if got := parsePrice(in); got != want {
			t.Errorf("parsePrice(%q)=%v want %v", in, got, want)
		}
	}
	if parsePrice(9.5) != 9.5 {
		t.Error("float")
	}
}

func TestRelevance(t *testing.T) {
	q := "esp32 s3"
	if relevance(q, "Espressif ESP32-S3-DevKitC-1") < minRelevance(q) {
		t.Error("should match ESP32-S3")
	}
	if relevance("cc1101", "CC-1101 433MHz Wireless Module") < 1 {
		t.Error("should match CC-1101")
	}
	if relevance("cc1101", "NRF24L01 module") >= minRelevance("cc1101") {
		t.Error("should not match")
	}
	if relevance("n20 gear motor", "N20 Micro Gear Motor 6V 100RPM") < minRelevance("n20 gear motor") {
		t.Error("should match n20")
	}
}

func TestRankOffersFilters(t *testing.T) {
	in := []Offer{{Title: "ESP32-S3 board", Price: 10, Currency: "USD"}, {Title: "Arduino Uno", Price: 5}, {Title: "ESP32-S3 free", Price: 0}}
	out := rankOffers("esp32-s3", in)
	if len(out) != 1 || out[0].Title != "ESP32-S3 board" {
		t.Fatalf("got %+v", out)
	}
}
