package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
	logFilePath string
)

func init() {
	if runtime.GOOS == "windows" {
		programData := os.Getenv("PROGRAMDATA")
		if programData == "" {
			programData = `C:\ProgramData`
		}
		configDir = filepath.Join(programData, "OblireachAgent")
	} else {
		configDir = "/etc/oblireach-agent"
	}
	configFile   = filepath.Join(configDir, "config.json")
	logFilePath  = filepath.Join(configDir, "oblireach.log")
}

// setupLogging redirects log output to both stdout and a persistent log file.
// The log file is created / appended to at configDir/oblireach.log.
// If the file cannot be opened (e.g. permission issue) we silently fall back
// to stdout-only so the process still starts.
func setupLogging() {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return // no directory — stdout only
	}
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return // can't open — stdout only
	}
	log.SetOutput(io.MultiWriter(f, os.Stdout))
	log.SetFlags(log.Ldate | log.Ltime)
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
	// Ensure DLLs next to the exe are found (required for Windows services
	// whose working directory is System32, not the exe's directory).
	if exe, err := os.Executable(); err == nil {
		setDLLSearchPath(filepath.Dir(exe))
	}

	setupLogging()

	urlFlag         := flag.String("url", "", "Obliance server URL (first run only)")
	keyFlag         := flag.String("key", "", "Oblireach API key (first run only)")
	captureHelper   := flag.Bool("capture-helper", false, "Run as a capture helper subprocess (internal use)")
	helperAddr      := flag.String("addr", "", "TCP address for capture helper to connect to")
	notifyTitle     := flag.String("notify-title", "", "Show a toast notification with this title (internal use)")
	notifyMsg       := flag.String("notify-msg", "", "Toast notification message body")
	notifyTimeout   := flag.Int("notify-timeout", 8, "Toast auto-close timeout in seconds")
	chatHelper      := flag.Bool("chat-helper", false, "Run as chat helper subprocess (internal use)")
	chatID          := flag.String("chat-id", "", "Chat session ID (chat-helper mode)")
	chatOperator    := flag.String("operator", "", "Operator display name (chat-helper mode)")
	flag.Parse()

	// ── Toast notification subprocess mode ───────────────────────────────────
	// Launched by the service process inside a user session to show a toast.
	if *notifyTitle != "" {
		runToastNotification(*notifyTitle, *notifyMsg, *notifyTimeout)
		return
	}

	// ── Chat helper subprocess mode ──────────────────────────────────────────
	if *chatHelper {
		if *helperAddr == "" || *chatID == "" {
			fmt.Fprintln(os.Stderr, "--addr and --chat-id required in chat-helper mode")
			os.Exit(1)
		}
		runChatHelperMode(*helperAddr, *chatID, *chatOperator)
		return
	}

	// ── Capture helper subprocess mode ───────────────────────────────────────
	// Launched by the service process inside a user session to perform DXGI capture.
	if *captureHelper {
		if *helperAddr == "" {
			fmt.Fprintln(os.Stderr, "--addr required in capture-helper mode")
			os.Exit(1)
		}
		runHelperMode(*helperAddr)
		return
	}

	// ── Normal agent mode ─────────────────────────────────────────────────────
	cfg := setupConfig(*urlFlag, *keyFlag)

	log.Printf("Oblireach Agent v%s starting (uuid=%s server=%s)",
		agentVersion, cfg.DeviceUUID, cfg.ServerURL)

	// Write version file so the Obliance tray can read it.
	_ = os.WriteFile(filepath.Join(configDir, "version.txt"), []byte(agentVersion), 0644)

	// Driver setup (VDD / Amyuni) is deferred until a no-user stream
	// actually asks for a virtual display. Running it synchronously at
	// service start, or even asynchronously in a goroutine during
	// START_PENDING, can push past the SCM's service-start timeout when
	// the system already has phantom monitors or the driver store takes
	// time to respond — causing MSI StartServices to return Error 1920
	// and the whole install to roll back with exit 1603.
	runFn := func() { runCmdWS(cfg) }

	// Try to run as a Windows service first; fall back to interactive mode.
	if tryRunAsService(runFn) {
		return
	}
	runFn()
}
