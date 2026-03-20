//go:build windows

package main

/*
#cgo LDFLAGS: -lwtsapi32
#include <windows.h>
#include <wtsapi32.h>

// showNotification sends a non-blocking message box to the specified WTS session.
// The dialog auto-closes after timeoutSec seconds.
static int showNotification(DWORD sessionId, const wchar_t *title, const wchar_t *message, DWORD timeoutSec) {
	DWORD response = 0;
	BOOL ok = WTSSendMessageW(
		WTS_CURRENT_SERVER_HANDLE,
		sessionId,
		(LPWSTR)title,
		(DWORD)(wcslen(title) * sizeof(wchar_t)),
		(LPWSTR)message,
		(DWORD)(wcslen(message) * sizeof(wchar_t)),
		MB_OK | MB_ICONINFORMATION,
		timeoutSec,
		&response,
		FALSE  // bWait=FALSE: return immediately
	);
	return ok ? 0 : -(int)GetLastError();
}

// getUILanguage returns the primary language ID of the user's default UI language.
static LANGID getUILanguage(void) {
	return PRIMARYLANGID(GetUserDefaultUILanguage());
}
*/
import "C"
import (
	"log"
	"syscall"
	"unsafe"
)

// notifySession shows a temporary notification in the given WTS session.
// It detects the OS UI language and displays the message in French or English.
func notifySession(sessionID int, username string, connected bool) {
	lang := uint16(C.getUILanguage())
	// LANG_FRENCH = 0x0C
	isFrench := lang == 0x0C

	var title, msg string
	if connected {
		if isFrench {
			title = "Oblireach — Connexion distante"
			msg = username + " ouvre une connexion Oblireach à votre machine."
		} else {
			title = "Oblireach — Remote Connection"
			msg = username + " is opening an Oblireach connection to your machine."
		}
	} else {
		if isFrench {
			title = "Oblireach — Connexion fermée"
			msg = username + " a fermé la connexion Oblireach à votre machine."
		} else {
			title = "Oblireach — Connection Closed"
			msg = username + " has closed the Oblireach connection to your machine."
		}
	}

	titleW, _ := syscall.UTF16FromString(title)
	msgW, _ := syscall.UTF16FromString(msg)

	rc := C.showNotification(
		C.DWORD(sessionID),
		(*C.wchar_t)(unsafe.Pointer(&titleW[0])),
		(*C.wchar_t)(unsafe.Pointer(&msgW[0])),
		10, // auto-close after 10 seconds
	)
	if rc != 0 {
		log.Printf("notifySession: WTSSendMessage failed for session %d: %d", sessionID, rc)
	}
}
