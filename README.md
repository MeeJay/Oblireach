# Oblireach

Cross-platform remote desktop agent for the **Obliance** ecosystem. Real-time screen streaming, bidirectional chat, remote input control, multi-monitor support, and session management - all from the browser or the native desktop client.

---

## Features at a Glance

- **5 video codecs** - H.264 (OpenH264), VP9 (libvpx), AV1 (SVT-AV1), JPEG fallback, optional H.265 (x265)
- **Adaptive bitrate** - 1-100 Mbps, auto-adjusted every 0.5s based on network conditions
- **Live codec switching** - change codec on the fly without reconnecting
- **Multi-monitor** - enumerate and switch between displays, mini-layout selector in the viewer
- **Audio streaming** - WASAPI loopback capture (Windows), PCM 16-bit mono
- **Remote input** - mouse, keyboard (layout-aware), clipboard sync, input blocking
- **Secure Attention Sequence** - Ctrl+Alt+Del via SAS API (login screen access)
- **Bidirectional chat** - WebView2 popup on the remote device with operator avatar, typing indicators, auto-reopen
- **Session notifications** - toast alerts and "REC" watermark on the remote device
- **Cross-session capture** - works from Session 0 (Windows services) via SYSTEM token spawning
- **Desktop client app** - native WebView2 viewer with screenshot, zoom, annotation, quick actions, session recording
- **Cross-platform** - Windows (DXGI/GDI), macOS (ScreenCaptureKit), Linux (X11/XRandr)

---

## Architecture

```
Remote Device (Agent)                          Operator
┌──────────────────────┐                  ┌──────────────────┐
│  Capture (DXGI/GDI)  │                  │  Browser / Client│
│  Encode (H.264/VP9…) │──── WebSocket ──>│  WebCodecs decode│
│  Audio (WASAPI)       │    via Obliance  │  Canvas render   │
│  Input (SendInput)   │<── WebSocket ────│  Mouse/Keyboard  │
│  Chat (WebView2)     │<── Socket.io ───>│  Chat panel      │
└──────────────────────┘     relay        └──────────────────┘
```

### Streaming Pipeline

```
User session (e.g. session 1)
  └─ Helper process (SYSTEM token, same session)
       ├─ captureInit()   → DXGI Desktop Duplication (fallback: GDI BitBlt)
       ├─ Encoder init    → OpenH264 > WMF H.264 > JPEG fallback
       ├─ Codec switch    → H.264, VP9, AV1, JPEG (live switch via set_codec)
       ├─ audioInit()     → WASAPI loopback (PCM 16-bit mono)
       └─ TCP pipe → Service (session 0)
                         └─ WebSocket → Obliance relay
                                          └─ WebSocket → Browser (WebCodecs)
```

### WebSocket Binary Frame Types

| Byte | Codec |
|------|-------|
| `0x01` | JPEG (fallback, quality 15) |
| `0x02` | H.264 (OpenH264 or WMF, Annex B) |
| `0x03` | VP9 (libvpx) |
| `0x04` | H.265/HEVC (x265, optional) |
| `0x05` | AV1 (SVT-AV1) |
| `0x06` | Audio (PCM 16-bit mono LE) |

---

## Video Codecs

All codecs are loaded dynamically via `LoadLibrary` at runtime - no compile-time dependency on codec DLLs.

| Codec | Library | License | Included |
|-------|---------|---------|----------|
| H.264 | OpenH264 (Cisco) | BSD 2-Clause | Yes |
| VP9 | libvpx | BSD 3-Clause | Yes |
| AV1 | SVT-AV1 | BSD 3-Clause Clear | Yes |
| JPEG | Pure Go | N/A | Yes (built-in) |
| H.265 | x265 | GPL v2 | No (opt-in build flag) |

**Automatic fallback chain:** OpenH264 → WMF H.264 → JPEG (if encoder produces 0 output after 30 frames).

### H.265 Support (Optional)

H.265 is behind a build tag due to its GPL v2 license. It is **not included** in default builds or the MSI installer. Users who need H.265 can supply their own `libx265-215.dll` in the agent directory - the encoder will be detected and loaded automatically at runtime.

---

## Capture by Platform

| Platform | Capture | Input | Audio |
|----------|---------|-------|-------|
| **Windows** | DXGI Desktop Duplication (fallback GDI) | SendInput + VkKeyScan (layout-aware) | WASAPI loopback |
| **macOS** | ScreenCaptureKit (SCStream) | CGEventPost | Not implemented |
| **Linux** | X11 XGetImage + XRandr | XTest | Not implemented |

---

## Remote Input

