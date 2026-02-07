package cfg

import "os"

// Cfg holds the broker configuration values loaded from AA_* environment variables.
type Cfg struct {
	Port        string
	LogLevel    string
	TrustDomain string
	DefaultTTL  int
}

// Load reads AA_* environment variables and returns a Cfg with defaults applied for any missing values.
func Load() Cfg {
	port := os.Getenv("AA_PORT")
	if port == "" {
		port = "8080"
	}

	logLevel := os.Getenv("AA_LOG_LEVEL")
	if logLevel == "" {
		logLevel = "verbose"
	}

	trustDomain := os.Getenv("AA_TRUST_DOMAIN")
	if trustDomain == "" {
		trustDomain = "agentauth.local"
	}

	defaultTTL := 300
	if v := os.Getenv("AA_DEFAULT_TTL"); v != "" {
		// Best-effort parse; invalid value falls back to secure default.
		var parsed int
		for _, ch := range v {
			if ch < '0' || ch > '9' {
				parsed = 0
				break
			}
			parsed = parsed*10 + int(ch-'0')
		}
		if parsed > 0 {
			defaultTTL = parsed
		}
	}

	return Cfg{
		Port:        port,
		LogLevel:    logLevel,
		TrustDomain: trustDomain,
		DefaultTTL:  defaultTTL,
	}
}
