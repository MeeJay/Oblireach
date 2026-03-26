package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ── Session persistence ────────────────────────────────────────────────────────

type SessionStore struct {
	mu      sync.Mutex
	Cookies map[string]string `json:"cookies"` // name → value
	path    string
}

func loadSession(path string) *SessionStore {
	s := &SessionStore{path: path, Cookies: map[string]string{}}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, s)
	}
	return s
}

func (s *SessionStore) save() {
	_ = os.MkdirAll(filepath.Dir(s.path), 0700)
	data, _ := json.MarshalIndent(s, "", "  ")
	_ = os.WriteFile(s.path, data, 0600)
}

func (s *SessionStore) setCookies(cookies []*http.Cookie) {
	s.mu.Lock()
	for _, c := range cookies {
		if c.MaxAge < 0 {
			delete(s.Cookies, c.Name)
		} else {
			s.Cookies[c.Name] = c.Value
		}
	}
	s.mu.Unlock()
	s.save()
}

func (s *SessionStore) inject(req *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, val := range s.Cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: val})
	}
}

func (s *SessionStore) hasCookies() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Cookies) > 0
}

func (s *SessionStore) clear() {
	s.mu.Lock()
	s.Cookies = map[string]string{}
	s.mu.Unlock()
	s.save()
}

// ── SSO state ─────────────────────────────────────────────────────────────────

type pendingSso struct {
	requestID string
	state     string
}

// ── Proxy ─────────────────────────────────────────────────────────────────────

type Proxy struct {
	cfg     *Config
	session *SessionStore
	mux     *http.ServeMux
	sso     *pendingSso // current desktop SSO flow (single-user app)
	port    int         // local server port (for SSO callback URL)
}

func newProxy(cfg *Config, cfgDir string, port int) *Proxy {
	p := &Proxy{
		cfg:     cfg,
		session: loadSession(filepath.Join(cfgDir, "session.json")),
		port:    port,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handleUI)
	mux.HandleFunc("/proxy/", p.handleProxy)
	mux.HandleFunc("/local/config", p.handleConfig)
	mux.HandleFunc("/local/logout", p.handleLogout)
	mux.HandleFunc("/local/favorites", p.handleFavorites)
	mux.HandleFunc("/sso/callback", p.handleSsoCallback)
	p.mux = mux
	return p
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS for local webview.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	p.mux.ServeHTTP(w, r)
}

func (p *Proxy) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, buildUI(p.cfg))
}

func (p *Proxy) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodPost {
		var body struct {
			ServerURL string `json:"serverUrl"`
			Username  string `json:"username"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		changed := false
		if body.ServerURL != "" && body.ServerURL != p.cfg.ServerURL {
			p.cfg.ServerURL = strings.TrimRight(body.ServerURL, "/")
			p.session.clear() // new server = new session
			changed = true
		}
		if body.Username != "" {
			p.cfg.Username = body.Username
			changed = true
		}
		if changed {
			saveConfig(p.cfg)
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"serverUrl":  p.cfg.ServerURL,
		"username":   p.cfg.Username,
		"hasSession": p.session.hasCookies(),
		"localPort":  p.port,
	})
}

func (p *Proxy) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Best-effort call to server logout endpoint.
	if p.cfg.ServerURL != "" {
		req, _ := http.NewRequest(http.MethodPost, p.cfg.ServerURL+"/api/auth/logout", nil)
		if req != nil {
			p.session.inject(req)
			client := &http.Client{Timeout: 5000000000}
			_, _ = client.Do(req)
		}
	}
	p.session.clear()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (p *Proxy) handleFavorites(w http.ResponseWriter, r *http.Request) {
	favPath := filepath.Join(filepath.Dir(p.session.path), "favorites.json")
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, `{"error":"read body"}`, http.StatusBadRequest)
			return
		}
		_ = os.MkdirAll(filepath.Dir(favPath), 0700)
		_ = os.WriteFile(favPath, body, 0600)
		fmt.Fprint(w, `{"ok":true}`)
		return
	}
	data, err := os.ReadFile(favPath)
	if err != nil {
		fmt.Fprint(w, `[]`)
		return
	}
	w.Write(data)
}

// handleSsoCallback handles the OAuth redirect from Obligate back to the local app.
// GET /sso/callback?code=xxx&state=yyy
func (p *Proxy) handleSsoCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" || p.sso == nil {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body style="background:#0d1117;color:#f85149;font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh"><div style="text-align:center"><h2>SSO Failed</h2><p>Missing code or state parameter.</p><p><a href="/" style="color:#58a6ff">Back to login</a></p></div></body></html>`)
		return
	}

	// Exchange code+state via Obliance desktop-complete endpoint.
	body, _ := json.Marshal(map[string]string{
		"requestId": p.sso.requestID,
		"code":      code,
		"state":     state,
	})
	p.sso = nil // single-use

	req, _ := http.NewRequest("POST", p.cfg.ServerURL+"/api/auth/sso-desktop-complete", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	p.session.inject(req)

	client := &http.Client{Timeout: 15000000000} // 15s
	resp, err := client.Do(req)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html><html><body style="background:#0d1117;color:#f85149;font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh"><div style="text-align:center"><h2>SSO Failed</h2><p>%s</p><p><a href="/" style="color:#58a6ff">Back to login</a></p></div></body></html>`, err.Error())
		return
	}
	defer resp.Body.Close()

	// Capture session cookies from Obliance response.
	if cookies := resp.Cookies(); len(cookies) > 0 {
		p.session.setCookies(cookies)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html><html><body style="background:#0d1117;color:#f85149;font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh"><div style="text-align:center"><h2>SSO Failed</h2><p>%s</p><p><a href="/" style="color:#58a6ff">Back to login</a></p></div></body></html>`, string(respBody))
		return
	}

	// Success — redirect to local UI.
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<!DOCTYPE html><html><head><meta http-equiv="refresh" content="0;url=/"><style>body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;background:#0d1117;color:#8b949e;font-family:sans-serif}.s{text-align:center}.d{width:28px;height:28px;border:2.5px solid #30363d;border-top-color:#58a6ff;border-radius:50%;animation:r .6s linear infinite;margin:0 auto 14px}@keyframes r{to{transform:rotate(360deg)}}</style></head><body><div class="s"><div class="d"></div><div>Signing in…</div></div></body></html>`)
}

