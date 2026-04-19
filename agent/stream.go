package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ── Frame type constants ───────────────────────────────────────────────────────

const (
	frameTypeJPEG = byte(0x01)
	frameTypeH264 = byte(0x02)
	frameTypeVP9  = byte(0x03)
	frameTypeH265 = byte(0x04)
	frameTypeAV1   = byte(0x05)
	frameTypeAudio = byte(0x06)
)

// encodeJPEG converts BGRA pixel data to JPEG bytes (pure Go, no CGo).
func encodeJPEG(bgra []byte, width, height, quality int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	pix := img.Pix
	// BGRA → RGBA: swap B and R in bulk (4 bytes at a time)
	n := width * height * 4
	for i := 0; i < n; i += 4 {
		pix[i+0] = bgra[i+2] // R
		pix[i+1] = bgra[i+1] // G
		pix[i+2] = bgra[i+0] // B
		pix[i+3] = 255       // A
	}
	jpegBuf.Reset()
	if err := jpeg.Encode(&jpegBuf, img, jpegOpts); err != nil {
		return nil, err
	}
	return jpegBuf.Bytes(), nil
}

// Reusable JPEG encoder state (avoids allocation per frame)
var (
	jpegBuf  bytes.Buffer
	jpegOpts = &jpeg.Options{Quality: 15}
)

const jpegFallbackThreshold = 30 // switch after N frames with 0 H.264 output

// ── Adaptive bitrate ─────────────────────────────────────────────────────────

const (
	bitrateMin      = 1_000_000   // 1 Mbps floor
	bitrateMax      = 100_000_000 // 100 Mbps ceiling
	bitrateStart    = 20_000_000  // 20 Mbps initial — start high, drop if needed
	bitrateWindow   = 15          // adjust every 0.5s at 30fps
	bitrateStepUp   = 1.40        // +40% when healthy
	bitrateStepDown = 0.50        // -50% when congested
)

type adaptiveBitrate struct {
	current    int
	slowFrames int // frames where send took > budget
	totalFrames int
	budget     time.Duration // max send time per frame
}

func newAdaptiveBitrate(fps int) *adaptiveBitrate {
	return &adaptiveBitrate{
		current: bitrateStart,
		budget:  time.Second / time.Duration(fps),
	}
}

// report records one frame's send duration and returns the new bitrate
// if an adjustment is needed (0 = no change).
func (ab *adaptiveBitrate) report(sendTime time.Duration) int {
	ab.totalFrames++
	if sendTime > ab.budget {
		ab.slowFrames++
	}
	if ab.totalFrames < bitrateWindow {
		return 0
	}

	slowRatio := float64(ab.slowFrames) / float64(ab.totalFrames)
	prev := ab.current

	if slowRatio > 0.2 {
		// >20% frames are slow → reduce
		ab.current = int(float64(ab.current) * bitrateStepDown)
	} else if slowRatio < 0.05 {
		// <5% slow → room to grow
		ab.current = int(float64(ab.current) * bitrateStepUp)
	}

	// Clamp
	if ab.current < bitrateMin { ab.current = bitrateMin }
	if ab.current > bitrateMax { ab.current = bitrateMax }

	ab.slowFrames = 0
	ab.totalFrames = 0

	if ab.current != prev {
		return ab.current
	}
	return 0
}

// ── Session management ────────────────────────────────────────────────────────

var activeStreams sync.Map // token → *StreamSession

type StreamSession struct {
	token  string
	ws     *wsConn
	stopCh chan struct{}
	once   sync.Once
}

func (s *StreamSession) stop() {
	s.once.Do(func() {
		close(s.stopCh)
		s.ws.Close()
		activeStreams.Delete(s.token)
		log.Printf("Stream %s: stopped", s.token)
	})
}

// dialStreamWS opens the relay WebSocket for the given token.
func dialStreamWS(cfg *Config, token string) (*wsConn, error) {
	base := strings.TrimRight(cfg.ServerURL, "/")
	var wsBase string
	switch {
	case strings.HasPrefix(base, "https://"):
		wsBase = "wss://" + base[8:]
	case strings.HasPrefix(base, "http://"):
		wsBase = "ws://" + base[7:]
	default:
		wsBase = base
	}
	wsURL := wsBase + "/api/remote/agent-tunnel/" + token
	log.Printf("Stream %s: connecting to %s", token, wsURL)
	return wsConnect(wsURL, http.Header{"X-Api-Key": []string{cfg.APIKey}})
}