- **Mouse**: `SendInput` with `MOUSEEVENTF_ABSOLUTE` + `MOUSEEVENTF_VIRTUALDESK` (multi-monitor)
- **Keyboard**: layout-aware via `VkKeyScanW` (AZERTY, QWERTZ, etc.)
- **Desktop switch**: `OpenInputDesktop` + `SetThreadDesktop` for login screen injection
- **Clipboard**: bidirectional sync via `OpenClipboard` / `SetClipboardData`
- **Input blocking**: `BlockInput(TRUE)` to lock remote keyboard/mouse
- **Ctrl+Alt+Del**: Secure Attention Sequence via `sas.dll`

---

## Chat System

- **WebView2 popup** on the remote device with dark charcoal theme
- **Operator avatar** forwarded from Obliance profile
- **Typing indicators** - bidirectional, debounced (2s), 3s timeout
- **Auto-reopen** - if the user closes the chat, it reopens when the operator sends a new message
- **Bubble minimize** - chat collapses to a floating bubble with unread badge
- **Remote control request** - operator can request control, user sees Accept/Deny buttons
- **i18n** - auto-detects French/English from system language

---

## Desktop Client App

Native Windows application (WebView2) for operators to connect to remote devices.

- **NeonUI** dark charcoal theme
- **10 built-in tools**: Screenshot, Clipboard Sync, Performance HUD, Zoom/Scale, System Keys, Session Recording, Quick Actions, Annotation/Whiteboard, Favorites/Recent, Multi-Tab Sessions
- **Chat panel** with typing indicators and operator avatar
- **SSO login** via Obliance Obligate or local credentials with "Remember me"
- **MSI installer** with Desktop & Start Menu shortcuts

---

## Build

### Agent - Windows (CGo + MinGW)

```bash
cd agent
go build -ldflags="-s -w -X main.agentVersion=$(cat VERSION)" -o dist/oblireach-agent.exe .
```

### Agent - Multi-platform (via build script)

```
000-Build-Agent.bat  →  Windows (CGo) + MSI (WiX) + Mac (SSH) + Linux (SSH)
```

### Client App

```
cd client-app
build.bat  →  EXE + MSI (WiX)
```

### Required DLLs (Windows, loaded dynamically)

| DLL | Purpose |
|-----|---------|
| `openh264-2.4.1-win64.dll` | H.264 encoder (Cisco, BSD) |
| `libvpx-1.dll` | VP9 encoder (MSYS2 prebuilt) |
| `libSvtAv1Enc-2.dll` | AV1 encoder (MSYS2 prebuilt) |
| `libwinpthread-1.dll` | MinGW runtime |
| `libstdc++-6.dll` | MinGW runtime |
| `libgcc_s_sjlj-1.dll` | MinGW runtime |

---

## Connection to Obliance

### Agent → Obliance

```
Command WS:  /api/oblireach/ws?uuid={deviceUUID}
  Header: X-Api-Key: {apiKey}
  Bidirectional: heartbeat ↑, commands ↓, chat messages ↑↓

Stream WS:   /api/remote/agent-tunnel/{sessionToken}
  Header: X-Api-Key: {apiKey}
```

### Browser → Obliance

```
Stream WS:   /api/remote/tunnel/{sessionToken}
  Auth: session cookie

Socket.io:   chat:open, chat:message, chat:close, chat:typing, chat:request_remote
```

---

## Session Management

- Helper process spawned with **SYSTEM token** (bypasses UIPI for admin windows)
- Environment block built from the user token (TEMP, APPDATA)
- **Inactivity timeout**: 10 minutes, with 30-second warning before disconnect
- **Available codecs** reported in the `init` message so the viewer only shows supported options

---

## Project Structure

```
Oblireach/
├── agent/                    # Remote desktop agent (Go + CGo)
│   ├── main.go               # Entrypoint, flags, service mode
│   ├── stream.go             # StreamSession, frame types, adaptive bitrate
│   ├── capture_windows.go    # DXGI / GDI capture + cursor overlay
│   ├── encode_*.go           # Per-codec encoders (dynamic loading)
│   ├── input_*.go            # Mouse/keyboard injection per platform
│   ├── chat*.go              # Chat session management + WebView2 window
│   ├── watermark_windows.go  # "REC" indicator overlay
│   ├── websocket.go          # WebSocket client (RFC 6455)
│   ├── cmd_ws.go             # Persistent command channel
│   ├── push.go               # Command handlers
│   ├── installer/product.wxs # WiX MSI template
│   └── VERSION               # Current version
├── client-app/               # Desktop viewer (Go + WebView2)
│   ├── main.go               # WebView2 window + local proxy
│   ├── ui.go                 # Embedded HTML/CSS/JS viewer
│   ├── proxy.go              # HTTP/WebSocket proxy to Obliance
│   └── installer/product.wxs # WiX MSI template
└── 000-Build-Agent.bat       # Multi-platform build script
```

---

## License

[Elastic License 2.0 (ELv2)](LICENSE) - free for internal use (including commercial), not permitted as a hosted service. See the LICENSE file for full terms and third-party notices.

---

> Built with [Claude Code](https://claude.ai/claude-code)
