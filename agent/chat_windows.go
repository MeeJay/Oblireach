//go:build windows

package main

/*
#cgo LDFLAGS: -lgdi32 -luser32 -lwinmm -lcomctl32

#include <windows.h>
#include <commctrl.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

// ── Constants ────────────────────────────────────────────────────────────────

#define CHAT_W           380
#define CHAT_H           480
#define CHAT_MARGIN      12
#define TITLEBAR_H       36
#define INPUT_H          40
#define SEND_BTN_W       70
#define REMOTE_PANEL_H   80
#define TAB_W            30
#define MAX_CHAT_MSGS   256
#define MAX_MSG_TEXT    1024

#define WM_CHAT_INCOMING (WM_USER + 100)
#define WM_CHAT_REMOTE   (WM_USER + 101)
#define WM_CHAT_CLOSE    (WM_USER + 102)

#define ID_INPUT         201
#define ID_SEND          202
#define ID_ALLOW         203
#define ID_DENY          204
#define ID_AUTOHIDE      205

// ── Colors ───────────────────────────────────────────────────────────────────

// Deep navy-purple theme matching the Obliance chat design
#define CLR_BG          RGB(15, 13, 46)   // #0f0d2e
#define CLR_TITLEBAR    RGB(15, 13, 46)   // same as bg
#define CLR_INPUT_BG    RGB(26, 22, 64)   // #1a1640
#define CLR_ACCENT      RGB(99, 102, 241) // #6366f1
#define CLR_TEXT         RGB(255, 255, 255)
#define CLR_TEXT_DIM     RGB(148, 163, 184)
#define CLR_OP_BUBBLE    RGB(99, 102, 241) // #6366f1 operator bubble
#define CLR_USER_BUBBLE  RGB(45, 39, 96)   // #2d2760 user bubble
#define CLR_ALLOW        RGB(34, 197, 94)  // #22c55e
#define CLR_DENY         RGB(239, 68, 68)  // #ef4444

// ── Message struct ───────────────────────────────────────────────────────────

typedef struct {
    wchar_t sender[64];
    wchar_t text[MAX_MSG_TEXT];
    int isOperator;
} ChatMsg;

// ── Global state ─────────────────────────────────────────────────────────────

static HWND g_chatWnd       = NULL;
static HWND g_chatInput     = NULL;
static HWND g_chatSendBtn   = NULL;
static HWND g_chatAllowBtn  = NULL;
static HWND g_chatDenyBtn   = NULL;

static ChatMsg g_msgs[MAX_CHAT_MSGS];
static int     g_msgCount   = 0;
static int     g_showRemote = 0;
static int     g_unread     = 0;
static int     g_minimized  = 0;
static int     g_dragging   = 0;
static POINT   g_dragStart;
static HFONT   g_fontTitle  = NULL;
static HFONT   g_fontMsg    = NULL;
static HFONT   g_fontSmall  = NULL;
static wchar_t g_opName[64] = L"Operator";
static wchar_t g_remoteMsg[512] = L"";

// Callback to Go (must match the Go export signature exactly: char*, not const char*)
extern void goChatSend(char *action, char *text);

// ── Helpers ──────────────────────────────────────────────────────────────────

static void chat_add_msg(const wchar_t *sender, const wchar_t *text, int isOp) {
    if (g_msgCount >= MAX_CHAT_MSGS) {
        memmove(&g_msgs[0], &g_msgs[1], sizeof(ChatMsg) * (MAX_CHAT_MSGS - 1));
        g_msgCount = MAX_CHAT_MSGS - 1;
    }
    ChatMsg *m = &g_msgs[g_msgCount++];
    wcsncpy(m->sender, sender, 63); m->sender[63] = 0;
    wcsncpy(m->text, text, MAX_MSG_TEXT - 1); m->text[MAX_MSG_TEXT - 1] = 0;
    m->isOperator = isOp;
}

static void chat_play_sound(void) {
    PlaySoundW(L"SystemNotification", NULL, SND_ALIAS | SND_ASYNC);
}

static void chat_invalidate(void) {
    if (g_chatWnd) InvalidateRect(g_chatWnd, NULL, FALSE);
}

static void chat_slide_minimize(void) {
    if (!g_chatWnd || g_minimized) return;
    RECT wa;
    SystemParametersInfoW(SPI_GETWORKAREA, 0, &wa, 0);
    SetWindowPos(g_chatWnd, HWND_TOPMOST,
        wa.right - TAB_W, wa.bottom - CHAT_H - CHAT_MARGIN, CHAT_W, CHAT_H,
        SWP_NOACTIVATE);
    g_minimized = 1;
}

static void chat_slide_restore(void) {
    if (!g_chatWnd || !g_minimized) return;
    RECT wa;
    SystemParametersInfoW(SPI_GETWORKAREA, 0, &wa, 0);
    SetWindowPos(g_chatWnd, HWND_TOPMOST,
        wa.right - CHAT_W - CHAT_MARGIN, wa.bottom - CHAT_H - CHAT_MARGIN, CHAT_W, CHAT_H,
        SWP_NOACTIVATE);
    g_minimized = 0;
    g_unread = 0;
    SetTimer(g_chatWnd, ID_AUTOHIDE, 30000, NULL);
}

// ── Painting ─────────────────────────────────────────────────────────────────

// Draw a filled rounded rectangle
static void fillRoundRect(HDC hdc, int x, int y, int w, int h, int r, COLORREF color) {
    HBRUSH br = CreateSolidBrush(color);
    HPEN pen = CreatePen(PS_SOLID, 1, color);
    HBRUSH oldBr = (HBRUSH)SelectObject(hdc, br);
    HPEN oldPen = (HPEN)SelectObject(hdc, pen);
    RoundRect(hdc, x, y, x + w, y + h, r, r);
    SelectObject(hdc, oldBr);
    SelectObject(hdc, oldPen);
    DeleteObject(br);
    DeleteObject(pen);
}

// Draw a circle with an initial letter
static void drawAvatarCircle(HDC hdc, int cx, int cy, int radius, COLORREF bgColor,
                              wchar_t initial, HFONT font) {
    fillRoundRect(hdc, cx - radius, cy - radius, radius * 2, radius * 2, radius * 2, bgColor);
    SetBkMode(hdc, TRANSPARENT);
    SelectObject(hdc, font);
    SetTextColor(hdc, RGB(255, 255, 255));
    RECT rc = {cx - radius, cy - radius, cx + radius, cy + radius};
    wchar_t buf[2] = {initial, 0};
    DrawTextW(hdc, buf, 1, &rc, DT_CENTER | DT_VCENTER | DT_SINGLELINE);
}

static void paint_chat(HWND hwnd) {
    PAINTSTRUCT ps;
    HDC hdc = BeginPaint(hwnd, &ps);
    RECT cr;
    GetClientRect(hwnd, &cr);

    HDC mem = CreateCompatibleDC(hdc);
    HBITMAP bmp = CreateCompatibleBitmap(hdc, cr.right, cr.bottom);
    HBITMAP old = (HBITMAP)SelectObject(mem, bmp);

    // Background gradient (solid for simplicity — deep navy purple)
    HBRUSH bgBr = CreateSolidBrush(CLR_BG);
    FillRect(mem, &cr, bgBr);
    DeleteObject(bgBr);

    // Title bar area
    RECT tbr = {0, 0, cr.right, TITLEBAR_H};
    HBRUSH tbBr = CreateSolidBrush(CLR_BG);
    FillRect(mem, &tbr, tbBr);
    DeleteObject(tbBr);

    SetBkMode(mem, TRANSPARENT);

    // Operator avatar circle (left side of title)
    drawAvatarCircle(mem, 24, TITLEBAR_H / 2, 14, CLR_ACCENT,
                     g_opName[0] ? g_opName[0] : L'O', g_fontSmall);

    // Title: "Obliance Support"
    SelectObject(mem, g_fontTitle);
    SetTextColor(mem, CLR_TEXT);
    RECT titleRc = {46, 6, cr.right - 30, 20};
    DrawTextW(mem, L"Obliance Support", -1, &titleRc, DT_LEFT | DT_SINGLELINE);

    // Subtitle: operator name + status dot
    SelectObject(mem, g_fontSmall);
    SetTextColor(mem, RGB(74, 222, 128)); // green
    RECT dotRc = {46, 22, 54, 32};
    DrawTextW(mem, L"\x2022", -1, &dotRc, DT_LEFT | DT_SINGLELINE); // bullet
    SetTextColor(mem, CLR_TEXT_DIM);
    RECT subRc = {54, 22, cr.right - 30, TITLEBAR_H};
    DrawTextW(mem, g_opName, -1, &subRc, DT_LEFT | DT_SINGLELINE | DT_END_ELLIPSIS);

    // Close button
    SelectObject(mem, g_fontTitle);
    SetTextColor(mem, CLR_TEXT_DIM);
    RECT closRc = {cr.right - 28, 8, cr.right - 4, TITLEBAR_H};
    DrawTextW(mem, L"\x2715", -1, &closRc, DT_CENTER | DT_SINGLELINE);

    // Thin separator line
    RECT sepLine = {0, TITLEBAR_H - 1, cr.right, TITLEBAR_H};
    HBRUSH sepBr = CreateSolidBrush(RGB(255, 255, 255));
    // Use 10% white
    HBRUSH sep10 = CreateSolidBrush(RGB(30, 28, 60));
    FillRect(mem, &sepLine, sep10);
    DeleteObject(sep10);
    DeleteObject(sepBr);

    // Messages area
    int msgTop = TITLEBAR_H + 4;
    int msgBot = cr.bottom - INPUT_H - (g_showRemote ? REMOTE_PANEL_H : 0);
    int y = msgBot - 8;
    int avatarR = 12; // avatar radius
    int bubblePad = 10;
    int bubbleRadius = 16;

    int i;
    for (i = g_msgCount - 1; i >= 0 && y > msgTop; i--) {
        ChatMsg *m = &g_msgs[i];

        // Measure text
        SelectObject(mem, g_fontMsg);
        int maxBubbleW = cr.right - 80;
        RECT mrc = {0, 0, maxBubbleW - bubblePad * 2, 0};
        DrawTextW(mem, m->text, -1, &mrc, DT_CALCRECT | DT_WORDBREAK);
        int textH = mrc.bottom;
        int bubbleH = textH + bubblePad * 2;
        int bubbleW = mrc.right + bubblePad * 2;
        if (bubbleW < 60) bubbleW = 60;

        y -= bubbleH;
        if (y + bubbleH < msgTop) break;

        if (m->isOperator) {
            // Operator: bubble on left + avatar
            int avX = 8 + avatarR;
            int avY = y + bubbleH - avatarR;
            drawAvatarCircle(mem, avX, avY, avatarR, CLR_ACCENT,
                             g_opName[0] ? g_opName[0] : L'O', g_fontSmall);

            int bx = 8 + avatarR * 2 + 6;
            fillRoundRect(mem, bx, y, bubbleW, bubbleH, bubbleRadius, CLR_OP_BUBBLE);

            SetTextColor(mem, CLR_TEXT);
            RECT txRc = {bx + bubblePad, y + bubblePad,
                         bx + bubbleW - bubblePad, y + bubbleH - bubblePad};
            DrawTextW(mem, m->text, -1, &txRc, DT_LEFT | DT_WORDBREAK);
        } else {
            // User: bubble on right + initials circle
            int bx = cr.right - 8 - avatarR * 2 - 6 - bubbleW;
            fillRoundRect(mem, bx, y, bubbleW, bubbleH, bubbleRadius, CLR_USER_BUBBLE);

            int avX = cr.right - 8 - avatarR;
            int avY = y + bubbleH - avatarR;
            drawAvatarCircle(mem, avX, avY, avatarR, RGB(67, 56, 202),
                             m->sender[0] ? m->sender[0] : L'U', g_fontSmall);

            SetTextColor(mem, CLR_TEXT);
            RECT txRc = {bx + bubblePad, y + bubblePad,
                         bx + bubbleW - bubblePad, y + bubbleH - bubblePad};
            DrawTextW(mem, m->text, -1, &txRc, DT_LEFT | DT_WORDBREAK);
        }

        y -= 6; // gap between messages
    }

    // Remote access panel
    if (g_showRemote) {
        int panelTop = cr.bottom - INPUT_H - REMOTE_PANEL_H;
        fillRoundRect(mem, 8, panelTop, cr.right - 16, REMOTE_PANEL_H, 12, RGB(30, 58, 138));

        SelectObject(mem, g_fontSmall);
        SetTextColor(mem, CLR_TEXT);
        RECT pmRc = {16, panelTop + 4, cr.right - 16, panelTop + REMOTE_PANEL_H - 28};
        if (wcslen(g_remoteMsg) > 0) {
            DrawTextW(mem, g_remoteMsg, -1, &pmRc, DT_LEFT | DT_WORDBREAK);
        } else {
            DrawTextW(mem, L"Remote control access requested.", -1, &pmRc, DT_LEFT | DT_WORDBREAK);
        }
    }

    // Input area background
    fillRoundRect(mem, 8, cr.bottom - INPUT_H + 4, cr.right - 16, INPUT_H - 8, 20, CLR_INPUT_BG);

    BitBlt(hdc, 0, 0, cr.right, cr.bottom, mem, 0, 0, SRCCOPY);
    SelectObject(mem, old);
    DeleteObject(bmp);
    DeleteDC(mem);
    EndPaint(hwnd, &ps);
}

// ── Window proc ──────────────────────────────────────────────────────────────

static LRESULT CALLBACK chatWndProc(HWND hwnd, UINT msg, WPARAM wp, LPARAM lp) {
    switch (msg) {
    case WM_CREATE:
        SetTimer(hwnd, ID_AUTOHIDE, 30000, NULL);
        return 0;

    case WM_PAINT:
        paint_chat(hwnd);
        return 0;

    case WM_CTLCOLOREDIT: {
        HDC hdcEdit = (HDC)wp;
        SetTextColor(hdcEdit, CLR_TEXT);
        SetBkColor(hdcEdit, CLR_INPUT_BG);
        static HBRUSH editBr = NULL;
        if (!editBr) editBr = CreateSolidBrush(CLR_INPUT_BG);
        return (LRESULT)editBr;
    }
    case WM_CTLCOLORBTN:
    case WM_CTLCOLORSTATIC: {
        HDC hdcBtn = (HDC)wp;
        SetTextColor(hdcBtn, CLR_TEXT);
        SetBkColor(hdcBtn, CLR_BG);
        static HBRUSH btnBr = NULL;
        if (!btnBr) btnBr = CreateSolidBrush(CLR_BG);
        return (LRESULT)btnBr;
    }

    case WM_COMMAND:
        if (LOWORD(wp) == ID_SEND) {
            wchar_t buf[MAX_MSG_TEXT];
            GetWindowTextW(g_chatInput, buf, MAX_MSG_TEXT);
            if (wcslen(buf) == 0) break;
            chat_add_msg(L"You", buf, 0);
            SetWindowTextW(g_chatInput, L"");
            chat_invalidate();
            KillTimer(hwnd, ID_AUTOHIDE);
            SetTimer(hwnd, ID_AUTOHIDE, 30000, NULL);
            // Send to Go
            int len = WideCharToMultiByte(CP_UTF8, 0, buf, -1, NULL, 0, NULL, NULL);
            char *utf8 = (char*)malloc(len);
            WideCharToMultiByte(CP_UTF8, 0, buf, -1, utf8, len, NULL, NULL);
            goChatSend("user_message", utf8);
            free(utf8);
        } else if (LOWORD(wp) == ID_ALLOW) {
            g_showRemote = 0;
            ShowWindow(g_chatAllowBtn, SW_HIDE);
            ShowWindow(g_chatDenyBtn, SW_HIDE);
            chat_add_msg(L"System", L"Remote control access granted.", 1);
            chat_invalidate();
            goChatSend("allow_remote", "");
        } else if (LOWORD(wp) == ID_DENY) {
            g_showRemote = 0;
            ShowWindow(g_chatAllowBtn, SW_HIDE);
            ShowWindow(g_chatDenyBtn, SW_HIDE);
            chat_add_msg(L"System", L"Remote control access denied.", 0);
            chat_invalidate();
            goChatSend("deny_remote", "");
        }
        return 0;

    case WM_CHAT_INCOMING: {
        // New operator message — wParam = pointer to UTF-8 text, lParam = pointer to sender
        char *text = (char*)wp;
        char *sender = (char*)lp;
        wchar_t wtext[MAX_MSG_TEXT], wsender[64];
        MultiByteToWideChar(CP_UTF8, 0, text, -1, wtext, MAX_MSG_TEXT);
        MultiByteToWideChar(CP_UTF8, 0, sender, -1, wsender, 64);
        chat_add_msg(wsender, wtext, 1);
        g_unread++;
        chat_play_sound();
        if (g_minimized) chat_slide_restore();
        chat_invalidate();
        KillTimer(hwnd, ID_AUTOHIDE);
        SetTimer(hwnd, ID_AUTOHIDE, 30000, NULL);
        free(text);
        free(sender);
        return 0;
    }

    case WM_CHAT_REMOTE: {
        char *rmsg = (char*)wp;
        if (rmsg && strlen(rmsg) > 0) {
            MultiByteToWideChar(CP_UTF8, 0, rmsg, -1, g_remoteMsg, 512);
        } else {
            g_remoteMsg[0] = 0;
        }
        free(rmsg);
        g_showRemote = 1;
        RECT cr;
        GetClientRect(hwnd, &cr);
        int panelTop = cr.bottom - INPUT_H - REMOTE_PANEL_H;
        SetWindowPos(g_chatAllowBtn, NULL, 12, panelTop + REMOTE_PANEL_H - 30, 100, 24, SWP_NOZORDER);
        SetWindowPos(g_chatDenyBtn, NULL, 120, panelTop + REMOTE_PANEL_H - 30, 100, 24, SWP_NOZORDER);
        ShowWindow(g_chatAllowBtn, SW_SHOW);
        ShowWindow(g_chatDenyBtn, SW_SHOW);
        chat_play_sound();
        if (g_minimized) chat_slide_restore();
        chat_invalidate();
        return 0;
    }

    case WM_TIMER:
        if (wp == ID_AUTOHIDE && g_unread == 0 && !g_minimized) {
            chat_slide_minimize();
        }
        return 0;

    case WM_LBUTTONDOWN: {
        int mx = LOWORD(lp), my = HIWORD(lp);
        RECT cr;
        GetClientRect(hwnd, &cr);
        // Close button
        if (mx >= cr.right - 30 && my < TITLEBAR_H) {
            goChatSend("user_closed", "");
            DestroyWindow(hwnd);
            return 0;
        }
        // Click on minimized tab
        if (g_minimized) {
            chat_slide_restore();
            return 0;
        }
        // Title bar drag
        if (my < TITLEBAR_H) {
            g_dragging = 1;
            g_dragStart.x = mx;
            g_dragStart.y = my;
            SetCapture(hwnd);
        }
        return 0;
    }

    case WM_MOUSEMOVE:
        if (g_dragging) {
            POINT pt;
            GetCursorPos(&pt);
            SetWindowPos(hwnd, HWND_TOPMOST,
                pt.x - g_dragStart.x, pt.y - g_dragStart.y,
                0, 0, SWP_NOSIZE | SWP_NOZORDER);
        }
        return 0;

    case WM_LBUTTONUP:
        if (g_dragging) {
            g_dragging = 0;
            ReleaseCapture();
        }
        return 0;

    case WM_DESTROY:
        PostQuitMessage(0);
        return 0;
    }
    return DefWindowProcW(hwnd, msg, wp, lp);
}

// Forward declaration
static LRESULT CALLBACK editSubProc(HWND, UINT, WPARAM, LPARAM, UINT_PTR, DWORD_PTR);

// ── Create chat window ───────────────────────────────────────────────────────

static HWND create_chat_window(const wchar_t *operatorName) {
    wcsncpy(g_opName, operatorName, 63);

    g_fontTitle = CreateFontW(-15, 0, 0, 0, FW_SEMIBOLD, 0, 0, 0,
        DEFAULT_CHARSET, 0, 0, CLEARTYPE_QUALITY, DEFAULT_PITCH, L"Segoe UI");
    g_fontMsg = CreateFontW(-13, 0, 0, 0, FW_NORMAL, 0, 0, 0,
        DEFAULT_CHARSET, 0, 0, CLEARTYPE_QUALITY, DEFAULT_PITCH, L"Segoe UI");
    g_fontSmall = CreateFontW(-11, 0, 0, 0, FW_NORMAL, 0, 0, 0,
        DEFAULT_CHARSET, 0, 0, CLEARTYPE_QUALITY, DEFAULT_PITCH, L"Segoe UI");

    WNDCLASSEXW wc;
    ZeroMemory(&wc, sizeof(wc));
    wc.cbSize = sizeof(wc);
    wc.lpfnWndProc = chatWndProc;
    wc.hInstance = GetModuleHandleW(NULL);
    wc.hCursor = LoadCursor(NULL, IDC_ARROW);
    wc.hbrBackground = CreateSolidBrush(CLR_BG);
    wc.lpszClassName = L"ObliReachChat";
    RegisterClassExW(&wc);

    RECT wa;
    SystemParametersInfoW(SPI_GETWORKAREA, 0, &wa, 0);
    int x = wa.right  - CHAT_W - CHAT_MARGIN;
    int y = wa.bottom - CHAT_H - CHAT_MARGIN;

    g_chatWnd = CreateWindowExW(
        WS_EX_TOPMOST | WS_EX_TOOLWINDOW,
        L"ObliReachChat", NULL,
        WS_POPUP | WS_VISIBLE,
        x, y, CHAT_W, CHAT_H,
        NULL, NULL, wc.hInstance, NULL);

    if (!g_chatWnd) return NULL;

    // Input edit
    g_chatInput = CreateWindowExW(0, L"EDIT", L"",
        WS_CHILD | WS_VISIBLE | ES_AUTOHSCROLL,
        8, CHAT_H - INPUT_H + 6, CHAT_W - SEND_BTN_W - 20, INPUT_H - 12,
        g_chatWnd, (HMENU)ID_INPUT, wc.hInstance, NULL);
    SendMessageW(g_chatInput, WM_SETFONT, (WPARAM)g_fontMsg, TRUE);
    // Subclass for Enter-to-send
    SetWindowSubclass(g_chatInput, editSubProc, 0, 0);

    // Send button
    g_chatSendBtn = CreateWindowExW(0, L"BUTTON", L"Send",
        WS_CHILD | WS_VISIBLE | BS_FLAT,
        CHAT_W - SEND_BTN_W - 8, CHAT_H - INPUT_H + 6, SEND_BTN_W, INPUT_H - 12,
        g_chatWnd, (HMENU)ID_SEND, wc.hInstance, NULL);
    SendMessageW(g_chatSendBtn, WM_SETFONT, (WPARAM)g_fontMsg, TRUE);

    // Allow/Deny buttons (hidden by default)
    g_chatAllowBtn = CreateWindowExW(0, L"BUTTON", L"Allow",
        WS_CHILD | BS_FLAT, 12, 0, 100, 24,
        g_chatWnd, (HMENU)ID_ALLOW, wc.hInstance, NULL);
    SendMessageW(g_chatAllowBtn, WM_SETFONT, (WPARAM)g_fontSmall, TRUE);

    g_chatDenyBtn = CreateWindowExW(0, L"BUTTON", L"Deny",
        WS_CHILD | BS_FLAT, 120, 0, 100, 24,
        g_chatWnd, (HMENU)ID_DENY, wc.hInstance, NULL);
    SendMessageW(g_chatDenyBtn, WM_SETFONT, (WPARAM)g_fontSmall, TRUE);

    g_msgCount = 0;
    g_showRemote = 0;
    g_unread = 0;
    g_minimized = 0;

    ShowWindow(g_chatWnd, SW_SHOWNOACTIVATE);
    chat_play_sound();

    return g_chatWnd;
}

// Subclass proc for Enter-to-send
static LRESULT CALLBACK editSubProc(HWND hwnd, UINT msg, WPARAM wp, LPARAM lp,
                                     UINT_PTR id, DWORD_PTR ref) {
    if (msg == WM_KEYDOWN && wp == VK_RETURN && !(GetKeyState(VK_SHIFT) & 0x8000)) {
        SendMessageW(g_chatWnd, WM_COMMAND, MAKEWPARAM(ID_SEND, BN_CLICKED), 0);
        return 0;
    }
    return DefSubclassProc(hwnd, msg, wp, lp);
}

// ── Post message from Go goroutine (thread-safe via PostMessage) ─────────

static void chat_post_operator_msg(const char *text, const char *sender) {
    // Allocate copies that the WndProc will free
    int tlen = (int)strlen(text) + 1;
    int slen = (int)strlen(sender) + 1;
    char *tcopy = (char*)malloc(tlen);
    char *scopy = (char*)malloc(slen);
    memcpy(tcopy, text, tlen);
    memcpy(scopy, sender, slen);
    PostMessageW(g_chatWnd, WM_CHAT_INCOMING, (WPARAM)tcopy, (LPARAM)scopy);
}

static void chat_post_remote_request(const char *message) {
    int mlen = (int)strlen(message) + 1;
    char *mcopy = (char*)malloc(mlen);
    memcpy(mcopy, message, mlen);
    PostMessageW(g_chatWnd, WM_CHAT_REMOTE, (WPARAM)mcopy, 0);
}

static void chat_post_close(void) {
    if (g_chatWnd) PostMessageW(g_chatWnd, WM_CLOSE, 0, 0);
}
*/
import "C"
import (
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// chatConn is the pipe connection to the service (set by runChatHelperMode)
var chatConn net.Conn
var chatConnMu sync.Mutex

//export goChatSend
func goChatSend(action *C.char, text *C.char) {
	a := C.GoString(action)
	t := C.GoString(text)

	chatConnMu.Lock()
	conn := chatConn
	chatConnMu.Unlock()
	if conn == nil {
		return
	}

	var msg []byte
	switch a {
	case "user_message":
		msg, _ = json.Marshal(map[string]interface{}{
			"action": "user_message",
			"text":   t,
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
	// Use the Windows username of the current session
	u, err := os.UserHomeDir()
	if err == nil {
		return filepath.Base(u)
	}
	h, _ := os.Hostname()
	return h
}

func runChatHelperMode(addr, chatID, operatorName string) {
	// Log to temp file
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
			// TODO: decode avatar data URI → HBITMAP for GDI painting
			// For now we just use the name
			log.Printf("chat-helper: init received, operator=%s avatar=%d bytes",
				operatorName, len(initData.OperatorAvatar))
		}
	}

	// Create the Win32 window
	opNameW, _ := syscall.UTF16FromString(operatorName)
	hwnd := C.create_chat_window((*C.wchar_t)(unsafe.Pointer(&opNameW[0])))
	if hwnd == nil {
		log.Fatalf("chat-helper: create window failed")
	}
	log.Printf("chat-helper: window created")

	// Read pipe messages in background
	go func() {
		for {
			msgType, payload, err := chatPipeRecv(conn)
			if err != nil {
				if err != io.EOF {
					log.Printf("chat-helper: pipe read error: %v", err)
				}
				C.chat_post_close()
				return
			}

			switch msgType {
			case chatPipeMsg:
				var msg struct {
					Action       string `json:"action"`
					Text         string `json:"text"`
					OperatorName string `json:"operatorName"`
					Message      string `json:"message"`
				}
				if json.Unmarshal(payload, &msg) != nil {
					continue
				}
				switch msg.Action {
				case "operator_message":
					ctext := C.CString(msg.Text)
					csender := C.CString(msg.OperatorName)
					C.chat_post_operator_msg(ctext, csender)
					// ctext and csender are freed by the WndProc
				case "request_remote":
					cmsg := C.CString(msg.Message)
					C.chat_post_remote_request(cmsg)
				}
			case chatPipeStop:
				C.chat_post_close()
				return
			}
		}
	}()

	// Win32 message loop (blocks until window closes)
	var msg C.MSG
	for C.GetMessageW(&msg, nil, 0, 0) > 0 {
		C.TranslateMessage(&msg)
		C.DispatchMessageW(&msg)
	}

	log.Printf("chat-helper: exiting")
}
