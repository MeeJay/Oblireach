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

// tryBorrowToken opens the primary token of exeName in dwSessionId and
// returns it DIRECTLY — no DuplicateTokenEx. RustDesk's approach: calling
// DuplicateTokenEx creates a copy whose logon-session affinity is
// disconnected from the source token, which Windows treats as unprivileged
// for GPU / desktop-duplication access. Using the original token handle
// preserves that affinity and lets DXGI see the real hardware adapters
// instead of falling back to "Microsoft Basic Render Driver" (WARP).
static HANDLE tryBorrowToken(DWORD dwSessionId, LPCWSTR exeName, DWORD *outPid) {
	DWORD pid = findProcPidInSession(dwSessionId, exeName);
	if (outPid) *outPid = pid;
	if (pid == 0) return NULL;
	HANDLE hProc = OpenProcess(PROCESS_ALL_ACCESS, FALSE, pid);
	if (!hProc) return NULL;
	HANDLE hToken = NULL;
	if (!OpenProcessToken(hProc, TOKEN_ALL_ACCESS, &hToken)) {
		CloseHandle(hProc);
		return NULL;
	}
	CloseHandle(hProc);
	return hToken;
}

// openSessionSystemToken returns winlogon.exe's primary token from
// dwSessionId — borrowed directly, not duplicated. This is the exact
// technique RustDesk uses for no-user sessions (their LaunchProcessWin
// with as_user=FALSE). Any other process (LogonUI, dwm, csrss) gave
// identical E_ACCESSDENIED results in our testing, so we match RustDesk
// 1:1.
static HANDLE openSessionSystemToken(DWORD dwSessionId, DWORD *outDiag, DWORD *outErr, DWORD *outPid) {
	*outDiag = 0; *outErr = 0; *outPid = 0;
	HANDLE h = tryBorrowToken(dwSessionId, L"winlogon.exe", outPid);
	if (!h) {
		*outDiag = 1;
		*outErr = GetLastError();
	}
	return h;
}

// spawnInSession launches cmdLine inside the given Windows session.
//
// Token strategy (in order):
//  1. WTSQueryUserToken — the logged-in user's primary token. Required for
//     DXGI Desktop Duplication to see the session's outputs: DXGI is WinSta-
//     scoped and a process running on the Winlogon WinSta (which is what
//     the winlogon.exe token gives you) enumerates zero outputs for the
//     user's session. Works for any Active/Disconnected user session.
//  2. winlogon.exe's token — SYSTEM-level, attached to both Default and
//     Winlogon desktops. Only used when no user is logged in (e.g. console
//     at login screen). Grants Secure Desktop access for capturing the
//     sign-in UI and UAC (when relevant).
//  3. Crafted SYSTEM token with SetTokenSessionId. Last-resort fallback.
//
// outPID receives the PID of the spawned process on success.
// outStrategy receives: 0=WTSQueryUserToken, 1=winlogon token,
// 2=SetTokenSessionId fallback. outDiag/outErr/outWlPid expose why winlogon
// path was skipped when strategy=2. Returns 0 on success, negative
// GetLastError() on failure.
// desktopName parameter: if non-NULL, passed to STARTUPINFO.lpDesktop so
// the child process is created directly on that desktop (e.g.
// "winsta0\\winlogon" for capturing the sign-in UI). NULL falls back to
// "winsta0\\default". Starting the helper on winlogon desktop is the
// only way to make Magnification capture the login prompt — once a
// thread has any window on one desktop, SetThreadDesktop refuses to
// move it (ERROR_BUSY, 170).
static int spawnInSessionOnDesktop(DWORD sessionId, wchar_t *cmdLine, wchar_t *desktopName,
		DWORD *outPID, DWORD *outStrategy, DWORD *outDiag, DWORD *outErr, DWORD *outWlPid);

static int spawnInSession(DWORD sessionId, wchar_t *cmdLine, DWORD *outPID,
		DWORD *outStrategy, DWORD *outDiag, DWORD *outErr, DWORD *outWlPid) {
	return spawnInSessionOnDesktop(sessionId, cmdLine, NULL,
		outPID, outStrategy, outDiag, outErr, outWlPid);
}

