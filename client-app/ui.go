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
  --bg:#0f172a;--bg2:#1e293b;--bg3:#334155;
  --border:#334155;--accent:#6366f1;--accent2:#818cf8;
  --text:#f1f5f9;--muted:#94a3b8;--success:#22c55e;--danger:#ef4444;--warn:#f59e0b;
  font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;
}
body{background:var(--bg);color:var(--text);height:100vh;overflow:hidden;display:flex;flex-direction:column}
.topbar{height:40px;background:var(--bg2);border-bottom:1px solid var(--border);display:flex;align-items:center;padding:0 12px;gap:8px;flex-shrink:0}
.topbar .logo{font-weight:700;font-size:14px;color:var(--accent2);letter-spacing:.5px;margin-right:auto}
.topbar .server-info{font-size:11px;color:var(--muted);max-width:220px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.topbar .btn-sm{padding:4px 10px;border-radius:6px;border:1px solid var(--border);background:var(--bg3);color:var(--text);font-size:11px;cursor:pointer;transition:background .15s}
.topbar .btn-sm:hover{background:var(--accent)/20}
.main{display:flex;flex:1;overflow:hidden}
/* ── Left panel ── */
.sidebar{width:260px;flex-shrink:0;border-right:1px solid var(--border);display:flex;flex-direction:column;background:var(--bg2)}
.sidebar-head{padding:8px 12px;border-bottom:1px solid var(--border);display:flex;align-items:center;gap:6px}
.sidebar-head input{flex:1;background:var(--bg3);border:1px solid var(--border);border-radius:6px;padding:4px 8px;font-size:12px;color:var(--text);outline:none}
.sidebar-head input:focus{border-color:var(--accent)}
.sidebar-body{flex:1;overflow-y:auto}
.group-label{padding:6px 12px;font-size:10px;font-weight:600;color:var(--muted);text-transform:uppercase;letter-spacing:.8px;display:flex;align-items:center;gap:4px}
.device-row{padding:6px 12px 6px 20px;cursor:pointer;display:flex;align-items:center;gap:8px;font-size:12px;border-radius:0;transition:background .1s}
.device-row:hover{background:var(--bg3)}
.device-row.active{background:var(--accent)/15;color:var(--accent2)}
.dot{width:7px;height:7px;border-radius:50%;flex-shrink:0}
.dot.online{background:var(--success)}
.dot.offline{background:var(--muted)}
.dot.warn{background:var(--warn)}
/* ── Right panel ── */
.content{flex:1;display:flex;flex-direction:column;overflow:hidden}
/* ── Login overlay ── */
#login-overlay{position:fixed;inset:0;background:var(--bg);display:flex;align-items:center;justify-content:center;z-index:100}
.login-box{background:var(--bg2);border:1px solid var(--border);border-radius:16px;padding:32px;width:380px;display:flex;flex-direction:column;gap:16px}
.login-box h2{font-size:20px;font-weight:700;text-align:center;color:var(--accent2)}
.login-box p{font-size:12px;color:var(--muted);text-align:center}
.form-group{display:flex;flex-direction:column;gap:4px}
.form-group label{font-size:11px;color:var(--muted);font-weight:500}
.form-group input{background:var(--bg3);border:1px solid var(--border);border-radius:8px;padding:8px 12px;font-size:13px;color:var(--text);outline:none;transition:border-color .15s}
.form-group input:focus{border-color:var(--accent)}
.btn-primary{background:var(--accent);border:none;color:white;border-radius:8px;padding:10px;font-size:13px;font-weight:600;cursor:pointer;transition:background .15s}
.btn-primary:hover{background:var(--accent2)}
.btn-primary:disabled{opacity:.5;cursor:default}
.err-msg{font-size:11px;color:var(--danger);text-align:center;min-height:16px}
/* ── Device detail pane ── */
.device-detail{flex:1;display:flex;flex-direction:column;overflow:hidden}
.device-header{padding:12px 16px;border-bottom:1px solid var(--border);display:flex;align-items:center;gap:10px;flex-shrink:0}
.device-header h2{font-size:15px;font-weight:600;flex:1}
.tabs{display:flex;gap:0;border-bottom:1px solid var(--border);flex-shrink:0;background:var(--bg2)}
.tab-btn{padding:8px 16px;font-size:12px;font-weight:500;color:var(--muted);border:none;background:none;cursor:pointer;border-bottom:2px solid transparent;margin-bottom:-1px;transition:color .15s}
.tab-btn.active{color:var(--accent2);border-bottom-color:var(--accent)}
.tab-btn:hover:not(.active){color:var(--text)}
.tab-content{flex:1;overflow:hidden;display:flex;flex-direction:column}
/* ── Remote viewer ── */
#remote-pane{flex:1;display:flex;flex-direction:column;overflow:hidden}
.remote-toolbar{padding:8px 12px;border-bottom:1px solid var(--border);display:flex;align-items:center;gap:8px;flex-shrink:0;background:var(--bg2)}
.session-select{background:var(--bg3);border:1px solid var(--border);border-radius:6px;padding:4px 8px;font-size:11px;color:var(--text);outline:none;cursor:pointer}
.remote-viewport{flex:1;background:#000;position:relative;overflow:hidden}
#remote-canvas{width:100%;height:100%;object-fit:contain;display:block}
.remote-placeholder{position:absolute;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:8px;color:var(--muted);font-size:13px}
/* ── Scripts pane ── */
#scripts-pane{flex:1;display:flex;overflow:hidden}
.scripts-list{width:220px;border-right:1px solid var(--border);overflow-y:auto;flex-shrink:0}
.script-item{padding:8px 12px;cursor:pointer;font-size:12px;border-bottom:1px solid var(--border)/30;transition:background .1s}
.script-item:hover{background:var(--bg3)}
.script-item.active{background:var(--accent)/15;color:var(--accent2)}
.script-item .sname{font-weight:500}
.script-item .sdesc{font-size:10px;color:var(--muted);margin-top:2px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.script-detail{flex:1;padding:16px;overflow-y:auto;display:flex;flex-direction:column;gap:12px}
.script-detail h3{font-size:13px;font-weight:600}
.script-code{background:var(--bg3);border:1px solid var(--border);border-radius:8px;padding:12px;font-family:monospace;font-size:11px;white-space:pre-wrap;max-height:200px;overflow-y:auto;color:var(--text)}
.exec-output{background:#000;border:1px solid var(--border);border-radius:8px;padding:12px;font-family:monospace;font-size:11px;white-space:pre-wrap;max-height:200px;overflow-y:auto;color:#4ade80;min-height:60px}
.badge{display:inline-block;padding:1px 6px;border-radius:4px;font-size:10px;font-weight:500}
.badge.windows{background:#1e3a5f;color:#60a5fa}
.badge.linux{background:#1c3a2f;color:#4ade80}
.badge.macos{background:#2d2214;color:#fbbf24}
.badge.all{background:var(--bg3);color:var(--muted)}
/* ── Empty states ── */
.empty{display:flex;flex-direction:column;align-items:center;justify-content:center;flex:1;gap:8px;color:var(--muted);font-size:13px}
.empty-icon{font-size:40px;opacity:.3}
.status-bar{padding:3px 12px;font-size:10px;color:var(--muted);border-top:1px solid var(--border);flex-shrink:0;background:var(--bg2)}
</style>
</head>
<body>

<!-- Login overlay (shown when not authenticated) -->
<div id="login-overlay">
  <div class="login-box">
    <h2>Oblireach</h2>
    <p>Connect to your Obliance server</p>
    <div class="form-group">
      <label>Server URL</label>
      <input id="inp-server" type="url" placeholder="https://obliance.example.com" value="%s"/>
    </div>
    <div class="form-group">
      <label>Username</label>
      <input id="inp-user" type="text" placeholder="admin" value="%s"/>
    </div>
    <div class="form-group">
      <label>Password</label>
      <input id="inp-pass" type="password" placeholder="••••••••"/>
    </div>
    <div class="err-msg" id="login-err"></div>
    <button class="btn-primary" id="btn-login">Connect</button>
  </div>
</div>

<!-- Main application (hidden until logged in) -->
<div id="app" style="display:none;flex-direction:column;height:100%%">
  <div class="topbar">
    <span class="logo">⬡ Oblireach</span>
    <span class="server-info" id="top-server"></span>
    <button class="btn-sm" id="btn-refresh" title="Refresh device list">↻ Refresh</button>
    <button class="btn-sm" id="btn-logout">Sign out</button>
  </div>
  <div class="main">
    <!-- Sidebar: device tree -->
    <div class="sidebar">
      <div class="sidebar-head">
        <input id="search-input" type="text" placeholder="Search devices…"/>
      </div>
      <div class="sidebar-body" id="device-tree"></div>
    </div>
    <!-- Right: device detail or empty state -->
    <div class="content" id="content-area">
      <div class="empty">
        <div class="empty-icon">🖥️</div>
        <span>Select a device from the list</span>
      </div>
    </div>
  </div>
  <div class="status-bar" id="status-bar">Ready</div>
</div>

<script>
// ── State ────────────────────────────────────────────────────────────────────
const API = ''; // relative to this page on local server
let overview = { groups: [] };
let reachScripts = [];
let selectedDevice = null;
let activeTab = 'remote';
let remoteWs = null;
let remoteDecoder = null;
let remoteTs = 0;
let execAbort = null;

// Detect IDR keyframe in H.264 Annex B stream
function isH264Keyframe(data) {
  let i = 0;
  while (i < data.length - 4) {
    if (data[i] === 0 && data[i+1] === 0) {
      let nalStart = -1;
      if (data[i+2] === 1) { nalStart = i + 3; i += 4; }
      else if (data[i+2] === 0 && data[i+3] === 1) { nalStart = i + 4; i += 5; }
      else { i++; continue; }
      if (nalStart < data.length) {
        const nalType = data[nalStart] & 0x1f;
        if (nalType === 5 || nalType === 7 || nalType === 8) return true;
      }
    } else { i++; }
  }
  return false;
}

// ── Helpers ──────────────────────────────────────────────────────────────────
async function api(method, path, body) {
  const opts = { method, headers: { 'Content-Type': 'application/json' } };
  if (body !== undefined) opts.body = JSON.stringify(body);
  const r = await fetch('/proxy' + path, opts);
  return r;
}

function setStatus(msg) { document.getElementById('status-bar').textContent = msg; }

// ── Login ────────────────────────────────────────────────────────────────────
document.getElementById('btn-login').addEventListener('click', doLogin);
document.getElementById('inp-pass').addEventListener('keydown', e => { if (e.key === 'Enter') doLogin(); });

async function doLogin() {
  const server = document.getElementById('inp-server').value.trim().replace(/\/$/, '');
  const username = document.getElementById('inp-user').value.trim();
  const password = document.getElementById('inp-pass').value;
  const errEl = document.getElementById('login-err');
  const btn = document.getElementById('btn-login');

  if (!server || !username || !password) { errEl.textContent = 'All fields required'; return; }

  btn.disabled = true;
  btn.textContent = 'Connecting…';
  errEl.textContent = '';

  try {
    // Save server URL to local config.
    await fetch('/local/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ serverUrl: server, username }),
    });

    // Authenticate against Obliance.
    const r = await api('POST', '/api/auth/login', { username, password });
    const data = await r.json();
    if (!r.ok || !data.success) {
      errEl.textContent = data.error || 'Login failed';
      return;
    }
    document.getElementById('top-server').textContent = server;
    await enterApp();
  } catch (err) {
    errEl.textContent = 'Connection failed: ' + err.message;
  } finally {
    btn.disabled = false;
    btn.textContent = 'Connect';
  }
}

async function enterApp() {
  document.getElementById('login-overlay').style.display = 'none';
  const app = document.getElementById('app');
  app.style.display = 'flex';

  // Fetch tenant list and select first tenant.
  try {
    const r = await api('GET', '/api/tenants');
    const data = await r.json();
    const tenants = data.tenants || data.data?.tenants || data;
    if (Array.isArray(tenants) && tenants.length > 0) {
      // Select the first tenant.
      await api('POST', '/api/tenant/' + tenants[0].id + '/select');
    }
  } catch {}

  await loadOverview();
  await loadScripts();
}

// ── Load data ─────────────────────────────────────────────────────────────────
async function loadOverview() {
  setStatus('Loading devices…');
  try {
    const r = await api('GET', '/api/reach/overview');
    const d = await r.json();
    overview = d.data || { groups: [] };
    renderTree();
    setStatus('Ready — ' + countDevices() + ' devices');
  } catch (err) {
    setStatus('Failed to load devices: ' + err.message);
  }
}

async function loadScripts() {
  try {
    const r = await api('GET', '/api/reach/scripts');
    const d = await r.json();
    reachScripts = d.data?.scripts || [];
  } catch {}
}

function countDevices() {
  return overview.groups.reduce((n, g) => n + g.devices.length, 0);
}

// ── Device tree ───────────────────────────────────────────────────────────────
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
      row.dataset.id = dev.id;

      const dotClass = dev.oblireach.online ? 'online' : 'warn';
      row.innerHTML = '<span class="dot ' + dotClass + '"></span>' +
        '<span style="flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">' + esc(dev.hostname) + '</span>';

      row.addEventListener('click', () => selectDevice(dev));
      tree.appendChild(row);
    }
  }
}

document.getElementById('search-input').addEventListener('input', renderTree);
document.getElementById('btn-refresh').addEventListener('click', async () => {
  await loadOverview();
  if (selectedDevice) {
    // Re-find device in fresh data.
    for (const g of overview.groups) {
      const d = g.devices.find(x => x.id === selectedDevice.id);
      if (d) { selectedDevice = d; break; }
    }
  }
});

// ── Device detail ─────────────────────────────────────────────────────────────
function selectDevice(dev) {
  selectedDevice = dev;
  renderTree(); // update active highlight

  const area = document.getElementById('content-area');
  area.innerHTML = '';
  area.style.display = 'flex';
  area.style.flexDirection = 'column';
  area.style.overflow = 'hidden';

  // Header
  const hdr = document.createElement('div');
  hdr.className = 'device-header';
  const dotClass = dev.oblireach.online ? 'online' : dev.oblireach.installed ? 'warn' : 'offline';
  const dotTitle = dev.oblireach.online ? 'Oblireach online' : dev.oblireach.installed ? 'Oblireach offline' : 'Oblireach not installed';
  hdr.innerHTML = '<span class="dot ' + dotClass + '" title="' + dotTitle + '" style="width:9px;height:9px"></span>' +
    '<h2>' + esc(dev.hostname) + '</h2>' +
    (dev.oblireach.updateAvailable ? '<span style="font-size:10px;color:var(--warn);font-weight:600;background:var(--warn)/15;padding:2px 6px;border-radius:4px">UPDATE</span>' : '') +
    '<span style="font-size:11px;color:var(--muted)">' + esc(dev.osType) + ' · ' + esc(dev.status) + '</span>';
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

  // Tab content container
  const tc = document.createElement('div');
  tc.className = 'tab-content';
  tc.id = 'tab-content';
  area.appendChild(tc);

  switchTab(activeTab, area);
}

function switchTab(tab, area) {
  activeTab = tab;
  area = area || document.getElementById('content-area');
  // Update tab button styles
  area.querySelectorAll('.tab-btn').forEach(b => {
    b.classList.toggle('active', b.dataset.tab === tab);
  });
  // Stop any existing remote session.
  if (tab !== 'remote') stopRemote();

  const tc = document.getElementById('tab-content');
  tc.innerHTML = '';

  if (tab === 'remote') renderRemoteTab(tc);
  else if (tab === 'scripts') renderScriptsTab(tc);
  else renderInfoTab(tc);
}

// ── Remote tab ────────────────────────────────────────────────────────────────
function renderRemoteTab(tc) {
  if (!selectedDevice?.oblireach?.online) {
    tc.innerHTML = '<div class="empty"><div class="empty-icon">📡</div>' +
      '<span>' + (selectedDevice?.oblireach?.installed ? 'Oblireach agent is offline' : 'Oblireach agent not installed on this device') + '</span></div>';
    return;
  }

  const pane = document.createElement('div');
  pane.id = 'remote-pane';

  // Toolbar
  const toolbar = document.createElement('div');
  toolbar.className = 'remote-toolbar';

  const sessions = selectedDevice.oblireach.sessions || [];
  const sessSelect = document.createElement('select');
  sessSelect.className = 'session-select';
  const optAuto = document.createElement('option');
  optAuto.value = '';
  optAuto.textContent = 'Auto (active session)';
  sessSelect.appendChild(optAuto);
  for (const s of sessions) {
    const opt = document.createElement('option');
    opt.value = s.id;
    opt.textContent = s.username + ' (' + s.state + (s.stationName ? ' · ' + s.stationName : '') + ')';
    sessSelect.appendChild(opt);
  }
  toolbar.appendChild(sessSelect);

  const startBtn = document.createElement('button');
  startBtn.className = 'btn-sm';
  startBtn.style.cssText = 'background:var(--accent);border-color:var(--accent);color:white';
  startBtn.textContent = '▶ Connect';
  startBtn.addEventListener('click', () => startRemote(sessSelect.value ? parseInt(sessSelect.value) : undefined));
  toolbar.appendChild(startBtn);

  const stopBtn = document.createElement('button');
  stopBtn.id = 'stop-btn';
  stopBtn.className = 'btn-sm';
  stopBtn.style.cssText = 'background:var(--danger)/20;border-color:var(--danger)/40;color:var(--danger);display:none';
  stopBtn.textContent = '■ Disconnect';
  stopBtn.addEventListener('click', stopRemote);
  toolbar.appendChild(stopBtn);

  const statusSpan = document.createElement('span');
  statusSpan.id = 'remote-status';
  statusSpan.style.cssText = 'font-size:11px;color:var(--muted);margin-left:auto';
  toolbar.appendChild(statusSpan);

  pane.appendChild(toolbar);

  // Viewport
  const vp = document.createElement('div');
  vp.className = 'remote-viewport';

  const canvas = document.createElement('canvas');
  canvas.id = 'remote-canvas';
  canvas.style.display = 'none';
  vp.appendChild(canvas);

  const placeholder = document.createElement('div');
  placeholder.className = 'remote-placeholder';
  placeholder.id = 'remote-placeholder';
  placeholder.innerHTML = '<span style="font-size:32px">🖥️</span><span>Click Connect to start remote session</span>';
  vp.appendChild(placeholder);

  pane.appendChild(vp);
  tc.appendChild(pane);

  // Input forwarding (mouse + keyboard).
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
  if (statusEl) statusEl.textContent = 'Starting session…';

  try {
    // Create remote session on server.
    const body = { deviceId: selectedDevice.id, protocol: 'oblireach' };
    if (wtsSessionId !== undefined) body.sessionId = wtsSessionId;
    const r = await api('POST', '/api/remote/sessions', body);
    const d = await r.json();
    const session = d.data;
    if (!session?.sessionToken) throw new Error('No session token in response');

    if (statusEl) statusEl.textContent = 'Connecting…';

    // Connect WebSocket to remote tunnel (via proxy).
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = proto + '//' + location.host + '/proxy/api/remote/tunnel/' + session.sessionToken;

    const ws = new WebSocket(wsUrl);
    ws.binaryType = 'arraybuffer';
    remoteWs = ws;

    const stopBtn = document.getElementById('stop-btn');
    if (stopBtn) stopBtn.style.display = '';

    ws.onopen = () => { if (statusEl) statusEl.textContent = 'Connected — waiting for stream…'; };
    ws.onclose = () => {
      if (statusEl) statusEl.textContent = 'Disconnected';
      if (stopBtn) stopBtn.style.display = 'none';
      const canvas = document.getElementById('remote-canvas');
      const ph = document.getElementById('remote-placeholder');
      if (canvas) canvas.style.display = 'none';
      if (ph) ph.style.display = 'flex';
      remoteWs = null;
      remoteDecoder = null;
    };
    ws.onerror = () => { if (statusEl) statusEl.textContent = 'WebSocket error'; };
    ws.onmessage = handleRemoteMessage;
  } catch (err) {
    if (statusEl) statusEl.textContent = 'Error: ' + err.message;
  }
}

function stopRemote() {
  if (remoteWs) { remoteWs.close(); remoteWs = null; }
  if (remoteDecoder) { try { remoteDecoder.close(); } catch {} remoteDecoder = null; }
}

async function handleRemoteMessage(event) {
  if (typeof event.data === 'string') {
    // Control messages: { type, ... }
    try {
      const info = JSON.parse(event.data);
      if (info.type === 'paired') return; // agent connected — stream starting
      // Init message: { type:'init', width, height, fps, codec, extradata? }
      if (!info.width || !info.height) return;
      await initDecoder(info);
      const statusEl = document.getElementById('remote-status');
      if (statusEl) statusEl.textContent = info.width + '×' + info.height + ' @ ' + info.fps + 'fps';
      const canvas = document.getElementById('remote-canvas');
      const ph = document.getElementById('remote-placeholder');
      if (canvas) { canvas.width = info.width; canvas.height = info.height; canvas.style.display = 'block'; }
      if (ph) ph.style.display = 'none';
    } catch {}
    return;
  }

  // Binary: [1-byte type][payload]
  const buf = new Uint8Array(event.data);
  if (buf.length < 1) return;
  const type = buf[0];
  const payload = buf.slice(1);

  if (type === 0x01) {
    // JPEG frame — decode with createImageBitmap
    const blob = new Blob([payload], { type: 'image/jpeg' });
    createImageBitmap(blob).then(bmp => {
      const canvas = document.getElementById('remote-canvas');
      if (!canvas) return;
      // Store agent dimensions for input coordinate mapping
      if (!canvas._agentW || canvas._agentW !== bmp.width || canvas._agentH !== bmp.height) {
        canvas._agentW = bmp.width;
        canvas._agentH = bmp.height;
      }
      // Size the canvas to its CSS layout size (viewport) and draw scaled
      const rect = canvas.getBoundingClientRect();
      const cw = Math.round(rect.width * (window.devicePixelRatio || 1));
      const ch = Math.round(rect.height * (window.devicePixelRatio || 1));
      if (canvas.width !== cw || canvas.height !== ch) {
        canvas.width = cw;
        canvas.height = ch;
      }
      canvas.style.display = 'block';
      const ph = document.getElementById('remote-placeholder');
      if (ph) ph.style.display = 'none';
      const ctx = canvas.getContext('2d');
      if (ctx) {
        // Fit the image while preserving aspect ratio
        const scale = Math.min(cw / bmp.width, ch / bmp.height);
        const dw = Math.round(bmp.width * scale);
        const dh = Math.round(bmp.height * scale);
        const dx = Math.round((cw - dw) / 2);
        const dy = Math.round((ch - dh) / 2);
        ctx.clearRect(0, 0, cw, ch);
        ctx.drawImage(bmp, dx, dy, dw, dh);
      }
      bmp.close();
    }).catch(() => {});
  } else if (type === 0x02 && remoteDecoder) {
    // H.264 frame
    const isKey = isH264Keyframe(payload);
    const chunk = new EncodedVideoChunk({
      type: isKey ? 'key' : 'delta',
      timestamp: remoteTs,
      data: payload,
    });
    remoteTs += Math.round(1000000 / 15);
    try { remoteDecoder.decode(chunk); } catch {}
  }
}

async function initDecoder(info) {
  if (remoteDecoder) { try { remoteDecoder.close(); } catch {} }

  const canvas = document.getElementById('remote-canvas');
  if (!canvas) return;
  const ctx = canvas.getContext('2d');

  remoteDecoder = new VideoDecoder({
    output(frame) {
      if (canvas) {
        canvas.width = frame.displayWidth;
        canvas.height = frame.displayHeight;
        ctx.drawImage(frame, 0, 0);
      }
      frame.close();
    },
    error(e) { console.warn('decoder error', e); }
  });

  remoteTs = 0;
  const config = { codec: 'avc1.640034', codedWidth: info.width, codedHeight: info.height, optimizeForLatency: true };
  if (info.extradata) {
    // extradata is base64 from JSON
    const bin = atob(info.extradata);
    const arr = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) arr[i] = bin.charCodeAt(i);
    config.description = arr;
  }
  remoteDecoder.configure(config);
}

function sendInput(type, e, canvas) {
  if (!remoteWs || remoteWs.readyState !== WebSocket.OPEN) return;
  const rect = canvas.getBoundingClientRect();
  // Use agent dimensions if available (JPEG path), otherwise use canvas internal size
  const aw = canvas._agentW || canvas.width;
  const ah = canvas._agentH || canvas.height;
  // Compute the scaled image rect within the canvas (accounting for aspect-ratio fit)
  const dpr = window.devicePixelRatio || 1;
  const cw = rect.width * dpr;
  const ch = rect.height * dpr;
  const scale = Math.min(cw / aw, ch / ah);
  const dw = aw * scale;
  const dh = ah * scale;
  const dx = (cw - dw) / 2;
  const dy = (ch - dh) / 2;
  const px = (e.clientX - rect.left) * dpr - dx;
  const py = (e.clientY - rect.top) * dpr - dy;
  const msg = { type, x: Math.round(px / scale), y: Math.round(py / scale) };
  if (type === 'wheel') { msg.deltaX = e.deltaX; msg.deltaY = e.deltaY; }
  if (type !== 'mousemove') msg.button = e.button;
  remoteWs.send(JSON.stringify(msg));
}

function sendInputKey(type, e) {
  if (!remoteWs || remoteWs.readyState !== WebSocket.OPEN) return;
  remoteWs.send(JSON.stringify({ type, key: e.key, code: e.code, shift: e.shiftKey, ctrl: e.ctrlKey, alt: e.altKey }));
}

// ── Scripts tab ───────────────────────────────────────────────────────────────
function renderScriptsTab(tc) {
  if (reachScripts.length === 0) {
    tc.innerHTML = '<div class="empty"><div class="empty-icon">📜</div><span>No scripts marked as "Available in Reach"</span></div>';
    return;
  }

  const pane = document.createElement('div');
  pane.id = 'scripts-pane';

  const list = document.createElement('div');
  list.className = 'scripts-list';

  const detail = document.createElement('div');
  detail.className = 'script-detail';
  detail.id = 'script-detail';
  detail.innerHTML = '<div style="color:var(--muted);font-size:12px;margin:auto">Select a script</div>';

  for (const s of reachScripts) {
    const item = document.createElement('div');
    item.className = 'script-item';
    item.innerHTML = '<div class="sname">' + esc(s.name) + '</div>' +
      '<div class="sdesc">' + esc(s.description || s.runtime) + '</div>';
    item.addEventListener('click', () => {
      list.querySelectorAll('.script-item').forEach(x => x.classList.remove('active'));
      item.classList.add('active');
      showScriptDetail(s, detail);
    });
    list.appendChild(item);
  }

  pane.appendChild(list);
  pane.appendChild(detail);
  tc.appendChild(pane);
}

function showScriptDetail(script, detail) {
  const platformColor = { windows: 'windows', linux: 'linux', macos: 'macos', all: 'all' };
  detail.innerHTML =
    '<h3>' + esc(script.name) + '</h3>' +
    '<div style="display:flex;gap:6px;align-items:center">' +
      '<span class="badge ' + (platformColor[script.platform] || 'all') + '">' + esc(script.platform) + '</span>' +
      '<span style="font-size:11px;color:var(--muted)">' + esc(script.runtime) + '</span>' +
    '</div>' +
    (script.description ? '<p style="font-size:12px;color:var(--muted)">' + esc(script.description) + '</p>' : '') +
    '<div class="script-code">' + esc(script.content) + '</div>' +
    '<button class="btn-primary" id="exec-btn" ' + (!selectedDevice ? 'disabled' : '') + '>' +
      (selectedDevice ? '▶ Execute on ' + esc(selectedDevice.hostname) : 'Select a device first') +
    '</button>' +
    '<div id="exec-output" class="exec-output" style="display:none"></div>';

  const execBtn = detail.querySelector('#exec-btn');
  if (execBtn && selectedDevice) {
    execBtn.addEventListener('click', () => executeScript(script, detail));
  }
}

async function executeScript(script, detail) {
  if (!selectedDevice) return;
  const btn = detail.querySelector('#exec-btn');
  const out = detail.querySelector('#exec-output');
  out.style.display = 'block';
  out.textContent = 'Executing…\n';
  btn.disabled = true;

  try {
    const r = await api('POST', '/api/scripts/' + script.id + '/execute', {
      deviceIds: [selectedDevice.id],
      parameterValues: {},
    });
    const d = await r.json();
    const executions = d.data || d;
    const exec = Array.isArray(executions) ? executions[0] : executions;
    if (!exec) { out.textContent += 'No execution returned\n'; return; }

    out.textContent += 'Execution ID: ' + exec.id + '\nStatus: ' + exec.status + '\n';

    // Poll for completion.
    let attempts = 0;
    const poll = setInterval(async () => {
      attempts++;
      if (attempts > 60) { clearInterval(poll); out.textContent += '\nTimeout waiting for result.\n'; return; }
      try {
        const pr = await api('GET', '/api/executions/' + exec.id);
        const pd = await pr.json();
        const e = pd.data || pd;
        if (e.status === 'success' || e.status === 'failure' || e.status === 'timeout' || e.status === 'cancelled') {
          clearInterval(poll);
          out.textContent = 'Status: ' + e.status + ' (exit ' + (e.exitCode ?? '?') + ')\n\n';
          if (e.stdout) out.textContent += '--- stdout ---\n' + e.stdout + '\n';
          if (e.stderr) out.textContent += '--- stderr ---\n' + e.stderr + '\n';
          btn.disabled = false;
        } else {
          out.textContent = 'Status: ' + e.status + '…\n';
        }
      } catch {}
    }, 2000);
  } catch (err) {
    out.textContent += 'Error: ' + err.message + '\n';
    btn.disabled = false;
  }
}

// ── Info tab ──────────────────────────────────────────────────────────────────
function renderInfoTab(tc) {
  if (!selectedDevice) return;
  const d = selectedDevice;
  let html = '<div style="padding:16px;display:flex;flex-direction:column;gap:8px;font-size:13px">';
  if (d.oblireach.updateAvailable) {
    html += '<div style="background:var(--warn)/15;border:1px solid var(--warn)/40;border-radius:8px;padding:8px 12px;display:flex;align-items:center;gap:8px">' +
      '<span style="color:var(--warn);font-weight:600;font-size:12px">Update available</span>' +
      '<span style="font-size:11px;color:var(--muted)">' + esc(d.oblireach.version || '?') + ' — a newer version is available. The agent will update automatically.</span>' +
    '</div>';
  }
  html += '<div><span style="color:var(--muted)">Hostname: </span>' + esc(d.hostname) + '</div>' +
    '<div><span style="color:var(--muted)">Device ID: </span>' + d.id + '</div>' +
    '<div><span style="color:var(--muted)">UUID: </span><span style="font-family:monospace;font-size:11px">' + esc(d.uuid) + '</span></div>' +
    '<div><span style="color:var(--muted)">OS: </span>' + esc(d.osType) + '</div>' +
    '<div><span style="color:var(--muted)">Agent status: </span>' + esc(d.status) + '</div>' +
    '<div><span style="color:var(--muted)">Oblireach: </span>' +
      (d.oblireach.installed ? (d.oblireach.online ? 'Online' : 'Offline') : 'Not installed') +
      (d.oblireach.version ? ' <span style="font-size:11px;color:var(--muted)">v' + esc(d.oblireach.version) + '</span>' : '') +
    '</div>' +
    (d.oblireach.sessions?.length ? '<div><span style="color:var(--muted)">WTS sessions: </span>' +
      d.oblireach.sessions.map(s => s.username + ' (' + s.state + ')').join(', ') + '</div>' : '') +
  '</div>';
  tc.innerHTML = html;
}

// ── Top bar buttons ───────────────────────────────────────────────────────────
document.getElementById('btn-logout').addEventListener('click', async () => {
  stopRemote();
  await fetch('/local/logout', { method: 'POST' });
  location.reload();
});

// ── Escape HTML ───────────────────────────────────────────────────────────────
function esc(s) {
  if (!s) return '';
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

// ── Auto-login if session exists ──────────────────────────────────────────────
(async function init() {
  // Check local config.
  const cfgR = await fetch('/local/config');
  const cfg = await cfgR.json();
  if (cfg.serverUrl) {
    document.getElementById('inp-server').value = cfg.serverUrl;
    document.getElementById('top-server').textContent = cfg.serverUrl;
  }
  if (cfg.username) {
    document.getElementById('inp-user').value = cfg.username;
  }

  if (cfg.hasSession && cfg.serverUrl) {
    // Try to resume session without re-login.
    try {
      const r = await api('GET', '/api/auth/me');
      if (r.ok) {
        await enterApp();
        return;
      }
    } catch {}
  }
  // Show login screen.
  document.getElementById('login-overlay').style.display = 'flex';
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
