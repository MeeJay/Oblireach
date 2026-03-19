package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"
)

const pushInterval = 30 * time.Second

type pushPayload struct {
	DeviceUUID string        `json:"deviceUuid"`
	Hostname   string        `json:"hostname"`
	OS         string        `json:"os"`
	Arch       string        `json:"arch"`
	Version    string        `json:"version"`
	Sessions   []SessionInfo `json:"sessions,omitempty"`
}

type pushResponse struct {
	Status  string   `json:"status"`
	Command *command `json:"command,omitempty"`
}

type command struct {
	Type    string                 `json:"type"`
	ID      string                 `json:"id"`
	Payload map[string]interface{} `json:"payload"`
}

func runPushLoop(cfg *Config) {
	client := &http.Client{Timeout: 15 * time.Second}

	for {
		if err := doPush(cfg, client); err != nil {
			log.Printf("Push failed: %v", err)
		}
		time.Sleep(pushInterval)
	}
}

func doPush(cfg *Config, client *http.Client) error {
	hostname, _ := os.Hostname()

	payload := pushPayload{
		DeviceUUID: cfg.DeviceUUID,
		Hostname:   hostname,
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		Version:    agentVersion,
		Sessions:   enumerateSessions(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal push payload: %w", err)
	}

	req, err := http.NewRequest("POST", cfg.ServerURL+"/api/oblireach/push", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build push request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", cfg.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("push HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("push: unauthorized (check API key)")
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("push: server returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("push: read body: %w", err)
	}

	var pr pushResponse
	if err := json.Unmarshal(data, &pr); err != nil {
		return fmt.Errorf("push: unmarshal response: %w", err)
	}

	if pr.Command != nil {
		go handleCommand(cfg, pr.Command)
	}

	return nil
}

func handleCommand(cfg *Config, cmd *command) {
	log.Printf("Command received: type=%s id=%s", cmd.Type, cmd.ID)

	switch cmd.Type {
	case "open_remote_tunnel":
		token, _ := cmd.Payload["sessionToken"].(string)
		if token == "" {
			log.Printf("Command %s: missing sessionToken", cmd.ID)
			return
		}
		// Optional sessionId from payload; -1 = use console session default.
		sessionID := -1
		if raw, ok := cmd.Payload["sessionId"]; ok {
			if f, ok := raw.(float64); ok {
				sessionID = int(f)
			}
		}
		if err := startStream(cfg, token, sessionID); err != nil {
			log.Printf("Command %s: startStream failed: %v", cmd.ID, err)
		}

	case "close_remote_tunnel":
		token, _ := cmd.Payload["sessionToken"].(string)
		if token != "" {
			stopStream(token)
		}

	case "update":
		urlStr, _ := cmd.Payload["url"].(string)
		version, _ := cmd.Payload["version"].(string)
		if urlStr != "" {
			go performUpdate(cfg, cmd, urlStr, version)
		} else {
			log.Printf("Command %s: update: missing url in payload", cmd.ID)
		}

	default:
		log.Printf("Command %s: unknown type %q", cmd.ID, cmd.Type)
	}
}

func payloadString(payload map[string]interface{}, key string) string {
	v, _ := payload[key].(string)
	return v
}