// startStream opens a remote-control session.
// sessionID < 0 means "use the console session".
// On Windows, if the target session differs from this process's session,
// a helper subprocess is spawned in that session.
func startStream(cfg *Config, token string, sessionID int) error {
	targetSession := sessionID
	if targetSession < 0 {
		targetSession = findCaptureSession()
	}

	// Cross-session on Windows: spawn capture helper.
	// Always use cross-session when running as a service (session 0),
	// even if the target is also session 0 — session 0 has no display,
	// so the helper must run in an interactive session for capture to work.
	if runtime.GOOS == "windows" {
		mySession := currentSessionID()
		if targetSession != mySession || mySession == 0 {
			// If target is session 0 (fallback), try the console session
			// which exists even at the login screen (Winlogon desktop).
			if targetSession == 0 {
				consoleID := consoleSessionID()
				if uint32(consoleID) != 0xFFFFFFFF {
					targetSession = consoleID
				}
			}
			return startCrossSessionStream(cfg, token, targetSession)
		}
	}

	// Direct capture path (same session or non-Windows).
	ws, err := dialStreamWS(cfg, token)
	if err != nil {
		return fmt.Errorf("stream %s: WS connect failed: %w", token, err)
	}

	session := &StreamSession{
		token:  token,
		ws:     ws,
		stopCh: make(chan struct{}),
	}
	activeStreams.Store(token, session)
	go session.run()
	return nil
}

func stopStream(token string) {
	if v, ok := activeStreams.Load(token); ok {
		v.(*StreamSession).stop()
	}
}

