package main

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed logo-login.svg
var logoLoginSVG string

//go:embed logo-icon.svg
var logoIconSVG string

// buildUI returns the full single-page HTML application embedded in the desktop client.
func buildUI(cfg *Config) string {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"/>
<meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>Oblireach</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
:root{
  --bg:#0d1117;--bg2:#161b22;--bg3:#21262d;--bg4:#1c2128;
  --border:rgba(255,255,255,.06);--border2:rgba(255,255,255,.10);
  --accent:#c2001b;--accent-h:#a80018;--accent-l:#e84050;
  --input-bg:#0d1117;--input-border:rgba(255,255,255,.1);
  --text:#e6edf3;--muted:#8b949e;--muted2:#484f58;
  --success:#4ade80;--danger:#ef4444;--warn:#f59e0b;
  font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;
}
body{background:var(--bg);color:var(--text);height:100vh;overflow:hidden;display:flex;flex-direction:column}
::-webkit-scrollbar{width:6px}
::-webkit-scrollbar-track{background:transparent}
::-webkit-scrollbar-thumb{background:rgba(255,255,255,.08);border-radius:3px}
::-webkit-scrollbar-thumb:hover{background:rgba(255,255,255,.15)}

/* ── Top bar ── */
.topbar{height:44px;background:linear-gradient(90deg,#161b22,#1c2128);border-bottom:1px solid var(--border);display:flex;align-items:center;padding:0 16px;gap:10px;flex-shrink:0}
.topbar .logo{margin-right:auto;display:flex;align-items:center}
.topbar .logo svg{height:24px;width:auto}
.topbar .server-info{font-size:11px;color:var(--muted2);max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.btn-sm{padding:5px 12px;border-radius:10px;border:1px solid var(--border2);background:rgba(255,255,255,.04);color:var(--muted);font-size:11px;cursor:pointer;transition:all .15s}
.btn-sm:hover{background:rgba(255,255,255,.08);color:var(--text);border-color:rgba(255,255,255,.15)}
.main{display:flex;flex:1;overflow:hidden}

/* ── Sidebar ── */
.sidebar{width:260px;flex-shrink:0;border-right:1px solid var(--border);display:flex;flex-direction:column;background:var(--bg2)}
.sidebar-head{padding:10px 12px;border-bottom:1px solid var(--border);display:flex;align-items:center;gap:6px}
.sidebar-head input{flex:1;background:var(--input-bg);border:1px solid var(--input-border);border-radius:10px;padding:6px 10px;font-size:12px;color:var(--text);outline:none;transition:border-color .15s}
.sidebar-head input:focus{border-color:var(--accent)}
.sidebar-body{flex:1;overflow-y:auto;padding:4px 0}
.group-label{padding:8px 14px 4px;font-size:10px;font-weight:600;color:var(--muted2);text-transform:uppercase;letter-spacing:.8px}
.device-row{padding:7px 14px 7px 22px;cursor:pointer;display:flex;align-items:center;gap:8px;font-size:12px;border-radius:0;transition:all .1s;margin:1px 6px;border-radius:8px}
.device-row:hover{background:rgba(255,255,255,.04)}
.device-row.active{background:rgba(194,0,27,.12);color:var(--accent-l)}
.dot{width:7px;height:7px;border-radius:50%%;flex-shrink:0}
.dot.online{background:var(--success)}
.dot.offline{background:var(--muted2)}
.dot.warn{background:var(--warn)}

/* ── Content ── */
.content{flex:1;display:flex;flex-direction:column;overflow:hidden}

/* ── Login ── */
#login-overlay{position:fixed;inset:0;background:linear-gradient(180deg,#0d1117 0%%,#161b22 100%%);display:flex;align-items:center;justify-content:center;z-index:100}
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
.remote-viewport{flex:1;background:#010409;position:relative;overflow:hidden}
#remote-canvas{width:100%%;height:100%%;object-fit:contain;display:block}
.remote-placeholder{position:absolute;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:10px;color:var(--muted2);font-size:13px}

/* ── Scripts ── */
#scripts-pane{flex:1;display:flex;overflow:hidden}
.scripts-list{width:220px;border-right:1px solid var(--border);overflow-y:auto;flex-shrink:0;background:var(--bg2)}
.script-item{padding:8px 12px;cursor:pointer;font-size:12px;border-bottom:1px solid var(--border);transition:all .1s}
.script-item:hover{background:rgba(255,255,255,.04)}
.script-item.active{background:rgba(194,0,27,.12);color:var(--accent-l)}
.script-item .sname{font-weight:500}
.script-item .sdesc{font-size:10px;color:var(--muted2);margin-top:2px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.script-detail{flex:1;padding:16px;overflow-y:auto;display:flex;flex-direction:column;gap:12px}
.script-detail h3{font-size:13px;font-weight:600}
.script-code{background:var(--bg4);border:1px solid var(--border);border-radius:10px;padding:12px;font-family:monospace;font-size:11px;white-space:pre-wrap;max-height:200px;overflow-y:auto;color:var(--text)}
.exec-output{background:#010409;border:1px solid var(--border);border-radius:10px;padding:12px;font-family:monospace;font-size:11px;white-space:pre-wrap;max-height:200px;overflow-y:auto;color:var(--success);min-height:60px}
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
.chat-panel{width:0;overflow:hidden;transition:width .25s ease;border-left:1px solid var(--border);display:flex;flex-direction:column;background:linear-gradient(180deg,#0d1117 0%%,#161b22 100%%);flex-shrink:0}
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
.chat-typing{display:none;align-self:flex-end;gap:8px;align-items:flex-end;flex-direction:row-reverse;padding:0 2px}
.chat-typing.visible{display:flex}
.chat-typing-dots{display:flex;gap:3px;padding:8px 14px;background:var(--bg3);border-radius:16px 16px 4px 16px}
.chat-typing-dots span{width:5px;height:5px;border-radius:50%%;background:rgba(255,255,255,.5);animation:chatTypeBounce 1.2s infinite}
.chat-typing-dots span:nth-child(2){animation-delay:.2s}
.chat-typing-dots span:nth-child(3){animation-delay:.4s}
@keyframes chatTypeBounce{0%%,60%%,100%%{transform:translateY(0);opacity:.4}30%%{transform:translateY(-3px);opacity:1}}
.chat-time{text-align:center;padding:4px 0}
.chat-time span{font-size:10px;color:var(--muted);background:rgba(255,255,255,.04);padding:2px 10px;border-radius:20px}
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

/* ── Toolbar extras ── */
.toolbar-sep{width:1px;height:20px;background:var(--border2);flex-shrink:0}
.toolbar-dropdown{position:relative;display:inline-block}
.toolbar-dropdown-menu{display:none;position:absolute;top:100%%;left:0;margin-top:4px;background:var(--bg2);border:1px solid var(--border2);border-radius:10px;padding:4px;min-width:160px;z-index:50;box-shadow:0 8px 24px rgba(0,0,0,.4)}
.toolbar-dropdown-menu.open{display:block}
.toolbar-dropdown-menu button{display:block;width:100%%;text-align:left;padding:6px 10px;font-size:11px;color:var(--text);background:none;border:none;border-radius:6px;cursor:pointer;white-space:nowrap}
.toolbar-dropdown-menu button:hover{background:rgba(255,255,255,.06)}

/* ── Performance HUD ── */
.perf-hud{position:absolute;top:8px;left:8px;background:rgba(0,0,0,.7);backdrop-filter:blur(4px);border:1px solid rgba(255,255,255,.1);border-radius:8px;padding:6px 10px;font-family:monospace;font-size:11px;color:#4ade80;z-index:20;pointer-events:none;display:none;line-height:1.6}
.perf-hud.visible{display:block}
.perf-hud .hud-row{display:flex;gap:8px}
.perf-hud .hud-label{color:var(--muted);min-width:48px}
.perf-hud .hud-val{font-weight:600}
.perf-hud .hud-warn{color:#f59e0b}
.perf-hud .hud-bad{color:#ef4444}

/* ── Recording indicator ── */
.rec-indicator{position:absolute;top:8px;right:8px;background:rgba(0,0,0,.7);border:1px solid rgba(239,68,68,.3);border-radius:8px;padding:5px 10px;font-size:11px;color:#ef4444;z-index:20;display:none;align-items:center;gap:6px;font-weight:600}
.rec-indicator.active{display:flex}
.rec-dot{width:8px;height:8px;border-radius:50%%;background:#ef4444;animation:rec-pulse 1s ease-in-out infinite}
@keyframes rec-pulse{0%%,100%%{opacity:1}50%%{opacity:.3}}

/* ── Annotation overlay ── */
.annotation-overlay{position:absolute;inset:0;z-index:15;cursor:crosshair;display:none}
.annotation-overlay.active{display:block}
.annotation-toolbar{position:absolute;bottom:12px;left:50%%;transform:translateX(-50%%);background:var(--bg2);border:1px solid var(--border2);border-radius:12px;padding:4px 6px;display:flex;gap:2px;z-index:16;box-shadow:0 8px 24px rgba(0,0,0,.4)}
.annotation-toolbar button{width:32px;height:32px;border:none;border-radius:8px;background:transparent;color:var(--muted);cursor:pointer;display:flex;align-items:center;justify-content:center;font-size:14px;transition:all .1s}
.annotation-toolbar button:hover{background:rgba(255,255,255,.06);color:var(--text)}
.annotation-toolbar button.active{background:rgba(194,0,27,.15);color:var(--accent-l)}

/* ── Quick Actions drawer ── */
.quick-actions{position:absolute;top:0;right:0;bottom:0;width:0;overflow:hidden;transition:width .2s ease;background:var(--bg2);border-left:1px solid var(--border);z-index:25;display:flex;flex-direction:column}
.quick-actions.open{width:220px}
.quick-actions-header{padding:10px 14px;border-bottom:1px solid var(--border);display:flex;align-items:center;justify-content:space-between;flex-shrink:0}
.quick-actions-header span{font-size:12px;font-weight:600;color:var(--text)}
.quick-actions-body{flex:1;overflow-y:auto;padding:6px}
.qa-btn{display:flex;align-items:center;gap:8px;width:100%%;padding:8px 10px;border:none;background:none;color:var(--text);font-size:12px;cursor:pointer;border-radius:8px;transition:background .1s;text-align:left}
.qa-btn:hover{background:rgba(255,255,255,.06)}
.qa-btn svg{flex-shrink:0;color:var(--muted)}

/* ── Favorites star ── */
.fav-star{opacity:0;cursor:pointer;color:var(--muted2);transition:all .15s;margin-left:auto;flex-shrink:0;font-size:13px}
.device-row:hover .fav-star{opacity:1}
.fav-star.active{opacity:1;color:#f59e0b}

/* ── Multi-tab bar ── */
.session-tabs{display:flex;align-items:center;border-bottom:1px solid var(--border);background:var(--bg2);flex-shrink:0;overflow-x:auto;min-height:0}
.session-tabs:empty{display:none}
.session-tab{display:flex;align-items:center;gap:6px;padding:6px 12px;font-size:11px;color:var(--muted);cursor:pointer;border-bottom:2px solid transparent;white-space:nowrap;transition:all .1s;flex-shrink:0}
.session-tab:hover{color:var(--text);background:rgba(255,255,255,.03)}
.session-tab.active{color:var(--accent-l);border-bottom-color:var(--accent)}
.session-tab .tab-close{width:14px;height:14px;border-radius:4px;display:flex;align-items:center;justify-content:center;opacity:0;transition:all .1s}
.session-tab:hover .tab-close{opacity:.6}
.session-tab .tab-close:hover{opacity:1;background:rgba(255,255,255,.1)}

/* ── Zoom viewport ── */
.remote-viewport.zoom-scroll{overflow:auto}
.remote-viewport.zoom-scroll #remote-canvas{object-fit:unset}

/* ── Generic modal (2FA prompt, confirmation dialogs, etc.) ── */
.modal-overlay{position:fixed;inset:0;background:rgba(0,0,0,.65);backdrop-filter:blur(6px);z-index:300;display:none;align-items:center;justify-content:center;padding:20px}
.modal-overlay.visible{display:flex}
.modal-box{background:var(--bg2);border:1px solid var(--border2);border-radius:18px;padding:22px 24px;width:420px;max-width:100%%;display:flex;flex-direction:column;gap:14px;box-shadow:0 20px 60px rgba(0,0,0,.6)}
.modal-head{display:flex;align-items:center;gap:10px}
.modal-head .icon{width:36px;height:36px;border-radius:10px;background:rgba(194,0,27,.15);color:var(--accent-l);display:flex;align-items:center;justify-content:center;flex-shrink:0}
.modal-head h3{font-size:15px;font-weight:600;color:var(--text)}
.modal-head .close{margin-left:auto;width:28px;height:28px;border-radius:8px;background:transparent;border:none;color:var(--muted);cursor:pointer;display:flex;align-items:center;justify-content:center}
.modal-head .close:hover{background:rgba(255,255,255,.06);color:var(--text)}
.modal-box p.desc{font-size:12px;color:var(--muted);line-height:1.5}
.otp-input{background:var(--input-bg);border:1px solid var(--input-border);border-radius:12px;padding:14px 16px;font-size:22px;font-family:monospace;color:var(--text);letter-spacing:10px;text-align:center;outline:none;transition:border-color .15s;width:100%%}
.otp-input:focus{border-color:var(--accent)}
.trust-row{display:flex;align-items:flex-start;gap:10px;background:rgba(255,255,255,.03);border:1px solid var(--border);border-radius:12px;padding:10px 12px;cursor:pointer}
.trust-row input{accent-color:var(--accent);margin-top:2px;flex-shrink:0;width:14px;height:14px;cursor:pointer}
.trust-row .labels{display:flex;flex-direction:column;gap:3px;min-width:0}
.trust-row .labels .title{font-size:12px;color:var(--text);font-weight:500}
.trust-row .labels .sub{font-size:11px;color:var(--muted2);line-height:1.4}
.trust-row .labels .ip{font-family:monospace;color:var(--muted);font-size:11px}
.modal-err{font-size:11px;color:var(--danger);min-height:14px}
.modal-actions{display:flex;gap:8px;justify-content:flex-end}
.btn-ghost{background:transparent;border:1px solid var(--border2);color:var(--muted);border-radius:10px;padding:8px 14px;font-size:12px;cursor:pointer;transition:all .15s}
.btn-ghost:hover{color:var(--text);border-color:var(--border2);background:rgba(255,255,255,.04)}

/* ── Nav rail (left vertical navigation, Rustdesk-console style) ── */
.nav-rail{width:220px;flex-shrink:0;border-right:1px solid var(--border);display:flex;flex-direction:column;background:var(--bg2);padding:8px 0;overflow-y:auto}
.nav-item{display:flex;align-items:center;gap:10px;padding:9px 14px;margin:1px 8px;border-radius:8px;cursor:pointer;color:var(--muted);font-size:12px;transition:all .1s;user-select:none}
.nav-item:hover{background:rgba(255,255,255,.04);color:var(--text)}
.nav-item.active{background:rgba(194,0,27,.12);color:var(--accent-l)}
.nav-item svg{flex-shrink:0;opacity:.75}
.nav-item.active svg{opacity:1}
.nav-item .chev{margin-left:auto;opacity:.5;transition:transform .15s}
.nav-item.expanded .chev{transform:rotate(90deg)}
.nav-sub{padding:1px 0 3px 0;display:none}
.nav-sub.open{display:block}
.nav-sub .nav-item{padding-left:40px;font-size:12px;color:var(--muted2)}
.nav-sub .nav-item.active{color:var(--accent-l);background:rgba(194,0,27,.08)}
.nav-group-label{padding:12px 18px 4px;font-size:9.5px;font-weight:700;color:var(--muted2);text-transform:uppercase;letter-spacing:1px}
.nav-item .count-pill{margin-left:auto;font-size:10px;font-weight:600;background:rgba(255,255,255,.06);color:var(--muted);padding:1px 6px;border-radius:8px}
.nav-item.active .count-pill{background:rgba(194,0,27,.18);color:var(--accent-l)}

/* ── Page frame ── */
.page-frame{flex:1;display:flex;flex-direction:column;overflow:hidden;background:var(--bg);min-width:0}
.page{flex:1;display:none;flex-direction:column;overflow:hidden;min-width:0}
.page.active{display:flex}
.page-header{padding:14px 24px;border-bottom:1px solid var(--border);display:flex;align-items:center;gap:12px;flex-shrink:0;background:var(--bg2)}
.page-header h1{font-size:17px;font-weight:600;color:var(--text)}
.page-header .breadcrumb{font-size:12px;color:var(--muted)}
.page-header .breadcrumb span.sep{margin:0 6px;color:var(--muted2)}
.page-header .page-actions{margin-left:auto;display:flex;gap:8px;align-items:center}
.page-body{flex:1;overflow:auto;padding:20px 24px}
.page-body.flush{padding:0}

/* ── Top bar widgets: tenant switcher + user menu ── */
.topbar-right{margin-left:auto;display:flex;align-items:center;gap:8px}
.tenant-switcher{display:flex;align-items:center;gap:6px;padding:5px 10px;border-radius:10px;border:1px solid var(--border2);background:rgba(255,255,255,.03);color:var(--text);font-size:12px;cursor:pointer;position:relative;max-width:240px}
.tenant-switcher:hover{background:rgba(255,255,255,.06)}
.tenant-switcher .label{max-width:160px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.tenant-switcher .chev{opacity:.6}
.tenant-menu{position:absolute;top:calc(100%% + 4px);left:0;background:var(--bg2);border:1px solid var(--border2);border-radius:12px;padding:4px;min-width:240px;z-index:60;box-shadow:0 12px 30px rgba(0,0,0,.5);display:none;max-height:360px;overflow-y:auto}
.tenant-menu.open{display:block}
.tenant-menu-item{padding:8px 12px;border-radius:8px;font-size:12px;color:var(--text);cursor:pointer;display:flex;align-items:center;gap:8px;white-space:nowrap}
.tenant-menu-item:hover{background:rgba(255,255,255,.06)}
.tenant-menu-item.active{color:var(--accent-l);background:rgba(194,0,27,.1)}
.user-menu-trigger{display:flex;align-items:center;gap:8px;padding:3px 10px 3px 3px;border-radius:20px;border:1px solid var(--border2);background:rgba(255,255,255,.03);color:var(--text);font-size:12px;cursor:pointer;position:relative}
.user-menu-trigger:hover{background:rgba(255,255,255,.06)}
.user-menu-trigger .avatar-sm{width:26px;height:26px;border-radius:50%%;background:var(--accent);display:flex;align-items:center;justify-content:center;flex-shrink:0;font-weight:700;font-size:10px;color:white;overflow:hidden}
.user-menu-trigger .avatar-sm img{width:100%%;height:100%%;object-fit:cover}
.user-menu-trigger .uname{max-width:120px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.user-menu{position:absolute;top:calc(100%% + 4px);right:0;background:var(--bg2);border:1px solid var(--border2);border-radius:12px;padding:6px;min-width:220px;z-index:60;box-shadow:0 12px 30px rgba(0,0,0,.5);display:none}
.user-menu.open{display:block}
.user-menu-head{padding:10px 12px;border-bottom:1px solid var(--border);margin-bottom:4px}
.user-menu-head .name{font-size:13px;color:var(--text);font-weight:600}
.user-menu-head .sub{font-size:11px;color:var(--muted);margin-top:2px;word-break:break-all}
.user-menu-item{padding:8px 12px;border-radius:7px;font-size:12px;color:var(--text);cursor:pointer;display:flex;align-items:center;gap:8px}
.user-menu-item:hover{background:rgba(255,255,255,.06)}
.user-menu-item.danger{color:var(--danger)}
.user-menu-item.danger:hover{background:rgba(239,68,68,.08)}
.user-menu-item svg{opacity:.7;flex-shrink:0}

/* ── KPI cards ── */
.kpi-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(220px,1fr));gap:14px;margin-bottom:24px}
.kpi-card{background:var(--bg2);border:1px solid var(--border);border-radius:14px;padding:16px 18px;display:flex;flex-direction:column;gap:4px;position:relative;overflow:hidden}
.kpi-card .label{font-size:10.5px;color:var(--muted);text-transform:uppercase;letter-spacing:.8px;font-weight:700}
.kpi-card .value{font-size:28px;font-weight:700;color:var(--text);font-variant-numeric:tabular-nums;line-height:1.1}
.kpi-card.online .value{color:var(--success)}
.kpi-card.offline .value{color:var(--muted)}
.kpi-card.warn .value{color:var(--warn)}
.kpi-card.danger .value{color:var(--danger)}
.kpi-card .sub{font-size:11px;color:var(--muted2);margin-top:2px}
.kpi-card .ico{position:absolute;top:14px;right:14px;opacity:.15;color:var(--text)}

/* ── Generic panel ── */
.panel{background:var(--bg2);border:1px solid var(--border);border-radius:14px;overflow:hidden;margin-bottom:16px}
.panel-header{padding:12px 18px;border-bottom:1px solid var(--border);display:flex;align-items:center;gap:10px;font-size:13px;font-weight:600;color:var(--text)}
.panel-header .panel-actions{margin-left:auto;display:flex;gap:6px}
.panel-body{padding:14px 18px}
.panel-body.flush{padding:0}

/* ── Data table ── */
.dt{width:100%%;border-collapse:collapse;font-size:12px}
.dt th{text-align:left;padding:10px 14px;font-size:10.5px;font-weight:700;color:var(--muted);background:var(--bg3);text-transform:uppercase;letter-spacing:.6px;border-bottom:1px solid var(--border);white-space:nowrap}
.dt td{padding:10px 14px;border-bottom:1px solid var(--border);color:var(--text);vertical-align:middle}
.dt tbody tr{transition:background .1s;cursor:pointer}
.dt tbody tr:hover td{background:rgba(255,255,255,.02)}
.dt tbody tr.selected td{background:rgba(194,0,27,.06)}
.dt tr:last-child td{border-bottom:none}
.dt .actions{display:flex;gap:6px;justify-content:flex-end}
.dt .cell-hostname{display:flex;align-items:center;gap:8px;font-weight:500}
.dt .icon-btn{width:26px;height:26px;border-radius:7px;background:transparent;border:none;color:var(--muted);cursor:pointer;display:inline-flex;align-items:center;justify-content:center;transition:all .1s}
.dt .icon-btn:hover{background:rgba(255,255,255,.06);color:var(--text)}
.dt .icon-btn.primary:hover{color:var(--accent-l)}
.dt .icon-btn:disabled{opacity:.25;cursor:default}
.dt .icon-btn:disabled:hover{background:transparent;color:var(--muted)}

/* ── Tag chips ── */
.tag-chip{display:inline-flex;align-items:center;gap:4px;padding:2px 8px;border-radius:12px;font-size:10px;font-weight:600;border:1px solid transparent;white-space:nowrap}
.tag-chip.win{background:rgba(96,165,250,.12);color:#60a5fa;border-color:rgba(96,165,250,.2)}
.tag-chip.linux{background:rgba(74,222,128,.12);color:#4ade80;border-color:rgba(74,222,128,.2)}
.tag-chip.mac{background:rgba(251,191,36,.12);color:#fbbf24;border-color:rgba(251,191,36,.2)}
.tag-chip.on{background:rgba(74,222,128,.12);color:#4ade80}
.tag-chip.off{background:rgba(255,255,255,.04);color:var(--muted2);border-color:rgba(255,255,255,.06)}
.tag-chip.warn{background:rgba(245,158,11,.12);color:#f59e0b}

/* ── Filter bar ── */
.filter-bar{display:flex;gap:10px;align-items:center;flex-wrap:wrap;padding:12px 16px;background:var(--bg2);border:1px solid var(--border);border-radius:14px;margin-bottom:14px}
.filter-bar input, .filter-bar select{background:var(--input-bg);border:1px solid var(--input-border);border-radius:10px;padding:6px 10px;font-size:12px;color:var(--text);outline:none;min-width:140px}
.filter-bar input:focus, .filter-bar select:focus{border-color:var(--accent)}
.filter-bar label{font-size:11px;color:var(--muted);margin-right:2px;white-space:nowrap}
.filter-bar .spacer{flex:1}

/* ── Recent list ── */
.recent-list{display:flex;flex-direction:column}
.recent-item{padding:10px 16px;border-bottom:1px solid var(--border);display:flex;align-items:center;gap:10px;font-size:12px;cursor:pointer;transition:background .1s}
.recent-item:hover{background:rgba(255,255,255,.02)}
.recent-item:last-child{border-bottom:none}
.recent-item .rname{font-weight:500;color:var(--text)}
.recent-item .rwhen{margin-left:auto;font-size:11px;color:var(--muted2)}
.recent-empty{padding:20px;text-align:center;color:var(--muted2);font-size:12px}

/* ── Settings sub-nav ── */
.settings-layout{display:flex;gap:18px;height:100%%}
.settings-nav{width:200px;flex-shrink:0;display:flex;flex-direction:column;gap:2px}
.settings-nav-item{padding:8px 12px;border-radius:8px;font-size:12px;color:var(--muted);cursor:pointer;transition:all .1s}
.settings-nav-item:hover{background:rgba(255,255,255,.04);color:var(--text)}
.settings-nav-item.active{background:rgba(194,0,27,.12);color:var(--accent-l);font-weight:500}
.settings-body{flex:1;min-width:0}

/* ── Settings form rows ── */
.setting-row{display:flex;align-items:center;gap:16px;padding:14px 0;border-bottom:1px solid var(--border)}
.setting-row:last-child{border-bottom:none}
.setting-row .setting-label{flex:1;min-width:0}
.setting-row .setting-label .title{font-size:13px;font-weight:500;color:var(--text)}
.setting-row .setting-label .sub{font-size:11px;color:var(--muted);margin-top:2px;line-height:1.5}
.setting-row .setting-control{flex-shrink:0}

/* ── Trusted IP row ── */
.tip-row{display:flex;align-items:center;gap:12px;padding:10px 14px;border:1px solid var(--border);border-radius:10px;margin-bottom:6px;background:var(--bg3)}
.tip-row .ipv{font-family:monospace;font-size:12px;color:var(--text);font-weight:500}
.tip-row .exp{font-size:11px;color:var(--muted)}
.tip-row .spacer{flex:1}
.tip-row button{font-size:11px}

/* ── Shared bits ── */
.hide{display:none !important}
.section-title{font-size:14px;font-weight:600;color:var(--text);margin-bottom:10px}
.muted-hint{font-size:11px;color:var(--muted)}

/* ── Update banner ── */
.update-banner{display:none;align-items:center;gap:12px;padding:8px 18px;background:linear-gradient(90deg,rgba(194,0,27,.15),rgba(194,0,27,.06));border-bottom:1px solid rgba(194,0,27,.3);font-size:12px;color:var(--text);flex-shrink:0}
.update-banner.visible{display:flex}
.update-banner .ub-icon{width:24px;height:24px;border-radius:6px;background:rgba(194,0,27,.18);color:var(--accent-l);display:flex;align-items:center;justify-content:center;flex-shrink:0}
.update-banner .ub-text{flex:1;line-height:1.4}
.update-banner .ub-text .ub-title{font-weight:600}
.update-banner .ub-text .ub-sub{color:var(--muted);font-size:11px}
.update-banner .btn-primary{padding:6px 14px;font-size:12px;border-radius:10px}
.update-banner .btn-ghost{padding:6px 12px;font-size:12px}
</style>
</head>
<body>

<!-- Login overlay -->
<div id="login-overlay">
  <div class="login-box">
    <div style="text-align:center;padding:4px 0 0" id="login-logo"></div>
    <p style="font-size:12px;color:var(--muted);text-align:center;margin-top:-4px">Connect to your Obliance server</p>
    <div class="form-group">
      <label>Server URL</label>
      <input id="inp-server" type="url" placeholder="https://obliance.example.com" value="%s"/>
    </div>
    <div id="sso-login-fields">
      <div class="err-msg" id="sso-err"></div>
      <button class="btn-primary" id="btn-sso" style="background:#534AB7;width:100%%">
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="vertical-align:-2px;margin-right:6px"><path d="M15 3h4a2 2 0 012 2v14a2 2 0 01-2 2h-4"/><polyline points="10 17 15 12 10 7"/><line x1="15" y1="12" x2="3" y2="12"/></svg>
        Sign in with Obligate SSO
      </button>
    </div>
    <div id="or-sep" style="display:flex;align-items:center;gap:10px;color:var(--muted2);font-size:10px;text-transform:uppercase;letter-spacing:2px;font-weight:600;margin:2px 0">
      <span style="flex:1;height:1px;background:var(--border2)"></span>
      <span>or local account</span>
      <span style="flex:1;height:1px;background:var(--border2)"></span>
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
      <label style="display:flex;align-items:center;gap:6px;font-size:11px;color:var(--muted);cursor:pointer;padding:2px 0">
        <input type="checkbox" id="inp-remember" checked style="accent-color:var(--accent);width:14px;height:14px;cursor:pointer" /> Remember me
      </label>
      <div class="err-msg" id="login-err"></div>
      <button class="btn-primary" id="btn-login">Connect</button>
    </div>
    <!-- btn-local-fallback kept as hidden stub for backwards compat with removed listener -->
    <span id="btn-local-fallback" style="display:none"></span>
  </div>
</div>

<!-- Main app -->
<div id="app" style="display:none;flex-direction:column;height:100%%">
  <div class="topbar">
    <span class="logo" id="topbar-logo"></span>
    <div class="tenant-switcher" id="tenant-switcher" style="display:none" title="Switch tenant">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 21h18M5 21V7l8-4v18M19 21V11l-6-4"/></svg>
      <span class="label" id="tenant-label">—</span>
      <svg class="chev" width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><polyline points="6 9 12 15 18 9"/></svg>
      <div class="tenant-menu" id="tenant-menu"></div>
    </div>
    <span class="topbar-right">
      <button class="btn-sm chat-toggle" id="btn-chat" style="display:none" title="Chat">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z"/></svg>
        Chat
        <span class="badge-dot" id="chat-badge"></span>
      </button>
      <button class="btn-sm" id="btn-refresh" title="Refresh">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M23 4v6h-6"/><path d="M1 20v-6h6"/><path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"/></svg>
      </button>
      <div class="user-menu-trigger" id="user-menu-trigger">
        <span class="avatar-sm" id="user-avatar-sm">?</span>
        <span class="uname" id="user-name-top">User</span>
        <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" style="opacity:.6"><polyline points="6 9 12 15 18 9"/></svg>
        <div class="user-menu" id="user-menu">
          <div class="user-menu-head">
            <div class="name" id="user-menu-name">—</div>
            <div class="sub" id="user-menu-sub">—</div>
          </div>
          <div class="user-menu-item" id="user-menu-settings">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 11-2.83 2.83l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09a1.65 1.65 0 00-1-1.51 1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 11-2.83-2.83l.06-.06a1.65 1.65 0 00.33-1.82 1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09a1.65 1.65 0 001.51-1 1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 112.83-2.83l.06.06a1.65 1.65 0 001.82.33H9a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 112.83 2.83l-.06.06a1.65 1.65 0 00-.33 1.82V9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z"/></svg>
            Settings
          </div>
          <div class="user-menu-item danger" id="user-menu-logout">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>
            Sign out
          </div>
        </div>
      </div>
    </span>
  </div>
  <div class="update-banner" id="update-banner">
    <div class="ub-icon">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
    </div>
    <div class="ub-text">
      <div class="ub-title">Update available — <span id="ub-version">v?</span></div>
      <div class="ub-sub" id="ub-sub">A newer version of Oblireach is ready to install.</div>
    </div>
    <button class="btn-ghost" id="ub-dismiss">Later</button>
    <button class="btn-primary" id="ub-download">Download update</button>
  </div>
  <div class="main">
    <div class="nav-rail" id="nav-rail"></div>
    <div class="page-frame">
      <div class="session-tabs" id="session-tabs"></div>
      <div class="page active" id="page-content"></div>
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
      <div class="chat-messages" id="chat-messages">
        <div class="chat-typing" id="chat-typing">
          <div style="width:28px;height:28px;border-radius:50%%;background:var(--bg3);display:flex;align-items:center;justify-content:center;flex-shrink:0;font-size:10px;font-weight:700;color:var(--muted)">...</div>
          <div class="chat-typing-dots"><span></span><span></span><span></span></div>
        </div>
      </div>
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

<!-- 2FA prompt modal -->
<div class="modal-overlay" id="tfa-overlay">
  <div class="modal-box">
    <div class="modal-head">
      <div class="icon">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>
      </div>
      <h3>Sensitive action</h3>
      <button class="close" id="tfa-cancel" title="Cancel">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
      </button>
    </div>
    <p class="desc"><span id="tfa-action">This action</span> is marked sensitive by your tenant admin. Enter your 6-digit authenticator code to continue.</p>
    <input id="tfa-code" class="otp-input" type="text" inputmode="numeric" placeholder="000000" autocomplete="one-time-code"/>
    <label class="trust-row">
      <input type="checkbox" id="tfa-trust"/>
      <div class="labels">
        <span class="title">Trust this IP for 24 hours</span>
        <span class="sub">Skip this prompt for sensitive actions from <span class="ip" id="tfa-ip">this IP</span>. Revocable any time from Settings.</span>
      </div>
    </label>
    <div class="modal-err" id="tfa-error"></div>
    <div class="modal-actions">
      <button class="btn-ghost" id="tfa-cancel2">Cancel</button>
      <button class="btn-primary" id="tfa-submit" style="padding:8px 18px;font-size:12px;border-radius:10px" disabled>Verify</button>
    </div>
  </div>
</div>

<script>
// ── State ────────────────────────────────────────────────────────────────────
let overview = { groups: [] };
let reachScripts = [];
let selectedDevice = null;
let activeTab = 'remote';
let execAbort = null;

// Remote state (active tab)
let remoteWs = null;
let remoteDecoder = null;
let remoteTs = 0;

// Chat state
let chatSocket = null;
let chatId = null;
let chatMessages = [];
let chatConnected = false;
let chatUserClosed = false;
let currentOperatorId = null;
let currentOperatorName = '';
let currentOperatorAvatar = '';
let currentTenantId = null;
let chatTypingTimer = null;
let lastChatTypingEmit = 0;

// Performance HUD state
let perfHudVisible = false;
let perfFrameCount = 0;
let perfByteCount = 0;
let perfLastTime = performance.now();
let perfFps = 0;
let perfBitrate = 0;
let perfCodec = 'H.264';
let perfInterval = null;

// Recording state
let recMediaRecorder = null;
let recChunks = [];
let recStartTime = 0;
let recTimerInterval = null;

// Annotation state
let annotationActive = false;
let annotationTool = 'pen';
let annotationColor = '#ef4444';
let annotationDrawing = false;
let annotationHistory = [];
let annotationCtx = null;

// Zoom state
let zoomLevel = 'fit';

// Favorites (localStorage)
let favorites = JSON.parse(localStorage.getItem('oblireach_favorites') || '[]');
let recents = JSON.parse(localStorage.getItem('oblireach_recents') || '[]');

// Multi-tab sessions
let sessionTabs = []; // [{id, device, ws, decoder, ts, label}]
let activeSessionTabId = null;

// ── Helpers ──────────────────────────────────────────────────────────────────
async function api(method, path, body) {
  return await _apiWithTfa(method, path, body, false);
}
async function _apiWithTfa(method, path, body, retried) {
  const opts = { method, headers: { 'Content-Type': 'application/json' } };
  if (body !== undefined) opts.body = JSON.stringify(body);
  const resp = await fetch('/proxy' + path, opts);
  if (resp.status !== 401 || retried) return resp;
  // Peek at body — use clone so the caller can still read it if we don't retry.
  let parsed = null;
  try { parsed = await resp.clone().json(); } catch { return resp; }
  if (!parsed?.twoFactorRequired) return resp;
  const tfa = await awaitTwoFactorCode(parsed.action || 'This action', parsed.currentIp || '');
  if (!tfa) return resp; // User cancelled — propagate the original 401.
  const merged = Object.assign({}, body || {}, { twoFactorCode: tfa.code });
  if (tfa.trustIp) merged.trustIp = true;
  return _apiWithTfa(method, path, merged, true);
}

// Prompt for a 6-digit TOTP code. Resolves to { code, trustIp } on submit, null on cancel.
// Mirrors D:/Obliance/client/src/components/common/TwoFactorPromptModal.tsx.
function awaitTwoFactorCode(actionLabel, currentIp) {
  return new Promise((resolve) => {
    const overlay = document.getElementById('tfa-overlay');
    const input = document.getElementById('tfa-code');
    const trust = document.getElementById('tfa-trust');
    const ipEl = document.getElementById('tfa-ip');
    const actionEl = document.getElementById('tfa-action');
    const errEl = document.getElementById('tfa-error');
    const submitBtn = document.getElementById('tfa-submit');
    const cancelBtn = document.getElementById('tfa-cancel');
    const cancelBtn2 = document.getElementById('tfa-cancel2');

    input.value = ''; trust.checked = false; errEl.textContent = '';
    ipEl.textContent = currentIp || 'this IP';
    actionEl.textContent = actionLabel;
    submitBtn.disabled = true;

    const done = (result) => {
      overlay.classList.remove('visible');
      submitBtn.onclick = null;
      cancelBtn.onclick = null;
      cancelBtn2.onclick = null;
      input.oninput = null;
      input.onkeydown = null;
      resolve(result);
    };
    const submit = () => { if (input.value.length === 6) done({ code: input.value, trustIp: trust.checked }); };
    const cancel = () => done(null);

    input.oninput = () => {
      input.value = input.value.replace(/\D/g, '').slice(0, 6);
      submitBtn.disabled = input.value.length !== 6;
    };
    input.onkeydown = (e) => {
      if (e.key === 'Enter') submit();
      else if (e.key === 'Escape') cancel();
    };
    submitBtn.onclick = submit;
    cancelBtn.onclick = cancel;
    cancelBtn2.onclick = cancel;
    overlay.classList.add('visible');
    setTimeout(() => input.focus(), 30);
  });
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

let ssoCheckTimer = null;
document.getElementById('inp-server').addEventListener('input', () => {
  clearTimeout(ssoCheckTimer);
  ssoCheckTimer = setTimeout(checkSso, 500);
});

// Probes the server for Obligate SSO support and adjusts UI visibility.
// The SSO button is shown by default (optimistic) so a user can always attempt
// SSO even if detection fails. We only HIDE the SSO button if the server
// explicitly returns obligateEnabled:false.
async function checkSso() {
  const server = document.getElementById('inp-server').value.trim().replace(/\/$/, '');
  if (!server) return;
  try {
    await fetch('/local/config', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ serverUrl: server }) });
  } catch {}
  const ssoEl = document.getElementById('sso-login-fields');
  const sepEl = document.getElementById('or-sep');
  try {
    const r = await fetch('/proxy/api/auth/sso-config');
    if (!r.ok) return; // leave SSO button visible — user can still try
    const d = await r.json();
    const sso = d.data || d;
    const explicitlyDisabled = sso && sso.obligateEnabled === false;
    const show = !explicitlyDisabled;
    ssoEl.style.display = show ? '' : 'none';
    sepEl.style.display = show ? 'flex' : 'none';
  } catch {
    // Network error reaching server — leave SSO button visible.
  }
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
    const remember = document.getElementById('inp-remember').checked;
    const r = await api('POST', '/api/auth/login', { username, password, remember });
    const data = await r.json();
    if (!r.ok || !data.success) { errEl.textContent = data.error || 'Login failed'; return; }
    const ts = document.getElementById('top-server'); if (ts) ts.textContent = server;
    await enterApp();
  } catch (err) { errEl.textContent = 'Connection failed: ' + err.message; }
  finally { btn.disabled = false; btn.textContent = 'Connect'; }
}

async function enterApp() {
  document.getElementById('login-overlay').style.display = 'none';
  document.getElementById('app').style.display = 'flex';

  // Get current user info — /api/auth/me returns { data: { user, permissions, currentTenantId } }
  try {
    const r = await api('GET', '/api/auth/me');
    if (r.ok) {
      const d = await r.json();
      const payload = d.data || d;
      const u = payload.user || payload;
      currentOperatorId = u.id ?? null;
      currentOperatorName = u.displayName || u.display_name || u.username || 'Operator';
      currentOperatorAvatar = u.profilePicture || u.profile_picture || u.avatar || '';
      currentTenantId = payload.currentTenantId ?? u.currentTenantId ?? null;
      // Chat header
      if (currentOperatorAvatar) {
        const hdrAvatar = document.querySelector('.chat-header .avatar');
        if (hdrAvatar) hdrAvatar.innerHTML = '<img src="' + currentOperatorAvatar + '" style="width:36px;height:36px;border-radius:50%%;object-fit:cover" />';
      }
      const hdrName = document.querySelector('.chat-header .info .name');
      if (hdrName) hdrName.textContent = currentOperatorName;
      // Top-bar user menu
      const initials = (currentOperatorName || '?').split(/\s+/).map(w => w[0]).join('').toUpperCase().slice(0, 2);
      const avSm = document.getElementById('user-avatar-sm');
      if (avSm) avSm.innerHTML = currentOperatorAvatar ? '<img src="' + currentOperatorAvatar + '"/>' : esc(initials);
      const nameTop = document.getElementById('user-name-top'); if (nameTop) nameTop.textContent = currentOperatorName;
      const nameMenu = document.getElementById('user-menu-name'); if (nameMenu) nameMenu.textContent = currentOperatorName;
      const subMenu = document.getElementById('user-menu-sub'); if (subMenu) subMenu.textContent = u.email || u.username || '';
    }
  } catch {}

  await loadOverview();
  await loadScripts();
  await loadTenants();
  buildNavRail();
  navigateTo('dashboard');
  initSocketIO();
  startUpdateChecker();
}

// ── Data loading ─────────────────────────────────────────────────────────────
async function loadOverview() {
  setStatus('Loading devices...');
  try {
    const r = await api('GET', '/api/reach/overview');
    const d = await r.json();
    overview = d.data || { groups: [] };
    if (typeof refreshDevicesTable === 'function') refreshDevicesTable();
    setStatus('Ready \u2014 ' + countDevices() + ' devices');
  } catch (err) { setStatus('Failed to load: ' + err.message); }
}
async function loadScripts() {
  try { const r = await api('GET', '/api/reach/scripts'); const d = await r.json(); reachScripts = d.data?.scripts || []; } catch {}
}
function countDevices() { return overview.groups.reduce((n, g) => n + g.devices.length, 0); }

// ── Favorites / Recents ──────────────────────────────────────────────────────
function toggleFavorite(deviceId) {
  const idx = favorites.indexOf(deviceId);
  if (idx >= 0) favorites.splice(idx, 1);
  else favorites.push(deviceId);
  localStorage.setItem('oblireach_favorites', JSON.stringify(favorites));
}
function addRecent(device) {
  recents = recents.filter(r => r.id !== device.id);
  recents.unshift({ id: device.id, hostname: device.hostname, ts: Date.now() });
  if (recents.length > 5) recents = recents.slice(0, 5);
  localStorage.setItem('oblireach_recents', JSON.stringify(recents));
}

// ── Router & nav rail ────────────────────────────────────────────────────────
let currentRoute = 'dashboard';
let currentRouteParams = null;

const NAV_TREE = [
  { group: 'Overview', items: [
    { route: 'dashboard',  label: 'Dashboard', icon: 'home' },
    { route: 'devices',    label: 'Devices',   icon: 'server' },
  ]},
  { group: 'Audit', items: [
    { route: 'logs',       label: 'Logs',      icon: 'file', sub: [
      { route: 'logs', sub: 'connection', label: 'Connection' },
      { route: 'logs', sub: 'alarm',      label: 'Alarm' },
      { route: 'logs', sub: 'console',    label: 'Console' },
      { route: 'logs', sub: 'script',     label: 'Script' },
    ]},
  ]},
  { group: 'Access', items: [
    { route: 'users',      label: 'Users',     icon: 'user'   },
    { route: 'groups',     label: 'Groups',    icon: 'users'  },
    { route: 'policies',   label: 'Policies',  icon: 'shield' },
  ]},
  { group: 'Config', items: [
    { route: 'deployment', label: 'Deployment', icon: 'download' },
    { route: 'settings',   label: 'Settings',   icon: 'cog' },
  ]},
];

const NAV_ICONS = {
  home:     '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 12l9-9 9 9M5 10v10h14V10"/></svg>',
  server:   '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="4" width="20" height="6" rx="2"/><rect x="2" y="14" width="20" height="6" rx="2"/><line x1="6" y1="7" x2="6.01" y2="7"/><line x1="6" y1="17" x2="6.01" y2="17"/></svg>',
  file:     '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>',
  user:     '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 21v-2a4 4 0 00-4-4H8a4 4 0 00-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>',
  users:    '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 00-3-3.87"/><path d="M16 3.13a4 4 0 010 7.75"/></svg>',
  shield:   '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>',
  download: '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>',
  cog:      '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 11-2.83 2.83l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09a1.65 1.65 0 00-1-1.51 1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 11-2.83-2.83l.06-.06a1.65 1.65 0 00.33-1.82 1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09a1.65 1.65 0 001.51-1 1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 112.83-2.83l.06.06a1.65 1.65 0 001.82.33H9a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 112.83 2.83l-.06.06a1.65 1.65 0 00-.33 1.82V9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z"/></svg>',
};

function buildNavRail() {
  const rail = document.getElementById('nav-rail');
  rail.innerHTML = '';
  for (const g of NAV_TREE) {
    const lbl = document.createElement('div'); lbl.className = 'nav-group-label'; lbl.textContent = g.group; rail.appendChild(lbl);
    for (const it of g.items) {
      const item = document.createElement('div');
      item.className = 'nav-item';
      item.dataset.route = it.route;
      item.innerHTML = (NAV_ICONS[it.icon] || '') + '<span>' + esc(it.label) + '</span>' +
        (it.sub ? '<svg class="chev" width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><polyline points="9 6 15 12 9 18"/></svg>' : '');
      if (it.sub) {
        const sub = document.createElement('div'); sub.className = 'nav-sub';
        for (const s of it.sub) {
          const si = document.createElement('div'); si.className = 'nav-item'; si.dataset.route = s.route; si.dataset.sub = s.sub;
          si.textContent = s.label;
          si.addEventListener('click', ev => { ev.stopPropagation(); navigateTo(s.route, { sub: s.sub }); });
          sub.appendChild(si);
        }
        item.addEventListener('click', () => {
          const isOpen = sub.classList.toggle('open');
          item.classList.toggle('expanded', isOpen);
          if (isOpen) navigateTo(it.route, { sub: it.sub[0].sub });
        });
        rail.appendChild(item); rail.appendChild(sub);
      } else {
        item.addEventListener('click', () => navigateTo(it.route));
        rail.appendChild(item);
      }
    }
  }
}

function navigateTo(route, params) {
  currentRoute = route; currentRouteParams = params || null;
  document.querySelectorAll('.tenant-menu.open,.user-menu.open').forEach(m => m.classList.remove('open'));
  document.querySelectorAll('#nav-rail .nav-item').forEach(el => {
    const r = el.dataset.route, s = el.dataset.sub;
    let active = false;
    if (r === route) {
      if (s) active = !!(params && params.sub === s);
      else if (!el.closest('.nav-sub')) active = true;
    }
    el.classList.toggle('active', active);
    if (r === route && s && active) {
      const parent = el.closest('.nav-sub'); if (parent) parent.classList.add('open');
      const head = parent ? parent.previousElementSibling : null;
      if (head) head.classList.add('expanded');
    }
  });
  const pc = document.getElementById('page-content');
  if (!pc) return;
  pc.innerHTML = '';
  switch (route) {
    case 'dashboard':     renderDashboardPage(pc); break;
    case 'devices':       renderDevicesPage(pc); break;
    case 'device-detail': renderDeviceDetail(pc, params && params.device); break;
    case 'logs':          renderLogsPage(pc, (params && params.sub) || 'connection'); break;
    case 'users':         renderStubPage(pc, 'Users', 'Manage operator accounts (coming soon).'); break;
    case 'groups':        renderStubPage(pc, 'Groups', 'User groups & cross-group ACL (coming soon).'); break;
    case 'policies':      renderStubPage(pc, 'Policies', 'Push bulk settings to endpoints (coming soon).'); break;
    case 'deployment':    renderStubPage(pc, 'Deployment', 'Download branded installers per tenant (coming soon).'); break;
    case 'settings':      renderSettingsPage(pc, (params && params.sub) || 'profile'); break;
    default:              renderDashboardPage(pc);
  }
}

function renderStubPage(pc, title, msg) {
  const h = document.createElement('div'); h.className = 'page-header';
  h.innerHTML = '<h1>' + esc(title) + '</h1>';
  const b = document.createElement('div'); b.className = 'page-body';
  b.innerHTML = '<div class="panel"><div class="panel-body"><div class="empty" style="padding:40px"><div class="empty-icon"><svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" style="opacity:.25"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg></div><span>' + esc(msg) + '</span></div></div></div>';
  pc.appendChild(h); pc.appendChild(b);
}

// ── Tenant switcher + user menu ──────────────────────────────────────────────
let tenants = [];
async function loadTenants() {
  try {
    const r = await api('GET', '/api/tenants');
    const d = await r.json();
    tenants = Array.isArray(d.data) ? d.data : (Array.isArray(d) ? d : []);
    renderTenantSwitcher();
  } catch {}
}

function renderTenantSwitcher() {
  const sw = document.getElementById('tenant-switcher');
  const lbl = document.getElementById('tenant-label');
  const menu = document.getElementById('tenant-menu');
  if (!tenants.length) { sw.style.display = 'none'; return; }
  sw.style.display = 'flex';
  const cur = tenants.find(t => t.id === currentTenantId) || tenants[0];
  lbl.textContent = cur?.name || ('Tenant #' + (currentTenantId || '?'));
  menu.innerHTML = '';
  for (const t of tenants) {
    const mi = document.createElement('div');
    mi.className = 'tenant-menu-item' + (t.id === currentTenantId ? ' active' : '');
    mi.innerHTML = '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" style="' + (t.id === currentTenantId ? '' : 'visibility:hidden') + '"><polyline points="20 6 9 17 4 12"/></svg><span>' + esc(t.name) + '</span>';
    mi.addEventListener('click', e => { e.stopPropagation(); switchTenant(t.id); });
    menu.appendChild(mi);
  }
}

async function switchTenant(tenantId) {
  try {
    const r = await api('POST', '/api/tenant/switch', { tenantId });
    if (!r.ok) return;
    currentTenantId = tenantId;
    document.getElementById('tenant-menu').classList.remove('open');
    renderTenantSwitcher();
    await loadOverview();
    if (currentRoute === 'dashboard' || currentRoute === 'devices') navigateTo(currentRoute, currentRouteParams);
  } catch {}
}

document.getElementById('tenant-switcher').addEventListener('click', e => {
  if (e.target.closest('.tenant-menu')) return;
  document.getElementById('user-menu').classList.remove('open');
  document.getElementById('tenant-menu').classList.toggle('open');
});
document.getElementById('user-menu-trigger').addEventListener('click', e => {
  if (e.target.closest('.user-menu')) return;
  document.getElementById('tenant-menu').classList.remove('open');
  document.getElementById('user-menu').classList.toggle('open');
});
document.addEventListener('click', e => {
  if (!e.target.closest('#tenant-switcher')) document.getElementById('tenant-menu')?.classList.remove('open');
  if (!e.target.closest('#user-menu-trigger')) document.getElementById('user-menu')?.classList.remove('open');
});
document.getElementById('user-menu-settings').addEventListener('click', () => navigateTo('settings'));

// ── Update checker ───────────────────────────────────────────────────────────
let updateInfo = null;       // latest version info from /api/oblireach-desktop/version
let updateCheckInterval = null;

function cmpSemver(a, b) {
  const pa = String(a || '0').split('.').map(n => parseInt(n, 10) || 0);
  const pb = String(b || '0').split('.').map(n => parseInt(n, 10) || 0);
  for (let i = 0; i < 3; i++) {
    const x = pa[i] || 0, y = pb[i] || 0;
    if (x !== y) return x > y ? 1 : -1;
  }
  return 0;
}

async function checkForUpdate() {
  try {
    const r = await fetch('/proxy/api/oblireach-desktop/version');
    if (!r.ok) return;
    const d = await r.json();
    const info = d.data || d;
    if (!info || !info.version) return;
    updateInfo = info;
    const cur = window.__reach_version || '0.0.0';
    if (cmpSemver(info.version, cur) <= 0) { hideUpdateBanner(); return; }
    // Respect per-version dismissal so the banner doesn't pester the user every hour.
    const dismissed = localStorage.getItem('oblireach_dismissed_update');
    if (dismissed === info.version) return;
    showUpdateBanner(info);
  } catch {}
}

function showUpdateBanner(info) {
  const el = document.getElementById('update-banner');
  if (!el) return;
  document.getElementById('ub-version').textContent = 'v' + info.version;
  const sub = document.getElementById('ub-sub');
  if (info.releaseNotes) {
    const oneLine = String(info.releaseNotes).split(/\r?\n/).find(l => l.trim()) || '';
    sub.textContent = oneLine.slice(0, 120);
  } else {
    sub.textContent = 'A newer version of Oblireach is ready to install.';
  }
  el.classList.add('visible');
}

function hideUpdateBanner() {
  document.getElementById('update-banner')?.classList.remove('visible');
}

document.getElementById('ub-dismiss').addEventListener('click', () => {
  if (updateInfo?.version) localStorage.setItem('oblireach_dismissed_update', updateInfo.version);
  hideUpdateBanner();
});

document.getElementById('ub-download').addEventListener('click', async () => {
  if (!updateInfo?.downloadUrl) return;
  // Resolve relative downloadUrl against configured server, then hand off to the OS browser.
  let abs = updateInfo.downloadUrl;
  if (abs.startsWith('/')) {
    try {
      const cfgR = await fetch('/local/config');
      const cfg = await cfgR.json();
      if (cfg.serverUrl) abs = cfg.serverUrl.replace(/\/$/, '') + abs;
    } catch {}
  }
  try {
    await fetch('/local/open-url?url=' + encodeURIComponent(abs));
  } catch {}
});

function startUpdateChecker() {
  if (updateCheckInterval) return;
  checkForUpdate(); // immediate check on launch
  updateCheckInterval = setInterval(checkForUpdate, 60 * 60 * 1000); // hourly
}

// ── Dashboard page ───────────────────────────────────────────────────────────
function renderDashboardPage(pc) {
  const h = document.createElement('div'); h.className = 'page-header';
  h.innerHTML = '<h1>Dashboard</h1><div class="breadcrumb">Welcome back</div>';
  pc.appendChild(h);
  const body = document.createElement('div'); body.className = 'page-body';

  let online = 0, offline = 0, notInstalled = 0, updates = 0;
  const allDevs = [];
  for (const g of overview.groups) for (const d of g.devices) {
    allDevs.push(d);
    if (!d.oblireach?.installed) notInstalled++;
    else if (d.oblireach?.online) online++;
    else offline++;
    if (d.oblireach?.updateAvailable) updates++;
  }

  const kpi = document.createElement('div'); kpi.className = 'kpi-grid';
  kpi.innerHTML =
    '<div class="kpi-card online"><div class="label">Agents online</div><div class="value">' + online + '</div><div class="sub">Ready for remote</div></div>' +
    '<div class="kpi-card offline"><div class="label">Offline</div><div class="value">' + offline + '</div><div class="sub">Installed but unreachable</div></div>' +
    '<div class="kpi-card warn"><div class="label">Updates pending</div><div class="value">' + updates + '</div><div class="sub">Newer agent available</div></div>' +
    '<div class="kpi-card"><div class="label">Total devices</div><div class="value">' + allDevs.length + '</div><div class="sub">' + notInstalled + ' without Reach agent</div></div>';
  body.appendChild(kpi);

  const recPanel = document.createElement('div'); recPanel.className = 'panel';
  recPanel.innerHTML =
    '<div class="panel-header">Recent connections<div class="panel-actions"><button class="btn-sm" id="clear-recents-btn">Clear</button></div></div>' +
    '<div class="panel-body flush"><div class="recent-list" id="recent-list"></div></div>';
  body.appendChild(recPanel);

  const favPanel = document.createElement('div'); favPanel.className = 'panel';
  favPanel.innerHTML =
    '<div class="panel-header">Favorites</div>' +
    '<div class="panel-body flush"><div class="recent-list" id="fav-list"></div></div>';
  body.appendChild(favPanel);

  pc.appendChild(body);

  const recList = body.querySelector('#recent-list');
  const recentDevs = recents.map(r => allDevs.find(d => d.id === r.id)).filter(Boolean);
  if (recentDevs.length === 0) {
    recList.innerHTML = '<div class="recent-empty">No recent sessions yet. Connect to a device to see it here.</div>';
  } else {
    for (const dev of recentDevs.slice(0, 8)) {
      const r = recents.find(x => x.id === dev.id);
      const row = document.createElement('div'); row.className = 'recent-item';
      const dc = dev.oblireach.online ? 'online' : 'warn';
      row.innerHTML = '<span class="dot ' + dc + '"></span><span class="rname">' + esc(dev.hostname) + '</span>' +
        '<span class="tag-chip ' + osTag(dev.osType) + '">' + esc(dev.osType || '?') + '</span>' +
        '<span class="rwhen">' + relTime(r?.ts || 0) + '</span>';
      row.addEventListener('click', () => openDeviceDetail(dev));
      recList.appendChild(row);
    }
  }
  body.querySelector('#clear-recents-btn').addEventListener('click', () => {
    recents = []; localStorage.setItem('oblireach_recents', '[]');
    renderDashboardPage(pc);
  });

  const favList = body.querySelector('#fav-list');
  const favDevs = allDevs.filter(d => favorites.includes(d.id));
  if (favDevs.length === 0) {
    favList.innerHTML = '<div class="recent-empty">No favorites yet. Star a device from the Devices page.</div>';
  } else {
    for (const dev of favDevs) {
      const row = document.createElement('div'); row.className = 'recent-item';
      const dc = dev.oblireach.online ? 'online' : 'warn';
      row.innerHTML = '<span class="dot ' + dc + '"></span><span class="rname">' + esc(dev.hostname) + '</span>' +
        '<span class="tag-chip ' + osTag(dev.osType) + '">' + esc(dev.osType || '?') + '</span>' +
        '<span class="rwhen"><span class="tag-chip ' + (dev.oblireach.online ? 'on' : 'off') + '">' + (dev.oblireach.online ? 'online' : 'offline') + '</span></span>';
      row.addEventListener('click', () => openDeviceDetail(dev));
      favList.appendChild(row);
    }
  }
}

function relTime(ts) {
  if (!ts) return '';
  const diff = Date.now() - ts;
  const m = Math.round(diff / 60000);
  if (m < 1) return 'just now';
  if (m < 60) return m + 'm ago';
  const h = Math.round(m / 60);
  if (h < 24) return h + 'h ago';
  return Math.round(h / 24) + 'd ago';
}

function osTag(osType) {
  const s = String(osType || '').toLowerCase();
  if (s.includes('windows') || s.includes('win')) return 'win';
  if (s.includes('linux')) return 'linux';
  if (s.includes('mac') || s.includes('darwin')) return 'mac';
  return 'off';
}

// ── Devices page (Rustdesk-style table) ──────────────────────────────────────
let devicesFilter = { q: '', online: '', os: '' };
function renderDevicesPage(pc) {
  const h = document.createElement('div'); h.className = 'page-header';
  h.innerHTML = '<h1>Devices</h1><div class="breadcrumb">Inventory</div>' +
    '<div class="page-actions"><button class="btn-sm" id="devices-refresh">Refresh</button></div>';
  pc.appendChild(h);
  const body = document.createElement('div'); body.className = 'page-body';
  const bar = document.createElement('div'); bar.className = 'filter-bar';
  bar.innerHTML =
    '<input id="dq" placeholder="Search hostname..." value="' + esc(devicesFilter.q) + '"/>' +
    '<select id="dq-online"><option value="">All states</option><option value="online">Online</option><option value="offline">Offline</option></select>' +
    '<select id="dq-os"><option value="">All OS</option><option value="windows">Windows</option><option value="linux">Linux</option><option value="macos">macOS</option></select>' +
    '<span class="spacer"></span>' +
    '<span class="muted-hint"><span id="dev-count">0</span> results</span>';
  body.appendChild(bar);
  const panel = document.createElement('div'); panel.className = 'panel';
  panel.innerHTML =
    '<div class="panel-body flush" style="overflow-x:auto"><table class="dt" id="devices-table">' +
    '<thead><tr><th style="width:40px"></th><th>Hostname</th><th>OS</th><th>Agent</th><th>Status</th><th>Sessions</th><th style="text-align:right">Actions</th></tr></thead>' +
    '<tbody id="devices-tbody"></tbody></table></div>';
  body.appendChild(panel);
  pc.appendChild(body);

  bar.querySelector('#dq').addEventListener('input', e => { devicesFilter.q = e.target.value; refreshDevicesTable(); });
  bar.querySelector('#dq-online').value = devicesFilter.online;
  bar.querySelector('#dq-online').addEventListener('change', e => { devicesFilter.online = e.target.value; refreshDevicesTable(); });
  bar.querySelector('#dq-os').value = devicesFilter.os;
  bar.querySelector('#dq-os').addEventListener('change', e => { devicesFilter.os = e.target.value; refreshDevicesTable(); });
  document.getElementById('devices-refresh').addEventListener('click', async () => { await loadOverview(); refreshDevicesTable(); });

  refreshDevicesTable();
}

function refreshDevicesTable() {
  const tbody = document.getElementById('devices-tbody');
  if (!tbody) return;
  tbody.innerHTML = '';
  const q = (devicesFilter.q || '').toLowerCase();
  const allDevs = [];
  for (const g of overview.groups) for (const d of g.devices) allDevs.push(d);
  const filtered = allDevs.filter(d => {
    if (q && !d.hostname.toLowerCase().includes(q)) return false;
    if (devicesFilter.online === 'online' && !d.oblireach?.online) return false;
    if (devicesFilter.online === 'offline' && d.oblireach?.online) return false;
    if (devicesFilter.os) {
      const s = String(d.osType || '').toLowerCase();
      if (devicesFilter.os === 'windows' && !(s.includes('windows') || s.includes('win'))) return false;
      if (devicesFilter.os === 'linux' && !s.includes('linux')) return false;
      if (devicesFilter.os === 'macos' && !(s.includes('mac') || s.includes('darwin'))) return false;
    }
    return true;
  });
  const cnt = document.getElementById('dev-count'); if (cnt) cnt.textContent = filtered.length;
  for (const dev of filtered) {
    const tr = document.createElement('tr');
    const isFav = favorites.includes(dev.id);
    const dc = dev.oblireach.online ? 'online' : dev.oblireach.installed ? 'warn' : 'offline';
    const sessions = dev.oblireach?.sessions?.length || 0;
    const starChar = isFav ? '★' : '☆';
    tr.innerHTML =
      '<td><span class="fav-star ' + (isFav ? 'active' : '') + '" title="Toggle favorite">' + starChar + '</span></td>' +
      '<td><div class="cell-hostname"><span class="dot ' + dc + '"></span>' + esc(dev.hostname) + (dev.oblireach?.updateAvailable ? ' <span class="tag-chip warn" style="margin-left:6px">UPDATE</span>' : '') + '</div></td>' +
      '<td><span class="tag-chip ' + osTag(dev.osType) + '">' + esc(dev.osType || '?') + '</span></td>' +
      '<td>' + (dev.oblireach?.version ? 'v' + esc(dev.oblireach.version) : '<span class="muted-hint">-</span>') + '</td>' +
      '<td><span class="tag-chip ' + (dev.oblireach?.online ? 'on' : 'off') + '">' + (dev.oblireach?.installed ? (dev.oblireach.online ? 'online' : 'offline') : 'not installed') + '</span></td>' +
      '<td>' + sessions + '</td>' +
      '<td class="actions">' +
      '<button class="icon-btn primary" title="Remote" data-act="remote" ' + (dev.oblireach?.online ? '' : 'disabled') + '><svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="6 3 21 12 6 21 6 3"/></svg></button>' +
      '<button class="icon-btn" title="Chat" data-act="chat" ' + (dev.oblireach?.online ? '' : 'disabled') + '><svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z"/></svg></button>' +
      '<button class="icon-btn" title="Info" data-act="info"><svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg></button>' +
      '</td>';
    tr.querySelector('.fav-star').addEventListener('click', e => { e.stopPropagation(); toggleFavorite(dev.id); refreshDevicesTable(); });
    tr.addEventListener('click', e => {
      const btn = e.target.closest('button[data-act]');
      if (btn) {
        e.stopPropagation();
        if (btn.disabled) return;
        const act = btn.dataset.act;
        if (act === 'remote') openDeviceDetail(dev, 'remote');
        else if (act === 'chat') { openDeviceDetail(dev); setTimeout(() => toggleChat(true), 120); }
        else if (act === 'info') openDeviceDetail(dev, 'info');
        return;
      }
      openDeviceDetail(dev);
    });
    tbody.appendChild(tr);
  }
  if (filtered.length === 0) {
    tbody.innerHTML = '<tr><td colspan="7" style="text-align:center;padding:30px;color:var(--muted2)">No devices match your filters.</td></tr>';
  }
}

function openDeviceDetail(dev, tab) {
  if (tab) activeTab = tab;
  navigateTo('device-detail', { device: dev });
}

// ── Device detail page (wraps existing Remote/Scripts/Info tabs) ─────────────
function renderDeviceDetail(pc, dev) {
  if (!dev) { renderDevicesPage(pc); return; }
  selectedDevice = dev; addRecent(dev);
  pc.innerHTML = '';
  selectDeviceInto(dev, pc);
}

// ── Logs pages ───────────────────────────────────────────────────────────────
function renderLogsPage(pc, sub) {
  const titles = { connection: 'Connection log', alarm: 'Alarm log', console: 'Console log', script: 'Script log' };
  const hints = {
    connection: 'History of remote sessions established on this tenant.',
    alarm:      'Security events — failed logins, blocked actions, unusual activity.',
    console:    'Audit trail of admin actions taken in the console.',
    script:     'Script executions on managed devices.',
  };
  const h = document.createElement('div'); h.className = 'page-header';
  h.innerHTML = '<h1>Logs</h1><div class="breadcrumb">' + esc(titles[sub] || sub) + '</div>';
  pc.appendChild(h);
  const body = document.createElement('div'); body.className = 'page-body';
  const panel = document.createElement('div'); panel.className = 'panel';
  panel.innerHTML = '<div class="panel-header">' + esc(titles[sub] || sub) + '<span class="muted-hint" style="margin-left:10px;font-weight:400">' + esc(hints[sub] || '') + '</span></div>' +
    '<div class="panel-body flush" style="overflow-x:auto"><table class="dt"><thead id="logs-thead"></thead><tbody id="logs-tbody"></tbody></table></div>';
  body.appendChild(panel);
  pc.appendChild(body);
  loadLogs(sub);
}

async function loadLogs(sub) {
  const thead = document.getElementById('logs-thead');
  const tbody = document.getElementById('logs-tbody');
  if (!thead || !tbody) return;
  tbody.innerHTML = '<tr><td colspan="6" style="text-align:center;padding:20px;color:var(--muted2)">Loading...</td></tr>';
  try {
    if (sub === 'connection') {
      thead.innerHTML = '<tr><th>Started</th><th>Device</th><th>Protocol</th><th>Operator</th><th>Status</th><th>Duration</th></tr>';
      const r = await api('GET', '/api/remote/sessions');
      const d = await r.json();
      const items = d?.data?.items || [];
      tbody.innerHTML = '';
      if (!items.length) { tbody.innerHTML = '<tr><td colspan="6" style="text-align:center;padding:20px;color:var(--muted2)">No sessions yet.</td></tr>'; return; }
      for (const s of items.slice(0, 100)) {
        const started = s.started_at ? new Date(s.started_at).toLocaleString() : '-';
        const ended = s.ended_at ? new Date(s.ended_at).getTime() : Date.now();
        const dur = s.started_at ? Math.round((ended - new Date(s.started_at).getTime()) / 1000) + 's' : '-';
        tbody.innerHTML += '<tr><td>' + esc(started) + '</td><td>' + esc(String(s.device_id || '-')) + '</td><td>' + esc(s.protocol || '-') + '</td><td>' + esc(String(s.started_by || '-')) + '</td><td><span class="tag-chip ' + (s.status === 'active' ? 'on' : 'off') + '">' + esc(s.status || '-') + '</span></td><td>' + dur + '</td></tr>';
      }
    } else if (sub === 'console' || sub === 'alarm') {
      thead.innerHTML = '<tr><th>Time</th><th>Actor</th><th>Action</th><th>Resource</th><th>IP</th><th>Details</th></tr>';
      const r = await api('GET', '/api/audit?limit=100' + (sub === 'alarm' ? '&category=security' : ''));
      const d = await r.json();
      const items = d?.data?.items || d?.items || [];
      tbody.innerHTML = '';
      if (!items.length) { tbody.innerHTML = '<tr><td colspan="6" style="text-align:center;padding:20px;color:var(--muted2)">No entries.</td></tr>'; return; }
      for (const e of items) {
        const ts = e.created_at || e.timestamp || '';
        tbody.innerHTML += '<tr><td>' + esc(ts ? new Date(ts).toLocaleString() : '-') + '</td><td>' + esc(e.username || e.user_id || '-') + '</td><td>' + esc(e.action || '-') + '</td><td>' + esc(e.resource_type || '-') + (e.resource_path ? ' / ' + esc(e.resource_path) : '') + '</td><td>' + esc(e.ip_address || '-') + '</td><td class="muted-hint" style="max-width:260px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">' + esc(JSON.stringify(e.details || {})) + '</td></tr>';
      }
    } else if (sub === 'script') {
      thead.innerHTML = '<tr><th>Time</th><th>Script</th><th>Device</th><th>Status</th><th>Exit</th><th>Duration</th></tr>';
      const r = await api('GET', '/api/executions?limit=100');
      const d = await r.json();
      const items = d?.data?.items || d?.items || [];
      tbody.innerHTML = '';
      if (!items.length) { tbody.innerHTML = '<tr><td colspan="6" style="text-align:center;padding:20px;color:var(--muted2)">No script runs yet.</td></tr>'; return; }
      for (const x of items) {
        const ts = x.created_at || x.started_at || '';
        tbody.innerHTML += '<tr><td>' + esc(ts ? new Date(ts).toLocaleString() : '-') + '</td><td>' + esc(x.script_name || x.script_id || '-') + '</td><td>' + esc(String(x.device_id || '-')) + '</td><td><span class="tag-chip ' + (x.status === 'success' ? 'on' : 'off') + '">' + esc(x.status || '-') + '</span></td><td>' + (x.exitCode ?? '-') + '</td><td>' + (x.duration_ms ? (x.duration_ms / 1000).toFixed(1) + 's' : '-') + '</td></tr>';
      }
    }
  } catch (err) {
    tbody.innerHTML = '<tr><td colspan="6" style="text-align:center;padding:20px;color:var(--danger)">Failed to load: ' + esc(err.message || String(err)) + '</td></tr>';
  }
}

// ── Settings page ────────────────────────────────────────────────────────────
function renderSettingsPage(pc, sub) {
  const h = document.createElement('div'); h.className = 'page-header';
  h.innerHTML = '<h1>Settings</h1>';
  pc.appendChild(h);
  const body = document.createElement('div'); body.className = 'page-body';
  body.innerHTML = '<div class="settings-layout">' +
    '<div class="settings-nav">' +
    '<div class="settings-nav-item ' + (sub === 'profile' ? 'active' : '') + '" data-sub="profile">Profile</div>' +
    '<div class="settings-nav-item ' + (sub === 'trusted-ips' ? 'active' : '') + '" data-sub="trusted-ips">Trusted IPs</div>' +
    '<div class="settings-nav-item ' + (sub === 'server' ? 'active' : '') + '" data-sub="server">Server</div>' +
    '<div class="settings-nav-item ' + (sub === 'about' ? 'active' : '') + '" data-sub="about">About</div>' +
    '</div>' +
    '<div class="settings-body" id="settings-body"></div></div>';
  pc.appendChild(body);
  body.querySelectorAll('.settings-nav-item').forEach(el => {
    el.addEventListener('click', () => navigateTo('settings', { sub: el.dataset.sub }));
  });
  const sb = body.querySelector('#settings-body');
  if (sub === 'trusted-ips') renderTrustedIpsSection(sb);
  else if (sub === 'server') renderServerSection(sb);
  else if (sub === 'about') renderAboutSection(sb);
  else renderProfileSection(sb);
}

function renderProfileSection(sb) {
  const panel = document.createElement('div'); panel.className = 'panel';
  panel.innerHTML = '<div class="panel-header">Profile</div><div class="panel-body" id="profile-body"></div>';
  sb.appendChild(panel);
  const b = panel.querySelector('#profile-body');
  b.innerHTML =
    '<div class="setting-row"><div class="setting-label"><div class="title">' + esc(currentOperatorName || 'Operator') + '</div><div class="sub" id="prof-email">-</div></div>' +
    '<div class="setting-control"><span id="prof-avatar" style="display:inline-block;width:44px;height:44px;border-radius:50%%;background:var(--accent);text-align:center;line-height:44px;color:white;font-weight:700">' + esc((currentOperatorName || '?').slice(0,2).toUpperCase()) + '</span></div></div>' +
    '<div class="setting-row"><div class="setting-label"><div class="title">Display name</div><div class="sub" id="prof-display">-</div></div></div>' +
    '<div class="setting-row"><div class="setting-label"><div class="title">Role</div><div class="sub" id="prof-role">-</div></div></div>' +
    '<div class="setting-row"><div class="setting-label"><div class="title">Authentication</div><div class="sub" id="prof-auth">-</div></div><div class="setting-control" id="prof-auth-action"></div></div>';
  // Populate from /api/auth/me
  api('GET', '/api/auth/me').then(r => r.json()).then(d => {
    const u = (d?.data?.user) || d?.data || {};
    if (u.profilePicture || u.profile_picture || u.avatar) {
      b.querySelector('#prof-avatar').innerHTML = '<img src="' + (u.profilePicture || u.profile_picture || u.avatar) + '" style="width:44px;height:44px;border-radius:50%%;object-fit:cover"/>';
    }
    b.querySelector('#prof-email').textContent = u.email || '-';
    b.querySelector('#prof-display').textContent = u.displayName || u.display_name || u.username || '-';
    b.querySelector('#prof-role').textContent = u.role || '-';
    const isObligate = (u.foreignSource || u.foreign_source) === 'obligate';
    b.querySelector('#prof-auth').textContent = isObligate ? 'Obligate SSO — manage TOTP / password from your Obligate profile.' : 'Local account';
  }).catch(() => {});
}

function renderTrustedIpsSection(sb) {
  const panel = document.createElement('div'); panel.className = 'panel';
  panel.innerHTML = '<div class="panel-header">Trusted IPs (2FA)<div class="panel-actions"><button class="btn-sm" id="revoke-all-tip">Revoke all</button></div></div>' +
    '<div class="panel-body">' +
    '<p class="muted-hint" style="margin-bottom:12px">IPs where you ticked "Trust this IP for 24h" when you last entered a 2FA code. Sensitive actions from these IPs skip the prompt until expiry.</p>' +
    '<div id="tip-list"></div></div>';
  sb.appendChild(panel);
  const list = panel.querySelector('#tip-list');
  list.innerHTML = '<div class="muted-hint" style="padding:14px">Loading...</div>';
  api('GET', '/api/profile/trusted-ips').then(r => r.json()).then(d => {
    const items = d?.data || d || [];
    if (!Array.isArray(items) || items.length === 0) {
      list.innerHTML = '<div class="muted-hint" style="padding:14px">No trusted IPs yet.</div>';
      return;
    }
    list.innerHTML = '';
    for (const it of items) {
      const row = document.createElement('div'); row.className = 'tip-row';
      const trustedUntil = it.trustedUntil || it.trusted_until;
      const remaining = trustedUntil ? Math.max(0, new Date(trustedUntil).getTime() - Date.now()) : 0;
      const mins = Math.floor(remaining / 60000);
      const exp = mins > 60 ? Math.round(mins / 60) + 'h ' + (mins %% 60) + 'm' : mins + 'm';
      row.innerHTML = '<span class="ipv">' + esc(it.ip) + '</span><span class="exp">expires in ' + esc(exp) + '</span><span class="spacer"></span>' +
        '<button class="btn-ghost">Revoke</button>';
      row.querySelector('button').addEventListener('click', async () => {
        await api('DELETE', '/api/profile/trusted-ips/' + encodeURIComponent(it.ip));
        renderTrustedIpsSection(sb);
      });
      list.appendChild(row);
    }
  }).catch(() => {
    list.innerHTML = '<div class="muted-hint" style="padding:14px">Not available (endpoint not found).</div>';
  });
  panel.querySelector('#revoke-all-tip').addEventListener('click', async () => {
    if (!confirm('Revoke all trusted IPs?')) return;
    await api('DELETE', '/api/profile/trusted-ips');
    renderTrustedIpsSection(sb);
  });
}

function renderServerSection(sb) {
  const panel = document.createElement('div'); panel.className = 'panel';
  panel.innerHTML = '<div class="panel-header">Server connection</div><div class="panel-body">' +
    '<div class="setting-row"><div class="setting-label"><div class="title">Obliance URL</div><div class="sub" id="srv-url">-</div></div>' +
    '<div class="setting-control"><button class="btn-ghost" id="btn-change-srv">Sign out & change</button></div></div>' +
    '<div class="setting-row"><div class="setting-label"><div class="title">Connection</div><div class="sub">Reload overview, reconnect socket.</div></div>' +
    '<div class="setting-control"><button class="btn-ghost" id="btn-reload-srv">Reload</button></div></div>' +
    '</div>';
  sb.appendChild(panel);
  fetch('/local/config').then(r => r.json()).then(c => { panel.querySelector('#srv-url').textContent = c.serverUrl || '-'; }).catch(() => {});
  panel.querySelector('#btn-change-srv').addEventListener('click', () => document.getElementById('user-menu-logout').click());
  panel.querySelector('#btn-reload-srv').addEventListener('click', async () => { await loadOverview(); await loadTenants(); setStatus('Reloaded'); });
}

function renderAboutSection(sb) {
  const panel = document.createElement('div'); panel.className = 'panel';
  const ver = window.__reach_version || '-';
  panel.innerHTML = '<div class="panel-header">About Oblireach</div><div class="panel-body">' +
    '<div class="setting-row"><div class="setting-label"><div class="title">Client version</div><div class="sub">v' + esc(ver) + '</div></div></div>' +
    '<div class="setting-row"><div class="setting-label"><div class="title">Desktop remote support for the Obliance RMM.</div><div class="sub">Built with WebView2. Charcoal UI with the Reach red accent.</div></div></div>' +
    '</div>';
  sb.appendChild(panel);
}

// ── Multi-tab sessions ───────────────────────────────────────────────────────
function renderSessionTabs() {
  const bar = document.getElementById('session-tabs');
  bar.innerHTML = '';
  for (const tab of sessionTabs) {
    const el = document.createElement('div');
    el.className = 'session-tab' + (tab.id === activeSessionTabId ? ' active' : '');
    el.innerHTML = '<span class="dot ' + (tab.ws?.readyState === WebSocket.OPEN ? 'online' : 'offline') + '" style="width:6px;height:6px"></span>' +
      '<span>' + esc(tab.label) + '</span>' +
      '<span class="tab-close" title="Close">&times;</span>';
    el.querySelector('.tab-close').addEventListener('click', e => { e.stopPropagation(); closeSessionTab(tab.id); });
    el.addEventListener('click', () => switchSessionTab(tab.id));
    bar.appendChild(el);
  }
}

function switchSessionTab(tabId) {
  activeSessionTabId = tabId;
  const tab = sessionTabs.find(t => t.id === tabId);
  if (!tab) return;
  // Restore device context and re-render
  selectedDevice = tab.device;
  remoteWs = tab.ws;
  remoteDecoder = tab.decoder;
  remoteTs = tab.ts;
  renderSessionTabs();
  // Re-render content for this device
  openDeviceDetail(tab.device);
}

function closeSessionTab(tabId) {
  const idx = sessionTabs.findIndex(t => t.id === tabId);
  if (idx < 0) return;
  const tab = sessionTabs[idx];
  if (tab.ws) { try { tab.ws.close(); } catch {} }
  if (tab.decoder) { try { tab.decoder.close(); } catch {} }
  sessionTabs.splice(idx, 1);
  if (activeSessionTabId === tabId) {
    if (sessionTabs.length > 0) {
      switchSessionTab(sessionTabs[Math.min(idx, sessionTabs.length - 1)].id);
    } else {
      activeSessionTabId = null;
      remoteWs = null; remoteDecoder = null;
    }
  }
  renderSessionTabs();
}

function addSessionTab(device) {
  // Check if tab already exists for this device
  const existing = sessionTabs.find(t => t.device.id === device.id);
  if (existing) { switchSessionTab(existing.id); return existing; }
  const tab = { id: 'tab_' + Date.now(), device, ws: null, decoder: null, ts: 0, label: device.hostname };
  sessionTabs.push(tab);
  activeSessionTabId = tab.id;
  renderSessionTabs();
  return tab;
}

// ── Device detail ────────────────────────────────────────────────────────────
function selectDeviceInto(dev, area) {
  // Close previous chat if switching device
  if (selectedDevice && selectedDevice.id !== dev.id) closeChat();
  selectedDevice = dev;

  // Show/hide chat button
  const chatBtn = document.getElementById('btn-chat');
  chatBtn.style.display = dev.oblireach?.online ? '' : 'none';

  area.style.display = 'flex';
  area.style.flexDirection = 'column';
  area.style.overflow = 'hidden';

  // Header
  const hdr = document.createElement('div');
  hdr.className = 'device-header';
  const dc = dev.oblireach.online ? 'online' : dev.oblireach.installed ? 'warn' : 'offline';
  hdr.innerHTML = '<button class="btn-sm" id="_back-to-devices" style="padding:5px 8px" title="Back to Devices"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="15 18 9 12 15 6"/></svg></button>' +
    '<span class="dot ' + dc + '" style="width:9px;height:9px"></span>' +
    '<h2>' + esc(dev.hostname) + '</h2>' +
    (dev.oblireach.updateAvailable ? '<span class="tag-chip warn">UPDATE</span>' : '') +
    '<span style="font-size:11px;color:var(--muted)">' + esc(dev.osType) + ' \u00B7 ' + esc(dev.status) + '</span>';
  area.appendChild(hdr);
  hdr.querySelector('#_back-to-devices').addEventListener('click', () => navigateTo('devices'));

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

// Back-compat alias
function selectDevice(dev) { openDeviceDetail(dev); }

function switchTab(tab, area) {
  activeTab = tab;
  area = area || document.getElementById('page-content');
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

  // ── Primary toolbar (connection) ──
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

  // ── Toolbar separator ──
  const sep1 = document.createElement('div'); sep1.className = 'toolbar-sep'; toolbar.appendChild(sep1);

  // ── Screenshot button ──
  const ssBtn = document.createElement('button');
  ssBtn.className = 'btn-sm'; ssBtn.title = 'Screenshot';
  ssBtn.innerHTML = '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="12" cy="13" r="4"/><path d="M5 3v2M19 3v2"/></svg>';
  ssBtn.addEventListener('click', takeScreenshot);
  toolbar.appendChild(ssBtn);

  // ── Clipboard buttons ──
  const clipPaste = document.createElement('button');
  clipPaste.className = 'btn-sm'; clipPaste.title = 'Paste to Remote';
  clipPaste.innerHTML = '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="8" y="2" width="8" height="4" rx="1"/><path d="M16 4h2a2 2 0 012 2v14a2 2 0 01-2 2H6a2 2 0 01-2-2V6a2 2 0 012-2h2"/><path d="M9 14l2 2 4-4"/></svg>';
  clipPaste.addEventListener('click', clipboardPasteToRemote);
  toolbar.appendChild(clipPaste);

  const clipCopy = document.createElement('button');
  clipCopy.className = 'btn-sm'; clipCopy.title = 'Copy from Remote';
  clipCopy.innerHTML = '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="8" y="2" width="8" height="4" rx="1"/><path d="M16 4h2a2 2 0 012 2v14a2 2 0 01-2 2H6a2 2 0 01-2-2V6a2 2 0 012-2h2"/></svg>';
  clipCopy.addEventListener('click', clipboardCopyFromRemote);
  toolbar.appendChild(clipCopy);

  // ── Separator ──
  const sep2 = document.createElement('div'); sep2.className = 'toolbar-sep'; toolbar.appendChild(sep2);

  // ── Zoom dropdown ──
  const zoomDrop = document.createElement('div'); zoomDrop.className = 'toolbar-dropdown';
  const zoomBtn = document.createElement('button');
  zoomBtn.className = 'btn-sm'; zoomBtn.id = 'zoom-btn'; zoomBtn.textContent = 'Fit';
  const zoomMenu = document.createElement('div'); zoomMenu.className = 'toolbar-dropdown-menu';
  ['Fit', '50%%', '75%%', '100%%', '150%%'].forEach(label => {
    const b = document.createElement('button');
    b.textContent = label;
    b.addEventListener('click', () => { setZoomLevel(label); zoomMenu.classList.remove('open'); });
    zoomMenu.appendChild(b);
  });
  zoomBtn.addEventListener('click', () => zoomMenu.classList.toggle('open'));
  zoomDrop.appendChild(zoomBtn); zoomDrop.appendChild(zoomMenu);
  toolbar.appendChild(zoomDrop);

  // ── System Keys dropdown ──
  const sysKeysDrop = document.createElement('div'); sysKeysDrop.className = 'toolbar-dropdown';
  const sysKeysBtn = document.createElement('button');
  sysKeysBtn.className = 'btn-sm'; sysKeysBtn.title = 'System Keys';
  sysKeysBtn.innerHTML = '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="6" width="20" height="12" rx="2"/><line x1="6" y1="10" x2="6" y2="10"/><line x1="10" y1="10" x2="10" y2="10"/><line x1="14" y1="10" x2="14" y2="10"/><line x1="18" y1="10" x2="18" y2="10"/><line x1="8" y1="14" x2="16" y2="14"/></svg>';
  const sysKeysMenu = document.createElement('div'); sysKeysMenu.className = 'toolbar-dropdown-menu';
  const sysKeys = [
    { label: 'Ctrl+Alt+Del', keys: [{key:'Control',code:'ControlLeft'},{key:'Alt',code:'AltLeft'},{key:'Delete',code:'Delete'}] },
    { label: 'Alt+Tab', keys: [{key:'Alt',code:'AltLeft'},{key:'Tab',code:'Tab'}] },
    { label: 'Alt+F4', keys: [{key:'Alt',code:'AltLeft'},{key:'F4',code:'F4'}] },
    { label: 'Win', keys: [{key:'Meta',code:'MetaLeft'}] },
    { label: 'Ctrl+Shift+Esc', keys: [{key:'Control',code:'ControlLeft'},{key:'Shift',code:'ShiftLeft'},{key:'Escape',code:'Escape'}] },
    { label: 'PrtScn', keys: [{key:'PrintScreen',code:'PrintScreen'}] },
  ];
  for (const sk of sysKeys) {
    const b = document.createElement('button');
    b.textContent = sk.label;
    b.addEventListener('click', () => { sendSystemKeys(sk.keys); sysKeysMenu.classList.remove('open'); });
    sysKeysMenu.appendChild(b);
  }
  sysKeysBtn.addEventListener('click', () => sysKeysMenu.classList.toggle('open'));
  sysKeysDrop.appendChild(sysKeysBtn); sysKeysDrop.appendChild(sysKeysMenu);
  toolbar.appendChild(sysKeysDrop);

  // ── Separator ──
  const sep3 = document.createElement('div'); sep3.className = 'toolbar-sep'; toolbar.appendChild(sep3);

  // ── Performance HUD toggle ──
  const hudBtn = document.createElement('button');
  hudBtn.className = 'btn-sm'; hudBtn.title = 'Performance HUD'; hudBtn.id = 'hud-toggle';
  hudBtn.innerHTML = '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 20V10"/><path d="M18 20V4"/><path d="M6 20v-4"/></svg>';
  hudBtn.addEventListener('click', togglePerfHud);
  toolbar.appendChild(hudBtn);

  // ── Recording button ──
  const recBtn = document.createElement('button');
  recBtn.className = 'btn-sm'; recBtn.title = 'Record Session'; recBtn.id = 'rec-toggle';
  recBtn.innerHTML = '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="4" fill="currentColor"/></svg>';
  recBtn.addEventListener('click', toggleRecording);
  toolbar.appendChild(recBtn);

  // ── Annotation toggle ──
  const annoBtn = document.createElement('button');
  annoBtn.className = 'btn-sm'; annoBtn.title = 'Annotation'; annoBtn.id = 'anno-toggle';
  annoBtn.innerHTML = '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 19l7-7 3 3-7 7-3-3z"/><path d="M18 13l-1.5-7.5L2 2l3.5 14.5L13 18l5-5z"/><path d="M2 2l7.586 7.586"/></svg>';
  annoBtn.addEventListener('click', toggleAnnotation);
  toolbar.appendChild(annoBtn);

  // ── Quick Actions toggle ──
  const qaBtn = document.createElement('button');
  qaBtn.className = 'btn-sm'; qaBtn.title = 'Quick Actions'; qaBtn.id = 'qa-toggle';
  qaBtn.innerHTML = '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/></svg>';
  qaBtn.addEventListener('click', toggleQuickActions);
  toolbar.appendChild(qaBtn);

  const statusSpan = document.createElement('span');
  statusSpan.id = 'remote-status';
  statusSpan.style.cssText = 'font-size:11px;color:var(--muted);margin-left:auto';
  toolbar.appendChild(statusSpan);

  pane.appendChild(toolbar);

  // ── Viewport ──
  const vp = document.createElement('div');
  vp.className = 'remote-viewport'; vp.id = 'remote-viewport';
  const canvas = document.createElement('canvas');
  canvas.id = 'remote-canvas'; canvas.style.display = 'none';
  vp.appendChild(canvas);

  // Performance HUD overlay
  const hud = document.createElement('div');
  hud.className = 'perf-hud'; hud.id = 'perf-hud';
  hud.innerHTML = '<div class="hud-row"><span class="hud-label">FPS</span><span class="hud-val" id="hud-fps">0</span></div>' +
    '<div class="hud-row"><span class="hud-label">Bitrate</span><span class="hud-val" id="hud-bitrate">0 Mbps</span></div>' +
    '<div class="hud-row"><span class="hud-label">Codec</span><span class="hud-val" id="hud-codec">H.264</span></div>';
  vp.appendChild(hud);

  // Recording indicator
  const recInd = document.createElement('div');
  recInd.className = 'rec-indicator'; recInd.id = 'rec-indicator';
  recInd.innerHTML = '<span class="rec-dot"></span><span id="rec-timer">00:00</span>';
  vp.appendChild(recInd);

  // Annotation canvas overlay
  const annoCanvas = document.createElement('canvas');
  annoCanvas.id = 'annotation-canvas'; annoCanvas.className = 'annotation-overlay';
  vp.appendChild(annoCanvas);

  // Annotation toolbar
  const annoToolbar = document.createElement('div');
  annoToolbar.className = 'annotation-toolbar'; annoToolbar.id = 'annotation-toolbar'; annoToolbar.style.display = 'none';
  annoToolbar.innerHTML =
    '<button data-tool="pen" class="active" title="Pen">\u270F</button>' +
    '<button data-tool="arrow" title="Arrow">\u2197</button>' +
    '<button data-tool="circle" title="Circle">\u25CB</button>' +
    '<button data-tool="text" title="Text">T</button>' +
    '<button data-tool="eraser" title="Eraser">\u2702</button>' +
    '<input type="color" value="#ef4444" style="width:28px;height:28px;border:none;background:none;cursor:pointer;padding:0" title="Color"/>' +
    '<button data-action="undo" title="Undo">\u21B6</button>' +
    '<button data-action="clear" title="Clear">\u2715</button>';
  vp.appendChild(annoToolbar);

  // Quick Actions drawer
  const qaDrawer = document.createElement('div');
  qaDrawer.className = 'quick-actions'; qaDrawer.id = 'quick-actions';
  qaDrawer.innerHTML =
    '<div class="quick-actions-header"><span>Quick Actions</span><button class="btn-sm" style="padding:3px 6px" id="qa-close">&times;</button></div>' +
    '<div class="quick-actions-body">' +
    '<button class="qa-btn" data-qa="lock"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0110 0v4"/></svg>Lock Workstation</button>' +
    '<button class="qa-btn" data-qa="taskmgr"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="14" rx="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg>Task Manager</button>' +
    '<button class="qa-btn" data-qa="cmd"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4,17 10,11 4,5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>Open CMD</button>' +
    '<button class="qa-btn" data-qa="powershell"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4,17 10,11 4,5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>Open PowerShell</button>' +
    '<button class="qa-btn" data-qa="reboot"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M23 4v6h-6"/><path d="M1 20v-6h6"/><path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"/></svg>Reboot</button>' +
    '</div>';
  vp.appendChild(qaDrawer);

  const ph = document.createElement('div');
  ph.className = 'remote-placeholder'; ph.id = 'remote-placeholder';
  ph.innerHTML = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" style="opacity:.2"><rect x="2" y="3" width="20" height="14" rx="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg><span>Click Connect to start remote session</span>';
  vp.appendChild(ph);
  pane.appendChild(vp);
  tc.appendChild(pane);

  // ── Canvas input events ──
  canvas.addEventListener('mousemove', e => { if (!annotationActive) sendInput('mousemove', e, canvas); });
  canvas.addEventListener('mousedown', e => { if (!annotationActive) sendInput('mousedown', e, canvas); });
  canvas.addEventListener('mouseup', e => { if (!annotationActive) sendInput('mouseup', e, canvas); });
  canvas.addEventListener('wheel', e => { if (!annotationActive) { e.preventDefault(); sendInput('wheel', e, canvas); } }, { passive: false });
  canvas.addEventListener('contextmenu', e => { e.preventDefault(); }); // prevent browser right-click menu
  canvas.addEventListener('keydown', e => { if (!annotationActive) { e.preventDefault(); sendInputKey('keydown', e); } });
  canvas.addEventListener('keyup', e => { if (!annotationActive) { e.preventDefault(); sendInputKey('keyup', e); } });
  canvas.setAttribute('tabindex', '0');

  // ── Annotation events ──
  initAnnotationEvents(annoCanvas, annoToolbar);

  // ── Quick Actions events ──
  initQuickActionsEvents(qaDrawer);

  // Close dropdowns on outside click
  document.addEventListener('click', e => {
    document.querySelectorAll('.toolbar-dropdown-menu.open').forEach(m => {
      if (!m.parentElement.contains(e.target)) m.classList.remove('open');
    });
  });
}

async function startRemote(wtsSessionId) {
  if (!selectedDevice) return;
  // Add/activate session tab
  const tab = addSessionTab(selectedDevice);
  const statusEl = document.getElementById('remote-status');
  if (statusEl) statusEl.textContent = 'Starting session...';
  try {
    const body = { deviceId: selectedDevice.id, protocol: 'oblireach' };
    if (wtsSessionId !== undefined) body.sessionId = wtsSessionId;
    const r = await api('POST', '/api/remote/sessions', body);
    let d = null;
    try { d = await r.json(); } catch {}
    // Non-2xx — surface the server's error message
    if (!r.ok) {
      if (r.status === 401 && d?.twoFactorRequired) {
        throw new Error('2FA code required to start a remote session');
      }
      throw new Error(d?.error || ('HTTP ' + r.status));
    }
    // 202 — pending approval (restriction matrix)
    if (r.status === 202 || d?.data?.status === 'pending_approval') {
      throw new Error('Remote session pending approval (check the Approvals page)');
    }
    const session = d?.data;
    if (!session?.sessionToken) throw new Error('Server returned no session token');
    if (statusEl) statusEl.textContent = 'Connecting...';
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(proto + '//' + location.host + '/proxy/api/remote/tunnel/' + session.sessionToken);
    ws.binaryType = 'arraybuffer';
    remoteWs = ws;
    tab.ws = ws;
    const stopBtn = document.getElementById('stop-btn');
    if (stopBtn) stopBtn.style.display = '';
    ws.onopen = () => {
      if (statusEl) statusEl.textContent = 'Connected \u2014 waiting for stream...';
      startPerfHudTimer();
      renderSessionTabs();
    };
    ws.onclose = () => {
      if (statusEl) statusEl.textContent = 'Disconnected';
      if (stopBtn) stopBtn.style.display = 'none';
      const c = document.getElementById('remote-canvas'), p = document.getElementById('remote-placeholder');
      if (c) c.style.display = 'none'; if (p) p.style.display = 'flex';
      remoteWs = null; remoteDecoder = null;
      tab.ws = null; tab.decoder = null;
      stopPerfHudTimer();
      if (recMediaRecorder) toggleRecording();
      renderSessionTabs();
    };
    ws.onerror = () => { if (statusEl) statusEl.textContent = 'WebSocket error'; };
    ws.onmessage = handleRemoteMessage;
  } catch (err) { if (statusEl) statusEl.textContent = 'Error: ' + err.message; }
}

function stopRemote() {
  if (remoteWs) { remoteWs.close(); remoteWs = null; }
  if (remoteDecoder) { try { remoteDecoder.close(); } catch {} remoteDecoder = null; }
  stopPerfHudTimer();
  if (recMediaRecorder) toggleRecording();
}

// ── Codec name map ──
const codecNames = { 0x01: 'JPEG', 0x02: 'H.264', 0x03: 'VP9', 0x04: 'H.265', 0x05: 'AV1' };

async function handleRemoteMessage(event) {
  if (typeof event.data === 'string') {
    try {
      const info = JSON.parse(event.data);
      if (info.type === 'paired') return;
      if (info.type === 'codec_switch') {
        perfCodec = info.codec === 'jpeg' ? 'JPEG' : info.codec === 'h265' ? 'H.265' : info.codec === 'vp9' ? 'VP9' : info.codec === 'av1' ? 'AV1' : 'H.264';
        const el = document.getElementById('remote-status');
        if (el) el.textContent = el.textContent.replace(/\s*\u00B7\s*\S+$/, '') + ' \u00B7 ' + perfCodec;
        return;
      }
      if (!info.width || !info.height) return;
      await initDecoder(info);
      perfCodec = 'H.264';
      const el = document.getElementById('remote-status');
      if (el) el.textContent = info.width + '\u00D7' + info.height + ' @ ' + info.fps + 'fps \u00B7 H.264';
      const c = document.getElementById('remote-canvas'), p = document.getElementById('remote-placeholder');
      if (c) { c.width = info.width; c.height = info.height; c.style.display = 'block'; }
      if (p) p.style.display = 'none';
      // Resize annotation canvas to match
      const ac = document.getElementById('annotation-canvas');
      if (ac && c) { ac.width = c.parentElement.clientWidth; ac.height = c.parentElement.clientHeight; }
    } catch {} return;
  }
  const buf = new Uint8Array(event.data);
  if (buf.length < 1) return;
  const type = buf[0], payload = buf.slice(1);

  // Perf tracking
  perfFrameCount++;
  perfByteCount += buf.length;

  if (type === 0x01) {
    perfCodec = 'JPEG';
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
  // Sync tab state
  const tab = sessionTabs.find(t => t.id === activeSessionTabId);
  if (tab) { tab.decoder = remoteDecoder; tab.ts = remoteTs; }
}

function sendInput(evType, e, canvas) {
  if (!remoteWs || remoteWs.readyState !== WebSocket.OPEN) return;
  const rect = canvas.getBoundingClientRect();
  const aw = canvas._agentW || canvas.width, ah = canvas._agentH || canvas.height;
  const dpr = window.devicePixelRatio || 1;
  const cw = rect.width * dpr, ch = rect.height * dpr;
  const scale = Math.min(cw / aw, ch / ah);
  const dw = aw * scale, dh = ah * scale;
  const dx = (cw - dw) / 2, dy = (ch - dh) / 2;
  const px = (e.clientX - rect.left) * dpr - dx, py = (e.clientY - rect.top) * dpr - dy;
  // Agent expects: type:"mouse", action:"move"/"down"/"up"/"scroll"
  const actionMap = { mousemove:'move', mousedown:'down', mouseup:'up', wheel:'scroll' };
  const msg = { type: 'mouse', action: actionMap[evType] || evType, x: Math.round(px / scale), y: Math.round(py / scale) };
  if (evType === 'wheel') { msg.delta = Math.sign(-e.deltaY); }
  if (evType === 'mousedown' || evType === 'mouseup') msg.button = e.button;
  remoteWs.send(JSON.stringify(msg));
}

function sendInputKey(evType, e) {
  if (!remoteWs || remoteWs.readyState !== WebSocket.OPEN) return;
  // Agent expects: type:"key", action:"down"/"up"
  const action = evType === 'keydown' ? 'down' : 'up';
  remoteWs.send(JSON.stringify({ type: 'key', action, key: e.key, code: e.code, shift: e.shiftKey, ctrl: e.ctrlKey, alt: e.altKey, meta: e.metaKey }));
}

// ── Feature: Screenshot ──────────────────────────────────────────────────────
function takeScreenshot() {
  const canvas = document.getElementById('remote-canvas');
  if (!canvas || canvas.style.display === 'none') return;
  canvas.toBlob(blob => {
    if (!blob) return;
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    const ts = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
    a.download = 'screenshot-' + (selectedDevice?.hostname || 'remote') + '-' + ts + '.png';
    a.click();
    URL.revokeObjectURL(a.href);
  }, 'image/png');
}

// ── Feature: Clipboard Sync ──────────────────────────────────────────────────
async function clipboardPasteToRemote() {
  if (!remoteWs || remoteWs.readyState !== WebSocket.OPEN) return;
  try {
    const text = await navigator.clipboard.readText();
    if (text) remoteWs.send(JSON.stringify({ type: 'clipboard_set', text }));
  } catch (err) { console.warn('clipboard read failed', err); }
}

async function clipboardCopyFromRemote() {
  if (!remoteWs || remoteWs.readyState !== WebSocket.OPEN) return;
  remoteWs.send(JSON.stringify({ type: 'clipboard_get' }));
  // Response will come as a message; handle in handleRemoteMessage
  const handler = (event) => {
    if (typeof event.data !== 'string') return;
    try {
      const msg = JSON.parse(event.data);
      if (msg.type === 'clipboard_content' && msg.text) {
        navigator.clipboard.writeText(msg.text).catch(() => {});
        remoteWs.removeEventListener('message', handler);
      }
    } catch {}
  };
  remoteWs.addEventListener('message', handler);
  setTimeout(() => remoteWs?.removeEventListener('message', handler), 3000);
}

// ── Feature: System Keys ─────────────────────────────────────────────────────
function sendSystemKeys(keys) {
  if (!remoteWs || remoteWs.readyState !== WebSocket.OPEN) return;
  // Ctrl+Alt+Del requires SAS — send special command
  const isCAD = keys.length === 3 &&
    keys.some(k => k.key === 'Control') &&
    keys.some(k => k.key === 'Alt') &&
    keys.some(k => k.key === 'Delete');
  if (isCAD) {
    remoteWs.send(JSON.stringify({ type: 'sas' }));
    return;
  }
  // Agent expects: type:"key", action:"down"/"up"
  for (const k of keys) {
    remoteWs.send(JSON.stringify({ type: 'key', action: 'down', key: k.key, code: k.code, shift: k.key === 'Shift', ctrl: k.key === 'Control', alt: k.key === 'Alt', meta: k.key === 'Meta' }));
  }
  for (const k of [...keys].reverse()) {
    remoteWs.send(JSON.stringify({ type: 'key', action: 'up', key: k.key, code: k.code, shift: false, ctrl: false, alt: false, meta: false }));
  }
}

// Helper: type a string character by character via key events
function typeString(str) {
  if (!remoteWs || remoteWs.readyState !== WebSocket.OPEN) return;
  for (const ch of str) {
    const key = ch === '\r' ? 'Enter' : ch;
    const code = ch === '\r' ? 'Enter' : ch === ' ' ? 'Space' : ch === '-' ? 'Minus' : ch === '.' ? 'Period' : ch === '/' ? 'Slash' : ch === '\\' ? 'Backslash' : 'Key' + ch.toUpperCase();
    remoteWs.send(JSON.stringify({ type: 'key', action: 'down', key, code, shift: false, ctrl: false, alt: false, meta: false }));
    remoteWs.send(JSON.stringify({ type: 'key', action: 'up', key, code, shift: false, ctrl: false, alt: false, meta: false }));
  }
}

// ── Feature: Performance HUD ─────────────────────────────────────────────────
function togglePerfHud() {
  perfHudVisible = !perfHudVisible;
  const hud = document.getElementById('perf-hud');
  const btn = document.getElementById('hud-toggle');
  if (hud) hud.classList.toggle('visible', perfHudVisible);
  if (btn) btn.style.color = perfHudVisible ? 'var(--accent-l)' : '';
}

function startPerfHudTimer() {
  stopPerfHudTimer();
  perfFrameCount = 0; perfByteCount = 0; perfLastTime = performance.now();
  perfInterval = setInterval(() => {
    const now = performance.now();
    const elapsed = (now - perfLastTime) / 1000;
    if (elapsed > 0) {
      perfFps = Math.round(perfFrameCount / elapsed);
      perfBitrate = ((perfByteCount * 8) / elapsed / 1000000).toFixed(1);
    }
    perfFrameCount = 0; perfByteCount = 0; perfLastTime = now;
    const fpsEl = document.getElementById('hud-fps');
    const brEl = document.getElementById('hud-bitrate');
    const codecEl = document.getElementById('hud-codec');
    if (fpsEl) {
      fpsEl.textContent = perfFps;
      fpsEl.className = 'hud-val' + (perfFps < 5 ? ' hud-bad' : perfFps < 15 ? ' hud-warn' : '');
    }
    if (brEl) brEl.textContent = perfBitrate + ' Mbps';
    if (codecEl) codecEl.textContent = perfCodec;
  }, 500);
}

function stopPerfHudTimer() {
  if (perfInterval) { clearInterval(perfInterval); perfInterval = null; }
}

// ── Feature: Zoom/Scale ──────────────────────────────────────────────────────
function setZoomLevel(label) {
  zoomLevel = label;
  const btn = document.getElementById('zoom-btn');
  if (btn) btn.textContent = label;
  const vp = document.getElementById('remote-viewport');
  const canvas = document.getElementById('remote-canvas');
  if (!vp || !canvas) return;
  if (label === 'Fit') {
    vp.classList.remove('zoom-scroll');
    canvas.style.width = '100%%'; canvas.style.height = '100%%';
    canvas.style.objectFit = 'contain';
  } else {
    vp.classList.add('zoom-scroll');
    const pct = parseInt(label) / 100;
    canvas.style.width = Math.round(canvas.width * pct) + 'px';
    canvas.style.height = Math.round(canvas.height * pct) + 'px';
    canvas.style.objectFit = 'unset';
  }
}

// ── Feature: Session Recording ───────────────────────────────────────────────
function toggleRecording() {
  const recInd = document.getElementById('rec-indicator');
  const recBtn = document.getElementById('rec-toggle');
  if (recMediaRecorder) {
    // Stop recording
    recMediaRecorder.stop();
    recMediaRecorder = null;
    clearInterval(recTimerInterval); recTimerInterval = null;
    if (recInd) recInd.classList.remove('active');
    if (recBtn) recBtn.style.color = '';
    // Tell agent to switch watermark back to LIVE
    if (remoteWs && remoteWs.readyState === 1) remoteWs.send(JSON.stringify({ type: 'set_recording', recording: false }));
    return;
  }
  // Start recording
  const canvas = document.getElementById('remote-canvas');
  if (!canvas || canvas.style.display === 'none') return;
  try {
    const stream = canvas.captureStream(15);
    recMediaRecorder = new MediaRecorder(stream, { mimeType: 'video/webm;codecs=vp9' });
  } catch {
    try {
      const stream = canvas.captureStream(15);
      recMediaRecorder = new MediaRecorder(stream, { mimeType: 'video/webm' });
    } catch (err) { console.warn('Recording not supported', err); return; }
  }
  recChunks = [];
  recStartTime = Date.now();
  recMediaRecorder.ondataavailable = e => { if (e.data.size > 0) recChunks.push(e.data); };
  recMediaRecorder.onstop = () => {
    if (recChunks.length === 0) return;
    const blob = new Blob(recChunks, { type: 'video/webm' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    const ts = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
    a.download = 'recording-' + (selectedDevice?.hostname || 'remote') + '-' + ts + '.webm';
    a.click();
    URL.revokeObjectURL(a.href);
    recChunks = [];
  };
  recMediaRecorder.start(1000);
  if (recInd) recInd.classList.add('active');
  if (recBtn) recBtn.style.color = 'var(--danger)';
  // Tell agent to switch watermark to REC
  if (remoteWs && remoteWs.readyState === 1) remoteWs.send(JSON.stringify({ type: 'set_recording', recording: true }));
  recTimerInterval = setInterval(() => {
    const elapsed = Math.floor((Date.now() - recStartTime) / 1000);
    const mm = String(Math.floor(elapsed / 60)).padStart(2, '0');
    const ss = String(elapsed %% 60).padStart(2, '0');
    const timerEl = document.getElementById('rec-timer');
    if (timerEl) timerEl.textContent = mm + ':' + ss;
  }, 1000);
}

// ── Feature: Quick Actions ───────────────────────────────────────────────────
function toggleQuickActions() {
  const drawer = document.getElementById('quick-actions');
  if (drawer) drawer.classList.toggle('open');
}

function initQuickActionsEvents(drawer) {
  drawer.querySelector('#qa-close')?.addEventListener('click', () => drawer.classList.remove('open'));
  drawer.querySelectorAll('.qa-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      const action = btn.dataset.qa;
      if (action === 'lock') sendSystemKeys([{key:'Meta',code:'MetaLeft'},{key:'l',code:'KeyL'}]);
      else if (action === 'taskmgr') sendSystemKeys([{key:'Control',code:'ControlLeft'},{key:'Shift',code:'ShiftLeft'},{key:'Escape',code:'Escape'}]);
      else if (action === 'cmd') {
        sendSystemKeys([{key:'Meta',code:'MetaLeft'},{key:'r',code:'KeyR'}]);
        setTimeout(() => typeString('cmd\r'), 600);
      }
      else if (action === 'powershell') {
        sendSystemKeys([{key:'Meta',code:'MetaLeft'},{key:'r',code:'KeyR'}]);
        setTimeout(() => typeString('powershell\r'), 600);
      }
      else if (action === 'reboot') {
        if (confirm('Reboot ' + (selectedDevice?.hostname || 'this device') + '?')) {
          sendSystemKeys([{key:'Meta',code:'MetaLeft'},{key:'r',code:'KeyR'}]);
          setTimeout(() => typeString('shutdown -r -t 0\r'), 600);
        }
      }
      drawer.classList.remove('open');
    });
  });
}

// ── Feature: Annotation/Whiteboard ───────────────────────────────────────────
function toggleAnnotation() {
  annotationActive = !annotationActive;
  const canvas = document.getElementById('annotation-canvas');
  const toolbar = document.getElementById('annotation-toolbar');
  const btn = document.getElementById('anno-toggle');
  if (canvas) canvas.classList.toggle('active', annotationActive);
  if (toolbar) toolbar.style.display = annotationActive ? 'flex' : 'none';
  if (btn) btn.style.color = annotationActive ? 'var(--accent-l)' : '';
  if (annotationActive) {
    const vp = document.getElementById('remote-viewport');
    if (vp && canvas) { canvas.width = vp.clientWidth; canvas.height = vp.clientHeight; }
    annotationCtx = canvas?.getContext('2d');
  }
}

function initAnnotationEvents(canvas, toolbar) {
  let startX, startY, lastImageData;

  canvas.addEventListener('mousedown', e => {
    if (!annotationActive) return;
    annotationDrawing = true;
    startX = e.offsetX; startY = e.offsetY;
    if (annotationTool === 'pen' || annotationTool === 'eraser') {
      annotationCtx.beginPath();
      annotationCtx.moveTo(startX, startY);
    }
    if (annotationTool === 'arrow' || annotationTool === 'circle') {
      lastImageData = annotationCtx.getImageData(0, 0, canvas.width, canvas.height);
    }
    if (annotationTool === 'text') {
      const text = prompt('Enter text:');
      if (text) {
        annotationCtx.font = '16px sans-serif';
        annotationCtx.fillStyle = annotationColor;
        annotationCtx.fillText(text, startX, startY);
        annotationHistory.push(annotationCtx.getImageData(0, 0, canvas.width, canvas.height));
      }
      annotationDrawing = false;
    }
  });

  canvas.addEventListener('mousemove', e => {
    if (!annotationDrawing || !annotationActive) return;
    const x = e.offsetX, y = e.offsetY;
    if (annotationTool === 'pen') {
      annotationCtx.strokeStyle = annotationColor;
      annotationCtx.lineWidth = 3;
      annotationCtx.lineCap = 'round';
      annotationCtx.lineTo(x, y);
      annotationCtx.stroke();
    } else if (annotationTool === 'eraser') {
      annotationCtx.strokeStyle = 'rgba(0,0,0,1)';
      annotationCtx.globalCompositeOperation = 'destination-out';
      annotationCtx.lineWidth = 20;
      annotationCtx.lineCap = 'round';
      annotationCtx.lineTo(x, y);
      annotationCtx.stroke();
      annotationCtx.globalCompositeOperation = 'source-over';
    } else if (annotationTool === 'arrow' && lastImageData) {
      annotationCtx.putImageData(lastImageData, 0, 0);
      annotationCtx.strokeStyle = annotationColor;
      annotationCtx.lineWidth = 3;
      annotationCtx.beginPath();
      annotationCtx.moveTo(startX, startY);
      annotationCtx.lineTo(x, y);
      annotationCtx.stroke();
      // Arrowhead
      const angle = Math.atan2(y - startY, x - startX);
      annotationCtx.beginPath();
      annotationCtx.moveTo(x, y);
      annotationCtx.lineTo(x - 12 * Math.cos(angle - 0.5), y - 12 * Math.sin(angle - 0.5));
      annotationCtx.moveTo(x, y);
      annotationCtx.lineTo(x - 12 * Math.cos(angle + 0.5), y - 12 * Math.sin(angle + 0.5));
      annotationCtx.stroke();
    } else if (annotationTool === 'circle' && lastImageData) {
      annotationCtx.putImageData(lastImageData, 0, 0);
      annotationCtx.strokeStyle = annotationColor;
      annotationCtx.lineWidth = 3;
      const rx = Math.abs(x - startX) / 2, ry = Math.abs(y - startY) / 2;
      const cx = (startX + x) / 2, cy = (startY + y) / 2;
      annotationCtx.beginPath();
      annotationCtx.ellipse(cx, cy, rx, ry, 0, 0, 2 * Math.PI);
      annotationCtx.stroke();
    }
  });

  canvas.addEventListener('mouseup', () => {
    if (annotationDrawing && annotationActive) {
      annotationDrawing = false;
      annotationHistory.push(annotationCtx.getImageData(0, 0, canvas.width, canvas.height));
    }
  });

  toolbar.querySelectorAll('button[data-tool]').forEach(btn => {
    btn.addEventListener('click', () => {
      annotationTool = btn.dataset.tool;
      toolbar.querySelectorAll('button[data-tool]').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
    });
  });

  toolbar.querySelector('input[type="color"]')?.addEventListener('input', e => { annotationColor = e.target.value; });

  toolbar.querySelector('button[data-action="undo"]')?.addEventListener('click', () => {
    if (annotationHistory.length > 0) {
      annotationHistory.pop();
      if (annotationHistory.length > 0) {
        annotationCtx.putImageData(annotationHistory[annotationHistory.length - 1], 0, 0);
      } else {
        annotationCtx.clearRect(0, 0, canvas.width, canvas.height);
      }
    }
  });

  toolbar.querySelector('button[data-action="clear"]')?.addEventListener('click', () => {
    annotationCtx?.clearRect(0, 0, canvas.width, canvas.height);
    annotationHistory = [];
  });
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
    // Obliance server requires handshake.auth.{userId, tenantId} — see socket.ts.
    // Session cookies alone are not sufficient; the socket.io middleware only
    // reads the auth payload.
    if (!currentOperatorId) {
      console.warn('socket.io: no user id yet, chat will not work');
      return;
    }
    chatSocket = io({
      path: '/proxy/socket.io',
      transports: ['polling', 'websocket'],
      auth: { userId: currentOperatorId, tenantId: currentTenantId },
    });
    chatSocket.on('connect', () => { console.log('socket.io connected'); });
    chatSocket.on('connect_error', (err) => { console.warn('socket.io connect_error:', err?.message || err); });
    chatSocket.on('chat:message', onChatMessage);
    chatSocket.on('chat:closed', onChatClosed);
    chatSocket.on('chat:remote_response', onChatRemoteResponse);
    chatSocket.on('chat:typing', onChatTyping);
  };
  s.onerror = () => { console.warn('socket.io client not available'); };
  document.head.appendChild(s);
}

document.getElementById('btn-chat').addEventListener('click', toggleChat);
document.getElementById('btn-chat-close').addEventListener('click', () => toggleChat(false));
document.getElementById('chat-send').addEventListener('click', sendChatMessage);
document.getElementById('chat-input').addEventListener('keydown', e => { if (e.key === 'Enter') sendChatMessage(); });
document.getElementById('chat-input').addEventListener('input', emitChatTyping);

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
    operatorAvatar: currentOperatorAvatar || undefined,
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
      const avatarEl = currentOperatorAvatar
        ? '<img src="' + esc(currentOperatorAvatar) + '" style="width:28px;height:28px;border-radius:50%%;object-fit:cover;flex-shrink:0" />'
        : '<div class="avatar" style="width:28px;height:28px;background:var(--accent);border-radius:50%%;display:flex;align-items:center;justify-content:center;flex-shrink:0">' +
          '<svg width="15" height="15" fill="white" viewBox="0 0 24 24"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z"/></svg></div>';
      div.innerHTML = avatarEl + '<div class="bubble">' + esc(msg.text) + '</div>';
    } else {
      const initials = msg.sender.split(/\s+/).map(w => w[0]).join('').toUpperCase().slice(0, 2);
      div.innerHTML = '<div class="bubble">' + esc(msg.text) + '</div>' +
        '<div style="width:28px;height:28px;border-radius:50%%;background:var(--bg3);display:flex;align-items:center;justify-content:center;flex-shrink:0;font-size:10px;font-weight:700;color:var(--muted);margin-top:auto">' + initials + '</div>';
    }
  }
  const typingEl = document.getElementById('chat-typing');
  if (typingEl) { container.insertBefore(div, typingEl); } else { container.appendChild(div); }
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
  hideChatTyping();
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

function onChatTyping(data) {
  if (data.chatId !== chatId) return;
  const el = document.getElementById('chat-typing');
  if (!el) return;
  el.classList.add('visible');
  const container = document.getElementById('chat-messages');
  if (container) container.scrollTop = container.scrollHeight;
  clearTimeout(chatTypingTimer);
  chatTypingTimer = setTimeout(function() { el.classList.remove('visible'); }, 3000);
}

function hideChatTyping() {
  clearTimeout(chatTypingTimer);
  const el = document.getElementById('chat-typing');
  if (el) el.classList.remove('visible');
}

function emitChatTyping() {
  const now = Date.now();
  if (now - lastChatTypingEmit < 2000) return;
  lastChatTypingEmit = now;
  if (chatSocket && chatId) {
    chatSocket.emit('chat:typing', { chatId });
  }
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
document.getElementById('user-menu-logout').addEventListener('click', async () => {
  stopRemote(); closeChat();
  await fetch('/local/logout', { method: 'POST' });
  // Close menus
  document.getElementById('user-menu').classList.remove('open');
  document.getElementById('tenant-menu').classList.remove('open');
  document.getElementById('app').style.display = 'none';
  document.getElementById('login-overlay').style.display = 'flex';
  document.getElementById('inp-pass').value = '';
  document.getElementById('login-err').textContent = '';
  document.getElementById('sso-err').textContent = '';
  const server = document.getElementById('inp-server').value.trim();
  if (server) checkSso();
});
document.getElementById('btn-refresh').addEventListener('click', async () => {
  await loadOverview();
  if (currentRoute === 'dashboard' || currentRoute === 'devices') navigateTo(currentRoute, currentRouteParams);
});

// ── Init ─────────────────────────────────────────────────────────────────────
(async function init() {
  const cfgR = await fetch('/local/config');
  const cfg = await cfgR.json();
  if (cfg.serverUrl) {
    document.getElementById('inp-server').value = cfg.serverUrl;
    const ts = document.getElementById('top-server'); if (ts) ts.textContent = cfg.serverUrl;
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

	// Inject embedded SVG logos (done via Replace, not Sprintf, because SVGs may contain %)
	// Both SVGs use Illustrator-generated class names (.cls-1, .cls-2, etc.) that would
	// conflict when both are inline in the same HTML. Prefix each to scope them.

	// Login logo: prefix cls- → lcls-
	loginSVG := strings.TrimPrefix(logoLoginSVG, `<?xml version="1.0" encoding="UTF-8"?>`)
	loginSVG = strings.TrimSpace(loginSVG)
	loginSVG = strings.ReplaceAll(loginSVG, "cls-", "lcls-")
	loginSVG = strings.Replace(loginSVG, "<svg ", `<svg style="width:260px;height:auto" `, 1)
	html = strings.Replace(html, `<div style="text-align:center;padding:4px 0 0" id="login-logo"></div>`,
		`<div style="text-align:center;padding:4px 0 0" id="login-logo">`+loginSVG+`</div>`, 1)

	// Topbar logo: same full logo as login, sized to fit the 44px topbar.
	// Must also prefix gradient/element IDs to avoid conflicts with the login copy.
	topbarSVG := strings.TrimPrefix(logoLoginSVG, `<?xml version="1.0" encoding="UTF-8"?>`)
	topbarSVG = strings.TrimSpace(topbarSVG)
	topbarSVG = strings.ReplaceAll(topbarSVG, "cls-", "tcls-")
	topbarSVG = strings.ReplaceAll(topbarSVG, `id="`, `id="tb_`)
	topbarSVG = strings.ReplaceAll(topbarSVG, `url(#`, `url(#tb_`)
	topbarSVG = strings.ReplaceAll(topbarSVG, `xlink:href="#`, `xlink:href="#tb_`)
	topbarSVG = strings.Replace(topbarSVG, "<svg ", `<svg style="height:24px;width:auto" `, 1)
	html = strings.Replace(html, `<span class="logo" id="topbar-logo"></span>`,
		`<span class="logo" id="topbar-logo">`+topbarSVG+`</span>`, 1)

	return html
}

func esc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
