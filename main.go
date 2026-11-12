// Electronics Finder — searches reputable electronics stores for a part and
// ranks where to buy it by price, store trust, shipping and stock.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

var version = "1.1.0"

func main() {
	browser := flag.Bool("browser", false, "open the UI in your default browser instead of an app window")
	flag.Parse()

	loadSettings()
	go refreshFX()

	addr, err := startServer()
	if err != nil {
		fatal("Could not start local server: " + err.Error())
	}
	fmt.Println("Electronics Finder running at", addr)
	if *browser || !runWindow(addr) {
		// No WebView2 available — fall back to the default browser.
		if err := openBrowser(addr); err != nil {
			fmt.Println("Open this address in your browser:", addr)
		}
		waitForever()
	}
}

func fatal(msg string) {
	showError(msg)
	log.Println(msg)
	os.Exit(1)
}
