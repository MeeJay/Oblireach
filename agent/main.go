package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// agentVersion is injected at build time via:
//
//	go build -ldflags="-X main.agentVersion=x.y.z"
var agentVersion = "dev"

var (
	configDir  string
	configFile string
)

func init() {
	if runtime.GOOS == "windows" {
		programData := os.Getenv("PROGRAMDATA")
		if programData == "" {
			programData = `C:\ProgramData`
		}
		configDir = filepath.Join(programData, "ObliReachAgent")
	} else {
		configDir = "/etc/oblireach-agent"
	}
	configFile = filepath.Join(configDir, "config.json")
}

// ── Config ────────────────────────────────────────────────────────────────────

type Config struct {
	ServerURL  string `json:"serverUrl"`
	APIKey     string `json:"apiKey"`
	DeviceUUID string `json:"deviceUuid"`
}

func loadConfig() (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveConfig(cfg *Config) error {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, data, 0644)
}

func generateUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func setupConfig(urlArg, keyArg string) *Config {
	cfg, err := loadConfig()
	if err != nil {
		cfg = nil
	}

	if cfg == nil {
		if urlArg == "" || keyArg == "" {
			fmt.Fprintf(os.Stderr, "First run: provide --url <serverUrl> --key <apiKey>\n")
			fmt.Fprintf(os.Stderr, "Example: oblireach-agent --url https://obliance.example.com --key your-api-key\n")
			os.Exit(1)
		}
		cfg = &Config{
			ServerURL:  strings.TrimRight(urlArg, "/"),
			APIKey:     keyArg,
			DeviceUUID: resolveDeviceUUID(""),
		}
		if err := saveConfig(cfg); err != nil {
			log.Printf("Warning: could not save config: %v", err)
		}
	} else {
		// Override URL/key if provided on command line
		if urlArg != "" {
			cfg.ServerURL = strings.TrimRight(urlArg, "/")
		}
		if keyArg != "" {
			cfg.APIKey = keyArg
		}
		// Upgrade to hardware UUID if possible
		cfg.DeviceUUID = resolveDeviceUUID(cfg.DeviceUUID)
		if cfg.DeviceUUID != "" {
			_ = saveConfig(cfg)
		}
	}

	return cfg
}

func main() {
	urlFlag := flag.String("url", "", "Obliance server URL (first run only)")
	keyFlag := flag.String("key", "", "Oblireach API key (first run only)")
	flag.Parse()

	cfg := setupConfig(*urlFlag, *keyFlag)

	log.Printf("Oblireach Agent v%s starting (uuid=%s server=%s)",
		agentVersion, cfg.DeviceUUID, cfg.ServerURL)

	runPushLoop(cfg)
}
