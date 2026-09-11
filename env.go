package main

import (
	"bufio"
	"log"
	"os"
	"strings"
)

// loadEnvFile reads simple KEY=VALUE lines from a .env file into the process
// environment. Blank lines and lines starting with '#' are ignored, and a
// variable already set in the real environment always wins, so an explicit
// `docker run -e ...` or `export` is never overridden by the file.
//
// A missing file is not an error: the service runs on its defaults.
func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)

		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			log.Printf("could not set %s from %s: %v", key, path, err)
		}
	}
}
