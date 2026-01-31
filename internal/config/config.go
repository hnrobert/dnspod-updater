package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// DNSPod common params
	LoginToken   string
	Format       string
	Lang         string
	ErrorOnEmpty string

	// DNSPod endpoints
	DNSPodBaseURL string

	// Record identification
	Domain   string
	DomainID int
	RecordID int

	// Record fields
	SubDomain    string
	RecordType   string
	RecordLine   string
	RecordLineID string
	TTL          int
	MX           int
	Status       string
	Weight       int

	// Runtime
	CheckInterval time.Duration
	OneShot       bool
	HTTPTimeout   time.Duration
	StartDelay    time.Duration

	// IP detection
	IPPreferredIface string
	IPDetectMethod   string
	WiFiSSID         string

	// Misc
	UserAgent string
}

func FromEnv() (Config, error) {
	var cfg Config

	cfg.LoginToken = strings.TrimSpace(os.Getenv("DNSPOD_LOGIN_TOKEN"))
	cfg.Format = envDefault("DNSPOD_FORMAT", "json")
	cfg.Lang = envDefault("DNSPOD_LANG", "cn")
	cfg.ErrorOnEmpty = envDefault("DNSPOD_ERROR_ON_EMPTY", "no")
	cfg.DNSPodBaseURL = envDefault("DNSPOD_BASE_URL", "https://dnsapi.cn")

	cfg.Domain = strings.TrimSpace(os.Getenv("DNSPOD_DOMAIN"))
	cfg.DomainID = envIntDefault("DNSPOD_DOMAIN_ID", 0)
	cfg.RecordID = envIntDefault("DNSPOD_RECORD_ID", 0)

	cfg.SubDomain = envDefault("DNSPOD_SUB_DOMAIN", "@")
	cfg.RecordType = strings.ToUpper(envDefault("DNSPOD_RECORD_TYPE", "A"))
	cfg.RecordLine = envDefault("DNSPOD_RECORD_LINE", "默认")
	cfg.RecordLineID = strings.TrimSpace(os.Getenv("DNSPOD_RECORD_LINE_ID"))
	cfg.TTL = envIntDefault("DNSPOD_TTL", 0)
	cfg.MX = envIntDefault("DNSPOD_MX", 0)
	cfg.Status = envDefault("DNSPOD_STATUS", "enable")
	cfg.Weight = envIntDefault("DNSPOD_WEIGHT", -1) // -1 means not set

	cfg.CheckInterval = envDurationDefault("CHECK_INTERVAL", 0)
	if cfg.CheckInterval == 0 {
		// Compatibility: seconds-based env
		sec := envIntDefault("CHECK_INTERVAL_SECONDS", 0)
		if sec > 0 {
			cfg.CheckInterval = time.Duration(sec) * time.Second
		}
	}
	cfg.OneShot = envBoolDefault("ONESHOT", false)
	cfg.HTTPTimeout = envDurationDefault("HTTP_TIMEOUT", 10*time.Second)
	cfg.StartDelay = envDurationDefault("START_DELAY", 0)

	cfg.IPPreferredIface = strings.TrimSpace(os.Getenv("IP_PREFERRED_IFACE"))
	cfg.IPDetectMethod = strings.TrimSpace(os.Getenv("IP_DETECT_METHOD")) // "auto" (default), "route", "udp", "iface"
	cfg.WiFiSSID = strings.TrimSpace(os.Getenv("WIFI_SSID"))

	cfg.UserAgent = envDefault("USER_AGENT", "dnspod-updater/1.0")

	if cfg.LoginToken == "" {
		return Config{}, errors.New("DNSPOD_LOGIN_TOKEN is required (format: id,token)")
	}
	if cfg.Domain == "" && cfg.DomainID == 0 {
		return Config{}, errors.New("DNSPOD_DOMAIN or DNSPOD_DOMAIN_ID is required")
	}
	if cfg.RecordType == "MX" && cfg.MX == 0 {
		return Config{}, errors.New("DNSPOD_MX is required when DNSPOD_RECORD_TYPE=MX")
	}
	if cfg.CheckInterval < 0 {
		return Config{}, fmt.Errorf("CHECK_INTERVAL must be >= 0, got %s", cfg.CheckInterval)
	}

	return cfg, nil
}

func envDefault(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func envIntDefault(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envBoolDefault(key string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return def
	}
}

func envDurationDefault(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	if allDigits(v) {
		sec, _ := strconv.Atoi(v)
		return time.Duration(sec) * time.Second
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