static int spawnInSessionOnDesktop(DWORD sessionId, wchar_t *cmdLine, wchar_t *desktopName,
		DWORD *outPID, DWORD *outStrategy, DWORD *outDiag, DWORD *outErr, DWORD *outWlPid) {
	HANDLE hToken = NULL;
	*outStrategy = 255;
	*outDiag = 0; *outErr = 0; *outWlPid = 0;

	// No explicit privilege enabling — RustDesk's LaunchProcessWin doesn't
	// do it, they rely on the service running as LocalSystem which already
	// has all the needed privileges enabled by default. Any extra enabling
	// we do can subtly change token semantics and is unnecessary.

	// Strategy 0: build a SYSTEM primary token relocated to the target
	// session. SYSTEM integrity is above Medium which is what UIPI on
	// Windows Server 2025 requires for SendInput to reach Medium-IL user
	// apps — an explorer.exe-borrowed or WTSQueryUserToken-derived user
	// token runs at Medium IL and gets err=5 (ACCESS_DENIED) on every
	// SendInput call, even targeting the same user's own windows. SYSTEM
	// bypasses UIPI. Capture side still works because Magnification API
	// operates independently of token integrity.
	//
	// This is the historically working path that we briefly abandoned in
	// pursuit of RustDesk parity. RustDesk seems to tolerate whatever UIPI
	// does on their test OSes (older Server / Windows 10) but Server 2025
	// is stricter.
	{
		// Privileges must be enabled — LocalSystem holds them but they are
		// not always active in the current token.
		HANDLE hSelf = NULL;
		if (OpenProcessToken(GetCurrentProcess(),
				TOKEN_ADJUST_PRIVILEGES | TOKEN_QUERY, &hSelf)) {
			enablePrivilege(hSelf, L"SeTcbPrivilege");
			enablePrivilege(hSelf, L"SeAssignPrimaryTokenPrivilege");
			enablePrivilege(hSelf, L"SeIncreaseQuotaPrivilege");
			enablePrivilege(hSelf, L"SeDebugPrivilege");
			CloseHandle(hSelf);
		}
		HANDLE hSysToken = NULL;
		if (OpenProcessToken(GetCurrentProcess(), TOKEN_ALL_ACCESS, &hSysToken)) {
			HANDLE hPrimary = NULL;
			if (DuplicateTokenEx(hSysToken, TOKEN_ALL_ACCESS, NULL,
					SecurityImpersonation, TokenPrimary, &hPrimary)) {
				DWORD sid = sessionId;
				if (SetTokenInformation(hPrimary, TokenSessionId, &sid, sizeof(sid))) {
					hToken = hPrimary;
					*outStrategy = 0;
				} else {
					CloseHandle(hPrimary);
				}
			}
			CloseHandle(hSysToken);
		}
	}

	// Strategy 1: borrow a SYSTEM-context process token from the target
	// session (dwm.exe / csrss.exe / winlogon.exe, in that order).
	// winlogon's token is intentionally denied real GPU access by Windows
	// — DXGI returns only "Microsoft Basic Render Driver" (WARP) and
	// E_ACCESSDENIED on DuplicateOutput. DWM's token holds the GPU access
	// and gives us real adapters.
	if (!hToken) {
		hToken = openSessionSystemToken(sessionId, outDiag, outErr, outWlPid);
		if (hToken) *outStrategy = 1;
	}

	// Strategy 2: craft a SYSTEM token relocated to the target session.
	if (!hToken) {
		*outStrategy = 2;
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

	// Build the environment block from a real user token whenever possible
	// (TEMP/APPDATA paths, DISPLAY routing). Fall back to our own token
	// (SYSTEM-crafted) when no user is logged in. The old working path did
	// this unconditionally; trimming it introduced UIPI regressions that
	// showed up as SendInput err=5.
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
	si.lpDesktop = desktopName ? desktopName : L"winsta0\\default";
	// Explicit desktop = put the child on the interactive desktop of the
	// target session. With lpDesktop=NULL Windows derives it from the
	// spawning token's logon-session defaults, which for a SYSTEM-crafted
	// token relocated via SetTokenInformation ends up on a service window
	// station. From there SendInput gets err=5 / ACCESS_DENIED even though
	// SYSTEM integrity should bypass UIPI — Windows also blocks service-
	// station processes from injecting into user desktops.

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
// Returns (pid, returnCode). returnCode 0 = success. Logs which token strategy
// was taken and, when the winlogon-token path is rejected, why.
func spawnInSessionGo(sessionID int, cmdLine string) (uint32, int) {
	cmdLineW, err := syscall.UTF16PtrFromString(cmdLine)
	if err != nil {
		return 0, -1
	}
	var pid, strategy, diag, errCode, wlPid C.DWORD
	rc := int(C.spawnInSession(
		C.DWORD(sessionID),
		(*C.wchar_t)(unsafe.Pointer(cmdLineW)),
		&pid, &strategy, &diag, &errCode, &wlPid,
	))
	logSpawnStrategy(sessionID, uint32(strategy), uint32(diag), uint32(errCode), uint32(wlPid))
	return uint32(pid), rc
}

// logSpawnStrategy prints which token path the helper spawn used.
//   0 → WTSQueryUserToken (logged-in user; required for DXGI to see the
//       session's outputs — user's WinSta\Default).
//   1 → winlogon.exe's token (no user in session; Secure Desktop access).
//   2 → crafted SYSTEM + SetTokenSessionId fallback (last resort, no
//       desktop or output visibility — rarely useful).
// For strategy=1 (winlogon), diag explains any earlier failure of the
// user-token path too if the code surfaces one.
func logSpawnStrategy(sessionID int, strategy, diag, errCode, wlPid uint32) {
	switch strategy {
	case 0:
		log.Printf("spawnInSession(%d): token strategy=user (WTSQueryUserToken)", sessionID)
	case 1:
		log.Printf("spawnInSession(%d): token strategy=system-process (PID %d — no user in session)",
			sessionID, wlPid)
	case 2:
		reason := "unknown"
		switch diag {
		case 1:
			reason = "winlogon.exe not found in session"
		case 2:
			reason = fmt.Sprintf("OpenProcess(winlogon PID %d) failed (err=%d)", wlPid, errCode)
		case 3:
			reason = fmt.Sprintf("OpenProcessToken failed (err=%d)", errCode)
		case 4:
			reason = fmt.Sprintf("DuplicateTokenEx failed (err=%d)", errCode)
		}
		log.Printf("spawnInSession(%d): token strategy=fallback (%s)", sessionID, reason)
	}
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
	pipeTypeSAS       = byte(0x0B) // helper → service: request SendSAS (Ctrl+Alt+Del). Must fire from Session 0.
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
	// Dual logging: primary log in per-user TEMP (current behaviour) plus a
	// shared log under C:\Windows\Temp keyed by session+PID so the admin
	// can read it regardless of which user owns the helper.
	var logFiles []*os.File
	if tmpDir := os.TempDir(); tmpDir != "" {
		if f, err := os.OpenFile(
			filepath.Join(tmpDir, "oblireach-helper.log"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			logFiles = append(logFiles, f)
		}
	}
	if f, err := os.OpenFile(
		fmt.Sprintf(`C:\Windows\Temp\oblireach-helper-s%d-pid%d.log`,
			currentSessionID(), os.Getpid()),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		logFiles = append(logFiles, f)
	}
	if len(logFiles) > 0 {
		writers := make([]io.Writer, len(logFiles))
		for i, f := range logFiles {
			writers[i] = f
		}
		log.SetOutput(io.MultiWriter(writers...))
		// Encoder diagnostic sink — use the first concrete file handle.
		SetEncoderDiagFromLog(logFiles[0])
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

	// Route Ctrl+Alt+Del back to the service. SendSAS from here (Session N,
	// SYSTEM token) returns TRUE but the secure desktop never appears —
	// Windows requires the call from Session 0. The service (SCM-launched,
	// LocalSystem, Session 0) handles pipeTypeSAS by invoking inputSAS()
	// directly. Same IPC pattern as RustDesk's ipc::Data::SAS.
	inputSASHook = func() {
		if err := pipeSend(conn, pipeTypeSAS, nil); err != nil {
			log.Printf("helper: failed to pipe SAS request to service: %v", err)
		}
	}

	// Lock this goroutine to its OS thread for the entire helper lifetime.
	// COM/DXGI/WMF require all calls on the same thread where CoInitializeEx ran.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Keep the display awake for the whole helper lifetime. On Hyper-V VMs
	// with no active console viewer, the Hyper-V Video adapter enters an
	// idle power state where DXGI Desktop Duplication captures a blank
	// framebuffer — the agent streams "nothing" and the client sits on
	// "Waiting for agent to connect…". Opening Hyper-V Manager's console
	// window or launching RustDesk resolves it symptomatically because
	// both hold ES_DISPLAY_REQUIRED while active. We do the same here, so
	// the fix is intrinsic to Oblireach.
	inputKeepDisplayAwake()

	// Log the helper's identity to help diagnose UIPI issues.
	// SYSTEM token = highest integrity, bypasses UIPI for admin windows.
	if u, err := os.UserHomeDir(); err == nil {
		log.Printf("helper: running as user home=%s", u)
	}

	// RustDesk doesn't call SetProcessWindowStation / SetThreadDesktop on
	// the capture thread — they let Windows route the process to the
	// correct WinSta/Desktop automatically from the spawning token. Doing
	// so explicitly adds token-ACL handshakes that can block DXGI access.

	// No-user (Winlogon / sign-in / lock screen) capture works because the
	// helper is spawned directly on winsta0\winlogon via STARTUPINFO.lpDesktop
	// in startCrossSessionStream when the target session has no user — DXGI
	// Desktop Duplication then runs natively against the physical display
	// adapter (Hyper-V Video / the console's real GPU output) that has the
	// Winlogon content already composed on it.
	//
	// We intentionally do NOT install or plug any Indirect Display Driver
	// here: neither TeamViewer nor RustDesk do for standard pre-login
	// capture (their IDDs are for privacy mode / headless-no-console use
	// cases that we haven't needed to support). The entire IDD saga was
	// chasing a distraction — the real fix was always the correct desktop
	// at spawn time.

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
	blockCh := make(chan bool, 4) // handled by main capture thread

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

	defer inputUnblock()
	for {
		select {
		case <-stopCh:
			return

		case block := <-blockCh:
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

	// Pre-flight: detect whether the target session has a logged-in user.
	// If not, spawn the helper directly on WinSta0\Winlogon so the
	// Magnification API in the helper process sees the sign-in UI (which
	// lives on that desktop) rather than an empty Default desktop.
	var desktopW *uint16
	{
		hasUser := false
		for _, s := range enumerateSessions() {
			if s.ID == sessionID && s.Username != "" {
				hasUser = true
				break
			}
		}
		if !hasUser {
			desktopW, _ = syscall.UTF16PtrFromString(`winsta0\winlogon`)
		}
	}

	var pid, strategy, diag, errCode, wlPid C.DWORD
	rc := C.spawnInSessionOnDesktop(
		C.DWORD(sessionID),
		(*C.wchar_t)(unsafe.Pointer(cmdLineW)),
		(*C.wchar_t)(unsafe.Pointer(desktopW)),
		&pid, &strategy, &diag, &errCode, &wlPid,
	)
	if rc != 0 {
		ln.Close()
		return fmt.Errorf("cross-session: spawnInSession failed (code %d)", -int(rc))
	}
	logSpawnStrategy(sessionID, uint32(strategy), uint32(diag), uint32(errCode), uint32(wlPid))
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
		} else if msgType == pipeTypeSAS {
			// Helper asked the service to trigger SendSAS. Must fire from
			// Session 0 (the Windows service) for the Secure Attention Sequence
			// to actually produce the secure-desktop transition — calling
			// SendSAS from the helper (Session N, SYSTEM token) returns TRUE
			// but silently no-ops. Mirrors RustDesk's ipc::Data::SAS flow.
			inputSAS()
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