// run manages the full streaming lifecycle:
//  1. Initialize capture and encoder
//  2. Send init frame
//  3. Start capture/encode/send loop
//  4. Handle incoming input frames
func (s *StreamSession) run() {
	defer s.stop()
	defer inputUnblock() // always unblock input when stream ends

	// Lock this goroutine to its OS thread for the entire stream lifetime.
	// COM/DXGI/WMF require all calls on the same thread where CoInitializeEx ran.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// ── Initialize capture ────────────────────────────────────────────────────
	if err := captureInit(); err != nil {
		log.Printf("Stream %s: captureInit failed: %v", s.token, err)
		// Notify browser with a clear error message
		errMsg, _ := json.Marshal(map[string]string{
			"type":    "error",
			"message": "Screen capture unavailable: " + err.Error(),
		})
		_ = s.ws.WriteFrame(0x1, errMsg)
		return
	}
	defer captureClose()

	width := captureWidth()
	height := captureHeight()
	if width == 0 || height == 0 {
		log.Printf("Stream %s: invalid capture dimensions %dx%d", s.token, width, height)
		return
	}

	fps := 30
	bitrate := 20_000_000 // 20 Mbps

	// ── Initialize encoder (OpenH264 > WMF > JPEG fallback) ─────────────────
	useOpenH264 := false
	if openH264Available() {
		if err := openH264Init(width, height, fps, bitrate); err != nil {
			log.Printf("Stream %s: OpenH264 init failed: %v — trying WMF", s.token, err)
		} else {
			useOpenH264 = true
			log.Printf("Stream %s: using OpenH264 encoder", s.token)
		}
	}
	if !useOpenH264 {
		if _, err := encoderInit(width, height, fps, bitrate); err != nil {
			log.Printf("Stream %s: WMF encoderInit failed: %v — JPEG only", s.token, err)
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

	// ── Send init message ─────────────────────────────────────────────────────
	// Build list of available codecs so the viewer only shows what this agent supports
	availCodecs := []string{"h264", "jpeg"}
	if h265Available() {
		availCodecs = append(availCodecs, "h265")
	}
	availCodecs = append(availCodecs, "vp9", "av1") // always compiled in

	initMsg := map[string]interface{}{
		"type":       "init",
		"width":      width,
		"height":     height,
		"fps":        fps,
		"codec":      "h264",
		"codecs":     availCodecs,
		"monitors":   enumerateMonitors(),
		"audioRate":  audioSampleRate(),
		"audioAvail": audioInitDone,
	}

	initJSON, _ := json.Marshal(initMsg)
	if err := s.ws.WriteFrame(0x1, initJSON); err != nil {
		log.Printf("Stream %s: send init failed: %v", s.token, err)
		return
	}

	log.Printf("Stream %s: started %dx%d@%dfps", s.token, width, height, fps)
	showWatermark("Remote session active")
	defer hideWatermark()

	// ── Keepalive ─────────────────────────────────────────────────────────────
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

	// ── Audio capture ────────────────────────────────────────────────────────
	audioInit()
	defer audioClose()
	if audioInitDone {
		go func() {
			ticker := time.NewTicker(50 * time.Millisecond) // 20 fps audio chunks (bigger, cleaner)
			defer ticker.Stop()
			for {
				select {
				case <-s.stopCh:
					return
				case <-ticker.C:
					data := audioCapture()
					if len(data) > 0 {
						frame := make([]byte, 1+len(data))
						frame[0] = frameTypeAudio
						copy(frame[1:], data)
						_ = s.ws.WriteFrame(0x2, frame)
					}
				}
			}
		}()
	}

	// ── Input handler (browser → agent) ──────────────────────────────────────
	inputCh := make(chan []byte, 64)
	codecCh := make(chan string, 4)
	monitorCh := make(chan int, 4) // monitor switch requests
	blockCh := make(chan bool, 4)  // input block requests (must run on main thread)
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
			case 0x1: // text = JSON control
				var peek struct {
					Type      string `json:"type"`
					Codec     string `json:"codec"`
					Index     int    `json:"index"`
					Block     bool   `json:"block"`
					Recording bool   `json:"recording"`
				}
				if json.Unmarshal(payload, &peek) == nil {
					switch peek.Type {
					case "set_recording":
						setWatermarkRecording(peek.Recording)
						continue
					case "set_codec":
						select { case codecCh <- peek.Codec: default: }
						continue
					case "set_monitor":
						select { case monitorCh <- peek.Index: default: }
						continue
					case "set_input_block":
						select { case blockCh <- peek.Block: default: }
						continue
					case "clipboard_set":
						// Browser → agent: set clipboard content
						var clipMsg struct{ Text string `json:"text"` }
						if json.Unmarshal(payload, &clipMsg) == nil {
							clipboardSet(clipMsg.Text)
						}
						continue
					case "clipboard_get":
						// Browser requests clipboard content
						text := clipboardGet()
						clipResp, _ := json.Marshal(map[string]string{"type": "clipboard_content", "text": text})
						_ = s.ws.WriteFrame(0x1, clipResp)
						continue
					}
				}
				select { case inputCh <- payload: default: }
			}
		}
	}()

	// ── Capture/encode/send loop ──────────────────────────────────────────────
	bgraSize := width * height * 4
	bgraBuf := make([]byte, bgraSize)
	frameTicker := time.NewTicker(time.Second / time.Duration(fps))
	defer frameTicker.Stop()

	var pts int64
	var tsMs int64
	useJPEG := false
	useVP9 := false
	useH265 := false
	useAV1 := false
	ab := newAdaptiveBitrate(fps)

	// Inactivity timeout: disconnect after 10 minutes without any input
	const inactivityTimeout = 10 * time.Minute
	const inactivityWarning = 30 * time.Second
	idleTimer := time.NewTimer(inactivityTimeout)
	defer idleTimer.Stop()
	warningTimer := time.NewTimer(inactivityTimeout - inactivityWarning)
	defer warningTimer.Stop()
	warningSent := false

	resetIdleTimer := func() {
		if !idleTimer.Stop() {
			select { case <-idleTimer.C: default: }
		}
		idleTimer.Reset(inactivityTimeout)
		if !warningTimer.Stop() {
			select { case <-warningTimer.C: default: }
		}
		warningTimer.Reset(inactivityTimeout - inactivityWarning)
		warningSent = false
	}

	sendAndAdapt := func(frame []byte) error {
		start := time.Now()
		err := s.ws.WriteFrame(0x2, frame)
		if err != nil {
			return err
		}
		if newBr := ab.report(time.Since(start)); newBr > 0 {
			if useOpenH264 {
				openH264SetBitrate(newBr)
			}
			// Notify browser of bitrate change
			brMsg, _ := json.Marshal(map[string]interface{}{"type": "bitrate", "bitrate": newBr})
			_ = s.ws.WriteFrame(0x1, brMsg)
			log.Printf("Stream %s: adaptive bitrate → %d kbps", s.token, newBr/1000)
		}
		return nil
	}

	for {
		select {
		case <-s.stopCh:
			return

		case <-warningTimer.C:
			if !warningSent {
				warningSent = true
				warnMsg, _ := json.Marshal(map[string]interface{}{
					"type": "inactivity_warning", "seconds": int(inactivityWarning.Seconds()),
				})
				_ = s.ws.WriteFrame(0x1, warnMsg)
			}

		case <-idleTimer.C:
			log.Printf("Stream %s: inactivity timeout — disconnecting", s.token)
			timeoutMsg, _ := json.Marshal(map[string]string{"type": "inactivity_timeout"})
			_ = s.ws.WriteFrame(0x1, timeoutMsg)
			return

		case block := <-blockCh:
			// BlockInput must run on the same OS thread as SendInput,
			// otherwise SendInput is also blocked (Windows restriction).
			inputBlock(block)
			confirm, _ := json.Marshal(map[string]interface{}{"type": "input_block_status", "blocked": block})
			_ = s.ws.WriteFrame(0x1, confirm)

		case payload := <-inputCh:
			resetIdleTimer()
			s.handleInput(payload, width, height)

		case newIdx := <-monitorCh:
			resetIdleTimer()
			log.Printf("Stream %s: monitor switch to %d", s.token, newIdx)
			// Tear down and reinit capture on new monitor
			captureClose()
			if useOpenH264 { openH264Close(); useOpenH264 = false }
			if useVP9 { vp9EncoderClose(); useVP9 = false }
			if useH265 { h265EncoderClose(); useH265 = false }
			if useAV1 { av1EncoderClose(); useAV1 = false }
			encoderClose()
			useJPEG = false

			if err := captureInitMonitor(newIdx); err != nil {
				log.Printf("Stream %s: monitor switch failed: %v", s.token, err)
				continue
			}
			width = captureWidth()
			height = captureHeight()
			bgraBuf = make([]byte, width*height*4)
			monOffX, monOffY = captureMonitorOffset()
			setInputMonitorOffset(monOffX, monOffY)

			// Reinit encoder
			if openH264Available() {
				if err := openH264Init(width, height, fps, bitrate); err == nil {
					useOpenH264 = true
				}
			}
			if !useOpenH264 { useJPEG = true }

			// Send new init to browser
			reInit := map[string]interface{}{
				"type": "init", "width": width, "height": height,
				"fps": fps, "codec": "h264", "codecs": availCodecs,
				"monitors": enumerateMonitors(),
			}
			reInitJSON, _ := json.Marshal(reInit)
			_ = s.ws.WriteFrame(0x1, reInitJSON)

		case newCodec := <-codecCh:
			resetIdleTimer()
			log.Printf("Stream %s: codec switch requested: %s", s.token, newCodec)
			// Tear down current encoder
			if useOpenH264 { openH264Close(); useOpenH264 = false }
			if useVP9 { vp9EncoderClose(); useVP9 = false }
			if useH265 { h265EncoderClose(); useH265 = false }
			if useAV1 { av1EncoderClose(); useAV1 = false }
			if !useJPEG { encoderClose() }
			useJPEG = false

			switch newCodec {
			case "h264":
				if openH264Available() {
					if err := openH264Init(width, height, fps, bitrate); err == nil {
						useOpenH264 = true
					}
				}
				if !useOpenH264 { useJPEG = true }
			case "h265":
				if h265Available() {
					if err := h265EncoderInit(width, height, fps, bitrate/1000); err == nil {
						useH265 = true
					} else {
						log.Printf("Stream %s: H.265 init failed: %v", s.token, err)
						useJPEG = true
					}
				} else { useJPEG = true }
			case "vp9":
				if err := vp9EncoderInit(width, height, fps, bitrate/1000); err == nil {
					useVP9 = true
				} else { useJPEG = true }
			case "av1":
				if av1Available() {
					if err := av1EncoderInit(width, height, fps, bitrate/1000); err == nil {
						useAV1 = true
					} else { useJPEG = true }
				} else { useJPEG = true }
			case "jpeg":
				useJPEG = true
			}
			log.Printf("Stream %s: switched to %s", s.token, newCodec)
			switchMsg, _ := json.Marshal(map[string]string{"type": "codec_switch", "codec": newCodec})
			_ = s.ws.WriteFrame(0x1, switchMsg)

		case <-frameTicker.C:
			w, h, err := captureFrame(bgraBuf)
			if err != nil {
				continue
			}
			if w != width || h != height {
				log.Printf("Stream %s: resolution changed %dx%d→%dx%d, reconnecting",
					s.token, width, height, w, h)
				return
			}

			if useJPEG {
				jpegData, err := encodeJPEG(bgraBuf, width, height, 15)
				if err != nil {
					continue
				}
				frame := make([]byte, 1+len(jpegData))
				frame[0] = frameTypeJPEG
				copy(frame[1:], jpegData)
				if err := sendAndAdapt(frame); err != nil {
					return
				}
			} else if useAV1 {
				av1Data, err := av1EncodeFrame(bgraBuf, width, height)
				if err != nil { continue }
				if len(av1Data) == 0 { continue }
				frame := make([]byte, 1+len(av1Data))
				frame[0] = frameTypeAV1
				copy(frame[1:], av1Data)
				if err := sendAndAdapt(frame); err != nil { return }
			} else if useH265 {
				h265Data, err := h265EncodeFrame(bgraBuf, width, height)
				if err != nil {
					continue
				}
				if len(h265Data) == 0 {
					continue
				}
				frame := make([]byte, 1+len(h265Data))
				frame[0] = frameTypeH265
				copy(frame[1:], h265Data)
				if err := sendAndAdapt(frame); err != nil {
					return
				}
			} else if useVP9 {
				vp9Data, err := vp9EncodeFrame(bgraBuf, width, height)
				if err != nil {
					continue
				}
				if len(vp9Data) == 0 {
					continue
				}
				frame := make([]byte, 1+len(vp9Data))
				frame[0] = frameTypeVP9
				copy(frame[1:], vp9Data)
				if err := sendAndAdapt(frame); err != nil {
					return
				}
			} else if useOpenH264 {
				nalUnits, err := openH264EncodeFrame(bgraBuf, width, height, tsMs)
				tsMs += int64(1000 / fps)
				if err != nil {
					continue
				}
				if len(nalUnits) == 0 {
					continue
				}
				frame := make([]byte, 1+len(nalUnits))
				frame[0] = frameTypeH264
				copy(frame[1:], nalUnits)
				if err := sendAndAdapt(frame); err != nil {
					return
				}
			} else {
				nalUnits, err := encodeFrame(bgraBuf, width, height, pts)
				if err != nil {
					continue
				}
				pts += int64(time.Second/time.Duration(fps)) / 100
				if len(nalUnits) == 0 {
					if encodeInputCount >= jpegFallbackThreshold && encodeOutputCount == 0 {
						useJPEG = true
						encoderClose()
						switchMsg, _ := json.Marshal(map[string]string{"type": "codec_switch", "codec": "jpeg"})
						_ = s.ws.WriteFrame(0x1, switchMsg)
					}
					continue
				}
				frame := make([]byte, 1+len(nalUnits))
				frame[0] = frameTypeH264
				copy(frame[1:], nalUnits)
				if err := sendAndAdapt(frame); err != nil {
					return
				}
			}
		}
	}
}

