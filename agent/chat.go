package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

// ── Chat pipe message types ──────────────────────────────────────────────────

const (
	chatPipeMsg   = byte(0x10) // bidirectional: JSON chat message
	chatPipeStop  = byte(0x11) // service → helper: close chat
	chatPipeEvent = byte(0x12) // helper → service: user events
	chatPipeInit  = byte(0x13) // service → helper: init with avatar data
)

// ── ChatSession ──────────────────────────────────────────────────────────────

type ChatSession struct {
	chatID         string
	operatorName   string
	operatorAvatar string // data URI (base64)
	sessionID      int
	conn           net.Conn
	stopCh         chan struct{}
	once           sync.Once
}

var activeChats sync.Map // chatID → *ChatSession

func (cs *ChatSession) stop() {
	cs.once.Do(func() {
		close(cs.stopCh)
		if cs.conn != nil {
			cs.conn.Close()
		}
		activeChats.Delete(cs.chatID)
		log.Printf("Chat %s: stopped", cs.chatID)
	})
}

// startChat spawns a chat helper in the target session and sets up the pipe relay.
func startChat(cfg *Config, chatID, operatorName, operatorAvatar string, sessionID int) error {
	if sessionID < 0 {
		sessionID = findCaptureSession()
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("chat: listen: %w", err)
	}
	addr := ln.Addr().String()
	log.Printf("Chat %s: listening on %s (session %d)", chatID, addr, sessionID)

	exe, err := executablePath()
	if err != nil {
		ln.Close()
		return fmt.Errorf("chat: executable: %w", err)
	}

	cmdLine := fmt.Sprintf(`"%s" --chat-helper --addr=%s --chat-id=%s --operator="%s"`,
		exe, addr, chatID, operatorName)

	pid, rc := spawnInSessionGo(sessionID, cmdLine)
	if rc != 0 {
		ln.Close()
		return fmt.Errorf("chat: spawn in session %d failed (code %d)", sessionID, rc)
	}
	log.Printf("Chat %s: spawned helper PID %d in session %d", chatID, pid, sessionID)

	cs := &ChatSession{
		chatID:         chatID,
		operatorName:   operatorName,
		operatorAvatar: operatorAvatar,
		sessionID:      sessionID,
		stopCh:         make(chan struct{}),
	}
	activeChats.Store(chatID, cs)

	// Accept the helper connection in background
	go func() {
		defer cs.stop()
		defer ln.Close()

		ln.(*net.TCPListener).SetDeadline(time.Now().Add(15 * time.Second))
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Chat %s: accept failed: %v", chatID, err)
			return
		}
		cs.conn = conn
		log.Printf("Chat %s: helper connected", chatID)

		// Send init data (including avatar) to helper
		initData, _ := json.Marshal(map[string]string{
			"operatorName":   cs.operatorName,
			"operatorAvatar": cs.operatorAvatar,
		})
		chatPipeSend(conn, chatPipeInit, initData)

		// Read events from helper and forward to command WS
		cs.relayFromHelper()
	}()

	return nil
}

func stopChat(chatID string) {
	if v, ok := activeChats.Load(chatID); ok {
		cs := v.(*ChatSession)
		// Send stop signal to helper
		if cs.conn != nil {
			chatPipeSend(cs.conn, chatPipeStop, nil)
		}
		cs.stop()
	}
}

func forwardChatMessage(chatID, operatorName, message string, timestamp int64) {
	v, ok := activeChats.Load(chatID)
	if !ok {
		return
	}
	cs := v.(*ChatSession)

	msg, _ := json.Marshal(map[string]interface{}{
		"action":       "operator_message",
		"text":         message,
		"operatorName": operatorName,
		"timestamp":    timestamp,
	})

	// Try to send — if the pipe is dead (user closed), respawn the helper
	if cs.conn == nil || chatPipeSend(cs.conn, chatPipeMsg, msg) != nil {
		log.Printf("Chat %s: helper disconnected, respawning for new message", chatID)
		// Clean up old session
		cs.stop()
		// Respawn — reuse the same chat session params
		// The caller (push.go) already has the config
		go func() {
			if err := startChat(nil, chatID, cs.operatorName, cs.operatorAvatar, cs.sessionID); err != nil {
				log.Printf("Chat %s: respawn failed: %v", chatID, err)
				return
			}
			// Wait for helper to connect, then send the pending message
			time.Sleep(2 * time.Second)
			if v2, ok := activeChats.Load(chatID); ok {
				cs2 := v2.(*ChatSession)
				if cs2.conn != nil {
					chatPipeSend(cs2.conn, chatPipeMsg, msg)
				}
			}
		}()
	}
}

