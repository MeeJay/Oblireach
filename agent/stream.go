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
	jpegOpts = &jpeg.Options{Quality: 40}
)

const jpegFallbackThreshold = 30 // switch after N frames with 0 H.264 output

// ── Adaptive bitrate ─────────────────────────────────────────────────────────

const (
	bitrateMin      = 500_000   // 500 Kbps floor
	bitrateMax      = 20_000_000 // 20 Mbps ceiling
	bitrateStart    = 5_000_000  // 5 Mbps initial
	bitrateWindow   = 30         // frames per adjustment window
	bitrateStepUp   = 1.15       // +15% when healthy
	bitrateStepDown = 0.70       // -30% when congested
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
	if runtime.GOOS == "windows" && targetSession != currentSessionID() {
		return startCrossSessionStream(cfg, token, targetSession)
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

	// Lock this goroutine to its OS thread for the entire stream lifetime.
	// COM/DXGI/WMF require all calls on the same thread where CoInitializeEx ran.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// ── Initialize capture ────────────────────────────────────────────────────
	if err := captureInit(); err != nil {
		log.Printf("Stream %s: captureInit failed: %v", s.token, err)
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
	bitrate := 5_000_000 // 5 Mbps

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

	// ── Send init message ─────────────────────────────────────────────────────
	initMsg := map[string]interface{}{
		"type":   "init",
		"width":  width,
		"height": height,
		"fps":    fps,
		"codec":  "h264",
	}

	initJSON, _ := json.Marshal(initMsg)
	if err := s.ws.WriteFrame(0x1, initJSON); err != nil {
		log.Printf("Stream %s: send init failed: %v", s.token, err)
		return
	}

	log.Printf("Stream %s: started %dx%d@%dfps", s.token, width, height, fps)

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

	// ── Input handler (browser → agent) ──────────────────────────────────────
	inputCh := make(chan []byte, 64)
	codecCh := make(chan string, 4) // codec switch requests
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
				// Check for codec switch command
				var peek struct{ Type, Codec string }
				if json.Unmarshal(payload, &peek) == nil && peek.Type == "set_codec" {
					select {
					case codecCh <- peek.Codec:
					default:
					}
				} else {
					select {
					case inputCh <- payload:
					default:
					}
				}
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
	ab := newAdaptiveBitrate(fps)

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

		case payload := <-inputCh:
			s.handleInput(payload, width, height)

		case newCodec := <-codecCh:
			log.Printf("Stream %s: codec switch requested: %s", s.token, newCodec)
			// Tear down current encoder
			if useOpenH264 { openH264Close(); useOpenH264 = false }
			if useVP9 { vp9EncoderClose(); useVP9 = false }
			if !useJPEG { encoderClose() }
			useJPEG = false

			switch newCodec {
			case "h264":
				if openH264Available() {
					if err := openH264Init(width, height, fps, bitrate); err == nil {
						useOpenH264 = true
						log.Printf("Stream %s: switched to OpenH264", s.token)
					}
				}
				if !useOpenH264 {
					useJPEG = true // fallback
				}
			case "vp9":
				if err := vp9EncoderInit(width, height, fps, bitrate/1000); err == nil {
					useVP9 = true
					log.Printf("Stream %s: switched to VP9", s.token)
				} else {
					log.Printf("Stream %s: VP9 init failed: %v — fallback JPEG", s.token, err)
					useJPEG = true
				}
			case "jpeg":
				useJPEG = true
				log.Printf("Stream %s: switched to JPEG", s.token)
			}
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
				jpegData, err := encodeJPEG(bgraBuf, width, height, 40)
				if err != nil {
					continue
				}
				frame := make([]byte, 1+len(jpegData))
				frame[0] = frameTypeJPEG
				copy(frame[1:], jpegData)
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
		vk := codeToVK(msg.Code)
		if vk != 0 {
			inputKey(vk, msg.Action == "down")
		}

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
