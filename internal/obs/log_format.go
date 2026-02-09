package obs

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the verbosity level for structured logging output.
type LogLevel int

// Log level constants control which messages are emitted.
const (
	// LvlQuiet suppresses all output except FAIL messages.
	LvlQuiet LogLevel = iota
	// LvlStandard emits FAIL, WARN, and OK messages.
	LvlStandard
	// LvlVerbose emits FAIL, WARN, and OK messages (default level).
	LvlVerbose
	// LvlTrace emits all messages including trace-level diagnostics.
	LvlTrace
)

var (
	mu          sync.Mutex
	currentLvl  = LvlVerbose
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

// Configure sets the global log level from a string value (quiet, standard, verbose, or trace).
func Configure(level string) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "quiet":
		currentLvl = LvlQuiet
	case "standard":
		currentLvl = LvlStandard
	case "verbose":
		currentLvl = LvlVerbose
	case "trace":
		currentLvl = LvlTrace
	default:
		currentLvl = LvlVerbose
	}
}

// SetWriters overrides the default stdout and stderr writers used for log output.
func SetWriters(out, err io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	stdout = out
	stderr = err
}

// Ok emits a structured log message at OK level.
func Ok(module, component, msg string, ctx ...string) {
	log("OK", module, component, msg, ctx...)
}

// Warn emits a structured log message at WARN level.
func Warn(module, component, msg string, ctx ...string) {
	log("WARN", module, component, msg, ctx...)
}

// Trace emits a structured log message at TRACE level.
func Trace(module, component, msg string, ctx ...string) {
	log("TRACE", module, component, msg, ctx...)
}

// Fail emits a structured log message at FAIL level, written to stderr.
func Fail(module, component, msg string, ctx ...string) {
	log("FAIL", module, component, msg, ctx...)
}

func log(level, module, component, msg string, ctx ...string) {
	if !shouldLog(level) {
		return
	}

	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	context := strings.Join(ctx, " ")
	line := fmt.Sprintf("[AA:%s:%s] %s | %s | %s", module, level, timestamp, component, msg)
	if context != "" {
		line += " | " + context
	}
	line += "\n"

	mu.Lock()
	defer mu.Unlock()
	if level == "FAIL" {
		_, _ = io.WriteString(stderr, line)
		return
	}
	_, _ = io.WriteString(stdout, line)
}

func shouldLog(level string) bool {
	switch currentLvl {
	case LvlQuiet:
		return level == "FAIL"
	case LvlStandard:
		return level == "FAIL" || level == "WARN" || level == "OK"
	case LvlVerbose:
		return level == "FAIL" || level == "WARN" || level == "OK"
	case LvlTrace:
		return true
	default:
		return true
	}
}