func forwardChatFile(chatID string, payload map[string]interface{}) {
	v, ok := activeChats.Load(chatID)
	if !ok {
		return
	}
	cs := v.(*ChatSession)
	if cs.conn == nil {
		return
	}

	msg, _ := json.Marshal(map[string]interface{}{
		"action":   "file_transfer",
		"fileName": payload["fileName"],
		"fileSize": payload["fileSize"],
		"fileData": payload["fileData"],
	})
	chatPipeSend(cs.conn, chatPipeMsg, msg)
}

func forwardRemoteRequest(chatID, message string) {
	v, ok := activeChats.Load(chatID)
	if !ok {
		return
	}
	cs := v.(*ChatSession)
	if cs.conn == nil {
		return
	}

	msg, _ := json.Marshal(map[string]interface{}{
		"action":  "request_remote",
		"message": message,
	})
	chatPipeSend(cs.conn, chatPipeMsg, msg)
}

// relayFromHelper reads pipe messages from the chat helper and forwards to the server.
func (cs *ChatSession) relayFromHelper() {
	for {
		select {
		case <-cs.stopCh:
			return
		default:
		}

		cs.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		msgType, payload, err := chatPipeRecv(cs.conn)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			log.Printf("Chat %s: helper disconnected: %v", cs.chatID, err)
			// Notify server that user closed
			agentMsg, _ := json.Marshal(map[string]interface{}{
				"type":   "chat_event",
				"chatId": cs.chatID,
				"payload": map[string]interface{}{
					"action": "user_closed",
				},
			})
			cmdWSSend(agentMsg)
			return
		}

		if msgType == chatPipeEvent {
			// Forward user event to server
			var event map[string]interface{}
			if json.Unmarshal(payload, &event) == nil {
				agentMsg, _ := json.Marshal(map[string]interface{}{
					"type":    "chat_event",
					"chatId":  cs.chatID,
					"payload": event,
				})
				cmdWSSend(agentMsg)
			}
		} else if msgType == chatPipeMsg {
			// User message → forward to server
			var msg map[string]interface{}
			if json.Unmarshal(payload, &msg) == nil {
				agentMsg, _ := json.Marshal(map[string]interface{}{
					"type":   "chat_message",
					"chatId": cs.chatID,
					"payload": map[string]interface{}{
						"text":      msg["text"],
						"from":      msg["from"],
						"timestamp": time.Now().UnixMilli(),
					},
				})
				cmdWSSend(agentMsg)
			}
		}
	}
}

// executablePath returns the path of the current executable.
func executablePath() (string, error) {
	return os.Executable()
}

// ── Chat pipe framing (same format as capture pipe) ─────────────────────────

func chatPipeSend(w io.Writer, msgType byte, payload []byte) error {
	hdr := make([]byte, 5)
	binary.LittleEndian.PutUint32(hdr[0:4], uint32(len(payload)))
	hdr[4] = msgType
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	if len(payload) > 0 {
		_, err := w.Write(payload)
		return err
	}
	return nil
}

func chatPipeRecv(r io.Reader) (msgType byte, payload []byte, err error) {
	hdr := make([]byte, 5)
	if _, err = io.ReadFull(r, hdr); err != nil {
		return 0, nil, err
	}
	length := binary.LittleEndian.Uint32(hdr[0:4])
	msgType = hdr[4]
	if length > 0 {
		payload = make([]byte, length)
		if _, err = io.ReadFull(r, payload); err != nil {
			return 0, nil, err
		}
	}
	return msgType, payload, nil
}
