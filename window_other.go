//go:build !windows

package main

import "fmt"

// On non-Windows platforms the UI opens in the default browser.
func runWindow(string) bool { return false }

func showError(msg string) { fmt.Println(msg) }
