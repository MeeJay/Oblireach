//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
)

var (
	chatUser32                  = syscall.NewLazyDLL("user32.dll")
	chatProcFindWindowW         = chatUser32.NewProc("FindWindowW")
	chatProcSetWindowLongPtrW   = chatUser32.NewProc("SetWindowLongPtrW")
	chatProcGetWindowLongPtrW   = chatUser32.NewProc("GetWindowLongPtrW")
	chatProcSetWindowPos        = chatUser32.NewProc("SetWindowPos")
	chatProcSetLayeredAttr      = chatUser32.NewProc("SetLayeredWindowAttributes")
	chatProcSysParamsInfo       = chatUser32.NewProc("SystemParametersInfoW")
	chatProcGetUILang           = syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")
)

type chatWinRECT struct{ Left, Top, Right, Bottom int32 }

func chatIsFrench() bool {
	ret, _, _ := chatProcGetUILang.Call()
	return (uint16(ret) & 0x3FF) == 0x0C
}

var (
	chatProcCreateRoundRgn = syscall.NewLazyDLL("gdi32.dll").NewProc("CreateRoundRectRgn")
	chatProcSetWindowRgn   = chatUser32.NewProc("SetWindowRgn")
)

func chatMakePopup(title string) {
	titleW, _ := syscall.UTF16PtrFromString(title)
	hwnd, _, _ := chatProcFindWindowW.Call(0, uintptr(unsafe.Pointer(titleW)))
	if hwnd == 0 { return }

	const gwlStyle = ^uintptr(15)   // GWL_STYLE = -16
	const gwlExStyle = ^uintptr(19) // GWL_EXSTYLE = -20

	// Remove titlebar — WS_POPUP | WS_VISIBLE
	chatProcSetWindowLongPtrW.Call(hwnd, gwlStyle, 0x80000000|0x10000000)

	// WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_LAYERED
	ex, _, _ := chatProcGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	chatProcSetWindowLongPtrW.Call(hwnd, gwlExStyle,
		ex|0x00000008|0x00000080|0x00080000)

	// Transparency 94%
	chatProcSetLayeredAttr.Call(hwnd, 0, 240, 2)

	// Rounded corners via region clipping (18px radius)
	rgn, _, _ := chatProcCreateRoundRgn.Call(0, 0, 380, 520, 18, 18)
	if rgn != 0 {
		chatProcSetWindowRgn.Call(hwnd, rgn, 1) // bRedraw=TRUE
	}

	// Position bottom-right
	var wa chatWinRECT
	chatProcSysParamsInfo.Call(0x0030, 0, uintptr(unsafe.Pointer(&wa)), 0)
	x := int(wa.Right) - 380 - 16
	y := int(wa.Bottom) - 520 - 16
	chatProcSetWindowPos.Call(hwnd, ^uintptr(0),
		uintptr(x), uintptr(y), 380, 520, 0x0040)
}

var chatConn net.Conn
var chatConnMu sync.Mutex
var chatWebview webview2.WebView

//export goChatSend
func goChatSend(action, text string) {
	chatConnMu.Lock()
	conn := chatConn
	chatConnMu.Unlock()
	if conn == nil {
		return
	}

	var msg []byte
	switch action {
	case "user_message":
		msg, _ = json.Marshal(map[string]interface{}{
			"action": "user_message",
			"text":   text,
			"from":   getUserDisplayName(),
		})
		chatPipeSend(conn, chatPipeMsg, msg)
	case "user_closed":
		msg, _ = json.Marshal(map[string]string{"action": "user_closed"})
		chatPipeSend(conn, chatPipeEvent, msg)
	case "allow_remote":
		msg, _ = json.Marshal(map[string]interface{}{"action": "allow_remote", "allowed": true})
		chatPipeSend(conn, chatPipeEvent, msg)
	case "deny_remote":
		msg, _ = json.Marshal(map[string]interface{}{"action": "deny_remote", "allowed": false})
		chatPipeSend(conn, chatPipeEvent, msg)
	}
}

func getUserDisplayName() string {
	u, err := os.UserHomeDir()
	if err == nil {
		return filepath.Base(u)
	}
	h, _ := os.Hostname()
	return h
}

