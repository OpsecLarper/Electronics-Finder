package main

// Store is a curated profile of a place to buy electronics. The ratings are
// editorial (1–5) and are combined with live price/stock data to rank offers.
type Store struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	Domain    string             `json:"domain"`
	Kind      string             `json:"kind"`      // Authorized distributor, Maker store, Manufacturer, Marketplace, Retailer
	Origin    string             `json:"origin"`    // Where it ships from
	Trust     float64            `json:"trust"`     // authenticity & reliability
	Price     float64            `json:"price"`     // general price level (5 = cheapest)
	Selection float64            `json:"selection"` // breadth of hobby electronics
	Support   float64            `json:"support"`   // returns, docs, customer service
	Shipping  map[string]float64 `json:"shipping"`  // speed/cost by region: NA, EU, UK, ASIA, OTHER
	BestFor   string             `json:"bestFor"`
	Notes     string             `json:"notes"`
	SearchURL string             `json:"searchUrl"` // %s = url-encoded query
	Live      string             `json:"live"`      // "", "live", "api" (needs keys), "beta"
}

func ship(na, eu, uk, asia, other float64) map[string]float64 {
	return map[string]float64{"NA": na, "EU": eu, "UK": uk, "ASIA": asia, "OTHER": other}
}

var stores = []Store{
	{ID: "digikey", Name: "DigiKey", Domain: "digikey.com", Kind: "Authorized distributor", Origin: "USA",
		Trust: 5, Price: 3, Selection: 5, Support: 5, Shipping: ship(5, 4.5, 4.5, 4, 4),
		BestFor:   "Genuine chips, modules & dev boards with datasheets",
		Notes:     "Every part is sourced from the manufacturer, so no counterfeits. Pricier for single hobby modules, but shipping is fast.",
		SearchURL: "https://www.digikey.com/en/products/result?keywords=%s", Live: "api"},
	{ID: "mouser", Name: "Mouser", Domain: "mouser.com", Kind: "Authorized distributor", Origin: "USA",
		Trust: 5, Price: 3, Selection: 5, Support: 4.5, Shipping: ship(5, 4.5, 4.5, 4, 4),
		BestFor:   "Genuine ICs, RF modules, Espressif & TI parts",
		Notes:     "An authorized distributor with a huge catalog. Free shipping above a threshold in most regions.",
		SearchURL: "https://www.mouser.com/c/?q=%s", Live: "api"},
	{ID: "adafruit", Name: "Adafruit", Domain: "adafruit.com", Kind: "Maker store", Origin: "USA (NYC)",
		Trust: 5, Price: 3, Selection: 4, Support: 5, Shipping: ship(4.5, 3, 3, 2.5, 2.5),
		BestFor:   "Feather/QT Py ESP32 boards, sensors and excellent tutorials",
		Notes:     "Designs and builds much of its own hardware. Its open-source libraries and guides are among the best around.",
		SearchURL: "https://www.adafruit.com/search?q=%s", Live: "live"},
	{ID: "sparkfun", Name: "SparkFun", Domain: "sparkfun.com", Kind: "Maker store", Origin: "USA (CO)",
		Trust: 5, Price: 3, Selection: 4, Support: 4.5, Shipping: ship(4.5, 3, 3, 2.5, 2.5),
		BestFor:   "Qwiic ecosystem, ESP32 Thing boards, breakouts",
		Notes:     "Reliable open-hardware vendor with strong documentation.",
		SearchURL: "https://www.sparkfun.com/search/?q=%s"},
	{ID: "pololu", Name: "Pololu", Domain: "pololu.com", Kind: "Manufacturer", Origin: "USA (NV)",
		Trust: 5, Price: 3.5, Selection: 3.5, Support: 5, Shipping: ship(4.5, 3.5, 3.5, 3, 3),
		BestFor:   "Small motors (micro metal gearmotors), motor drivers, regulators",
		Notes:     "The go-to place for quality small DC gearmotors, encoders, stepper drivers and voltage regulators.",
		SearchURL: "https://www.pololu.com/search?query=%s"},
	{ID: "thepihut", Name: "The Pi Hut", Domain: "thepihut.com", Kind: "Maker store", Origin: "UK",
		Trust: 4.5, Price: 3.5, Selection: 4, Support: 4.5, Shipping: ship(3, 4, 5, 2.5, 2.5),
		BestFor:   "UK buyers: ESP32, Pico, sensors, motors",
		Notes:     "Large UK maker retailer that stocks many brands and ships quickly within the UK and EU.",
		SearchURL: "https://thepihut.com/search?q=%s", Live: "live"},
	{ID: "pimoroni", Name: "Pimoroni", Domain: "shop.pimoroni.com", Kind: "Maker store", Origin: "UK",
		Trust: 4.5, Price: 3.5, Selection: 3.5, Support: 4.5, Shipping: ship(3, 4, 5, 2.5, 2.5),
		BestFor:   "Pico/ESP32 boards, displays, Pimoroni breakouts",
		Notes:     "UK designer and retailer with good-quality in-house boards.",
		SearchURL: "https://shop.pimoroni.com/search?q=%s", Live: "live"},
	{ID: "lilygo", Name: "LILYGO", Domain: "lilygo.cc", Kind: "Manufacturer", Origin: "China",
		Trust: 4, Price: 4.5, Selection: 3, Support: 3, Shipping: ship(3, 3, 3, 4, 3),
		BestFor:   "T-Display / T-Embed CC1101 / LoRa ESP32 boards direct from the maker",
		Notes:     "Official store for LILYGO boards. Buying here avoids clones, but shipping from China takes about 1–3 weeks.",
		SearchURL: "https://lilygo.cc/search?q=%s", Live: "live"},
	{ID: "m5stack", Name: "M5Stack", Domain: "shop.m5stack.com", Kind: "Manufacturer", Origin: "China",
		Trust: 4.5, Price: 4, Selection: 3, Support: 3.5, Shipping: ship(3.5, 3, 3, 4, 3),
		BestFor:   "M5Stack ESP32 cores, Cardputer, Atom, units (incl. CC1101 units)",
		Notes:     "Official M5Stack store.",
		SearchURL: "https://shop.m5stack.com/search?q=%s", Live: "live"},
	{ID: "robotshop", Name: "RobotShop", Domain: "robotshop.com", Kind: "Retailer", Origin: "Canada / USA / EU",
		Trust: 4, Price: 3, Selection: 4.5, Support: 3.5, Shipping: ship(4.5, 3.5, 3.5, 2.5, 2.5),
		BestFor:   "Motors, servos, robot kits, many brands in one cart",
		Notes:     "Huge robotics catalog. Some items are drop-shipped from other vendors.",
		SearchURL: "https://www.robotshop.com/search?q=%s", Live: "live"},
	{ID: "seeed", Name: "Seeed Studio", Domain: "seeedstudio.com", Kind: "Manufacturer", Origin: "China / USA",
		Trust: 4.5, Price: 4, Selection: 4, Support: 3.5, Shipping: ship(3.5, 3, 3, 4.5, 3),
		BestFor:   "XIAO ESP32 boards, Grove modules, LoRa",
		Notes:     "Official maker of the XIAO series and Grove modules.",
		SearchURL: "https://www.seeedstudio.com/catalogsearch/result/?q=%s"},
	{ID: "dfrobot", Name: "DFRobot", Domain: "dfrobot.com", Kind: "Manufacturer", Origin: "China",
		Trust: 4, Price: 4, Selection: 3.5, Support: 3.5, Shipping: ship(3, 3, 3, 4, 3),
		BestFor:   "Gravity sensors, FireBeetle ESP32, motors",
		Notes:     "Established maker brand with a wiki for most products.",
		SearchURL: "https://www.dfrobot.com/search-%s.html"},
	{ID: "waveshare", Name: "Waveshare", Domain: "waveshare.com", Kind: "Manufacturer", Origin: "China",
		Trust: 4, Price: 4, Selection: 4, Support: 3, Shipping: ship(3, 3, 3, 4, 3),
		BestFor:   "ESP32 display boards, e-paper, RF & CAN modules",
		Notes:     "Makes a huge range of boards. Its wiki documentation is decent.",
		SearchURL: "https://www.waveshare.com/catalogsearch/result/?q=%s"},
	{ID: "lcsc", Name: "LCSC", Domain: "lcsc.com", Kind: "Distributor", Origin: "China",
		Trust: 4.5, Price: 5, Selection: 5, Support: 3, Shipping: ship(3.5, 3.5, 3.5, 5, 3),
		BestFor:   "Bare chips: ESP32 modules, CC1101 ICs, regulators, passives at the lowest prices",
		Notes:     "JLCPCB's parts house. Parts are genuine and very cheap, but it sells components rather than finished hobby modules.",
		SearchURL: "https://www.lcsc.com/search?q=%s", Live: "beta"},
	{ID: "aliexpress", Name: "AliExpress", Domain: "aliexpress.com", Kind: "Marketplace", Origin: "China",
		Trust: 2.5, Price: 5, Selection: 5, Support: 2.5, Shipping: ship(2, 2, 2, 3.5, 2),
		BestFor:   "Cheapest generic modules (CC1101, N20 motors, ESP32 dev kits) when you can wait",
		Notes:     "Quality varies by seller. Stick to official brand stores and sellers with high ratings, and expect 1–4 weeks shipping. Live results factor in each listing's star rating and number sold.",
		SearchURL: "https://www.aliexpress.com/wholesale?SearchText=%s", Live: "beta"},
	{ID: "amazon", Name: "Amazon", Domain: "amazon.com", Kind: "Marketplace", Origin: "Local warehouses",
		Trust: 3.5, Price: 3.5, Selection: 4, Support: 4.5, Shipping: ship(5, 4.5, 4.5, 3.5, 3),
		BestFor:   "Fast delivery of multipacks (ESP32 dev boards, SG90 servos, motor kits)",
		Notes:     "Returns are easy, but many listings are rebadged generic modules. Check reviews for DOA reports. Live results factor in each listing's star rating and review count.",
		SearchURL: "https://www.amazon.com/s?k=%s", Live: "beta"},
	{ID: "ebay", Name: "eBay", Domain: "ebay.com", Kind: "Marketplace", Origin: "Worldwide",
		Trust: 3, Price: 4, Selection: 4.5, Support: 3.5, Shipping: ship(3.5, 3.5, 3.5, 3.5, 3),
		BestFor:   "Hard-to-find, used or surplus parts",
		Notes:     "Seller quality varies. With API keys, each offer is rated using that seller's feedback score.",
		SearchURL: "https://www.ebay.com/sch/i.html?_nkw=%s", Live: "api"},
	{ID: "tindie", Name: "Tindie", Domain: "tindie.com", Kind: "Marketplace", Origin: "Worldwide (indie makers)",
		Trust: 3.5, Price: 3, Selection: 3, Support: 3, Shipping: ship(3, 3, 3, 3, 3),
		BestFor:   "Unique hacker boards: CC1101 add-ons, ESP32 RF tools, Flipper accessories",
		Notes:     "Products come from independent makers, so quality and support depend on who sells it.",
		SearchURL: "https://www.tindie.com/search/?q=%s"},
	{ID: "microcenter", Name: "Micro Center", Domain: "microcenter.com", Kind: "Retailer", Origin: "USA (stores)",
		Trust: 4.5, Price: 4, Selection: 2.5, Support: 4.5, Shipping: ship(4, 1, 1, 1, 1),
		BestFor:   "Same-day in-store pickup in the US (Inland ESP32 boards, kits)",
		Notes:     "Great prices on its Inland-brand boards, but only practical if you live near a store.",
		SearchURL: "https://www.microcenter.com/search/search_results.aspx?Ntt=%s"},
	{ID: "arrow", Name: "Arrow", Domain: "arrow.com", Kind: "Authorized distributor", Origin: "USA",
		Trust: 5, Price: 3.5, Selection: 4.5, Support: 4, Shipping: ship(4.5, 4, 4, 4, 3.5),
		BestFor:   "Genuine parts, frequently free shipping",
		Notes:     "Authorized distributor that often has free shipping with no minimum order.",
		SearchURL: "https://www.arrow.com/en/products/search?q=%s"},
}

func storeByID(id string) *Store {
	for i := range stores {
		if stores[i].ID == id {
			return &stores[i]
		}
	}
	return nil
}
