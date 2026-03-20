//go:build windows

package main

/*
#cgo LDFLAGS: -lwtsapi32 -ladvapi32 -luserenv
#include <windows.h>
#include <wtsapi32.h>
#include <userenv.h>
#include <stdlib.h>
#include <wchar.h>

// enablePrivilege enables a named privilege in the given token.
// Returns TRUE on success; FALSE if the privilege is not held at all.
static BOOL enablePrivilege(HANDLE hToken, LPCWSTR privName) {
	TOKEN_PRIVILEGES tp;
	ZeroMemory(&tp, sizeof(tp));
	tp.PrivilegeCount = 1;
	tp.Privileges[0].Attributes = SE_PRIVILEGE_ENABLED;
	if (!LookupPrivilegeValueW(NULL, privName, &tp.Privileges[0].Luid))
		return FALSE;
	return AdjustTokenPrivileges(hToken, FALSE, &tp, 0, NULL, NULL);
}

// spawnInSession launches cmdLine inside the given Windows session.
// Uses WTSQueryUserToken for logged-in sessions; falls back to a
// SYSTEM token with the session ID forced when no user is logged in.
// outPID receives the PID of the spawned process on success.
// Returns 0 on success, negative GetLastError() on failure.
static int spawnInSession(DWORD sessionId, wchar_t *cmdLine, DWORD *outPID) {
	HANDLE hToken = NULL;

	// Enable SeTcbPrivilege on our own token before calling WTSQueryUserToken.
	// Even though LocalSystem holds this privilege, it may not be enabled in
	// the active token — AdjustTokenPrivileges is required to activate it.
	{
		HANDLE hSelf = NULL;
		if (OpenProcessToken(GetCurrentProcess(),
				TOKEN_ADJUST_PRIVILEGES | TOKEN_QUERY, &hSelf)) {
			enablePrivilege(hSelf, L"SeTcbPrivilege");
			enablePrivilege(hSelf, L"SeAssignPrimaryTokenPrivilege");
			enablePrivilege(hSelf, L"SeIncreaseQuotaPrivilege");
			CloseHandle(hSelf);
		}
	}

	if (!WTSQueryUserToken(sessionId, &hToken)) {
		// No interactive user in this session — use SYSTEM token and relocate it.
		if (!OpenProcessToken(GetCurrentProcess(), TOKEN_ALL_ACCESS, &hToken)) {
			return -(int)GetLastError();
		}
		HANDLE hDup = NULL;
		if (!DuplicateTokenEx(hToken, TOKEN_ALL_ACCESS, NULL,
				SecurityImpersonation, TokenPrimary, &hDup)) {
			CloseHandle(hToken);
			return -(int)GetLastError();
		}
		CloseHandle(hToken);
		hToken = hDup;
		DWORD sid = sessionId;
		if (!SetTokenInformation(hToken, TokenSessionId, &sid, sizeof(sid))) {
			CloseHandle(hToken);
			return -(int)GetLastError();
		}
	}

	LPVOID pEnv = NULL;
	CreateEnvironmentBlock(&pEnv, hToken, FALSE);

	STARTUPINFOW si;
	ZeroMemory(&si, sizeof(si));
	si.cb = sizeof(si);
	si.lpDesktop = L"winsta0\\default";

	PROCESS_INFORMATION pi;
	ZeroMemory(&pi, sizeof(pi));

	// CreateProcessAsUserW may modify cmdLine, so pass a writable copy.
	wchar_t *mutableCmd = _wcsdup(cmdLine);
	if (!mutableCmd) {
		if (pEnv) DestroyEnvironmentBlock(pEnv);
		CloseHandle(hToken);
		return -ERROR_NOT_ENOUGH_MEMORY;
	}

	DWORD flags = CREATE_UNICODE_ENVIRONMENT | CREATE_NO_WINDOW;
	BOOL ok = CreateProcessAsUserW(
		hToken, NULL, mutableCmd,
		NULL, NULL, FALSE,
		flags, pEnv, NULL, &si, &pi
	);
	free(mutableCmd);

	if (pEnv) DestroyEnvironmentBlock(pEnv);
	CloseHandle(hToken);

	if (!ok) {
		return -(int)GetLastError();
	}

	*outPID = pi.dwProcessId;
	CloseHandle(pi.hProcess);
	CloseHandle(pi.hThread);
	return 0;
}
*/
import "C"

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

// ── TCP pipe message types ────────────────────────────────────────────────────
//
//	Framing: [4-byte uint32 LE payload length][1-byte type][payload]
const (
	pipeTypeInit  = byte(0x01) // helper → service: init JSON
	pipeTypeFrame = byte(0x02) // helper → service: H.264 NAL units
	pipeTypeInput = byte(0x03) // service → helper: input JSON
	pipeTypeStop  = byte(0x04) // service → helper: stop signal
)

