@echo off
REM Build on Windows: requires Go 1.26+ (https://go.dev/dl/)
set CGO_ENABLED=0
if not exist dist mkdir dist
go build -trimpath -ldflags "-s -w -H windowsgui" -o dist\ElectronicsFinder.exe .
echo Built dist\ElectronicsFinder.exe
