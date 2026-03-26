package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	webview "github.com/jchv/go-webview2"
)

// appVersion is injected at build time via -ldflags "-X main.appVersion=x.y.z"
var appVersion = "1.0.0"

// ── Config ────────────────────────────────────────────────────────────────────

type Config struct {
	ServerURL   string `json:"serverUrl"`
	Username    string `json:"username"`
	SessionFile string `json:"-"`
}

var (
	configDir  string
	configFile string
	cookieFile string
)

func init() {
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
		configDir = filepath.Join(appData, "OblireachClient")
	} else {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".oblireach-client")
	}
	configFile = filepath.Join(configDir, "config.json")
	cookieFile = filepath.Join(configDir, "session.json")
}

func loadConfig() *Config {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return &Config{}
	}
	var cfg Config
	_ = json.Unmarshal(data, &cfg)
	return &cfg
}

func saveConfig(cfg *Config) {
	_ = os.MkdirAll(configDir, 0700)
	data, _ := json.MarshalIndent(cfg, "", "  ")
	_ = os.WriteFile(configFile, data, 0600)
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	cfg := loadConfig()

	// Start local proxy server on a random port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	localBase := fmt.Sprintf("http://127.0.0.1:%d", port)

	proxy := newProxy(cfg, configDir, port)
	go func() {
		if err := http.Serve(ln, proxy); err != nil && !strings.Contains(err.Error(), "use of closed") {
			log.Printf("proxy: %v", err)
		}
	}()

	// Create WebView2 window.
	w := webview.New(true)
	defer w.Destroy()
	w.SetTitle("Oblireach")
	w.SetSize(1280, 800, webview.HintNone)

	// Inject Go bindings into every page.
	w.Init(fmt.Sprintf(`window.__reach_version = %q;`, appVersion))

	// Navigate to the local UI.
	w.Navigate(localBase + "/")
	w.Run()
}