// handleSsoInit is called from the proxy to start the desktop SSO flow.
// POST /proxy/api/auth/sso-desktop-init — intercepts to store the requestId locally.
func (p *Proxy) startSsoFlow(w http.ResponseWriter, r *http.Request) {
	callbackURL := fmt.Sprintf("http://127.0.0.1:%d/sso/callback", p.port)

	body, _ := json.Marshal(map[string]string{"localCallbackUrl": callbackURL})
	req, _ := http.NewRequest("POST", p.cfg.ServerURL+"/api/auth/sso-desktop-init", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	p.session.inject(req)

	client := &http.Client{Timeout: 10000000000}
	resp, err := client.Do(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, fmt.Sprintf(`{"success":false,"error":"%s"}`, err.Error()), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Capture session cookies.
	if cookies := resp.Cookies(); len(cookies) > 0 {
		p.session.setCookies(cookies)
	}

	respBody, _ := io.ReadAll(resp.Body)

	// Parse response to extract requestId.
	var result struct {
		Success bool `json:"success"`
		Data    struct {
			RequestID    string `json:"requestId"`
			AuthorizeURL string `json:"authorizeUrl"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err == nil && result.Success {
		p.sso = &pendingSso{
			requestID: result.Data.RequestID,
			state:     "", // state is managed server-side
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

// handleProxy reverse-proxies /proxy/<path> → <serverUrl>/<path>.
func (p *Proxy) handleProxy(w http.ResponseWriter, r *http.Request) {
	serverURL := p.cfg.ServerURL
	if serverURL == "" {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"no server configured"}`, http.StatusServiceUnavailable)
		return
	}

	target, err := url.Parse(serverURL)
	if err != nil {
		http.Error(w, `{"error":"invalid server url"}`, http.StatusInternalServerError)
		return
	}

	// Intercept desktop SSO init to store requestId locally.
	path := strings.TrimPrefix(r.URL.Path, "/proxy")
	if path == "/api/auth/sso-desktop-init" && r.Method == http.MethodPost {
		p.startSsoFlow(w, r)
		return
	}

	// Strip /proxy prefix to get the real path.
	r.URL.Path = path
	r.URL.Host = target.Host
	r.URL.Scheme = target.Scheme
	r.RequestURI = ""

	// Inject session cookies.
	p.session.inject(r)

	// WebSocket upgrade → special handling.
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		p.handleWSProxy(w, r, target)
		return
	}

	// Regular HTTP reverse proxy.
	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		},
		ModifyResponse: func(resp *http.Response) error {
			if cookies := resp.Cookies(); len(cookies) > 0 {
				p.session.setCookies(cookies)
			}
			resp.Header.Del("Set-Cookie")
			resp.Header.Set("Access-Control-Allow-Origin", "*")
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("proxy error: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, `{"error":"proxy: %s"}`, err.Error())
		},
	}
	rp.ServeHTTP(w, r)
}

// handleWSProxy tunnels a WebSocket connection to the upstream server.
func (p *Proxy) handleWSProxy(w http.ResponseWriter, r *http.Request, target *url.URL) {
	// Dial upstream (raw TCP, then HTTP upgrade).
	host := target.Host
	if !strings.Contains(host, ":") {
		if target.Scheme == "https" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	var upConn net.Conn
	var err error
	if target.Scheme == "https" {
		upConn, err = tls.Dial("tcp", host, &tls.Config{ServerName: target.Hostname()})
	} else {
		upConn, err = net.Dial("tcp", host)
	}
	if err != nil {
		http.Error(w, "upstream dial failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer upConn.Close()

	// Send upgrade request to upstream.
	upReq, _ := http.NewRequest("GET", r.URL.String(), nil)
	for k, vs := range r.Header {
		for _, v := range vs {
			upReq.Header.Add(k, v)
		}
	}
	p.session.inject(upReq)
	upReq.Host = target.Host
	upReq.Header.Del("Origin")
	_ = upReq.Write(upConn)

	// Read upstream response to complete the handshake.
	upResp, err := http.ReadResponse(bufio.NewReader(upConn), upReq)
	if err != nil || upResp.StatusCode != http.StatusSwitchingProtocols {
		code := http.StatusBadGateway
		if upResp != nil {
			code = upResp.StatusCode
		}
		http.Error(w, fmt.Sprintf("upstream WS handshake failed: %v", err), code)
		return
	}

	// Hijack the client connection.
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack not supported", http.StatusInternalServerError)
		return
	}
	clientConn, buf, err := hj.Hijack()
	if err != nil {
		http.Error(w, "hijack failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// Forward the 101 response to the client.
	_ = upResp.Write(buf)
	_ = buf.Flush()

	// Bidirectional tunnel.
	done := make(chan struct{}, 2)
	go func() { io.Copy(upConn, clientConn); done <- struct{}{} }()
	go func() { io.Copy(clientConn, upConn); done <- struct{}{} }()
	<-done
}
