//go:build !windows

package main

// SessionInfo describes a logon session (non-Windows stub).
type SessionInfo struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	State       string `json:"state"`
	StationName string `json:"stationName,omitempty"`
	IsConsole   bool   `json:"isConsole"`
}

func enumerateSessions() []SessionInfo { return nil }
func consoleSessionID() int            { return 0 }
func currentSessionID() int            { return 0 }
