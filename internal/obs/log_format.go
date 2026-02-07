package obs

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type LogLevel int

const (
	LvlQuiet LogLevel = iota
	LvlStandard
	LvlVerbose
	LvlTrace
)

var (
	mu          sync.Mutex
	currentLvl  = LvlVerbose
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

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

func SetWriters(out, err io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	stdout = out
	stderr = err
}

func Ok(module, component, msg string, ctx ...string) {
	log("OK", module, component, msg, ctx...)
}

func Warn(module, component, msg string, ctx ...string) {
	log("WARN", module, component, msg, ctx...)
}

func Trace(module, component, msg string, ctx ...string) {
	log("TRACE", module, component, msg, ctx...)
}

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