func pipeSend(w io.Writer, msgType byte, payload []byte) error {
	hdr := make([]byte, 5)
	binary.LittleEndian.PutUint32(hdr[:4], uint32(len(payload)))
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

func pipeRecv(r io.Reader) (msgType byte, payload []byte, err error) {
	hdr := make([]byte, 5)
	if _, err = io.ReadFull(r, hdr); err != nil {
		return
	}
	msgType = hdr[4]
	length := binary.LittleEndian.Uint32(hdr[:4])
	if length > 8*1024*1024 { // 8 MB sanity cap
		err = fmt.Errorf("pipe: oversized message %d bytes", length)
		return
	}
	if length > 0 {
		payload = make([]byte, length)
		_, err = io.ReadFull(r, payload)
	}
	return
}

// ── Helper mode ───────────────────────────────────────────────────────────────

// runHelperMode is called when this binary is launched with --capture-helper.
// It connects to addr (TCP), initialises DXGI capture + WMF encoder in its
// own session, then streams frames to the service process and handles input.
func runHelperMode(addr string) {
	// The helper runs as the interactive user — C:\ProgramData is not writable
	// by standard users, so setupLogging() falls back to stdout (discarded for
	// a no-window process).  Redirect to %TEMP% so crash messages are visible.
	if tmpDir := os.TempDir(); tmpDir != "" {
		if f, err := os.OpenFile(
			filepath.Join(tmpDir, "oblireach-helper.log"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			log.SetOutput(io.MultiWriter(f, os.Stdout))
		}
	}
	log.Printf("helper: connecting to service at %s", addr)

	var conn net.Conn
	var err error
	for i := 0; i < 20; i++ {
		conn, err = net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		log.Fatalf("helper: could not connect to service: %v", err)
	}
	defer conn.Close()

	// ── Init capture ─────────────────────────────────────────────────────────
	if err := captureInit(); err != nil {
		log.Fatalf("helper: captureInit failed: %v", err)
	}
	defer captureClose()

	w := captureWidth()
	h := captureHeight()
	if w == 0 || h == 0 {
		log.Fatalf("helper: invalid capture dimensions %dx%d", w, h)
	}

	fps := 15
	bitrate := 3_000_000

	// ── Init encoder ─────────────────────────────────────────────────────────
	extradata, err := encoderInit(w, h, fps, bitrate)
	if err != nil {
		log.Fatalf("helper: encoderInit failed: %v", err)
	}
	defer encoderClose()

	// ── Send init message to service ─────────────────────────────────────────
	initMsg := map[string]interface{}{
		"type":   "init",
		"width":  w,
		"height": h,
		"fps":    fps,
		"codec":  "h264",
	}
	if len(extradata) > 0 {
		initMsg["extradata"] = extradata
	}
	initJSON, _ := json.Marshal(initMsg)
	if err := pipeSend(conn, pipeTypeInit, initJSON); err != nil {
		log.Fatalf("helper: send init failed: %v", err)
	}
	log.Printf("helper: streaming %dx%d@%dfps", w, h, fps)

	stopCh := make(chan struct{})
	inputCh := make(chan []byte, 64)

	// ── Reader goroutine: input / stop from service ───────────────────────────
	go func() {
		defer close(inputCh)
		for {
			msgType, payload, err := pipeRecv(conn)
			if err != nil {
				return
			}
			switch msgType {
			case pipeTypeInput:
				select {
				case inputCh <- payload:
				default:
				}
			case pipeTypeStop:
				close(stopCh)
				return
			}
		}
	}()

	// ── Capture/encode/send loop ──────────────────────────────────────────────
	bgraBuf := make([]byte, w*h*4)
	frameTicker := time.NewTicker(time.Second / time.Duration(fps))
	defer frameTicker.Stop()

	var pts int64
	firstFrameLogged := false

	for {
		select {
		case <-stopCh:
			return

		case payload, ok := <-inputCh:
			if !ok {
				return // service disconnected
			}
			dispatchInputJSON(payload, w, h)

		case <-frameTicker.C:
			fw, fh, err := captureFrame(bgraBuf)
			if err != nil {
				continue
			}
			if !firstFrameLogged && len(bgraBuf) >= 16 {
				firstFrameLogged = true
				log.Printf("helper: first frame captured %dx%d — pixels[0..3] BGRA: %d %d %d %d | [4..7]: %d %d %d %d",
					fw, fh,
					bgraBuf[0], bgraBuf[1], bgraBuf[2], bgraBuf[3],
					bgraBuf[4], bgraBuf[5], bgraBuf[6], bgraBuf[7])
			}
			if fw != w || fh != h {
				log.Printf("helper: resolution changed %dx%d→%dx%d, restarting", w, h, fw, fh)
				return
			}
			nalUnits, err := encodeFrame(bgraBuf, w, h, pts)
			if err != nil {
				continue
			}
			pts += int64(time.Second/time.Duration(fps)) / 100
			if len(nalUnits) == 0 {
				continue
			}
			if err := pipeSend(conn, pipeTypeFrame, nalUnits); err != nil {
				return
			}
		}
	}
}

// ── Cross-session stream ──────────────────────────────────────────────────────

// startCrossSessionStream spawns this binary as a capture helper in sessionID,
// accepts the TCP connection, then relays frames ↔ WebSocket.
func startCrossSessionStream(cfg *Config, token string, sessionID int) error {
	// Listen on a random local port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("cross-session: listen: %w", err)
	}
	addr := ln.Addr().String()
	log.Printf("Stream %s: cross-session relay listening on %s (session %d)", token, addr, sessionID)

	// Spawn the helper in the target session.
	exe, err := os.Executable()
	if err != nil {
		ln.Close()
		return fmt.Errorf("cross-session: get exe path: %w", err)
	}
	cmdLine := fmt.Sprintf(`"%s" --capture-helper --addr=%s`, exe, addr)
	cmdLineW, err := syscall.UTF16PtrFromString(cmdLine)
	if err != nil {
		ln.Close()
		return fmt.Errorf("cross-session: utf16: %w", err)
	}
	var pid C.DWORD
	if rc := C.spawnInSession(C.DWORD(sessionID), (*C.wchar_t)(unsafe.Pointer(cmdLineW)), &pid); rc != 0 {
		ln.Close()
		return fmt.Errorf("cross-session: spawnInSession failed (code %d)", -int(rc))
	}
	log.Printf("Stream %s: spawned helper PID %d in session %d", token, uint32(pid), sessionID)

	// Build WS URL (same logic as startStream).
	ws, err := dialStreamWS(cfg, token)
	if err != nil {
		ln.Close()
		return err
	}

	session := &StreamSession{
		token:  token,
		ws:     ws,
		stopCh: make(chan struct{}),
	}
	activeStreams.Store(token, session)
	go session.runCrossSession(ln)
	return nil
}

// runCrossSession manages a cross-session stream that uses a helper process.
func (s *StreamSession) runCrossSession(ln net.Listener) {
	defer s.stop()
	defer ln.Close()

	// Accept the helper connection.
	ln.(*net.TCPListener).SetDeadline(time.Now().Add(15 * time.Second))
	conn, err := ln.Accept()
	if err != nil {
		log.Printf("Stream %s: accept helper connection failed: %v", s.token, err)
		return
	}
	ln.(*net.TCPListener).SetDeadline(time.Time{}) // clear deadline
	defer conn.Close()

	log.Printf("Stream %s: helper connected from %s", s.token, conn.RemoteAddr())

	// Read init message from helper.
	msgType, initPayload, err := pipeRecv(conn)
	if err != nil || msgType != pipeTypeInit {
		log.Printf("Stream %s: expected init from helper, got type=%d err=%v", s.token, msgType, err)
		return
	}

	// Forward init as WebSocket text frame.
	if err := s.ws.WriteFrame(0x1, initPayload); err != nil {
		log.Printf("Stream %s: send init to browser failed: %v", s.token, err)
		return
	}

	var initInfo struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}
	_ = json.Unmarshal(initPayload, &initInfo)
	log.Printf("Stream %s (cross-session): started %dx%d", s.token, initInfo.Width, initInfo.Height)

	// Keepalive pings to the browser.
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				if err := s.ws.WriteFrame(0x9, nil); err != nil {
					s.stop()
					return
				}
			}
		}
	}()

	// Browser → helper: read WS input frames, forward to helper as pipe messages.
	go func() {
		defer s.stop()
		for {
			opcode, payload, err := s.ws.ReadFrame()
			if err != nil {
				return
			}
			switch opcode {
			case 0x8: // close
				return
			case 0x9: // ping
				_ = s.ws.SendPong(payload)
			case 0x1: // JSON input
				_ = pipeSend(conn, pipeTypeInput, payload)
			}
		}
	}()

	// Helper → browser: read pipe frames, forward as WS binary frames.
	for {
		select {
		case <-s.stopCh:
			_ = pipeSend(conn, pipeTypeStop, nil)
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		msgType, payload, err := pipeRecv(conn)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // check stopCh
			}
			log.Printf("Stream %s: helper disconnected: %v", s.token, err)
			return
		}

		if msgType == pipeTypeFrame {
			frame := make([]byte, 1+len(payload))
			frame[0] = frameTypeH264
			copy(frame[1:], payload)
			if err := s.ws.WriteFrame(0x2, frame); err != nil {
				log.Printf("Stream %s: send frame to browser failed: %v", s.token, err)
				return
			}
		}
	}
}