// dispatchInputJSON parses and dispatches a browser input JSON frame.
// Called both by the service process (direct capture) and by the helper
// process (cross-session capture).
func dispatchInputJSON(payload []byte, screenW, screenH int) {
	var msg struct {
		Type   string  `json:"type"`
		Action string  `json:"action"`
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
		Button int     `json:"button"`
		Delta  float64 `json:"delta"`
		Code   string  `json:"code"`
		Key    string  `json:"key"`
		Ctrl   bool    `json:"ctrl"`
		Shift  bool    `json:"shift"`
		Alt    bool    `json:"alt"`
		Meta   bool    `json:"meta"`
		Width  int     `json:"width"`
		Height int     `json:"height"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		return
	}

	switch msg.Type {
	case "mouse":
		x := int(msg.X)
		y := int(msg.Y)
		switch msg.Action {
		case "move":
			inputMouseMove(x, y)
		case "down":
			inputMouseButton(msg.Button, true, x, y)
		case "up":
			inputMouseButton(msg.Button, false, x, y)
		case "scroll":
			inputMouseScroll(int(msg.Delta))
		}

	case "key":
		down := msg.Action == "down"
		// Detect Ctrl+Alt+Del / Ctrl+Alt+End on the operator's keyboard and
		// translate to SAS (SendSAS). Browsers + Windows intercept CAD locally
		// so the 3-key combo rarely reaches us as-is; but operators often have
		// it wired through via System Keys panels which send the keys. Map
		// them to SAS here so the login / lock screen can actually be reached
		// while Obliance's front-end catches up.
		if down && msg.Ctrl && msg.Alt &&
			(msg.Key == "Delete" || msg.Key == "Del" || msg.Key == "End") {
			inputSAS()
			break
		}
		// Try layout-aware mapping from e.key first (handles AZERTY, QWERTZ, etc.)
		if len([]rune(msg.Key)) == 1 {
			vk, mods := inputVKFromKey(msg.Key)
			if vk != 0 {
				// If the character needs Shift on the remote layout but the
				// browser didn't set Shift, inject Shift press/release around it.
				needsShift := (mods & 1) != 0
				if down && needsShift && !msg.Shift {
					inputKey(0x10, true) // VK_SHIFT down
				}
				inputKey(vk, down)
				if down && needsShift && !msg.Shift {
					inputKey(0x10, false) // VK_SHIFT up
				}
				break
			}
		}
		// Fallback: use physical code mapping (for F-keys, arrows, modifiers, etc.)
		vk := codeToVK(msg.Code)
		if vk != 0 {
			inputKey(vk, down)
		}

	case "sas":
		// Secure Attention Sequence (Ctrl+Alt+Del)
		inputSAS()

	case "resize_viewport":
		// No action needed — we capture at native resolution
	}
}

// handleInput is the StreamSession convenience wrapper for dispatchInputJSON.
func (s *StreamSession) handleInput(payload []byte, screenW, screenH int) {
	dispatchInputJSON(payload, screenW, screenH)
}

// codeToVK maps a browser KeyboardEvent.code string to a Windows VK_ code.
// Only covers common keys; extend as needed.
func codeToVK(code string) int {
	m := map[string]int{
		"KeyA": 0x41, "KeyB": 0x42, "KeyC": 0x43, "KeyD": 0x44, "KeyE": 0x45,
		"KeyF": 0x46, "KeyG": 0x47, "KeyH": 0x48, "KeyI": 0x49, "KeyJ": 0x4A,
		"KeyK": 0x4B, "KeyL": 0x4C, "KeyM": 0x4D, "KeyN": 0x4E, "KeyO": 0x4F,
		"KeyP": 0x50, "KeyQ": 0x51, "KeyR": 0x52, "KeyS": 0x53, "KeyT": 0x54,
		"KeyU": 0x55, "KeyV": 0x56, "KeyW": 0x57, "KeyX": 0x58, "KeyY": 0x59,
		"KeyZ": 0x5A,
		"Digit0": 0x30, "Digit1": 0x31, "Digit2": 0x32, "Digit3": 0x33, "Digit4": 0x34,
		"Digit5": 0x35, "Digit6": 0x36, "Digit7": 0x37, "Digit8": 0x38, "Digit9": 0x39,
		"Space":       0x20,
		"Enter":       0x0D,
		"Backspace":   0x08,
		"Tab":         0x09,
		"Escape":      0x1B,
		"Delete":      0x2E,
		"Insert":      0x2D,
		"Home":        0x24,
		"End":         0x23,
		"PageUp":      0x21,
		"PageDown":    0x22,
		"ArrowLeft":   0x25,
		"ArrowUp":     0x26,
		"ArrowRight":  0x27,
		"ArrowDown":   0x28,
		"ShiftLeft":   0x10,
		"ShiftRight":  0x10,
		"ControlLeft": 0x11,
		"ControlRight":0x11,
		"AltLeft":     0x12,
		"AltRight":    0x12,
		"MetaLeft":    0x5B,
		"MetaRight":   0x5C,
		"F1": 0x70, "F2": 0x71, "F3": 0x72, "F4": 0x73,
		"F5": 0x74, "F6": 0x75, "F7": 0x76, "F8": 0x77,
		"F9": 0x78, "F10": 0x79, "F11": 0x7A, "F12": 0x7B,
		"Minus":        0xBD, "Equal":        0xBB,
		"BracketLeft":  0xDB, "BracketRight": 0xDD,
		"Backslash":    0xDC, "Semicolon":    0xBA,
		"Quote":        0xDE, "Comma":        0xBC,
		"Period":       0xBE, "Slash":        0xBF,
		"Backquote":    0xC0,
		"CapsLock":     0x14,
		"NumLock":      0x90,
		"ScrollLock":   0x91,
	}
	vk, ok := m[code]
	if !ok {
		return 0
	}
	return vk
}
