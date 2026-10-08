package main

import (
	"fmt"
)

func findLegacyPassThrough(command string) (legacyPassThrough, bool) {
	for _, passThrough := range legacyPassThroughs() {
		if passThrough.Command == command {
			return passThrough, true
		}
	}
	return legacyPassThrough{}, false
}

func legacyPassThroughUsageLine(passThrough legacyPassThrough) string {
	return fmt.Sprintf("zv %s [%s args]", passThrough.Command, passThrough.Binary)
}
