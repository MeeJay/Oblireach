package main

import (
	"fmt"
	"strings"
)

// buildUI returns the full single-page HTML application embedded in the desktop client.
func buildUI(cfg *Config) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"/>
<meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>Oblireach</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
:root{
  --bg:#1a0a0c;--bg2:#200d10;--bg3:#3a1015;--bg4:#250a0e;
  --border:rgba(255,255,255,.08);--border2:rgba(255,255,255,.12);
  --accent:#c2001b;--accent-h:#a80018;--accent-l:#e84050;
  --input-bg:#1a1640;--input-border:rgba(255,255,255,.1);
  --text:#f1f5f9;--muted:#9ca3af;--muted2:#6b7280;
  --success:#4ade80;--danger:#ef4444;--warn:#f59e0b;
  font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;
}
body{background:var(--bg);color:var(--text);height:100vh;overflow:hidden;display:flex;flex-direction:column}
::-webkit-scrollbar{width:6px}
::-webkit-scrollbar-track{background:transparent}
::-webkit-scrollbar-thumb{background:rgba(255,255,255,.08);border-radius:3px}
::-webkit-scrollbar-thumb:hover{background:rgba(255,255,255,.15)}

/* ── Top bar ── */
.topbar{height:44px;background:linear-gradient(90deg,#200d10,#250a0e);border-bottom:1px solid var(--border);display:flex;align-items:center;padding:0 16px;gap:10px;flex-shrink:0}
.topbar .logo{font-weight:700;font-size:15px;color:var(--accent-l);letter-spacing:.5px;margin-right:auto;display:flex;align-items:center;gap:6px}
.topbar .logo svg{width:20px;height:20px}
.topbar .server-info{font-size:11px;color:var(--muted2);max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.btn-sm{padding:5px 12px;border-radius:10px;border:1px solid var(--border2);background:rgba(255,255,255,.04);color:var(--muted);font-size:11px;cursor:pointer;transition:all .15s}
.btn-sm:hover{background:rgba(194,0,27,.15);color:var(--accent-l);border-color:rgba(194,0,27,.3)}
.main{display:flex;flex:1;overflow:hidden}

/* ── Sidebar ── */
.sidebar{width:260px;flex-shrink:0;border-right:1px solid var(--border);display:flex;flex-direction:column;background:var(--bg2)}
.sidebar-head{padding:10px 12px;border-bottom:1px solid var(--border);display:flex;align-items:center;gap:6px}
.sidebar-head input{flex:1;background:var(--input-bg);border:1px solid var(--input-border);border-radius:10px;padding:6px 10px;font-size:12px;color:var(--text);outline:none;transition:border-color .15s}
.sidebar-head input:focus{border-color:var(--accent)}
.sidebar-body{flex:1;overflow-y:auto;padding:4px 0}
.group-label{padding:8px 14px 4px;font-size:10px;font-weight:600;color:var(--muted2);text-transform:uppercase;letter-spacing:.8px}
.device-row{padding:7px 14px 7px 22px;cursor:pointer;display:flex;align-items:center;gap:8px;font-size:12px;border-radius:0;transition:all .1s;margin:1px 6px;border-radius:8px}
.device-row:hover{background:rgba(194,0,27,.08)}
.device-row.active{background:rgba(194,0,27,.15);color:var(--accent-l)}
.dot{width:7px;height:7px;border-radius:50%%;flex-shrink:0}
.dot.online{background:var(--success)}
.dot.offline{background:var(--muted2)}
.dot.warn{background:var(--warn)}

/* ── Content ── */
.content{flex:1;display:flex;flex-direction:column;overflow:hidden}

/* ── Login ── */
#login-overlay{position:fixed;inset:0;background:linear-gradient(180deg,#1a0a0c 0%%,#200d10 100%%);display:flex;align-items:center;justify-content:center;z-index:100}
.login-box{background:var(--bg2);border:1px solid var(--border2);border-radius:20px;padding:36px;width:400px;display:flex;flex-direction:column;gap:18px;box-shadow:0 20px 60px rgba(0,0,0,.5)}
.login-box h2{font-size:22px;font-weight:700;text-align:center;color:var(--accent-l);display:flex;align-items:center;justify-content:center;gap:8px}
.login-box h2 svg{width:24px;height:24px}
.login-box p{font-size:12px;color:var(--muted);text-align:center;margin-top:-8px}
.form-group{display:flex;flex-direction:column;gap:5px}
.form-group label{font-size:11px;color:var(--muted);font-weight:500;padding-left:2px}
.form-group input{background:var(--input-bg);border:1px solid var(--input-border);border-radius:12px;padding:10px 14px;font-size:13px;color:var(--text);outline:none;transition:border-color .15s}
.form-group input:focus{border-color:var(--accent)}
.btn-primary{background:var(--accent);border:none;color:white;border-radius:12px;padding:11px;font-size:13px;font-weight:600;cursor:pointer;transition:all .15s}
.btn-primary:hover{background:var(--accent-h);transform:translateY(-1px);box-shadow:0 4px 12px rgba(194,0,27,.3)}
.btn-primary:disabled{opacity:.5;cursor:default;transform:none;box-shadow:none}
.err-msg{font-size:11px;color:var(--danger);text-align:center;min-height:16px}

/* ── Device detail ── */
.device-header{padding:12px 16px;border-bottom:1px solid var(--border);display:flex;align-items:center;gap:10px;flex-shrink:0;background:var(--bg2)}
.device-header h2{font-size:15px;font-weight:600;flex:1}
.tabs{display:flex;gap:0;border-bottom:1px solid var(--border);flex-shrink:0;background:var(--bg2)}
.tab-btn{padding:9px 18px;font-size:12px;font-weight:500;color:var(--muted);border:none;background:none;cursor:pointer;border-bottom:2px solid transparent;margin-bottom:-1px;transition:all .15s}
.tab-btn.active{color:var(--accent-l);border-bottom-color:var(--accent)}
.tab-btn:hover:not(.active){color:var(--text)}
.tab-content{flex:1;overflow:hidden;display:flex;flex-direction:column}

/* ── Remote ── */
#remote-pane{flex:1;display:flex;flex-direction:column;overflow:hidden}
.remote-toolbar{padding:8px 12px;border-bottom:1px solid var(--border);display:flex;align-items:center;gap:8px;flex-shrink:0;background:var(--bg2)}
.session-select{background:var(--input-bg);border:1px solid var(--input-border);border-radius:8px;padding:5px 8px;font-size:11px;color:var(--text);outline:none;cursor:pointer}
.remote-viewport{flex:1;background:#0a0405;position:relative;overflow:hidden}
#remote-canvas{width:100%%;height:100%%;object-fit:contain;display:block}
.remote-placeholder{position:absolute;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:10px;color:var(--muted2);font-size:13px}

/* ── Scripts ── */
#scripts-pane{flex:1;display:flex;overflow:hidden}
.scripts-list{width:220px;border-right:1px solid var(--border);overflow-y:auto;flex-shrink:0;background:var(--bg2)}
.script-item{padding:8px 12px;cursor:pointer;font-size:12px;border-bottom:1px solid var(--border);transition:all .1s}
.script-item:hover{background:rgba(194,0,27,.08)}
.script-item.active{background:rgba(194,0,27,.15);color:var(--accent-l)}
.script-item .sname{font-weight:500}
.script-item .sdesc{font-size:10px;color:var(--muted2);margin-top:2px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.script-detail{flex:1;padding:16px;overflow-y:auto;display:flex;flex-direction:column;gap:12px}
.script-detail h3{font-size:13px;font-weight:600}
.script-code{background:var(--bg4);border:1px solid var(--border);border-radius:10px;padding:12px;font-family:monospace;font-size:11px;white-space:pre-wrap;max-height:200px;overflow-y:auto;color:var(--text)}
.exec-output{background:#0a0405;border:1px solid var(--border);border-radius:10px;padding:12px;font-family:monospace;font-size:11px;white-space:pre-wrap;max-height:200px;overflow-y:auto;color:var(--success);min-height:60px}
.badge{display:inline-block;padding:2px 8px;border-radius:6px;font-size:10px;font-weight:500}
.badge.windows{background:rgba(96,165,250,.12);color:#60a5fa}
.badge.linux{background:rgba(74,222,128,.12);color:#4ade80}
.badge.macos{background:rgba(251,191,36,.12);color:#fbbf24}
.badge.all{background:rgba(255,255,255,.05);color:var(--muted)}

/* ── Empty / status ── */
.empty{display:flex;flex-direction:column;align-items:center;justify-content:center;flex:1;gap:10px;color:var(--muted2);font-size:13px}
.empty-icon{font-size:40px;opacity:.25}
.status-bar{padding:4px 14px;font-size:10px;color:var(--muted2);border-top:1px solid var(--border);flex-shrink:0;background:var(--bg2)}

/* ── Chat panel ── */
.chat-panel{width:0;overflow:hidden;transition:width .25s ease;border-left:1px solid var(--border);display:flex;flex-direction:column;background:linear-gradient(180deg,#1a0a0c 0%%,#200d10 100%%);flex-shrink:0}
.chat-panel.open{width:360px}
.chat-header{display:flex;align-items:center;gap:10px;padding:12px 14px;border-bottom:1px solid var(--border);flex-shrink:0}
.chat-header .avatar{width:36px;height:36px;border-radius:50%%;background:var(--accent);display:flex;align-items:center;justify-content:center;flex-shrink:0}
.chat-header .avatar svg{width:20px;height:20px;color:white}
.chat-header .info{flex:1;min-width:0}
.chat-header .info .name{font-size:13px;font-weight:600;color:white}
.chat-header .info .status{font-size:11px;color:var(--muted);display:flex;align-items:center;gap:5px}
.chat-messages{flex:1;overflow-y:auto;padding:12px 14px;display:flex;flex-direction:column;gap:10px}
.chat-msg{display:flex;gap:8px;max-width:90%%}
.chat-msg.op{align-self:flex-start}
.chat-msg.user{align-self:flex-end;flex-direction:row-reverse}
.chat-msg .bubble{padding:8px 14px;border-radius:16px;font-size:13px;line-height:1.5;word-break:break-word}
.chat-msg.op .bubble{background:var(--accent);color:white;border-bottom-left-radius:4px}
.chat-msg.user .bubble{background:var(--bg3);color:white;border-bottom-right-radius:4px}
.chat-msg.sys{align-self:center;max-width:100%%}
.chat-msg.sys .bubble{background:rgba(250,204,21,.1);color:rgba(250,204,21,.8);font-size:11px;padding:4px 12px;border-radius:20px}
.chat-time{text-align:center;padding:4px 0}
.chat-time span{font-size:10px;color:rgba(194,0,27,.5);background:rgba(194,0,27,.08);padding:2px 10px;border-radius:20px}
.chat-input-area{padding:10px 14px;flex-shrink:0;display:flex;flex-direction:column;gap:8px}
.chat-input-row{display:flex;align-items:center;gap:8px;background:var(--input-bg);border:1px solid var(--input-border);border-radius:16px;padding:4px 6px 4px 14px}
.chat-input-row input{flex:1;background:transparent;border:none;outline:none;font-size:13px;color:white}
.chat-input-row input::placeholder{color:var(--muted2)}
.chat-send-btn{width:32px;height:32px;border-radius:10px;background:var(--accent);border:none;cursor:pointer;display:flex;align-items:center;justify-content:center;transition:background .15s;flex-shrink:0}
.chat-send-btn:hover{background:var(--accent-h)}
.chat-send-btn:disabled{opacity:.3;cursor:default}
.chat-send-btn svg{width:14px;height:14px;color:white}
.chat-toggle{position:relative}
.chat-toggle .badge-dot{position:absolute;top:-2px;right:-2px;width:8px;height:8px;border-radius:50%%;background:var(--accent);display:none}
</style>
</head>
<body>

<!-- Login overlay -->
<div id="login-overlay">
  <div class="login-box">
    <h2>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/></svg>
      Oblireach
    </h2>
    <p>Connect to your Obliance server</p>
    <div class="form-group">
      <label>Server URL</label>
      <input id="inp-server" type="url" placeholder="https://obliance.example.com" value="%s"/>
    </div>
    <div id="local-login-fields">
      <div class="form-group">
        <label>Username</label>
        <input id="inp-user" type="text" placeholder="admin" value="%s"/>
      </div>
      <div class="form-group">
        <label>Password</label>
        <input id="inp-pass" type="password" placeholder=""/>
      </div>
      <div class="err-msg" id="login-err"></div>
      <button class="btn-primary" id="btn-login">Connect</button>
    </div>
    <div id="sso-login-fields" style="display:none">
      <div class="err-msg" id="sso-err"></div>
      <button class="btn-primary" id="btn-sso" style="background:#7c3aed">Sign in with SSO</button>
      <button style="background:transparent;border:1px solid var(--border2);color:var(--muted);border-radius:10px;padding:7px;font-size:11px;cursor:pointer;margin-top:4px;width:100%%" id="btn-local-fallback">Use local login instead</button>
    </div>
  </div>
</div>

<!-- Main app -->
<div id="app" style="display:none;flex-direction:column;height:100%%">
  <div class="topbar">
    <span class="logo">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/></svg>
      Oblireach
    </span>
    <span class="server-info" id="top-server"></span>
    <button class="btn-sm chat-toggle" id="btn-chat" style="display:none" title="Chat">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z"/></svg>
      Chat
      <span class="badge-dot" id="chat-badge"></span>
    </button>
    <button class="btn-sm" id="btn-refresh" title="Refresh">
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M23 4v6h-6"/><path d="M1 20v-6h6"/><path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"/></svg>
    </button>
    <button class="btn-sm" id="btn-logout">Sign out</button>
  </div>
  <div class="main">
    <div class="sidebar">
      <div class="sidebar-head">
        <input id="search-input" type="text" placeholder="Search devices..."/>
      </div>
      <div class="sidebar-body" id="device-tree"></div>
    </div>
    <div class="content" id="content-area">
      <div class="empty">
        <div class="empty-icon">
          <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" style="opacity:.3"><rect x="2" y="3" width="20" height="14" rx="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg>
        </div>
        <span>Select a device from the list</span>
      </div>
    </div>
    <!-- Chat panel -->
    <div class="chat-panel" id="chat-panel">
      <div class="chat-header">
        <div class="avatar">
          <svg fill="currentColor" viewBox="0 0 24 24"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z"/></svg>
        </div>
        <div class="info">
          <div class="name">Support Chat</div>
          <div class="status">
            <span class="dot" id="chat-status-dot" style="background:var(--muted2)"></span>
            <span id="chat-status-text">Disconnected</span>
          </div>
        </div>
        <button class="btn-sm" id="btn-chat-close" style="padding:4px 8px" title="Close chat">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
        </button>
      </div>
      <div class="chat-messages" id="chat-messages"></div>
      <div class="chat-input-area">
        <div id="chat-request-btn-wrap"></div>
        <div class="chat-input-row">
          <input id="chat-input" type="text" placeholder="Your message..." disabled/>
          <button class="chat-send-btn" id="chat-send" disabled>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg>
          </button>
        </div>
      </div>
    </div>
  </div>
  <div class="status-bar" id="status-bar">Ready</div>
</div>

<script>
// ── State ────────────────────────────────────────────────────────────────────
let overview = { groups: [] };
let reachScripts = [];
let selectedDevice = null;
let activeTab = 'remote';
let remoteWs = null;
let remoteDecoder = null;
let remoteTs = 0;
let execAbort = null;

// Chat state
let chatSocket = null;
let chatId = null;
let chatMessages = [];
let chatConnected = false;
let chatUserClosed = false;
let currentOperatorName = '';

// ── Helpers ──────────────────────────────────────────────────────────────────
async function api(method, path, body) {
  const opts = { method, headers: { 'Content-Type': 'application/json' } };
  if (body !== undefined) opts.body = JSON.stringify(body);
  return await fetch('/proxy' + path, opts);
}
function setStatus(msg) { document.getElementById('status-bar').textContent = msg; }
function esc(s) {
  if (!s) return '';
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}
function fmtTime(ts) {
  return new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

// ── H.264 keyframe detection ─────────────────────────────────────────────────
function isH264Keyframe(data) {
  let i = 0;
  while (i < data.length - 4) {
    if (data[i] === 0 && data[i+1] === 0) {
      let ns = -1;
      if (data[i+2] === 1) { ns = i + 3; i += 4; }
      else if (data[i+2] === 0 && data[i+3] === 1) { ns = i + 4; i += 5; }
      else { i++; continue; }
      if (ns < data.length) { const t = data[ns] & 0x1f; if (t === 5 || t === 7 || t === 8) return true; }
    } else { i++; }
  }
  return false;
}

// ── Login ────────────────────────────────────────────────────────────────────
document.getElementById('btn-login').addEventListener('click', doLogin);
document.getElementById('inp-pass').addEventListener('keydown', e => { if (e.key === 'Enter') doLogin(); });
document.getElementById('btn-sso').addEventListener('click', doSsoLogin);
document.getElementById('btn-local-fallback').addEventListener('click', () => {
  document.getElementById('sso-login-fields').style.display = 'none';
  document.getElementById('local-login-fields').style.display = '';
});

let ssoCheckTimer = null;
document.getElementById('inp-server').addEventListener('input', () => {
  clearTimeout(ssoCheckTimer);
  ssoCheckTimer = setTimeout(checkSso, 500);
});

async function checkSso() {
  const server = document.getElementById('inp-server').value.trim().replace(/\/$/, '');
  if (!server) return;
  await fetch('/local/config', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ serverUrl: server }) });
  try {
    const r = await fetch('/proxy/api/auth/sso-config');
    if (!r.ok) return;
    const d = await r.json();
    const sso = d.data || d;
    if (sso.obligateEnabled && sso.obligateReachable) {
      document.getElementById('local-login-fields').style.display = 'none';
      document.getElementById('sso-login-fields').style.display = '';
    } else {
      document.getElementById('local-login-fields').style.display = '';
      document.getElementById('sso-login-fields').style.display = 'none';
    }
  } catch {}
}

async function doSsoLogin() {
  const server = document.getElementById('inp-server').value.trim().replace(/\/$/, '');
  const errEl = document.getElementById('sso-err');
  const btn = document.getElementById('btn-sso');
  if (!server) { errEl.textContent = 'Server URL required'; return; }
  btn.disabled = true; btn.textContent = 'Connecting...'; errEl.textContent = '';
  try {
    await fetch('/local/config', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ serverUrl: server }) });
    const r = await fetch('/proxy/api/auth/sso-desktop-init', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' });
    const d = await r.json();
    if (!d.success || !d.data?.authorizeUrl) { errEl.textContent = d.error || 'SSO init failed'; return; }
    window.location.href = d.data.authorizeUrl;
  } catch (err) { errEl.textContent = 'Connection failed: ' + err.message; }
  finally { btn.disabled = false; btn.textContent = 'Sign in with SSO'; }
}

async function doLogin() {
  const server = document.getElementById('inp-server').value.trim().replace(/\/$/, '');
  const username = document.getElementById('inp-user').value.trim();
  const password = document.getElementById('inp-pass').value;
  const errEl = document.getElementById('login-err');
  const btn = document.getElementById('btn-login');
  if (!server || !username || !password) { errEl.textContent = 'All fields required'; return; }
  btn.disabled = true; btn.textContent = 'Connecting...'; errEl.textContent = '';
  try {
    await fetch('/local/config', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ serverUrl: server, username }) });
    const r = await api('POST', '/api/auth/login', { username, password });
    const data = await r.json();
    if (!r.ok || !data.success) { errEl.textContent = data.error || 'Login failed'; return; }
    document.getElementById('top-server').textContent = server;
    await enterApp();
  } catch (err) { errEl.textContent = 'Connection failed: ' + err.message; }
  finally { btn.disabled = false; btn.textContent = 'Connect'; }
}

async function enterApp() {
  document.getElementById('login-overlay').style.display = 'none';
  document.getElementById('app').style.display = 'flex';

  // Get current user info for chat
  try {
    const r = await api('GET', '/api/auth/me');
    if (r.ok) {
      const d = await r.json();
      const u = d.data || d.user || d;
      currentOperatorName = u.displayName || u.display_name || u.username || 'Operator';
    }
  } catch {}

  // Select first tenant
  try {
    const r = await api('GET', '/api/tenants');
    const data = await r.json();
    const tenants = data.tenants || data.data?.tenants || data;
    if (Array.isArray(tenants) && tenants.length > 0) {
      await api('POST', '/api/tenant/' + tenants[0].id + '/select');
    }
  } catch {}

  await loadOverview();
  await loadScripts();
  initSocketIO();
}

// ── Data loading ─────────────────────────────────────────────────────────────
async function loadOverview() {
  setStatus('Loading devices...');
  try {
    const r = await api('GET', '/api/reach/overview');
    const d = await r.json();
    overview = d.data || { groups: [] };
    renderTree();
    setStatus('Ready \u2014 ' + countDevices() + ' devices');
  } catch (err) { setStatus('Failed to load: ' + err.message); }
}
async function loadScripts() {
  try { const r = await api('GET', '/api/reach/scripts'); const d = await r.json(); reachScripts = d.data?.scripts || []; } catch {}
}
function countDevices() { return overview.groups.reduce((n, g) => n + g.devices.length, 0); }

// ── Device tree ──────────────────────────────────────────────────────────────
function renderTree() {
  const filter = document.getElementById('search-input').value.toLowerCase();
  const tree = document.getElementById('device-tree');
  tree.innerHTML = '';
  for (const group of overview.groups) {
    const devs = group.devices.filter(d => !filter || d.hostname.toLowerCase().includes(filter));
    if (devs.length === 0) continue;
    const gl = document.createElement('div');
    gl.className = 'group-label';
    gl.textContent = group.name;
    tree.appendChild(gl);
    for (const dev of devs) {
      const row = document.createElement('div');
      row.className = 'device-row' + (selectedDevice?.id === dev.id ? ' active' : '');
      const dc = dev.oblireach.online ? 'online' : 'warn';
      row.innerHTML = '<span class="dot ' + dc + '"></span><span style="flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">' + esc(dev.hostname) + '</span>';
      row.addEventListener('click', () => selectDevice(dev));
      tree.appendChild(row);
    }
  }
}
document.getElementById('search-input').addEventListener('input', renderTree);
document.getElementById('btn-refresh').addEventListener('click', async () => {
  await loadOverview();
  if (selectedDevice) {
    for (const g of overview.groups) { const d = g.devices.find(x => x.id === selectedDevice.id); if (d) { selectedDevice = d; break; } }
  }
});

// ── Device detail ────────────────────────────────────────────────────────────
function selectDevice(dev) {
  // Close previous chat if switching device
  if (selectedDevice && selectedDevice.id !== dev.id) closeChat();
  selectedDevice = dev;
  renderTree();

  // Show/hide chat button
  const chatBtn = document.getElementById('btn-chat');
  chatBtn.style.display = dev.oblireach?.online ? '' : 'none';

  const area = document.getElementById('content-area');
  area.innerHTML = '';
  area.style.display = 'flex';
  area.style.flexDirection = 'column';
  area.style.overflow = 'hidden';

  // Header
  const hdr = document.createElement('div');
  hdr.className = 'device-header';
  const dc = dev.oblireach.online ? 'online' : dev.oblireach.installed ? 'warn' : 'offline';
  hdr.innerHTML = '<span class="dot ' + dc + '" style="width:9px;height:9px"></span>' +
    '<h2>' + esc(dev.hostname) + '</h2>' +
    (dev.oblireach.updateAvailable ? '<span style="font-size:10px;color:var(--warn);font-weight:600;background:rgba(245,158,11,.12);padding:2px 8px;border-radius:6px">UPDATE</span>' : '') +
    '<span style="font-size:11px;color:var(--muted)">' + esc(dev.osType) + ' \u00B7 ' + esc(dev.status) + '</span>';
  area.appendChild(hdr);

  // Tabs
  const tabs = document.createElement('div');
  tabs.className = 'tabs';
  ['remote', 'scripts', 'info'].forEach(t => {
    const btn = document.createElement('button');
    btn.className = 'tab-btn' + (t === activeTab ? ' active' : '');
    btn.dataset.tab = t;
    btn.textContent = t.charAt(0).toUpperCase() + t.slice(1);
    btn.addEventListener('click', () => switchTab(t, area));
    tabs.appendChild(btn);
  });
  area.appendChild(tabs);

  const tc = document.createElement('div');
  tc.className = 'tab-content';
  tc.id = 'tab-content';
  area.appendChild(tc);
  switchTab(activeTab, area);
}

function switchTab(tab, area) {
  activeTab = tab;
  area = area || document.getElementById('content-area');
  area.querySelectorAll('.tab-btn').forEach(b => b.classList.toggle('active', b.dataset.tab === tab));
  if (tab !== 'remote') stopRemote();
  const tc = document.getElementById('tab-content');
  tc.innerHTML = '';
  if (tab === 'remote') renderRemoteTab(tc);
  else if (tab === 'scripts') renderScriptsTab(tc);
  else renderInfoTab(tc);
}

// ── Remote tab ───────────────────────────────────────────────────────────────
function renderRemoteTab(tc) {
  if (!selectedDevice?.oblireach?.online) {
    tc.innerHTML = '<div class="empty"><div class="empty-icon"><svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" style="opacity:.3"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 014 10 15.3 15.3 0 01-4 10 15.3 15.3 0 01-4-10 15.3 15.3 0 014-10z"/></svg></div>' +
      '<span>' + (selectedDevice?.oblireach?.installed ? 'Oblireach agent is offline' : 'Oblireach agent not installed') + '</span></div>';
    return;
  }
  const pane = document.createElement('div');
  pane.id = 'remote-pane';

  const toolbar = document.createElement('div');
  toolbar.className = 'remote-toolbar';

  const sessions = selectedDevice.oblireach.sessions || [];
  const sessSelect = document.createElement('select');
  sessSelect.className = 'session-select';
  const oa = document.createElement('option'); oa.value = ''; oa.textContent = 'Auto (active session)'; sessSelect.appendChild(oa);
  for (const s of sessions) {
    const opt = document.createElement('option'); opt.value = s.id;
    opt.textContent = s.username + ' (' + s.state + (s.stationName ? ' \u00B7 ' + s.stationName : '') + ')';
    sessSelect.appendChild(opt);
  }
  toolbar.appendChild(sessSelect);

  const startBtn = document.createElement('button');
  startBtn.className = 'btn-sm';
  startBtn.style.cssText = 'background:var(--accent);border-color:var(--accent);color:white';
  startBtn.textContent = '\u25B6 Connect';
  startBtn.addEventListener('click', () => startRemote(sessSelect.value ? parseInt(sessSelect.value) : undefined));
  toolbar.appendChild(startBtn);

  const stopBtn = document.createElement('button');
  stopBtn.id = 'stop-btn'; stopBtn.className = 'btn-sm';
  stopBtn.style.cssText = 'background:rgba(239,68,68,.12);border-color:rgba(239,68,68,.3);color:var(--danger);display:none';
  stopBtn.textContent = '\u25A0 Disconnect';
  stopBtn.addEventListener('click', stopRemote);
  toolbar.appendChild(stopBtn);

  const statusSpan = document.createElement('span');
  statusSpan.id = 'remote-status';
  statusSpan.style.cssText = 'font-size:11px;color:var(--muted);margin-left:auto';
  toolbar.appendChild(statusSpan);

  pane.appendChild(toolbar);

  const vp = document.createElement('div');
  vp.className = 'remote-viewport';
  const canvas = document.createElement('canvas');
  canvas.id = 'remote-canvas'; canvas.style.display = 'none';
  vp.appendChild(canvas);
  const ph = document.createElement('div');
  ph.className = 'remote-placeholder'; ph.id = 'remote-placeholder';
  ph.innerHTML = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" style="opacity:.2"><rect x="2" y="3" width="20" height="14" rx="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg><span>Click Connect to start remote session</span>';
  vp.appendChild(ph);
  pane.appendChild(vp);
  tc.appendChild(pane);

  canvas.addEventListener('mousemove', e => sendInput('mousemove', e, canvas));
  canvas.addEventListener('mousedown', e => sendInput('mousedown', e, canvas));
  canvas.addEventListener('mouseup', e => sendInput('mouseup', e, canvas));
  canvas.addEventListener('wheel', e => { e.preventDefault(); sendInput('wheel', e, canvas); }, { passive: false });
  canvas.addEventListener('keydown', e => { e.preventDefault(); sendInputKey('keydown', e); });
  canvas.addEventListener('keyup', e => { e.preventDefault(); sendInputKey('keyup', e); });
  canvas.setAttribute('tabindex', '0');
}

async function startRemote(wtsSessionId) {
  if (!selectedDevice) return;
  const statusEl = document.getElementById('remote-status');
  if (statusEl) statusEl.textContent = 'Starting session...';
  try {
    const body = { deviceId: selectedDevice.id, protocol: 'oblireach' };
    if (wtsSessionId !== undefined) body.sessionId = wtsSessionId;
    const r = await api('POST', '/api/remote/sessions', body);
    const d = await r.json();
    const session = d.data;
    if (!session?.sessionToken) throw new Error('No session token');
    if (statusEl) statusEl.textContent = 'Connecting...';
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(proto + '//' + location.host + '/proxy/api/remote/tunnel/' + session.sessionToken);
    ws.binaryType = 'arraybuffer';
    remoteWs = ws;
    const stopBtn = document.getElementById('stop-btn');
    if (stopBtn) stopBtn.style.display = '';
    ws.onopen = () => { if (statusEl) statusEl.textContent = 'Connected \u2014 waiting for stream...'; };
    ws.onclose = () => {
      if (statusEl) statusEl.textContent = 'Disconnected';
      if (stopBtn) stopBtn.style.display = 'none';
      const c = document.getElementById('remote-canvas'), p = document.getElementById('remote-placeholder');
      if (c) c.style.display = 'none'; if (p) p.style.display = 'flex';
      remoteWs = null; remoteDecoder = null;
    };
    ws.onerror = () => { if (statusEl) statusEl.textContent = 'WebSocket error'; };
    ws.onmessage = handleRemoteMessage;
  } catch (err) { if (statusEl) statusEl.textContent = 'Error: ' + err.message; }
}

function stopRemote() {
  if (remoteWs) { remoteWs.close(); remoteWs = null; }
  if (remoteDecoder) { try { remoteDecoder.close(); } catch {} remoteDecoder = null; }
}

async function handleRemoteMessage(event) {
  if (typeof event.data === 'string') {
    try {
      const info = JSON.parse(event.data);
      if (info.type === 'paired') return;
      if (info.type === 'codec_switch') {
        const el = document.getElementById('remote-status');
        if (el) el.textContent = el.textContent.replace(/\s*\u00B7\s*(H\.264|JPEG)/, '') + ' \u00B7 ' + (info.codec === 'jpeg' ? 'JPEG' : 'H.264');
        return;
      }
      if (!info.width || !info.height) return;
      await initDecoder(info);
      const el = document.getElementById('remote-status');
      if (el) el.textContent = info.width + '\u00D7' + info.height + ' @ ' + info.fps + 'fps \u00B7 H.264';
      const c = document.getElementById('remote-canvas'), p = document.getElementById('remote-placeholder');
      if (c) { c.width = info.width; c.height = info.height; c.style.display = 'block'; }
      if (p) p.style.display = 'none';
    } catch {} return;
  }
  const buf = new Uint8Array(event.data);
  if (buf.length < 1) return;
  const type = buf[0], payload = buf.slice(1);
  if (type === 0x01) {
    const blob = new Blob([payload], { type: 'image/jpeg' });
    createImageBitmap(blob).then(bmp => {
      const canvas = document.getElementById('remote-canvas');
      if (!canvas) return;
      if (!canvas._agentW || canvas._agentW !== bmp.width || canvas._agentH !== bmp.height) { canvas._agentW = bmp.width; canvas._agentH = bmp.height; }
      const rect = canvas.getBoundingClientRect();
      const cw = Math.round(rect.width * (window.devicePixelRatio || 1));
      const ch = Math.round(rect.height * (window.devicePixelRatio || 1));
      if (canvas.width !== cw || canvas.height !== ch) { canvas.width = cw; canvas.height = ch; }
      canvas.style.display = 'block';
      const ph = document.getElementById('remote-placeholder'); if (ph) ph.style.display = 'none';
      const ctx = canvas.getContext('2d');
      if (ctx) {
        const scale = Math.min(cw / bmp.width, ch / bmp.height);
        const dw = Math.round(bmp.width * scale), dh = Math.round(bmp.height * scale);
        const dx = Math.round((cw - dw) / 2), dy = Math.round((ch - dh) / 2);
        ctx.clearRect(0, 0, cw, ch); ctx.drawImage(bmp, dx, dy, dw, dh);
      }
      bmp.close();
    }).catch(() => {});
  } else if (type === 0x02 && remoteDecoder) {
    const isKey = isH264Keyframe(payload);
    const chunk = new EncodedVideoChunk({ type: isKey ? 'key' : 'delta', timestamp: remoteTs, data: payload });
    remoteTs += Math.round(1000000 / 15);
    try { remoteDecoder.decode(chunk); } catch {}
  }
}

async function initDecoder(info) {
  if (remoteDecoder) { try { remoteDecoder.close(); } catch {} }
  const canvas = document.getElementById('remote-canvas'); if (!canvas) return;
  const ctx = canvas.getContext('2d');
  remoteDecoder = new VideoDecoder({
    output(frame) { if (canvas) { canvas.width = frame.displayWidth; canvas.height = frame.displayHeight; ctx.drawImage(frame, 0, 0); } frame.close(); },
    error(e) { console.warn('decoder error', e); }
  });
  remoteTs = 0;
  const config = { codec: 'avc1.640034', codedWidth: info.width, codedHeight: info.height, optimizeForLatency: true };
  if (info.extradata) { const bin = atob(info.extradata); const arr = new Uint8Array(bin.length); for (let i = 0; i < bin.length; i++) arr[i] = bin.charCodeAt(i); config.description = arr; }
  remoteDecoder.configure(config);
}

function sendInput(type, e, canvas) {
  if (!remoteWs || remoteWs.readyState !== WebSocket.OPEN) return;
  const rect = canvas.getBoundingClientRect();
  const aw = canvas._agentW || canvas.width, ah = canvas._agentH || canvas.height;
  const dpr = window.devicePixelRatio || 1;
  const cw = rect.width * dpr, ch = rect.height * dpr;
  const scale = Math.min(cw / aw, ch / ah);
  const dw = aw * scale, dh = ah * scale;
  const dx = (cw - dw) / 2, dy = (ch - dh) / 2;
  const px = (e.clientX - rect.left) * dpr - dx, py = (e.clientY - rect.top) * dpr - dy;
  const msg = { type, x: Math.round(px / scale), y: Math.round(py / scale) };
  if (type === 'wheel') { msg.deltaX = e.deltaX; msg.deltaY = e.deltaY; }
  if (type !== 'mousemove') msg.button = e.button;
  remoteWs.send(JSON.stringify(msg));
}

function sendInputKey(type, e) {
  if (!remoteWs || remoteWs.readyState !== WebSocket.OPEN) return;
  remoteWs.send(JSON.stringify({ type, key: e.key, code: e.code, shift: e.shiftKey, ctrl: e.ctrlKey, alt: e.altKey }));
}

// ── Scripts tab ──────────────────────────────────────────────────────────────
function renderScriptsTab(tc) {
  if (reachScripts.length === 0) {
    tc.innerHTML = '<div class="empty"><div class="empty-icon"><svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" style="opacity:.3"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14,2 14,8 20,8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg></div><span>No scripts available</span></div>';
    return;
  }
  const pane = document.createElement('div'); pane.id = 'scripts-pane';
  const list = document.createElement('div'); list.className = 'scripts-list';
  const detail = document.createElement('div'); detail.className = 'script-detail'; detail.id = 'script-detail';
  detail.innerHTML = '<div style="color:var(--muted2);font-size:12px;margin:auto">Select a script</div>';
  for (const s of reachScripts) {
    const item = document.createElement('div'); item.className = 'script-item';
    item.innerHTML = '<div class="sname">' + esc(s.name) + '</div><div class="sdesc">' + esc(s.description || s.runtime) + '</div>';
    item.addEventListener('click', () => { list.querySelectorAll('.script-item').forEach(x => x.classList.remove('active')); item.classList.add('active'); showScriptDetail(s, detail); });
    list.appendChild(item);
  }
  pane.appendChild(list); pane.appendChild(detail); tc.appendChild(pane);
}

function showScriptDetail(script, detail) {
  const pc = { windows: 'windows', linux: 'linux', macos: 'macos', all: 'all' };
  detail.innerHTML =
    '<h3>' + esc(script.name) + '</h3>' +
    '<div style="display:flex;gap:6px;align-items:center"><span class="badge ' + (pc[script.platform] || 'all') + '">' + esc(script.platform) + '</span>' +
    '<span style="font-size:11px;color:var(--muted)">' + esc(script.runtime) + '</span></div>' +
    (script.description ? '<p style="font-size:12px;color:var(--muted)">' + esc(script.description) + '</p>' : '') +
    '<div class="script-code">' + esc(script.content) + '</div>' +
    '<button class="btn-primary" id="exec-btn" ' + (!selectedDevice ? 'disabled' : '') + '>' +
    (selectedDevice ? '\u25B6 Execute on ' + esc(selectedDevice.hostname) : 'Select a device first') + '</button>' +
    '<div id="exec-output" class="exec-output" style="display:none"></div>';
  const execBtn = detail.querySelector('#exec-btn');
  if (execBtn && selectedDevice) execBtn.addEventListener('click', () => executeScript(script, detail));
}

async function executeScript(script, detail) {
  if (!selectedDevice) return;
  const btn = detail.querySelector('#exec-btn'), out = detail.querySelector('#exec-output');
  out.style.display = 'block'; out.textContent = 'Executing...\n'; btn.disabled = true;
  try {
    const r = await api('POST', '/api/scripts/' + script.id + '/execute', { deviceIds: [selectedDevice.id], parameterValues: {} });
    const d = await r.json();
    const execs = d.data || d, exec = Array.isArray(execs) ? execs[0] : execs;
    if (!exec) { out.textContent += 'No execution returned\n'; return; }
    out.textContent += 'Execution ID: ' + exec.id + '\nStatus: ' + exec.status + '\n';
    let att = 0;
    const poll = setInterval(async () => {
      att++; if (att > 60) { clearInterval(poll); out.textContent += '\nTimeout.\n'; return; }
      try {
        const pr = await api('GET', '/api/executions/' + exec.id); const pd = await pr.json(); const e = pd.data || pd;
        if (['success','failure','timeout','cancelled'].includes(e.status)) {
          clearInterval(poll);
          out.textContent = 'Status: ' + e.status + ' (exit ' + (e.exitCode ?? '?') + ')\n\n';
          if (e.stdout) out.textContent += '--- stdout ---\n' + e.stdout + '\n';
          if (e.stderr) out.textContent += '--- stderr ---\n' + e.stderr + '\n';
          btn.disabled = false;
        } else { out.textContent = 'Status: ' + e.status + '...\n'; }
      } catch {}
    }, 2000);
  } catch (err) { out.textContent += 'Error: ' + err.message + '\n'; btn.disabled = false; }
}

// ── Info tab ─────────────────────────────────────────────────────────────────
function renderInfoTab(tc) {
  if (!selectedDevice) return;
  const d = selectedDevice;
  let html = '<div style="padding:16px;display:flex;flex-direction:column;gap:10px;font-size:13px">';
  if (d.oblireach.updateAvailable) {
    html += '<div style="background:rgba(245,158,11,.08);border:1px solid rgba(245,158,11,.2);border-radius:10px;padding:10px 14px;display:flex;align-items:center;gap:8px">' +
      '<span style="color:var(--warn);font-weight:600;font-size:12px">Update available</span>' +
      '<span style="font-size:11px;color:var(--muted)">' + esc(d.oblireach.version || '?') + '</span></div>';
  }
  const row = (label, val) => '<div><span style="color:var(--muted2)">' + label + ': </span>' + val + '</div>';
  html += row('Hostname', esc(d.hostname));
  html += row('Device ID', String(d.id));
  html += row('UUID', '<span style="font-family:monospace;font-size:11px">' + esc(d.uuid) + '</span>');
  html += row('OS', esc(d.osType));
  html += row('Status', esc(d.status));
  html += row('Oblireach', (d.oblireach.installed ? (d.oblireach.online ? '<span style="color:var(--success)">Online</span>' : 'Offline') : 'Not installed') +
    (d.oblireach.version ? ' <span style="font-size:11px;color:var(--muted)">v' + esc(d.oblireach.version) + '</span>' : ''));
  if (d.oblireach.sessions?.length) {
    html += row('Sessions', d.oblireach.sessions.map(s => s.username + ' (' + s.state + ')').join(', '));
  }
  html += '</div>';
  tc.innerHTML = html;
}

// ── Socket.io / Chat ─────────────────────────────────────────────────────────
function initSocketIO() {
  // Dynamically load socket.io client from the server
  const s = document.createElement('script');
  s.src = '/proxy/socket.io/socket.io.js';
  s.onload = () => {
    if (!window.io) return;
    chatSocket = io({ path: '/proxy/socket.io', transports: ['polling', 'websocket'] });
    chatSocket.on('connect', () => { console.log('socket.io connected'); });
    chatSocket.on('chat:message', onChatMessage);
    chatSocket.on('chat:closed', onChatClosed);
    chatSocket.on('chat:remote_response', onChatRemoteResponse);
  };
  s.onerror = () => { console.warn('socket.io client not available'); };
  document.head.appendChild(s);
}

document.getElementById('btn-chat').addEventListener('click', toggleChat);
document.getElementById('btn-chat-close').addEventListener('click', () => toggleChat(false));
document.getElementById('chat-send').addEventListener('click', sendChatMessage);
document.getElementById('chat-input').addEventListener('keydown', e => { if (e.key === 'Enter') sendChatMessage(); });

function toggleChat(forceOpen) {
  const panel = document.getElementById('chat-panel');
  const isOpen = panel.classList.contains('open');
  const shouldOpen = typeof forceOpen === 'boolean' ? forceOpen : !isOpen;

  if (shouldOpen) {
    panel.classList.add('open');
    if (!chatId && selectedDevice && chatSocket) openChatSession();
    document.getElementById('chat-badge').style.display = 'none';
  } else {
    panel.classList.remove('open');
  }
}

function openChatSession() {
  if (!chatSocket || !selectedDevice) return;
  const statusDot = document.getElementById('chat-status-dot');
  const statusText = document.getElementById('chat-status-text');
  statusText.textContent = 'Connecting...';

  chatSocket.emit('chat:open', {
    deviceUuid: selectedDevice.uuid,
    sessionId: selectedDevice.oblireach.sessions?.[0]?.id,
    operatorName: currentOperatorName,
  }, (res) => {
    if (res?.chatId) {
      chatId = res.chatId;
      chatConnected = true;
      chatUserClosed = false;
      chatSocket.emit('join', 'chat:' + chatId);
      statusDot.style.background = 'var(--success)';
      statusText.textContent = 'Connected';
      document.getElementById('chat-input').disabled = false;
      document.getElementById('chat-send').disabled = false;
      addChatMsg('System', 'Chat session opened.', true);
      renderRequestRemoteBtn();
    } else {
      statusText.textContent = 'Failed';
      addChatMsg('System', 'Failed: ' + (res?.error || 'agent offline'), true);
    }
  });

  setTimeout(() => {
    if (!chatId) {
      statusText.textContent = 'Timeout';
      addChatMsg('System', 'Chat connection timed out.', true);
    }
  }, 5000);
}

function closeChat() {
  if (chatSocket && chatId) chatSocket.emit('chat:close', { chatId });
  chatId = null;
  chatConnected = false;
  chatUserClosed = false;
  chatMessages = [];
  const el = document.getElementById('chat-messages');
  if (el) el.innerHTML = '';
  const statusDot = document.getElementById('chat-status-dot');
  const statusText = document.getElementById('chat-status-text');
  if (statusDot) statusDot.style.background = 'var(--muted2)';
  if (statusText) statusText.textContent = 'Disconnected';
  document.getElementById('chat-input').disabled = true;
  document.getElementById('chat-send').disabled = true;
  document.getElementById('chat-panel').classList.remove('open');
}

function addChatMsg(sender, text, isSystem) {
  const ts = Date.now();
  chatMessages.push({ sender, text, timestamp: ts, isSystem });
  renderChatMsg({ sender, text, timestamp: ts, isSystem });
}

function renderChatMsg(msg) {
  const container = document.getElementById('chat-messages');
  if (!container) return;

  // Time separator
  if (chatMessages.length <= 1 || msg.timestamp - chatMessages[chatMessages.length - 2]?.timestamp > 300000) {
    const timeDiv = document.createElement('div');
    timeDiv.className = 'chat-time';
    timeDiv.innerHTML = '<span>' + fmtTime(msg.timestamp) + '</span>';
    container.appendChild(timeDiv);
  }

  const div = document.createElement('div');
  if (msg.isSystem) {
    div.className = 'chat-msg sys';
    div.innerHTML = '<div class="bubble">' + esc(msg.text) + '</div>';
  } else {
    const isOp = msg.sender === currentOperatorName;
    div.className = 'chat-msg ' + (isOp ? 'op' : 'user');
    if (isOp) {
      div.innerHTML = '<div class="avatar" style="width:28px;height:28px;background:var(--accent);border-radius:50%%;display:flex;align-items:center;justify-content:center;flex-shrink:0">' +
        '<svg width="15" height="15" fill="white" viewBox="0 0 24 24"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z"/></svg>' +
        '</div><div class="bubble">' + esc(msg.text) + '</div>';
    } else {
      const initials = msg.sender.split(/\s+/).map(w => w[0]).join('').toUpperCase().slice(0, 2);
      div.innerHTML = '<div class="bubble">' + esc(msg.text) + '</div>' +
        '<div style="width:28px;height:28px;border-radius:50%%;background:var(--bg3);display:flex;align-items:center;justify-content:center;flex-shrink:0;font-size:10px;font-weight:700;color:var(--muted);margin-top:auto">' + initials + '</div>';
    }
  }
  container.appendChild(div);
  container.scrollTop = container.scrollHeight;
}

function sendChatMessage() {
  const input = document.getElementById('chat-input');
  const text = input.value.trim();
  if (!text || !chatId || !chatSocket) return;
  addChatMsg(currentOperatorName, text, false);
  chatSocket.emit('chat:message', { chatId, message: text, operatorName: currentOperatorName });
  input.value = '';
  input.focus();
  if (chatUserClosed) chatUserClosed = false;
}

function onChatMessage(data) {
  if (data.chatId !== chatId) return;
  addChatMsg(data.sender, data.message, false);
  if (chatUserClosed) chatUserClosed = false;
  // Show badge if panel is closed
  const panel = document.getElementById('chat-panel');
  if (!panel.classList.contains('open')) {
    document.getElementById('chat-badge').style.display = 'block';
    playNotifSound();
  }
}

function onChatClosed(data) {
  if (data.chatId !== chatId) return;
  chatUserClosed = true;
  addChatMsg('System', 'User has closed the chat. You can still send messages.', true);
  const statusDot = document.getElementById('chat-status-dot');
  statusDot.style.background = 'var(--warn)';
  document.getElementById('chat-status-text').textContent = 'User disconnected';
}

function onChatRemoteResponse(data) {
  if (data.chatId !== chatId) return;
  addChatMsg('System', data.allowed ? 'Remote control access granted.' : 'Remote control access denied.', true);
  renderRequestRemoteBtn();
}

function renderRequestRemoteBtn() {
  const wrap = document.getElementById('chat-request-btn-wrap');
  if (!wrap) return;
  if (!chatConnected || chatUserClosed) { wrap.innerHTML = ''; return; }
  wrap.innerHTML = '<button class="btn-sm" style="width:100%%;background:rgba(194,0,27,.08);border-color:rgba(194,0,27,.2);color:var(--accent-l);font-size:11px;padding:7px;border-radius:10px" id="btn-request-remote">' +
    '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="vertical-align:-2px;margin-right:4px"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>' +
    'Request Remote Control</button>';
  document.getElementById('btn-request-remote')?.addEventListener('click', () => {
    if (!chatSocket || !chatId) return;
    chatSocket.emit('chat:request_remote', { chatId, message: '' });
    addChatMsg('System', 'Remote control request sent.', true);
    wrap.innerHTML = '<div style="font-size:11px;text-align:center;color:rgba(250,204,21,.7);padding:4px">Waiting for user response...</div>';
  });
}

function playNotifSound() {
  try {
    const ctx = new AudioContext();
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.connect(gain); gain.connect(ctx.destination);
    osc.frequency.value = 800; gain.gain.value = 0.12;
    osc.start(); gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.25);
    osc.stop(ctx.currentTime + 0.25);
  } catch {}
}

// ── Top bar ──────────────────────────────────────────────────────────────────
document.getElementById('btn-logout').addEventListener('click', async () => {
  stopRemote(); closeChat();
  await fetch('/local/logout', { method: 'POST' });
  location.reload();
});

// ── Init ─────────────────────────────────────────────────────────────────────
(async function init() {
  const cfgR = await fetch('/local/config');
  const cfg = await cfgR.json();
  if (cfg.serverUrl) {
    document.getElementById('inp-server').value = cfg.serverUrl;
    document.getElementById('top-server').textContent = cfg.serverUrl;
  }
  if (cfg.username) document.getElementById('inp-user').value = cfg.username;

  if (cfg.hasSession && cfg.serverUrl) {
    try { const r = await api('GET', '/api/auth/me'); if (r.ok) { await enterApp(); return; } } catch {}
  }
  document.getElementById('login-overlay').style.display = 'flex';
  if (cfg.serverUrl) checkSso();
})();
</script>
</body>
</html>`,
		esc(cfg.ServerURL),
		esc(cfg.Username),
	)
}

func esc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
