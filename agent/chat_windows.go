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

func chatMakePopup(title string) {
	titleW, _ := syscall.UTF16PtrFromString(title)
	hwnd, _, _ := chatProcFindWindowW.Call(0, uintptr(unsafe.Pointer(titleW)))
	if hwnd == 0 { return }

	// Remove titlebar — WS_POPUP | WS_VISIBLE
	const gwlStyle = ^uintptr(15) // -16
	const gwlExStyle = ^uintptr(19) // -20
	chatProcSetWindowLongPtrW.Call(hwnd, gwlStyle, 0x80000000|0x10000000)

	// WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_LAYERED
	ex, _, _ := chatProcGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	chatProcSetWindowLongPtrW.Call(hwnd, gwlExStyle,
		ex|0x00000008|0x00000080|0x00080000)

	// Transparency 94%
	chatProcSetLayeredAttr.Call(hwnd, 0, 240, 2) // LWA_ALPHA=2

	// Position bottom-right
	var wa chatWinRECT
	chatProcSysParamsInfo.Call(0x0030, 0, uintptr(unsafe.Pointer(&wa)), 0) // SPI_GETWORKAREA
	x := int(wa.Right) - 380 - 16
	y := int(wa.Bottom) - 520 - 16
	chatProcSetWindowPos.Call(hwnd, ^uintptr(0), // HWND_TOPMOST
		uintptr(x), uintptr(y), 380, 520, 0x0040) // SWP_SHOWWINDOW
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

	// Make the window borderless, transparent, positioned bottom-right, topmost
	go func() {
		time.Sleep(300 * time.Millisecond) // wait for window creation
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
	avatarHTML := `<div style="width:32px;height:32px;border-radius:50%;background:linear-gradient(135deg,#7F77DD,#534AB7);display:flex;align-items:center;justify-content:center;flex-shrink:0"><span style="font-size:12px;font-weight:600;color:rgba(255,255,255,0.9)">` + string([]rune(operatorName)[0:1]) + `</span></div>`
	smallAvatarHTML := avatarHTML
	if operatorAvatar != "" {
		avatarHTML = fmt.Sprintf(`<img src="%s" style="width:32px;height:32px;border-radius:50%%;object-fit:cover;flex-shrink:0" />`, operatorAvatar)
		smallAvatarHTML = fmt.Sprintf(`<img src="%s" style="width:28px;height:28px;border-radius:50%%;object-fit:cover;flex-shrink:0" />`, operatorAvatar)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="UTF-8">
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;background:transparent;overflow:hidden;height:100vh;display:flex;flex-direction:column}
::-webkit-scrollbar{width:4px}
::-webkit-scrollbar-thumb{background:rgba(127,119,221,0.3);border-radius:4px}
::-webkit-scrollbar-track{background:transparent}
.chat-container{display:flex;flex-direction:column;height:100vh;border-radius:18px;overflow:hidden;border:0.5px solid rgba(127,119,221,0.35);background:#12102a}
.header{background:#16142b;padding:14px 16px;display:flex;align-items:center;justify-content:space-between;border-bottom:0.5px solid rgba(127,119,221,0.2);-webkit-app-region:drag}
.header button{-webkit-app-region:no-drag;background:transparent;border:none;cursor:pointer;padding:4px;display:flex;align-items:center;border-radius:6px}
.header button:hover{background:rgba(255,255,255,0.1)}
.messages{flex:1;overflow-y:auto;padding:16px;display:flex;flex-direction:column;gap:12px}
.msg-row{display:flex;gap:8px;align-items:flex-end}
.msg-row.user{justify-content:flex-end}
.bubble{border-radius:14px 14px 14px 2px;padding:10px 14px;max-width:75%%;font-size:13px;line-height:1.5}
.bubble.op{background:rgba(127,119,221,0.18);border:0.5px solid rgba(127,119,221,0.3);color:rgba(255,255,255,0.85)}
.bubble.user{background:#534AB7;border-radius:14px 14px 2px 14px;color:rgba(255,255,255,0.92)}
.bubble.system{background:rgba(255,200,0,0.1);border:0.5px solid rgba(255,200,0,0.2);color:rgba(255,200,0,0.8);font-size:11px;text-align:center;border-radius:10px;align-self:center;max-width:90%%}
.avatar{width:28px;height:28px;border-radius:50%%;flex-shrink:0;display:flex;align-items:center;justify-content:center}
.avatar.op{background:linear-gradient(135deg,#7F77DD,#534AB7)}
.avatar.user{background:rgba(255,255,255,0.1)}
.avatar img{width:28px;height:28px;border-radius:50%%;object-fit:cover}
.avatar span{font-size:11px;font-weight:500;color:rgba(255,255,255,0.6)}
.timestamp{text-align:center;margin:4px 0}
.timestamp span{font-size:11px;color:rgba(255,255,255,0.25);background:rgba(255,255,255,0.05);padding:3px 10px;border-radius:20px}
.typing{display:flex;gap:4px;margin-top:6px}
.typing span{width:6px;height:6px;border-radius:50%%;background:rgba(127,119,221,0.5);animation:pulse 1s infinite}
.typing span:nth-child(2){animation-delay:.2s}
.typing span:nth-child(3){animation-delay:.4s}
@keyframes pulse{0%%,100%%{opacity:.3;transform:scale(.9)}50%%{opacity:1;transform:scale(1.1)}}
.input-area{background:#16142b;padding:12px;border-top:0.5px solid rgba(127,119,221,0.2)}
.input-row{display:flex;align-items:center;gap:8px;background:rgba(255,255,255,0.05);border:0.5px solid rgba(127,119,221,0.3);border-radius:10px;padding:8px 12px}
.input-row input{flex:1;background:transparent;border:none;outline:none;font-size:13px;color:rgba(255,255,255,0.8)}
.input-row input::placeholder{color:rgba(255,255,255,0.25)}
.send-btn{background:#534AB7;border:none;cursor:pointer;width:30px;height:30px;border-radius:8px;display:flex;align-items:center;justify-content:center}
.send-btn:hover{background:#6358d4}
.remote-panel{background:rgba(30,58,138,0.5);border:0.5px solid rgba(100,150,255,0.3);border-radius:12px;padding:12px;margin:8px 0;text-align:center}
.remote-panel p{font-size:12px;color:rgba(255,255,255,0.8);margin-bottom:8px}
.remote-panel button{padding:6px 16px;border-radius:8px;border:none;cursor:pointer;font-size:12px;font-weight:500;margin:0 4px}
.remote-panel .allow{background:#22c55e;color:white}
.remote-panel .deny{background:#ef4444;color:white}
</style>
</head><body>
<div class="chat-container">
  <div class="header">
    <div style="display:flex;align-items:center;gap:10px">
      %s
      <div>
        <p style="font-size:13px;font-weight:500;color:rgba(255,255,255,0.92)">` + i18n["chattingWith"] + ` ` + operatorName + `</p>
        <div style="display:flex;align-items:center;gap:5px">
          <div style="width:6px;height:6px;border-radius:50%%;background:#5DCAA5"></div>
          <span style="font-size:11px;color:rgba(255,255,255,0.4)">` + i18n["online"] + `</span>
        </div>
      </div>
    </div>
    <div style="display:flex;gap:8px">
      <button onclick="goCloseChat()">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none"><path d="M18 6L6 18M6 6l12 12" stroke="rgba(255,255,255,0.4)" stroke-width="1.5" stroke-linecap="round"/></svg>
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

function escHtml(s) {
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}
</script>
</body></html>`,
		avatarHTML, smallAvatarHTML, userInitials)
}
