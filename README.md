# Electronics Finder

A Windows app that finds the best place to buy a part, such as an ESP32, a CC1101, a small gear motor or a servo. It searches trusted stores and ranks the results by price, trust, shipping speed to your region, stock and support. The interface uses a liquid-glass look.

**Download:** [`dist/ElectronicsFinder.exe`](dist/ElectronicsFinder.exe). It is one file with no install step. Double-click it to run.

## What it does
- **Live search** across Adafruit, The Pi Hut, Pimoroni, LILYGO, M5Stack, RobotShop and LCSC (beta). Results stream in as each store responds.
- **Optional free API keys** in Settings turn on live prices and stock from **Mouser**, **DigiKey** and **eBay**. eBay listings are rated using that seller's feedback score.
- **Top picks**: best overall, lowest price, most trusted and fastest to you.
- **By-store ranking** shows rating bars for Trust, Shipping, Value and Support. You can also view all listings as a grid.
- **"What matters to you" sliders** change how much each factor counts. Rankings update instantly.
- **Also worth checking**: one-click searches at stores that can't be queried directly (Pololu, SparkFun, Arrow, Seeed, DFRobot, Waveshare, AliExpress, Amazon, Tindie, Micro Center).
- **Store directory** with editorial ratings and what each store is best for.
- Prices are converted to your currency using live exchange rates, with built-in estimates as a fallback.

Settings and API keys are saved in `%APPDATA%\ElectronicsFinder\settings.json` and stay on your PC.

## Notes
- The app runs in a WebView2 window. WebView2 comes with Windows 11 and up-to-date Windows 10. If it's missing, the app opens in your default browser instead. You can also force browser mode with `ElectronicsFinder.exe --browser`.
- Stores sometimes change their sites or block automated searches. When a live search fails, the store still appears with a direct search link.
- Store ratings are editorial judgements, not data from the stores.

## Build
Requires Go 1.26 or newer.
- Linux/macOS (cross-compile): `./build.sh`
- Windows: `build.bat`
- Offline UI preview with fake data: `EF_DEMO=1 go run . --browser`
