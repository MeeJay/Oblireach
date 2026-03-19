//go:build windows

package main

/*
#cgo LDFLAGS: -lwtsapi32
#include <windows.h>
#include <wtsapi32.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
	DWORD sessionId;
	int   state;
	char  username[256];
	char  stationName[64];
} sessionDetail;

// enumWTSSessions fills a malloc'd array of sessionDetail.
// Caller must free(*out) with free().
static int enumWTSSessions(sessionDetail **out, int *count) {
	WTS_SESSION_INFOW *pSI = NULL;
	DWORD n = 0;
	if (!WTSEnumerateSessionsW(WTS_CURRENT_SERVER_HANDLE, 0, 1, &pSI, &n)) {
		return -(int)GetLastError();
	}
	sessionDetail *arr = (sessionDetail*)calloc(n, sizeof(sessionDetail));
	if (!arr) {
		WTSFreeMemory(pSI);
		return -1;
	}
	for (DWORD i = 0; i < n; i++) {
		arr[i].sessionId = pSI[i].SessionId;
		arr[i].state = (int)pSI[i].State;
		if (pSI[i].pWinStationName) {
			WideCharToMultiByte(CP_UTF8, 0, pSI[i].pWinStationName, -1,
				arr[i].stationName, sizeof(arr[i].stationName)-1, NULL, NULL);
		}
		LPWSTR pUser = NULL;
		DWORD  uLen  = 0;
		if (WTSQuerySessionInformationW(WTS_CURRENT_SERVER_HANDLE, pSI[i].SessionId,
				WTSUserName, &pUser, &uLen) && pUser && uLen > 2) {
			WideCharToMultiByte(CP_UTF8, 0, pUser, -1,
				arr[i].username, sizeof(arr[i].username)-1, NULL, NULL);
			WTSFreeMemory(pUser);
		}
	}
	WTSFreeMemory(pSI);
	*out = arr;
	*count = (int)n;
	return 0;
}

static DWORD getConsoleSessionID(void) {
	return WTSGetActiveConsoleSessionId();
}

static DWORD getMySessionID(void) {
	DWORD id = 0;
	ProcessIdToSessionId(GetCurrentProcessId(), &id);
	return id;
}
*/
import "C"
import "unsafe"

// SessionInfo describes a Windows logon session.
type SessionInfo struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	State       string `json:"state"`
	StationName string `json:"stationName,omitempty"`
	IsConsole   bool   `json:"isConsole"`
}

func wtsStateName(n int) string {
	switch n {
	case 0:
		return "Active"
	case 1:
		return "Connected"
	case 2:
		return "ConnectQuery"
	case 3:
		return "Shadow"
	case 4:
		return "Disconnected"
	case 5:
		return "Idle"
	case 6:
		return "Listen"
	default:
		return "Unknown"
	}
}

// enumerateSessions returns WTS sessions relevant to show the admin
// (Active, Connected, Disconnected). Session 0 and headless service sessions
// are filtered out.
func enumerateSessions() []SessionInfo {
	var pArr *C.sessionDetail
	var count C.int
	if rc := C.enumWTSSessions(&pArr, &count); rc != 0 || count == 0 {
		return nil
	}
	defer C.free(unsafe.Pointer(pArr))

	n := int(count)
	arr := (*[1 << 16]C.sessionDetail)(unsafe.Pointer(pArr))[:n:n]
	conID := uint32(C.getConsoleSessionID())

	var out []SessionInfo
	for i := 0; i < n; i++ {
		s := arr[i]
		state := int(s.state)
		// Skip: Listen (6), Reset (7), Down (8), Init (9)
		if state >= 6 {
			continue
		}
		username := C.GoString(&s.username[0])
		stationName := C.GoString(&s.stationName[0])
		// Skip Session 0 (services) and anonymous non-active sessions
		if username == "" && state != 0 {
			continue
		}
		out = append(out, SessionInfo{
			ID:          int(s.sessionId),
			Username:    username,
			State:       wtsStateName(state),
			StationName: stationName,
			IsConsole:   uint32(s.sessionId) == conID,
		})
	}
	return out
}

// consoleSessionID returns the physical console session ID.
func consoleSessionID() int { return int(C.getConsoleSessionID()) }

// currentSessionID returns this process's own session ID.
func currentSessionID() int { return int(C.getMySessionID()) }
