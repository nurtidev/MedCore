package config

import (
	"os"
	"regexp"
	"strings"
)

var envPlaceholderPattern = regexp.MustCompile(`\$\{([A-Z0-9_]+)(?::([^}]*))?\}`)

// ExpandEnvPlaceholders replaces ${VAR} and ${VAR:default} placeholders.
func ExpandEnvPlaceholders(input string) string {
	return envPlaceholderPattern.ReplaceAllStringFunc(input, func(match string) string {
		parts := envPlaceholderPattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}

		key := parts[1]
		def := parts[2]
		if value, ok := os.LookupEnv(key); ok && value != "" {
			return value
		}
		return strings.TrimSpace(def)
	})
}
