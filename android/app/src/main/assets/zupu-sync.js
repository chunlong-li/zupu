// 族谱客户端 · 局域网同步桥接脚本
//
// 这个脚本运行在 **页面主世界**（由 preload 注入，或由控制端 WebView 直接加载），
// 负责：
//   1. 把本机的「页面跳转」和「写请求」广播出去
//   2. 应用对端发来的跳转 / 刷新
//
// 两种连接方式：
//   - host 模式：Electron 客户端内部，通过 window.postMessage ↔ preload ↔ 主进程
//   - controller 模式：URL 带 ?zupu-remote=ip:port&pin=xxxxxx，自己开 WebSocket
//
// 协议见 docs/remote-sync-protocol.md

(function () {
  if (window.__zupuSyncInstalled) return;
  window.__zupuSyncInstalled = true;

  // ─────────── 配置解析 ───────────

  /** 把用户填的地址解析成 {host, port}：兼容 "1.2.3.4"、"1.2.3.4:47810"、"http://1.2.3.4:47810/" 等写法 */
  function parseEndpoint(raw) {
    var s = String(raw || '').trim();
    s = s.replace(/^wss?:\/\//i, '').replace(/^https?:\/\//i, '');
    s = s.replace(/\/.*$/, '').replace(/\/+$/, '').trim();   // 去掉路径与末尾斜杠
    var host = s;
    var port = 47810;
    var m = s.match(/^(.*):(\d{1,5})$/);
    if (m) {
      host = m[1];
      port = parseInt(m[2], 10);
    }
    host = String(host || '').replace(/^\[|\]$/g, '').replace(/:+$/, '').trim();
    if (!port || isNaN(port) || port < 1 || port > 65535) port = 47810;
    return { host: host, port: port };
  }

  function readRemoteParams() {
    try {
      var params = new URLSearchParams(location.search);
      var remote = params.get('zupu-remote');
      if (!remote) {
        // 刷新/重载后地址栏里的参数已被抹掉，从 sessionStorage 恢复控制端身份
        try {
          var saved = JSON.parse(sessionStorage.getItem('zupu.sync.remote') || 'null');
          if (saved && saved.host) {
            var sep = parseEndpoint(saved.host + ':' + (saved.port || 47810));
            saved.host = sep.host;
            saved.port = sep.port;
            return saved;
          }
        } catch (e) {}
        return null;
      }
      var pin = params.get('pin') || '';
      var refresh = params.get('refresh') || 'reload';
      params.delete('zupu-remote');
      params.delete('pin');
      params.delete('refresh');
      var q = params.toString();
      // 抹掉参数，避免影响前端路由
      history.replaceState(history.state, '', location.pathname + (q ? '?' + q : '') + location.hash);
      var ep = parseEndpoint(remote);
      var host = ep.host;
      var port = ep.port;
      try { sessionStorage.setItem('zupu.remote.pin', pin); } catch (e) {}
      var cfg = { host: host, port: port, pin: pin, refresh: refresh };
      try { sessionStorage.setItem('zupu.sync.remote', JSON.stringify(cfg)); } catch (e) {}
      return cfg;
    } catch (e) {
      return null;
    }
  }

  var remoteCfg = readRemoteParams();
  var isHost = !!window.__ZUPU_HOST__ && !remoteCfg;
  var mode = remoteCfg ? 'controller' : (isHost ? 'host' : 'off');

  var refreshMode = (remoteCfg && remoteCfg.refresh) || 'reload';
  // 被控端只读：控制端连上后本机不再提交写操作，只跟随刷新
  var readonly = false;
  var controllers = 0;
  // 被控端：连上后改用控制端的身份（token/user 存 localStorage）
  var IDENTITY_KEYS = ['token', 'user', 'role_aliases', 'local_guest'];
  var SNAP_KEY = 'zupu.sync.identity.snapshot';
  var APPLIED_KEY = 'zupu.sync.identity.applied';
  var suppress = 0;
  var pingTimer = null;

  // ─────────── 与宿主通信 ───────────

  function postToShell(payload) {
    try {
      window.postMessage(Object.assign({ __zupuSync: 1 }, payload), '*');
    } catch (e) {}
  }

  function send(action) {
    if (mode === 'host') {
      postToShell({ kind: 'action', action: action });
    } else if (mode === 'controller' && ws && ws.readyState === 1) {
      try { ws.send(JSON.stringify(action)); } catch (e) {}
    }
  }

  // ─────────── 本机操作 → 广播 ───────────

  function currentPath() {
    return location.pathname + (location.search || '');
  }

  function emitNavigate() {
    if (suppress > 0 || mode === 'off') return;
    send({ type: 'navigate', path: currentPath() });
  }

  var rawPush = history.pushState;
  history.pushState = function () {
    var r = rawPush.apply(this, arguments);
    emitNavigate();
    return r;
  };

  var rawReplace = history.replaceState;
  history.replaceState = function () {
    var r = rawReplace.apply(this, arguments);
    // 初次进入/改 query 也会触发，但 path 没变时 encode 出去也无害
    emitNavigate();
    return r;
  };

  window.addEventListener('popstate', emitNavigate);

  function isMutating(method) {
    return /^(POST|PUT|PATCH|DELETE)$/i.test(String(method || ''));
  }

  function emitMutate(method, url) {
    // 被控端改了也不往外发：只有控制端的修改生效
    if (mode !== 'controller') return;
    send({ type: 'mutate', method: String(method).toUpperCase(), url: String(url || '') });
  }

  function blockLocalWrite(what) {
    toast('被控端只读：请在手机/平板上操作');
    try { console.info('[zupu-sync] 已拦截本机写操作', what); } catch (e) {}
  }

  if (window.fetch) {
    var rawFetch = window.fetch;
    window.fetch = function (input, init) {
      try {
        var method = (init && init.method) || (input && input.method) || 'GET';
        var url = typeof input === 'string' ? input : (input && input.url);
        if (isMutating(method)) {
          if (mode === 'host' && readonly && controllers > 0) {
            blockLocalWrite(method + ' ' + url);
            return Promise.resolve(new Response(JSON.stringify({ ok: false, error: 'readonly' }), {
              status: 403,
              headers: { 'Content-Type': 'application/json; charset=utf-8' }
            }));
          }
          emitMutate(method, url);
        }
      } catch (e) {}
      return rawFetch.apply(this, arguments);
    };
  }

  var rawOpen = XMLHttpRequest.prototype.open;
  var rawSend = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.open = function (method, url) {
    try {
      this.__zupuMutate = isMutating(method) ? (method + ' ' + url) : null;
      if (this.__zupuMutate && !(mode === 'host' && readonly && controllers > 0)) {
        emitMutate(method, url);
      }
    } catch (e) {}
    return rawOpen.apply(this, arguments);
  };
  XMLHttpRequest.prototype.send = function () {
    try {
      if (this.__zupuMutate && mode === 'host' && readonly && controllers > 0) {
        blockLocalWrite(this.__zupuMutate);
        var self = this;
        setTimeout(function () {
          try {
            var ev = new ProgressEvent('error');
            self.dispatchEvent(ev);
            if (typeof self.onerror === 'function') self.onerror(ev);
          } catch (e) {}
        }, 0);
        return;
      }
    } catch (e) {}
    return rawSend.apply(this, arguments);
  };

  // ─────────── 应用对端操作 ───────────

  function apply(action) {
    if (!action || mode === 'off') return;
    if (action.type === 'navigate') {
      var target = action.path || '/';
      if (target === currentPath()) return;
      suppress++;
      try {
        rawPush.call(history, null, '', target);
        window.dispatchEvent(new PopStateEvent('popstate', { state: null }));
      } catch (e) {}
      setTimeout(function () { suppress = Math.max(0, suppress - 1); }, 80);
      toast('已同步');
    } else if (action.type === 'refresh' || action.type === 'mutate') {
      if (refreshMode === 'reload') {
        setTimeout(function () { location.reload(); }, 120);
      } else {
        try { window.dispatchEvent(new CustomEvent('zupu:data-changed')); } catch (e) {}
      }
    }
  }

  // ─────────── 小提示 ───────────

  var toastEl = null;
  var toastTimer = null;
  function toast(text) {
    try {
      if (!toastEl) {
        toastEl = document.createElement('div');
        toastEl.style.cssText = [
          'position:fixed', 'left:50%', 'bottom:28px', 'transform:translateX(-50%)',
          'background:rgba(23,24,26,.86)', 'color:#fff', 'font-size:13px',
          'padding:7px 14px', 'border-radius:8px', 'z-index:2147483647',
          'pointer-events:none', 'font-family:inherit', 'opacity:0',
          'transition:opacity .18s ease'
        ].join(';');
        (document.body || document.documentElement).appendChild(toastEl);
      }
      toastEl.textContent = text;
      toastEl.style.opacity = '1';
      clearTimeout(toastTimer);
      toastTimer = setTimeout(function () { toastEl.style.opacity = '0'; }, 1400);
    } catch (e) {}
  }

  // ─────────── host 模式：收主进程消息 ───────────

  window.addEventListener('message', function (ev) {
    if (!ev || !ev.data || ev.data.__zupuSync !== 1) return;
    if (ev.data.kind === 'apply') apply(ev.data.action);
    // 注意：不再把手机身份套用到电脑上。
    // 电脑是自己的账号，手机借电脑身份（auth-host）；套用手机身份会让电脑被登出，
    // 断开后也回不来（用户反馈的问题）。
    else if (ev.data.kind === 'auth') { /* 忽略 */ }
    else if (ev.data.kind === 'config') {
      if (ev.data.refresh) refreshMode = ev.data.refresh;
      if (typeof ev.data.controllers === 'number') {
        var prev = controllers;
        controllers = ev.data.controllers;
        if (prev > 0 && controllers === 0) restoreIdentity();
      }
      if (typeof ev.data.readonly === 'boolean') readonly = ev.data.readonly;
      // 只有「被控端窗口」才受这个开关影响；控制端窗口不要被强制关掉
      if (isHost) {
        if (ev.data.enabled === false) mode = 'off';
        else mode = 'host';
      }
    }
  });

  if (mode === 'host') {
    // 自愈：旧版本会把手机身份套用到本机（断开后电脑处于登出状态），这里还原一次
    try {
      if (sessionStorage.getItem(APPLIED_KEY) === '1') {
        var legacyRaw = sessionStorage.getItem(SNAP_KEY);
        sessionStorage.removeItem(APPLIED_KEY);
        sessionStorage.removeItem(SNAP_KEY);
        if (legacyRaw) {
          var legacySnap = JSON.parse(legacyRaw);
          for (var li = 0; li < IDENTITY_KEYS.length; li++) {
            var lk = IDENTITY_KEYS[li];
            if (legacySnap[lk] === null || legacySnap[lk] === undefined) localStorage.removeItem(lk);
            else localStorage.setItem(lk, legacySnap[lk]);
          }
          location.reload();
        }
      }
    } catch (e) {}
    postToShell({ kind: 'ready', path: currentPath() });
  }

  // ─────────── controller 模式：自己连 WebSocket ───────────

  var ws = null;
  var retry = 0;
  var hostServer = '';
  var pin = (remoteCfg && remoteCfg.pin) || '';

  // ─────────── 连接状态条（控制端） ───────────
  var statusEl = null;
  var statusTimer = null;

  function ensureStatusEl() {
    if (statusEl) return statusEl;
    statusEl = document.createElement('div');
    statusEl.id = 'zupu-conn-status';
    statusEl.style.cssText = [
      'position:fixed;top:0;left:0;right:0;z-index:2147482000',
      'display:flex;align-items:center;gap:10px;padding:9px 14px',
      'font:13px/1.4 "Microsoft YaHei UI","Noto Sans CJK SC",system-ui,sans-serif',
      'color:#fff;background:#B4762A;box-shadow:0 2px 8px rgba(0,0,0,.18)',
      'transition:transform .25s,opacity .25s'
    ].join(';');
    statusEl.setAttribute('data-kind', 'warn');

    var txt = document.createElement('div');
    txt.id = 'zupu-conn-text';
    txt.style.cssText = 'flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap';
    statusEl.appendChild(txt);

    var retryBtn = document.createElement('button');
    retryBtn.id = 'zupu-conn-retry';
    retryBtn.textContent = '重试';
    retryBtn.style.cssText = [
      'flex:none;height:26px;padding:0 12px;border-radius:6px;cursor:pointer',
      'border:1px solid rgba(255,255,255,.5);background:rgba(255,255,255,.16)',
      'color:#fff;font:12.5px inherit;display:none'
    ].join(';');
    retryBtn.addEventListener('click', function () {
      retry = 0;
      mode = 'controller';
      setStatus('正在重新连接…', 'warn', 0);
      connectController();
    });
    statusEl.appendChild(retryBtn);

    var closeBtn = document.createElement('button');
    closeBtn.textContent = '✕';
    closeBtn.style.cssText = 'flex:none;width:26px;height:26px;border:0;background:transparent;color:#fff;cursor:pointer;font-size:14px';
    closeBtn.addEventListener('click', function () { hideStatus(); });
    statusEl.appendChild(closeBtn);

    (document.body || document.documentElement).appendChild(statusEl);
    return statusEl;
  }

  function setStatus(text, kind, autoHideMs, showRetry) {
    try {
      var el = ensureStatusEl();
      var colors = { warn: '#B4762A', ok: '#2E7D4F', error: '#C23B3B', info: '#3B5BA5' };
      el.style.background = colors[kind] || colors.warn;
      el.setAttribute('data-kind', kind || 'warn');
      el.style.transform = 'translateY(0)';
      el.style.opacity = '1';
      el.style.pointerEvents = 'auto';
      document.getElementById('zupu-conn-text').textContent = text;
      document.getElementById('zupu-conn-retry').style.display = showRetry ? '' : 'none';
      clearTimeout(statusTimer);
      if (autoHideMs > 0) statusTimer = setTimeout(hideStatus, autoHideMs);
    } catch (e) {}
  }

  function hideStatus() {
    try {
      if (!statusEl) return;
      statusEl.style.opacity = '0';
      statusEl.style.pointerEvents = 'none';
      statusTimer = setTimeout(function () {
        try { if (statusEl && statusEl.parentNode) statusEl.parentNode.removeChild(statusEl); } catch (e) {}
        statusEl = null;
      }, 300);
    } catch (e) {}
  }

  function hostLabel() {
    return remoteCfg ? (remoteCfg.host + ':' + remoteCfg.port) : '电脑';
  }

  function connectController() {
    if (mode !== 'controller') return;
    if (!pin) {
      pin = (prompt('请输入电脑上显示的 6 位配对码') || '').trim();
      if (!pin) { mode = 'off'; return; }
    }
    if (!remoteCfg || !remoteCfg.host) {
      setStatus('❌ 电脑地址为空：请在手机设置里填写电脑的 IP（例如 192.168.1.21）', 'error', 0, true);
      return;
    }
    var url = 'ws://' + remoteCfg.host + ':' + remoteCfg.port + '/';
    setStatus('正在连接 ' + hostLabel() + (retry > 0 ? '（第 ' + retry + ' 次重试）' : '') + '…', 'warn', 0);
    var openedAt = 0;
    try {
      ws = new WebSocket(url);
    } catch (e) {
      var msg = String((e && e.message) || e);
      if (/insecure|mixed|secure/i.test(msg)) {
        setStatus(
          '❌ 手机端页面是 HTTPS，浏览器不允许连 ws://' + hostLabel() +
          '。请把手机上的服务器地址改成 http:// 开头的局域网地址（电脑端「开启被控」里有显示），再点重试。',
          'error', 0, true);
      } else {
        setStatus('❌ 无法发起连接：' + msg + '（地址 ' + hostLabel() + '）', 'error', 0, true);
      }
      scheduleRetry();
      return;
    }

    ws.onopen = function () {
      openedAt = Date.now();
      retry = 0;
      setStatus('已连上电脑，正在核对配对码…', 'info', 0);
      var devId = '';
      try {
        devId = sessionStorage.getItem('zupu.controller.id') || '';
        if (!devId) {
          devId = 'd' + Date.now().toString(36) + Math.random().toString(36).slice(2, 8);
          sessionStorage.setItem('zupu.controller.id', devId);
        }
      } catch (e) {}
      send({
        type: 'hello', role: 'controller', pin: pin,
        device: navigator.userAgent.slice(0, 60),
        deviceId: devId,
        origin: location.origin
      });
    };

    ws.onmessage = function (ev) {
      var msg;
      try { msg = JSON.parse(ev.data); } catch (e) { return; }
      if (msg.type === 'welcome') {
        hostServer = msg.server || '';
        // 控制端自己的服务器和被控端不一致 → 直接切到被控端的服务器（数据才一致）
        try {
          if (hostServer && location.origin !== new URL(hostServer).origin &&
              !sessionStorage.getItem('zupu.sync.switched')) {
            sessionStorage.setItem('zupu.sync.switched', '1');
            var joiner = hostServer.indexOf('?') >= 0 ? '&' : '?';
            location.href = hostServer + joiner +
              'zupu-remote=' + encodeURIComponent(remoteCfg.host + ':' + remoteCfg.port) +
              '&pin=' + encodeURIComponent(pin);
            return;
          }
        } catch (e) {}
        sessionStorage.removeItem('zupu.sync.switched');
        setStatus('✅ 已连接：' + ((msg.host && msg.host.name) || hostLabel()) + '（手机操作会同步到电脑）', 'ok', 4000);
        toast('已连接：' + ((msg.host && msg.host.name) || remoteCfg.host));
        sendIdentity();
        if (msg.state && msg.state.path) apply({ type: 'navigate', path: msg.state.path });
        pingTimer = setInterval(function () {
          try { ws.send(JSON.stringify({ type: 'ping', ts: Date.now() })); } catch (e) {}
        }, 20000);
      } else if (msg.type === 'auth-host') {
        // 只有手机在这个服务器上「完全没有登录态」时才借用电脑身份；
        // 否则会把用户刚登录的账号覆盖掉，导致每步操作都要求重新登录。
        try {
          var mine = localStorage.getItem('token');
          if (mine && mine === msg.token) return;          // 本来就是同一个身份
          if (mine) {
            // 手机自己登录过 → 告诉电脑用手机的身份，而不是反过来
            sendIdentity();
            return;
          }
          if (msg.token && !sessionStorage.getItem('zupu.sync.hostAuth')) {
            sessionStorage.setItem('zupu.sync.hostAuth', '1');
            localStorage.setItem('token', msg.token);
            if (msg.user) localStorage.setItem('user', JSON.stringify(msg.user));
            toast('已使用电脑的登录身份');
            setTimeout(function () { location.reload(); }, 400);
          }
        } catch (e) {}
      } else if (msg.type === 'auth_fail') {
        setStatus(
          msg.reason === 'busy'
            ? '❌ 电脑上已连满设备（最多 3 台），请先断开一台再试'
            : '❌ 配对码不对：请核对电脑「开启被控」里显示的 6 位数字',
          'error', 0, true);
        toast(msg.reason === 'busy' ? '电脑上已连满设备' : '配对码不对');
        try { ws.close(); } catch (e) {}
        if (msg.reason === 'pin') { try { sessionStorage.removeItem('zupu.remote.pin'); } catch (e) {} }
        mode = 'off';
      } else if (msg.type === 'navigate' || msg.type === 'refresh' || msg.type === 'mutate') {
        apply(msg);
      }
    };

    ws.onclose = function () {
      clearInterval(pingTimer);
      if (mode !== 'controller') return;
      // 连上过又断了 → 断线重连；从来没连上 → 端口不通/被防火墙拦
      if (openedAt) {
        setStatus('⚠ 与电脑的连接已断开，正在自动重连…', 'warn', 0);
      } else if (retry >= 1) {
        setStatus(
          '❌ 连不上 ' + hostLabel() + '：' +
          '请确认电脑端「开启被控」已打开、手机和电脑在同一 WiFi，' +
          '并允许防火墙访问（也可能是路由器开了 AP 隔离）',
          'error', 0, true);
      }
      scheduleRetry();
    };
    ws.onerror = function (ev) {
      // 浏览器不暴露具体错误，仅作占位；真正的原因在 onclose 里给
      if (!openedAt && retry === 0) setStatus('无法连接 ' + hostLabel() + '，正在重试…', 'warn', 0);
    };
  }

  /** 控制端：把本机登录身份发给被控端 */
  function sendIdentity() {
    try {
      // 只有和被控端同一个服务器时才推身份，否则 token 对不上
      if (hostServer) {
        try { if (new URL(hostServer).origin !== location.origin) return; } catch (e) {}
      }
      var token = localStorage.getItem('token') || '';
      if (!token) return;
      var payload = { type: 'auth', token: token };
      try { payload.user = JSON.parse(localStorage.getItem('user') || 'null'); } catch (e) {}
      try { payload.roleAliases = JSON.parse(localStorage.getItem('role_aliases') || 'null'); } catch (e) {}
      payload.localGuest = localStorage.getItem('local_guest') || '';
      send(payload);
    } catch (e) {}
  }

  /** 被控端：接受控制端身份（首次会做快照，断开时还原） */
  function applyIdentity(auth) {
    try {
      if (!auth || !auth.token) return;
      if (localStorage.getItem('token') === auth.token) return;   // 已经是这个身份，别重复重载
      if (!sessionStorage.getItem(SNAP_KEY)) {
        var snap = {};
        for (var i = 0; i < IDENTITY_KEYS.length; i++) snap[IDENTITY_KEYS[i]] = localStorage.getItem(IDENTITY_KEYS[i]);
        sessionStorage.setItem(SNAP_KEY, JSON.stringify(snap));
      }
      localStorage.setItem('token', auth.token);
      if (auth.user) localStorage.setItem('user', JSON.stringify(auth.user));
      if (auth.roleAliases) localStorage.setItem('role_aliases', JSON.stringify(auth.roleAliases));
      if (auth.localGuest) localStorage.setItem('local_guest', auth.localGuest);
      sessionStorage.setItem(APPLIED_KEY, '1');
      var who = (auth.user && (auth.user.username || auth.user.name)) || '';
      toast('已切换为手机端身份' + (who ? '：' + who : ''));
      setTimeout(function () { location.reload(); }, 400);
    } catch (e) {}
  }

  /** 被控端：控制端断开，还原本机原本身份 */
  function restoreIdentity() {
    try {
      if (sessionStorage.getItem(APPLIED_KEY) !== '1') return;
      var raw = sessionStorage.getItem(SNAP_KEY);
      sessionStorage.removeItem(APPLIED_KEY);
      sessionStorage.removeItem(SNAP_KEY);
      if (!raw) return;
      var snap = JSON.parse(raw);
      for (var i = 0; i < IDENTITY_KEYS.length; i++) {
        var k = IDENTITY_KEYS[i];
        if (snap[k] === null || snap[k] === undefined) localStorage.removeItem(k);
        else localStorage.setItem(k, snap[k]);
      }
      toast('控制端已断开，恢复本机身份');
      setTimeout(function () { location.reload(); }, 400);
    } catch (e) {}
  }

  function scheduleRetry() {
    if (mode !== 'controller') return;
    retry = Math.min(retry + 1, 6);
    setTimeout(connectController, Math.min(1000 * Math.pow(1.6, retry), 15000));
  }

  if (mode === 'controller') {
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', connectController);
    } else {
      connectController();
    }
  }

  // 对外暴露一点状态，方便页面/调试
  window.zupuSync = {
    get mode() { return mode; },
    get host() { return remoteCfg ? remoteCfg.host + ':' + remoteCfg.port : ''; },
    resync: function () { send({ type: 'sync-request', path: currentPath() }); },
    apply: apply
  };
})();
