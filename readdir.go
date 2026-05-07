package main

import "os"

// readDir is split out so it's mockable if we ever need to in tests.
func readDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}
