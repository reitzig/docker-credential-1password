package logging

import (
	"fmt"
	"os"
	"strings"

	"github.com/docker-credential-1password/internal/app"
)

var debugEnvVar = strings.ToUpper(strings.ReplaceAll(app.Name, "-", "_")) + "_DEBUG"

func isDebugEnabled(value string) bool {
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

var DebugEnabled = isDebugEnabled(os.Getenv(debugEnvVar))

func Debug(format string, args ...any) {
	if DebugEnabled {
		_, _ = fmt.Fprintf(os.Stderr, "debug: "+format+"\n", args...)
	}
}
