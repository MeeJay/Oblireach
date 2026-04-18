//go:build windows

package main

/*
#cgo LDFLAGS: -lwtsapi32 -ladvapi32 -luserenv
#include <windows.h>
#include <wtsapi32.h>
#include <userenv.h>
#include <tlhelp32.h>
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

// findProcPidInSession scans running processes for exeName (case-insensitive)
// running in the given WTS session and returns its PID (0 if not found).
// Used to locate winlogon.exe (SYSTEM, session-attached, Secure Desktop rights)
// in the target session so we can borrow its token — the same technique
// used by TeamViewer / RustDesk to capture UAC Secure Desktop and login
// screens from a service running in session 0.
static DWORD findProcPidInSession(DWORD dwSessionId, LPCWSTR exeName) {
	DWORD found = 0;
	HANDLE hSnap = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
	if (hSnap == INVALID_HANDLE_VALUE) return 0;
	PROCESSENTRY32W pe;
	pe.dwSize = sizeof(pe);
	if (Process32FirstW(hSnap, &pe)) {
		do {
			if (_wcsicmp(pe.szExeFile, exeName) != 0) continue;
			DWORD sid = 0;
			if (ProcessIdToSessionId(pe.th32ProcessID, &sid) && sid == dwSessionId) {
				found = pe.th32ProcessID;
				break;
			}
		} while (Process32NextW(hSnap, &pe));
	}
	CloseHandle(hSnap);
	return found;
}

// openSessionSystemToken returns a primary SYSTEM token for dwSessionId by
// borrowing winlogon.exe's token in that session. The returned token is
// already attached to the session's desktops (Default + Winlogon), so a
// process spawned with it can switch to the Secure Desktop to capture UAC
// prompts and the login screen.
//
// Returns NULL if winlogon.exe is not running in dwSessionId (e.g. a purely
// idle RDS listener with no RDP client connected) — the caller should then
// fall back to the crafted SYSTEM+SetTokenSessionId path.
static HANDLE openSessionSystemToken(DWORD dwSessionId) {
	DWORD pid = findProcPidInSession(dwSessionId, L"winlogon.exe");
	if (pid == 0) return NULL;
	HANDLE hProc = OpenProcess(PROCESS_QUERY_INFORMATION, FALSE, pid);
	if (!hProc) return NULL;
	HANDLE hSrc = NULL;
	if (!OpenProcessToken(hProc, TOKEN_DUPLICATE | TOKEN_QUERY, &hSrc)) {
		CloseHandle(hProc);
		return NULL;
	}
	CloseHandle(hProc);
	HANDLE hPrimary = NULL;
	if (!DuplicateTokenEx(hSrc, TOKEN_ALL_ACCESS, NULL,
			SecurityImpersonation, TokenPrimary, &hPrimary)) {
		CloseHandle(hSrc);
		return NULL;
	}
	CloseHandle(hSrc);
	return hPrimary;
}

// spawnInSession launches cmdLine inside the given Windows session.
//
// Token strategy (in order):
//  1. Borrow winlogon.exe's token from the target session. winlogon runs as
//     SYSTEM with high integrity AND is natively attached to the session's
//     Winlogon desktop + Default desktop — the spawned helper inherits
//     Secure Desktop access, so DXGI Desktop Duplication captures the
//     UAC prompt and the login screen correctly (same technique as
//     TeamViewer / RustDesk).
//  2. If winlogon.exe isn't running in the target session (idle RDS with
//     no RDP client yet, or a transient state), fall back to crafting a
//     SYSTEM primary token from our own (service) token and forcing its
//     session id via SetTokenInformation. Works for already-rendered
//     desktops but does not grant Secure Desktop rights.
//
// outPID receives the PID of the spawned process on success.
// Returns 0 on success, negative GetLastError() on failure.
static int spawnInSession(DWORD sessionId, wchar_t *cmdLine, DWORD *outPID) {
	HANDLE hToken = NULL;

	// Enable privileges on our own token before any token manipulation.
	// LocalSystem holds these but they are not always enabled by default.
	{
		HANDLE hSelf = NULL;
		if (OpenProcessToken(GetCurrentProcess(),
				TOKEN_ADJUST_PRIVILEGES | TOKEN_QUERY, &hSelf)) {
			enablePrivilege(hSelf, L"SeTcbPrivilege");
			enablePrivilege(hSelf, L"SeAssignPrimaryTokenPrivilege");
			enablePrivilege(hSelf, L"SeIncreaseQuotaPrivilege");
			enablePrivilege(hSelf, L"SeDebugPrivilege"); // needed to OpenProcess winlogon.exe
			CloseHandle(hSelf);
		}
	}

	// Strategy 1: borrow winlogon.exe's token from the target session.
	hToken = openSessionSystemToken(sessionId);

	// Strategy 2: craft a SYSTEM token relocated to the target session.
	if (!hToken) {
		HANDLE hSysToken = NULL;
		if (!OpenProcessToken(GetCurrentProcess(), TOKEN_ALL_ACCESS, &hSysToken)) {
			return -(int)GetLastError();
		}
		if (!DuplicateTokenEx(hSysToken, TOKEN_ALL_ACCESS, NULL,
				SecurityImpersonation, TokenPrimary, &hToken)) {
			CloseHandle(hSysToken);
			return -(int)GetLastError();
		}
		CloseHandle(hSysToken);
		DWORD sid = sessionId;
		if (!SetTokenInformation(hToken, TokenSessionId, &sid, sizeof(sid))) {
			CloseHandle(hToken);
			return -(int)GetLastError();
		}
	}

	// Build environment from the user's token for TEMP/APPDATA paths
	LPVOID pEnv = NULL;
	{
		HANDLE hUserToken = NULL;
		if (WTSQueryUserToken(sessionId, &hUserToken)) {
			CreateEnvironmentBlock(&pEnv, hUserToken, FALSE);
			CloseHandle(hUserToken);
		}
		if (!pEnv) CreateEnvironmentBlock(&pEnv, hToken, FALSE);
	}

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
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

// spawnInSessionGo is a Go-friendly wrapper around the C spawnInSession function.
// Returns (pid, returnCode). returnCode 0 = success.
func spawnInSessionGo(sessionID int, cmdLine string) (uint32, int) {
	cmdLineW, err := syscall.UTF16PtrFromString(cmdLine)
	if err != nil {
		return 0, -1
	}
	var pid C.DWORD
	rc := int(C.spawnInSession(C.DWORD(sessionID), (*C.wchar_t)(unsafe.Pointer(cmdLineW)), &pid))
	return uint32(pid), rc
}

// ── TCP pipe message types ────────────────────────────────────────────────────
//
//	Framing: [4-byte uint32 LE payload length][1-byte type][payload]
const (
	pipeTypeInit      = byte(0x01) // helper → service: init JSON
	pipeTypeFrame     = byte(0x02) // helper → service: H.264 NAL units
	pipeTypeInput     = byte(0x03) // service → helper: input JSON
	pipeTypeStop      = byte(0x04) // service → helper: stop signal
	pipeTypeJPEGFrame = byte(0x05) // helper → service: JPEG frame data
	pipeTypeVP9Frame  = byte(0x06) // helper → service: VP9 frame data
	pipeTypeControl   = byte(0x07) // helper → service: JSON control (forwarded as WS text)
	pipeTypeH265Frame = byte(0x08) // helper → service: H.265 frame data
	pipeTypeAV1Frame  = byte(0x09) // helper → service: AV1 frame data
	pipeTypeAudioData = byte(0x0A) // helper → service: audio PCM data
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
			// Do NOT include os.Stdout — the helper runs as a no-window
			// process; stdout may be a broken pipe that blocks writes.
			log.SetOutput(f)
			// Also use this file for encoder diagnostics (bypasses log pkg)
			SetEncoderDiagFromLog(f)
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

	// Lock this goroutine to its OS thread for the entire helper lifetime.
	// COM/DXGI/WMF require all calls on the same thread where CoInitializeEx ran.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Log the helper's identity to help diagnose UIPI issues.
	// SYSTEM token = highest integrity, bypasses UIPI for admin windows.
	if u, err := os.UserHomeDir(); err == nil {
		log.Printf("helper: running as user home=%s", u)
	}

	// Attach this thread to whichever desktop is currently receiving user
	// input. Matters when the session is locked (Winlogon desktop) or at
	// the login screen — DXGI duplication must run on the active desktop
	// to capture it, otherwise we only see a black frame.
	inputSwitchActiveDesktop()

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

	fps := 30
	bitrate := 20_000_000

	// ── Init encoder (OpenH264 > WMF > JPEG fallback) ───────────────────────
	useOpenH264 := false
	if openH264Available() {
		if err := openH264Init(w, h, fps, bitrate); err != nil {
			log.Printf("helper: OpenH264 init failed: %v — trying WMF", err)
		} else {
			useOpenH264 = true
			log.Printf("helper: using OpenH264 encoder")
		}
	}
	if !useOpenH264 {
		if _, err := encoderInit(w, h, fps, bitrate); err != nil {
			log.Printf("helper: WMF encoderInit failed: %v — JPEG only", err)
		}
	}
	defer func() {
		if useOpenH264 {
			openH264Close()
		} else {
			encoderClose()
		}
	}()

	// Set monitor offset for input coordinate mapping
	monOffX, monOffY := captureMonitorOffset()
	setInputMonitorOffset(monOffX, monOffY)
	defer inputUnblock()

	// Init audio before sending init message so we can report availability
	audioInit()

	// ── Send init message to service ─────────────────────────────────────────
	initMsg := map[string]interface{}{
		"type":       "init",
		"width":      w,
		"height":     h,
		"fps":        fps,
		"codec":      "h264",
		"monitors":   enumerateMonitors(),
		"audioRate":  audioSampleRate(),
		"audioAvail": audioInitDone,
	}
	initJSON, _ := json.Marshal(initMsg)
	if err := pipeSend(conn, pipeTypeInit, initJSON); err != nil {
		log.Fatalf("helper: send init failed: %v", err)
	}
	log.Printf("helper: streaming %dx%d@%dfps", w, h, fps)

	// Show a persistent indicator that a remote session is active
	showWatermark("Remote session active")
	defer hideWatermark()

	stopCh := make(chan struct{})
	inputCh := make(chan []byte, 64)
	codecCh := make(chan string, 4)
	monitorCh := make(chan int, 4)
	blockCh := make(chan bool, 4) // input block requests (must run on main thread)

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
				var peek struct {
					Type      string `json:"type"`
					Codec     string `json:"codec"`
					Bitrate   int    `json:"bitrate"`
					Index     int    `json:"index"`
					Block     bool   `json:"block"`
					Recording bool   `json:"recording"`
				}
				if json.Unmarshal(payload, &peek) == nil {
					switch peek.Type {
					case "set_codec":
						select { case codecCh <- peek.Codec: default: }
						continue
					case "set_bitrate":
						if peek.Bitrate > 0 { openH264SetBitrate(peek.Bitrate) }
						continue
					case "set_monitor":
						select { case monitorCh <- peek.Index: default: }
						continue
					case "set_input_block":
						select { case blockCh <- peek.Block: default: }
						continue
					case "set_recording":
						setWatermarkRecording(peek.Recording)
						continue
					}
				}
				select { case inputCh <- payload: default: }
			case pipeTypeStop:
				close(stopCh)
				return
			}
		}
	}()

	// ── Audio capture (in the user's session context) ───────────────────────
	defer audioClose()
	if audioInitDone {
		go func() {
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					data := audioCapture()
					if len(data) > 0 {
						_ = pipeSend(conn, pipeTypeAudioData, data)
					}
				}
			}
		}()
	}

	// ── Capture/encode/send loop ──────────────────────────────────────────────
	bgraBuf := make([]byte, w*h*4)
	frameTicker := time.NewTicker(time.Second / time.Duration(fps))
	defer frameTicker.Stop()

	var pts int64
	var tsMs int64
	firstFrameLogged := false
	useJPEG := false
	useVP9 := false
	useH265 := false
	useAV1 := false

	for {
		select {
		case <-stopCh:
			return

		case block := <-blockCh:
			// BlockInput must run on the same OS thread as SendInput,
			// otherwise SendInput is also blocked (Windows restriction).
			inputBlock(block)
			confirm, _ := json.Marshal(map[string]interface{}{"type": "input_block_status", "blocked": block})
			_ = pipeSend(conn, pipeTypeControl, confirm)

		case payload, ok := <-inputCh:
			if !ok {
				return
			}
			dispatchInputJSON(payload, w, h)

		case newIdx := <-monitorCh:
			log.Printf("helper: monitor switch to %d", newIdx)
			captureClose()
			if useOpenH264 { openH264Close(); useOpenH264 = false }
			if useVP9 { vp9EncoderClose(); useVP9 = false }
			if useH265 { h265EncoderClose(); useH265 = false }
			if useAV1 { av1EncoderClose(); useAV1 = false }
			encoderClose()
			useJPEG = false

			if err := captureInitMonitor(newIdx); err != nil {
				log.Printf("helper: monitor switch failed: %v", err)
				continue
			}
			w = captureWidth()
			h = captureHeight()
			bgraBuf = make([]byte, w*h*4)
			monOffX, monOffY = captureMonitorOffset()
			setInputMonitorOffset(monOffX, monOffY)

			if openH264Available() {
				if err := openH264Init(w, h, fps, bitrate); err == nil {
					useOpenH264 = true
				}
			}
			if !useOpenH264 { useJPEG = true }

			reInit := map[string]interface{}{
				"type": "init", "width": w, "height": h,
				"fps": fps, "codec": "h264", "monitors": enumerateMonitors(),
			}
			reInitJSON, _ := json.Marshal(reInit)
			_ = pipeSend(conn, pipeTypeInit, reInitJSON)

		case newCodec := <-codecCh:
			log.Printf("helper: codec switch requested: %s", newCodec)
			if useOpenH264 { openH264Close(); useOpenH264 = false }
			if useVP9 { vp9EncoderClose(); useVP9 = false }
			if useH265 { h265EncoderClose(); useH265 = false }
			if useAV1 { av1EncoderClose(); useAV1 = false }
			if !useJPEG { encoderClose() }
			useJPEG = false

			switch newCodec {
			case "h264":
				if openH264Available() {
					if err := openH264Init(w, h, fps, bitrate); err == nil {
						useOpenH264 = true
					}
				}
				if !useOpenH264 { useJPEG = true }
			case "h265":
				if h265Available() {
					if err := h265EncoderInit(w, h, fps, bitrate/1000); err == nil {
						useH265 = true
					} else { useJPEG = true }
				} else { useJPEG = true }
			case "vp9":
				if err := vp9EncoderInit(w, h, fps, bitrate/1000); err == nil {
					useVP9 = true
				} else { useJPEG = true }
			case "av1":
				if av1Available() {
					if err := av1EncoderInit(w, h, fps, bitrate/1000); err == nil {
						useAV1 = true
					} else { useJPEG = true }
				} else { useJPEG = true }
			case "jpeg":
				useJPEG = true
			}
			log.Printf("helper: switched to %s", newCodec)
			// Send codec_switch confirmation to service (→ browser)
			switchJSON, _ := json.Marshal(map[string]string{"type": "codec_switch", "codec": newCodec})
			_ = pipeSend(conn, pipeTypeControl, switchJSON)

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
			if useJPEG {
				jpegData, err := encodeJPEG(bgraBuf, w, h, 15)
				if err != nil {
					continue
				}
				if err := pipeSend(conn, pipeTypeJPEGFrame, jpegData); err != nil {
					return
				}
			} else if useAV1 {
				av1Data, err := av1EncodeFrame(bgraBuf, w, h)
				if err != nil { continue }
				if len(av1Data) == 0 { continue }
				if err := pipeSend(conn, pipeTypeAV1Frame, av1Data); err != nil { return }
			} else if useH265 {
				h265Data, err := h265EncodeFrame(bgraBuf, w, h)
				if err != nil { continue }
				if len(h265Data) == 0 { continue }
				if err := pipeSend(conn, pipeTypeH265Frame, h265Data); err != nil { return }
			} else if useVP9 {
				vp9Data, err := vp9EncodeFrame(bgraBuf, w, h)
				if err != nil { continue }
				if len(vp9Data) == 0 { continue }
				if err := pipeSend(conn, pipeTypeVP9Frame, vp9Data); err != nil { return }
			} else if useOpenH264 {
				nalUnits, err := openH264EncodeFrame(bgraBuf, w, h, tsMs)
				tsMs += int64(1000 / fps)
				if err != nil {
					log.Printf("helper: OpenH264 error: %v", err)
					continue
				}
				if len(nalUnits) == 0 {
					continue
				}
				if err := pipeSend(conn, pipeTypeFrame, nalUnits); err != nil {
					return
				}
			} else {
				nalUnits, err := encodeFrame(bgraBuf, w, h, pts)
				if err != nil {
					continue
				}
				pts += int64(time.Second/time.Duration(fps)) / 100
				if len(nalUnits) == 0 {
					if encodeInputCount >= jpegFallbackThreshold && encodeOutputCount == 0 {
						log.Printf("helper: WMF H.264 produced 0 output after %d frames → JPEG fallback", encodeInputCount)
						useJPEG = true
						encoderClose()
					}
					continue
				}
				if err := pipeSend(conn, pipeTypeFrame, nalUnits); err != nil {
					return
				}
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
	ab := newAdaptiveBitrate(30) // match the helper's fps
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

		if msgType == pipeTypeControl {
			// Forward JSON control message to browser as WS text frame
			if err := s.ws.WriteFrame(0x1, payload); err != nil {
				log.Printf("Stream %s: send control to browser failed: %v", s.token, err)
				return
			}
		} else if msgType == pipeTypeAudioData {
			// Audio data from helper → forward as WS binary to browser
			frame := make([]byte, 1+len(payload))
			frame[0] = frameTypeAudio
			copy(frame[1:], payload)
			_ = s.ws.WriteFrame(0x2, frame) // best-effort, don't abort stream on audio error
		} else if msgType == pipeTypeFrame || msgType == pipeTypeJPEGFrame || msgType == pipeTypeVP9Frame || msgType == pipeTypeH265Frame || msgType == pipeTypeAV1Frame {
			ft := frameTypeH264
			if msgType == pipeTypeJPEGFrame {
				ft = frameTypeJPEG
			} else if msgType == pipeTypeVP9Frame {
				ft = frameTypeVP9
			} else if msgType == pipeTypeH265Frame {
				ft = frameTypeH265
			} else if msgType == pipeTypeAV1Frame {
				ft = frameTypeAV1
			}
			frame := make([]byte, 1+len(payload))
			frame[0] = ft
			copy(frame[1:], payload)

			sendStart := time.Now()
			if err := s.ws.WriteFrame(0x2, frame); err != nil {
				log.Printf("Stream %s: send frame to browser failed: %v", s.token, err)
				return
			}
			if newBr := ab.report(time.Since(sendStart)); newBr > 0 {
				// Tell helper to adjust bitrate
				brCmd, _ := json.Marshal(map[string]interface{}{"type": "set_bitrate", "bitrate": newBr})
				_ = pipeSend(conn, pipeTypeInput, brCmd)
				// Tell browser the new bitrate
				brMsg, _ := json.Marshal(map[string]interface{}{"type": "bitrate", "bitrate": newBr})
				_ = s.ws.WriteFrame(0x1, brMsg)
				log.Printf("Stream %s: adaptive bitrate → %d kbps", s.token, newBr/1000)
			}
		}
	}
}
