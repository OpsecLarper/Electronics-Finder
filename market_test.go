package main

import "testing"

const amazonFixture = `<html><body>
<div role="listitem" data-asin="B0C5X1Y2Z3" data-index="2" data-uuid="abc" data-component-type="s-search-result" class="s-result-item">
 <div><img class="s-image" src="https://m.media-amazon.com/images/I/71abc.jpg" alt=""></div>
 <h2 aria-label="ESP32-S3 DevKitC-1 N16R8 Development Board, Dual-Core" class="a-size-medium"><span>ESP32-S3 DevKitC-1 N16R8 Development Board, Dual-Core</span></h2>
 <span class="a-icon-alt">4.5 out of 5 stars</span>
 <a aria-label="2,317 ratings" href="#"><span class="a-size-base s-underline-text">2,317</span></a>
 <span class="a-price"><span class="a-offscreen">$15.99</span><span aria-hidden="true">$15<span>99</span></span></span>
</div>
<div data-asin="" data-component-type="s-impression-logger"></div>
<div role="listitem" data-asin="B0D9ZZZZZZ" data-component-type="s-search-result">
 <h2 class="a-size-base-plus"><a href="/x"><span>Sponsored Ad - 2PCS ESP32-S3 WROOM-1 Board</span></a></h2>
 <span class="a-icon-alt">4.1 out of 5 stars</span>
 <span class="a-size-base s-underline-text">(98)</span>
 <span class="a-offscreen">$21.49</span>
</div>
<div role="listitem" data-asin="B0NOPRICE1" data-component-type="s-search-result"><h2><span>No price item</span></h2></div>
</body></html>`

func TestParseAmazon(t *testing.T) {
	got := parseAmazon(amazonFixture, "www.amazon.com", "USD")
	if len(got) != 2 {
		t.Fatalf("want 2 offers, got %d: %+v", len(got), got)
	}
	a := got[0]
	if a.URL != "https://www.amazon.com/dp/B0C5X1Y2Z3" || a.Price != 15.99 || a.Rating != 4.5 || a.Reviews != 2317 || a.Image == "" || a.SellerTrust == nil {
		t.Errorf("bad first offer: %+v", a)
	}
	b := got[1]
	if b.Title != "2PCS ESP32-S3 WROOM-1 Board" || b.Price != 21.49 || b.Reviews != 98 {
		t.Errorf("bad second offer: %+v", b)
	}
	if !isAmazonBlock(`<form action="/errors/validateCaptcha">`) {
		t.Error("captcha not detected")
	}
}

const aliFixture = `<html><script>
window._dida_config_._init_data_= { data: {"success":true,"data":{"root":{"fields":{"mods":{"itemList":{"content":[
 {"productId":"1005006123456789","title":{"displayTitle":"CC1101 Wireless Module 433MHz SMA Antenna"},
  "prices":{"salePrice":{"currencyCode":"USD","formattedPrice":"US $2.18","minPrice":2.18}},
  "image":{"imgUrl":"//ae01.alicdn.com/kf/S1.jpg"},"evaluation":{"starRating":4.8},
  "trade":{"tradeDesc":"1,000+ sold"},"store":{"storeName":"RF Parts Store"}},
 {"productId":"1005006987654321","title":{"displayTitle":"E07-M1101D CC1101 Module"},
  "prices":{"salePrice":{"currencyCode":"USD","formattedPrice":"US $3.05"}},
  "image":{"imgUrl":"https://ae01.alicdn.com/kf/S2.jpg"},"trade":{"tradeDesc":"57 sold"}}
]}}}}}} }
</script></html>`

func TestParseAliExpress(t *testing.T) {
	// The real blob uses an unquoted "data:" key wrapper; extraction must find the inner JSON.
	got := parseAliExpress(aliFixture, "USD")
	if len(got) != 2 {
		t.Fatalf("want 2 offers, got %d: %+v", len(got), got)
	}
	a := got[0]
	if a.Price != 2.18 || a.Rating != 4.8 || a.Reviews != 1000 || a.Image != "https://ae01.alicdn.com/kf/S1.jpg" ||
		a.URL != "https://www.aliexpress.com/item/1005006123456789.html" || a.SellerName != "RF Parts Store" {
		t.Errorf("bad first offer: %+v", a)
	}
	if got[1].Price != 3.05 {
		t.Errorf("formattedPrice fallback failed: %+v", got[1])
	}
}

func TestParseCount(t *testing.T) {
	for in, want := range map[string]int{"2,317": 2317, "2.5K": 2500, "98": 98, "1.234": 1234} {
		if got := parseCount(in); got != want {
			t.Errorf("parseCount(%q)=%d want %d", in, got, want)
		}
	}
}