func runChatHelperMode(addr, chatID, operatorName string) {
	if tmpDir := os.TempDir(); tmpDir != "" {
		if f, err := os.OpenFile(
			filepath.Join(tmpDir, "oblireach-chat.log"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			log.SetOutput(f)
		}
	}
	log.Printf("chat-helper: connecting to %s (chatID=%s operator=%s)", addr, chatID, operatorName)

	runtime.LockOSThread()

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
		log.Fatalf("chat-helper: connect failed: %v", err)
	}

	chatConnMu.Lock()
	chatConn = conn
	chatConnMu.Unlock()
	defer conn.Close()

	// Wait for init message with avatar data
	operatorAvatar := ""
	initType, initPayload, err := chatPipeRecv(conn)
	if err != nil || initType != chatPipeInit {
		log.Printf("chat-helper: expected init, got type=%d err=%v", initType, err)
	} else {
		var initData struct {
			OperatorName   string `json:"operatorName"`
			OperatorAvatar string `json:"operatorAvatar"`
		}
		if json.Unmarshal(initPayload, &initData) == nil {
			if initData.OperatorName != "" {
				operatorName = initData.OperatorName
			}
			operatorAvatar = initData.OperatorAvatar
			log.Printf("chat-helper: init received, operator=%s avatar=%d bytes",
				operatorName, len(operatorAvatar))
		}
	}

	userName := getUserDisplayName()
	userInitials := strings.ToUpper(string([]rune(userName)[0:1]))
	if len([]rune(userName)) > 1 {
		parts := strings.Fields(userName)
		if len(parts) >= 2 {
			userInitials = strings.ToUpper(string([]rune(parts[0])[0:1]) + string([]rune(parts[len(parts)-1])[0:1]))
		}
	}

	// i18n strings
	fr := chatIsFrench()
	i18n := map[string]string{
		"chattingWith":    "Chatting with",
		"online":          "Online",
		"yourMessage":     "Your message...",
		"remoteRequested": "Remote control access requested.",
		"allow":           "Allow",
		"deny":            "Deny",
		"remoteGranted":   "Remote control access granted.",
		"remoteDenied":    "Remote control access denied.",
	}
	if fr {
		i18n["chattingWith"] = "Discussion avec"
		i18n["online"] = "En ligne"
		i18n["yourMessage"] = "Votre message..."
		i18n["remoteRequested"] = "Demande de prise de contrôle à distance reçue."
		i18n["allow"] = "Accepter"
		i18n["deny"] = "Refuser"
		i18n["remoteGranted"] = "Contrôle à distance autorisé."
		i18n["remoteDenied"] = "Contrôle à distance refusé."
	}

	// Build the HTML for the chat window
	windowTitle := "Obliance Chat"
	html := buildChatHTML(operatorName, operatorAvatar, userName, userInitials, i18n)

	// Create WebView2 window
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  windowTitle,
			Width:  380,
			Height: 520,
			IconId: 0,
			Center: false,
		},
	})
	if w == nil {
		log.Fatalf("chat-helper: failed to create WebView2 window")
	}
	chatWebview = w
	defer w.Destroy()

	// Make the window borderless, positioned bottom-right, topmost
	// We need a slight delay for the window to be created
	go func() {
		time.Sleep(500 * time.Millisecond)
		chatMakePopup(windowTitle)
	}()

	// Bind Go functions callable from JavaScript
	w.Bind("goSendMessage", func(text string) {
		goChatSend("user_message", text)
		// Add message to the chat UI
		jsText := strings.ReplaceAll(text, `\`, `\\`)
		jsText = strings.ReplaceAll(jsText, `'`, `\'`)
		jsText = strings.ReplaceAll(jsText, "\n", `\n`)
		w.Dispatch(func() {
			w.Eval(fmt.Sprintf(`addMessage('%s', '%s', false)`, jsText, userInitials))
		})
	})
	w.Bind("goMinimizeChat", func() {
		// Shrink window to a 56x56 chat bubble
		go func() {
			titleW, _ := syscall.UTF16PtrFromString(windowTitle)
			hwnd, _, _ := chatProcFindWindowW.Call(0, uintptr(unsafe.Pointer(titleW)))
			if hwnd == 0 { return }
			var wa chatWinRECT
			chatProcSysParamsInfo.Call(0x0030, 0, uintptr(unsafe.Pointer(&wa)), 0)
			// Circular region for bubble
			rgn, _, _ := chatProcCreateRoundRgn.Call(0, 0, 56, 56, 56, 56)
			if rgn != 0 { chatProcSetWindowRgn.Call(hwnd, rgn, 1) }
			x := int(wa.Right) - 56 - 20
			y := int(wa.Bottom) - 56 - 20
			chatProcSetWindowPos.Call(hwnd, ^uintptr(0),
				uintptr(x), uintptr(y), 56, 56, 0x0040)
		}()
	})
	w.Bind("goRestoreChat", func() {
		// Restore full chat window from bubble
		go func() {
			titleW, _ := syscall.UTF16PtrFromString(windowTitle)
			hwnd, _, _ := chatProcFindWindowW.Call(0, uintptr(unsafe.Pointer(titleW)))
			if hwnd == 0 { return }
			var wa chatWinRECT
			chatProcSysParamsInfo.Call(0x0030, 0, uintptr(unsafe.Pointer(&wa)), 0)
			// Rounded rect region for full chat
			rgn, _, _ := chatProcCreateRoundRgn.Call(0, 0, 380, 520, 18, 18)
			if rgn != 0 { chatProcSetWindowRgn.Call(hwnd, rgn, 1) }
			x := int(wa.Right) - 380 - 16
			y := int(wa.Bottom) - 520 - 16
			chatProcSetWindowPos.Call(hwnd, ^uintptr(0),
				uintptr(x), uintptr(y), 380, 520, 0x0040)
		}()
	})
	w.Bind("goCloseChat", func() {
		goChatSend("user_closed", "")
		w.Dispatch(func() { w.Terminate() })
	})
	w.Bind("goAllowRemote", func() {
		goChatSend("allow_remote", "")
	})
	w.Bind("goDenyRemote", func() {
		goChatSend("deny_remote", "")
	})

	w.SetHtml(html)

	// Read pipe messages in background
	go func() {
		for {
			msgType, payload, err := chatPipeRecv(conn)
			if err != nil {
				if err != io.EOF {
					log.Printf("chat-helper: pipe read error: %v", err)
				}
				w.Dispatch(func() { w.Terminate() })
				return
			}

			switch msgType {
			case chatPipeMsg:
				var msg struct {
					Action       string `json:"action"`
					Text         string `json:"text"`
					OperatorName string `json:"operatorName"`
					Message      string `json:"message"`
					FileName     string `json:"fileName"`
					FileData     string `json:"fileData"`
				}
				if json.Unmarshal(payload, &msg) != nil {
					continue
				}
				switch msg.Action {
				case "operator_message":
					jsText := strings.ReplaceAll(msg.Text, `\`, `\\`)
					jsText = strings.ReplaceAll(jsText, `'`, `\'`)
					jsText = strings.ReplaceAll(jsText, "\n", `\n`)
					w.Dispatch(func() {
						w.Eval(fmt.Sprintf(`addMessage('%s', '', true); playSound()`, jsText))
					})
				case "request_remote":
					jsMsg := strings.ReplaceAll(msg.Message, `'`, `\'`)
					w.Dispatch(func() {
						w.Eval(fmt.Sprintf(`showRemoteRequest('%s')`, jsMsg))
					})
				case "file_transfer":
					w.Dispatch(func() {
						w.Eval(fmt.Sprintf(`addMessage('📎 File received: %s', '', true)`, msg.FileName))
					})
				}
			case chatPipeStop:
				w.Dispatch(func() { w.Terminate() })
				return
			}
		}
	}()

	// Run the WebView2 event loop (blocks until window closes)
	w.Run()
	log.Printf("chat-helper: exiting")
}

func buildChatHTML(operatorName, operatorAvatar, userName, userInitials string, i18n map[string]string) string {
	// Build avatar HTML — use image if available, otherwise initials
	avatarHTML := `<div style="width:32px;height:32px;border-radius:50%;background:#c2001b;display:flex;align-items:center;justify-content:center;flex-shrink:0"><span style="font-size:12px;font-weight:600;color:rgba(255,255,255,0.95)">` + string([]rune(operatorName)[0:1]) + `</span></div>`
	smallAvatarHTML := avatarHTML
	if operatorAvatar != "" {
		avatarHTML = fmt.Sprintf(`<img src="%s" style="width:32px;height:32px;border-radius:50%%;object-fit:cover;flex-shrink:0" />`, operatorAvatar)
		smallAvatarHTML = fmt.Sprintf(`<img src="%s" style="width:28px;height:28px;border-radius:50%%;object-fit:cover;flex-shrink:0" />`, operatorAvatar)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="UTF-8">
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;background:#0d1117;overflow:hidden;height:100vh;display:flex;flex-direction:column;margin:0}
::-webkit-scrollbar{width:4px}
::-webkit-scrollbar-thumb{background:rgba(255,255,255,0.08);border-radius:4px}
::-webkit-scrollbar-track{background:transparent}

/* ── Full chat view ── */
.chat-container{display:flex;flex-direction:column;height:100vh;border-radius:18px;overflow:hidden;background:#0d1117}
.header{background:#161b22;padding:14px 16px;display:flex;align-items:center;justify-content:space-between;border-bottom:1px solid rgba(255,255,255,0.06);-webkit-app-region:drag}
.header button{-webkit-app-region:no-drag;background:transparent;border:none;cursor:pointer;padding:4px;display:flex;align-items:center;border-radius:6px}
.header button:hover{background:rgba(255,255,255,0.08)}
.messages{flex:1;overflow-y:auto;padding:16px;display:flex;flex-direction:column;gap:12px}
.msg-row{display:flex;gap:8px;align-items:flex-end}
.msg-row.user{justify-content:flex-end}
.bubble{border-radius:14px 14px 14px 2px;padding:10px 14px;max-width:75%%;font-size:13px;line-height:1.5}
.bubble.op{background:rgba(194,0,27,0.12);border:1px solid rgba(194,0,27,0.2);color:#e6edf3}
.bubble.user{background:#c2001b;border-radius:14px 14px 2px 14px;color:rgba(255,255,255,0.95)}
.bubble.system{background:rgba(250,204,21,0.08);border:1px solid rgba(250,204,21,0.15);color:rgba(250,204,21,0.8);font-size:11px;text-align:center;border-radius:10px;align-self:center;max-width:90%%}
.avatar{width:28px;height:28px;border-radius:50%%;flex-shrink:0;display:flex;align-items:center;justify-content:center}
.avatar.op{background:#c2001b}
.avatar.user{background:#21262d}
.avatar img{width:28px;height:28px;border-radius:50%%;object-fit:cover}
.avatar span{font-size:11px;font-weight:500;color:#8b949e}
.timestamp{text-align:center;margin:4px 0}
.timestamp span{font-size:11px;color:#484f58;background:rgba(255,255,255,0.04);padding:3px 10px;border-radius:20px}
.input-area{background:#161b22;padding:12px;border-top:1px solid rgba(255,255,255,0.06)}
.input-row{display:flex;align-items:center;gap:8px;background:#0d1117;border:1px solid rgba(255,255,255,0.1);border-radius:10px;padding:8px 12px}
.input-row input{flex:1;background:transparent;border:none;outline:none;font-size:13px;color:#e6edf3}
.input-row input::placeholder{color:#484f58}
.input-row input:focus{outline:none}
.send-btn{background:#c2001b;border:none;cursor:pointer;width:30px;height:30px;border-radius:8px;display:flex;align-items:center;justify-content:center;transition:background .15s}
.send-btn:hover{background:#a80018}
.remote-panel{background:rgba(194,0,27,0.08);border:1px solid rgba(194,0,27,0.2);border-radius:12px;padding:12px;margin:8px 0;text-align:center}
.remote-panel p{font-size:12px;color:#e6edf3;margin-bottom:8px}
.remote-panel button{padding:6px 16px;border-radius:8px;border:none;cursor:pointer;font-size:12px;font-weight:500;margin:0 4px}
.remote-panel .allow{background:#22c55e;color:white}
.remote-panel .deny{background:#ef4444;color:white}

/* ── Bubble mode ── */
.bubble-btn{display:none;width:56px;height:56px;border-radius:50%%;background:#c2001b;cursor:pointer;align-items:center;justify-content:center;position:relative;box-shadow:0 4px 16px rgba(0,0,0,0.5)}
.bubble-btn:hover{background:#a80018}
.bubble-btn svg{width:24px;height:24px}
.unread-badge{position:absolute;top:-2px;right:-2px;min-width:18px;height:18px;border-radius:9px;background:#ef4444;color:white;font-size:10px;font-weight:700;display:none;align-items:center;justify-content:center;padding:0 4px}
body.minimized .chat-container{display:none}
body.minimized .bubble-btn{display:flex}
</style>
</head><body>

<!-- Bubble button (shown when minimized) -->
<div class="bubble-btn" id="bubble-btn" onclick="restoreChat()">
  <svg viewBox="0 0 24 24" fill="none"><path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z" fill="white"/></svg>
  <span class="unread-badge" id="unread-badge">0</span>
</div>

<!-- Full chat container -->
<div class="chat-container">
  <div class="header">
    <div style="display:flex;align-items:center;gap:10px">
      %s
      <div>
        <p style="font-size:13px;font-weight:500;color:#e6edf3">` + i18n["chattingWith"] + ` ` + operatorName + `</p>
        <div style="display:flex;align-items:center;gap:5px">
          <div style="width:6px;height:6px;border-radius:50%%;background:#4ade80"></div>
          <span style="font-size:11px;color:#8b949e">` + i18n["online"] + `</span>
        </div>
      </div>
    </div>
    <div style="display:flex;gap:4px">
      <button onclick="minimizeChat()" title="Minimize">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none"><path d="M4 12h16" stroke="#8b949e" stroke-width="1.5" stroke-linecap="round"/></svg>
      </button>
      <button onclick="goCloseChat()" title="Close">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none"><path d="M18 6L6 18M6 6l12 12" stroke="#8b949e" stroke-width="1.5" stroke-linecap="round"/></svg>
      </button>
    </div>
  </div>
  <div class="messages" id="messages"></div>
  <div id="remote-panel" style="display:none;padding:0 12px">
    <div class="remote-panel">
      <p id="remote-msg">` + i18n["remoteRequested"] + `</p>
      <button class="allow" onclick="goAllowRemote();document.getElementById('remote-panel').style.display='none';addSystemMessage('` + i18n["remoteGranted"] + `')">` + i18n["allow"] + `</button>
      <button class="deny" onclick="goDenyRemote();document.getElementById('remote-panel').style.display='none';addSystemMessage('` + i18n["remoteDenied"] + `')">` + i18n["deny"] + `</button>
    </div>
  </div>
  <div class="input-area">
    <div class="input-row">
      <input id="input" type="text" placeholder="` + i18n["yourMessage"] + `" onkeydown="if(event.key==='Enter')sendMsg()" />
      <button class="send-btn" onclick="sendMsg()">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none"><path d="M22 2L11 13M22 2L15 22l-4-9-9-4 20-7z" stroke="white" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
      </button>
    </div>
  </div>
</div>
<script>
const opAvatarSmall = '%s';
const userInitials = '%s';
let isMinimized = false;
let unreadCount = 0;

function sendMsg() {
  const inp = document.getElementById('input');
  const text = inp.value.trim();
  if (!text) return;
  inp.value = '';
  goSendMessage(text);
}

function addMessage(text, initials, isOp) {
  const msgs = document.getElementById('messages');
  const row = document.createElement('div');
  row.className = 'msg-row ' + (isOp ? '' : 'user');
  if (isOp) {
    row.innerHTML = '<div class="avatar op">' + opAvatarSmall + '</div>' +
      '<div class="bubble op">' + escHtml(text) + '</div>';
  } else {
    row.innerHTML = '<div class="bubble user">' + escHtml(text) + '</div>' +
      '<div class="avatar user"><span>' + (initials || userInitials) + '</span></div>';
  }
  msgs.appendChild(row);
  msgs.scrollTop = msgs.scrollHeight;

  // Update unread badge when in bubble mode
  if (isMinimized && isOp) {
    unreadCount++;
    const badge = document.getElementById('unread-badge');
    badge.textContent = unreadCount;
    badge.style.display = 'flex';
  }
}

function addSystemMessage(text) {
  const msgs = document.getElementById('messages');
  const row = document.createElement('div');
  row.style.cssText = 'display:flex;justify-content:center';
  row.innerHTML = '<div class="bubble system">' + escHtml(text) + '</div>';
  msgs.appendChild(row);
  msgs.scrollTop = msgs.scrollHeight;
}

function showRemoteRequest(msg) {
  // Auto-restore from bubble if minimized
  if (isMinimized) restoreChat();
  const panel = document.getElementById('remote-panel');
  const rmsg = document.getElementById('remote-msg');
  rmsg.textContent = msg || 'Remote control access requested.';
  panel.style.display = 'block';
  playSound();
}

function playSound() {
  try {
    const ctx = new AudioContext();
    const osc = ctx.createOscillator();
    const g = ctx.createGain();
    osc.connect(g); g.connect(ctx.destination);
    osc.frequency.value = 800; g.gain.value = 0.15;
    osc.start(); g.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.3);
    osc.stop(ctx.currentTime + 0.3);
  } catch(e) {}
}

function minimizeChat() {
  isMinimized = true;
  document.body.classList.add('minimized');
  goMinimizeChat();
}

function restoreChat() {
  isMinimized = false;
  unreadCount = 0;
  document.body.classList.remove('minimized');
  document.getElementById('unread-badge').style.display = 'none';
  goRestoreChat();
  // Scroll to bottom and focus input
  setTimeout(function() {
    var msgs = document.getElementById('messages');
    if (msgs) msgs.scrollTop = msgs.scrollHeight;
    var inp = document.getElementById('input');
    if (inp) inp.focus();
  }, 100);
}

function escHtml(s) {
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}
</script>
</body></html>`,
		avatarHTML, smallAvatarHTML, userInitials)
}
