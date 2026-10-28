// PROTOTYPE (#124) — throwaway. Shared plumbing for the visual variants:
// the replay of the recorded run, the page state, hop derivation and pacing,
// the terminal drawer, the node tooltip and the variant switcher. Layout and
// drawing live in variants.js, one function per variant.
//
// The real engine will emit Hop events itself (#123). Here hops are derived
// from the recorded log lines, so the recording stays a raw capture.

const NAMES = ['Issuer', 'Sender', 'Ana', 'Bruno', 'Carla', 'Diego', 'Elena', 'Felipe'];
const NODES = [
  ...NAMES.map((n, i) => ({ id: 'a' + i, label: n, group: 'actor' })),
  { id: 'auditor', label: 'Auditor', group: 'actor' },
  { id: 'funder', label: 'Gas funder', group: 'actor' },
  { id: 'gnark', label: 'gnark server', group: 'service' },
  { id: 'relayer', label: 'Relayer', group: 'service' },
  { id: 'registry', label: 'UserRegistry', group: 'contract' },
  { id: 'vault', label: 'RaylsERC20 vault', group: 'contract' },
  { id: 'usdrVault', label: 'USDr vault', group: 'contract' },
  { id: 'dvp', label: 'EnygmaDvp', group: 'contract' },
  { id: 'tagChan', label: 'TagChannelRegistry', group: 'contract' },
  { id: 'tagReg', label: 'TagRegistry', group: 'contract' },
];
const NODE = Object.fromEntries(NODES.map(n => [n.id, n]));
const byName = n => 'a' + NAMES.indexOf(n);
const PARTICIPANTS = ['a2', 'a3', 'a4', 'a5', 'a6', 'a7'];

// The recorded run's options (engine.DefaultOptions).
const OPTIONS = { shieldTotal: 1000, payouts: [120, 90, 150, 60, 200, 80], usdrFee: true, tags: true };

// Which proto fields each node's hover shows.
const NODE_FIELDS = {
  registry: ['registryAddr'], vault: ['vaultAddr', 'erc20Addr', 'shieldGas'],
  usdrVault: ['usdrVaultAddr', 'usdrFixedFee'], dvp: ['dvpAddr'],
  tagChan: ['tagChannelAddr'], tagReg: ['tagRegistryAddr'],
  relayer: ['relayerAddr', 'relayerUsdrIncome'], auditor: ['auditorPk'],
};

// ---- state ----------------------------------------------------------------

function freshState() {
  const steps = [];
  for (const e of RECORDING.events)
    if (e.kind === 'proto' && e.field === 'steps') { for (const s of JSON.parse(e.value)) steps.push({ ...s, status: 'pending', msg: '' }); break; }
  return {
    status: 'idle', steps, current: null,
    actors: NAMES.map((name, id) => ({ id, name, f: {} })),
    proto: {}, audit: null, totals: null, lastTransfer: null,
  };
}

const S = { state: freshState(), variant: null, speed: 1 };

// ---- hops -----------------------------------------------------------------

const ctx = { step: '', payer: '' };
const hop = (from, to, label, detail = '', together = false) => ({ from, to, label, detail, together, step: ctx.step });

// hopsFor maps one recorded event to the hops the engine will emit for it.
function hopsFor(e) {
  if (e.kind === 'step_started') {
    ctx.step = e.step;
    const m = /^Pay (\w+)$/.exec(e.label) || /^(\w+) → (\w+)$/.exec(e.label);
    ctx.payer = m ? (m[2] ? byName(m[1]) : 'a1') : '';
    return [];
  }
  if (e.kind === 'step_failed') return [{ ...hop(ctx.payer || 'relayer', 'dvp', 'failed', e.msg), failed: true }];
  if (e.kind !== 'log') return [];
  const m = e.msg, p = ctx.payer;
  let r;
  if ((r = /^auditor ML-KEM-768 ek/.exec(m))) return [hop('auditor', 'auditor', 'view key', 'ML-KEM-768 key pair')];
  if ((r = /^(\w+)\s+pk_spend (\S+)\s+ML-KEM ek (\S+)/.exec(m))) return [
    hop(byName(r[1]), byName(r[1]), 'spend key', 'BabyJubJub · ' + r[2]),
    hop(byName(r[1]), byName(r[1]), 'view key', 'ML-KEM-768 · ek ' + r[3] + ' (1184 B)')];
  if ((r = /^funded (\d+) new accounts with ([\d.]+ ETH)/.exec(m)))
    return NAMES.slice(0, +r[1]).map((_, i) => hop('funder', 'a' + i, 'fund gas', r[2] + ' to a fresh signing account', i > 0));
  if ((r = /^UserRegistry\.register\((\w+)\).*\((\d+) \+ (\d+) bytes\)/.exec(m))) return [
    hop(byName(r[1]), byName(r[1]), 'audit ciphertexts', `ML-KEM capsule to the auditor (${r[2]} B) + AES-GCM of the view key's seed (${r[3]} B)`),
    hop(byName(r[1]), 'registry', 'register', 'gas is high: the 1184-byte ML-KEM view key is stored on chain')];
  if ((r = /^auditor opened (\w+)'s audit ciphertexts/.exec(m))) return [hop('registry', 'auditor', 'recover view key', r[1] + "'s view key, from their audit ciphertexts")];
  if ((r = /openChannel\((\w+) → (\w+)\) via relayer → channel #(\d+).*gas ([\d,]+)/.exec(m))) return [
    hop(byName(r[1]), 'relayer', 'open channel', r[1] + ' → ' + r[2]),
    hop('relayer', 'tagChan', 'openChannel', `channel #${r[3]} · gas ${r[4]}`)];
  if ((r = /^(\w+) checked (\d+) channel records/.exec(m))) return [hop('tagChan', byName(r[1]), 'scan channels', r[2] + ' records checked')];
  if ((r = /^UsdrERC20\.mint\((\d+)\)/.exec(m))) return [hop('a0', 'a0', 'mint USDr', r[1] + ' USDr (public)')];
  if ((r = /^UsdrCoinVault\.depositV2\((\d+)\) → (\w+)'s note/.exec(m))) return [hop('a0', 'usdrVault', 'grant ' + r[1] + ' USDr', 'shielded fee credit → ' + r[2])];
  if (/each decrypted their USDr note/.test(m)) return ['a1', ...PARTICIPANTS].map((a, i) => hop('usdrVault', a, 'scan', 'decrypt USDr note', i > 0));
  if ((r = /^RaylsERC20\.mint\((\d+)\)/.exec(m))) return [hop('a1', 'a1', 'mint', r[1] + ' RaylsERC20 (public)')];
  if ((r = /^Erc20CoinVault\.depositV2\((\d+)\).*gas ([\d,]+)/.exec(m))) return [hop('a1', 'vault', 'shield ' + r[1], 'gas ' + r[2])];
  if ((r = /^rebuilt vault tree from (\d+)/.exec(m))) return [hop('vault', p, 'read tree', r[1] + ' Commitment events')];
  if ((r = /^(Payment2in|Payment) proof in (\d+) ms/.exec(m))) return [hop(p, 'gnark', 'prove ' + r[1], r[2] + ' ms')];
  if ((r = /^UsdrFee proof in (\d+) ms/.exec(m))) return [hop(p, 'gnark', 'prove UsdrFee', r[1] + ' ms · 1 USDr fee note to the relayer')];
  if ((r = /paymentWithUsdrFee\(\) via relayer → block (\d+)\s+gas ([\d,]+)\s+round-trip (\d+)/.exec(m))) return [
    hop(p, 'relayer', 'relay', 'round-trip ' + r[3] + ' ms'),
    hop('relayer', 'dvp', 'paymentWithUsdrFee', `block ${r[1]} · gas ${r[2]}`),
    hop('dvp', 'vault', 'payment', 'block ' + r[1] + ' — same transaction'),
    hop('dvp', 'usdrVault', 'fee 1 USDr', 'block ' + r[1] + ' — same transaction', true)];
  if ((r = /^TagRegistry\.publishTag\(\) via relayer → (\w+)'s tag/.exec(m))) return [hop('relayer', 'tagReg', 'publishTag', 'tag for ' + r[1])];
  if ((r = /^(\w+) checked (\d+) tags, matched/.exec(m))) return [hop('tagReg', byName(r[1]), 'scan tags', r[2] + ' tags checked → 1 note decrypted')];
  if ((r = /^auditor decrypted (\d+) transactions/.exec(m))) return [
    hop('vault', 'auditor', 'decrypt', r[1] + ' transactions across both vaults'),
    hop('usdrVault', 'auditor', 'decrypt', 'USDr leg', true)];
  return [];
}

// Pacing is in the page only: each hop (or batch of `together` hops) holds
// for HOLD ms, shrinking when the queue is more than CATCHUP ms behind.
const HOLD = 600, CATCHUP = 3000;
const hopQ = [];
let hopTimer = null;
function queueHops(hs) { hopQ.push(...hs); if (!hopTimer) pumpHops(); }
function pumpHops() {
  hopTimer = null;
  if (!hopQ.length) return;
  const batch = [hopQ.shift()];
  while (hopQ.length && hopQ[0].together) batch.push(hopQ.shift());
  const hold = hopQ.length * HOLD > CATCHUP ? Math.max(80, CATCHUP / hopQ.length) : HOLD;
  S.variant?.hop?.(batch, hold);
  S.state.hopsShown = (S.state.hopsShown || 0) + batch.length;
  S.state.queued = hopQ.length;
  dirty();
  hopTimer = setTimeout(pumpHops, hold);
}

// ---- applying events ------------------------------------------------------

function apply(e) {
  const st = S.state;
  const step = st.steps.find(s => s.id === e.step);
  switch (e.kind) {
    case 'step_started':
      if (!step) st.steps.push({ id: e.step, label: e.label, status: 'running', msg: e.msg });
      else Object.assign(step, { status: 'running', msg: e.msg });
      st.current = e.step; st.status = 'running';
      term('engine', `${clock(e.t)} \x1b[36m▶ ${e.label}\x1b[0m  ${e.msg}`);
      break;
    case 'step_succeeded':
      Object.assign(step, { status: 'done', msg: e.msg });
      term('engine', `${clock(e.t)} \x1b[32m✓ ${e.label}\x1b[0m  ${e.msg}`);
      break;
    case 'step_failed':
      Object.assign(step, { status: 'failed', msg: e.msg });
      term('engine', `${clock(e.t)} \x1b[31m✗ ${e.label}\x1b[0m  ${e.msg}`);
      break;
    case 'log':
      term('engine', `${clock(e.t)} \x1b[90m[${(e.cat || '·').padEnd(5)}]\x1b[0m ${e.msg}`);
      break;
    case 'actor': st.actors[e.actor].f[e.field] = e.value; break;
    case 'proto':
      if (e.field === 'audit') st.audit = JSON.parse(e.value);
      else if (e.field === 'totals') st.totals = JSON.parse(e.value);
      else if (e.field === 'fanout' || e.field === 'transfer') st.lastTransfer = JSON.parse(e.value);
      else if (e.field !== 'steps') st.proto[e.field] = e.value;
      break;
    case 'done': st.status = 'done'; st.current = null; term('engine', `${clock(e.t)} \x1b[32;1m${e.msg}\x1b[0m`); break;
    case 'failed': st.status = 'failed'; break;
  }
  const hs = hopsFor(e);
  if (hs.length && !replay.skipping) queueHops(hs);
}

const clock = t => { const s = t / 1000; return `\x1b[90m+${s.toFixed(3).padStart(7)}s\x1b[0m`; };

// ---- replay ---------------------------------------------------------------

const replay = {
  clock: 0, ei: 0, si: { hardhat: 0, gnark: 0, relayer: 0 }, playing: false, last: 0, skipping: false,
  start() {
    S.state = freshState(); Object.assign(ctx, { step: '', payer: '' });
    hopQ.length = 0; clearTimeout(hopTimer); hopTimer = null;
    termReset();
    Object.assign(this, { clock: 0, ei: 0, si: { hardhat: 0, gnark: 0, relayer: 0 }, playing: true, last: performance.now() });
    S.variant?.reset?.();
    dirty();
    const gen = this.gen = (this.gen || 0) + 1;
    setTimeout(() => this.tick(performance.now(), gen), 16);
  },
  // setTimeout, not requestAnimationFrame, so the replay also runs in
  // headless Chrome's virtual time (used for screenshots).
  tick(now, gen) {
    if (!this.playing || gen !== this.gen) return;
    this.clock += (now - this.last) * S.speed; this.last = now;
    this.advance(this.clock);
    if (this.ei >= RECORDING.events.length) this.playing = false;
    else setTimeout(() => this.tick(performance.now(), gen), 16);
  },
  advance(upTo) {
    const ev = RECORDING.events;
    while (this.ei < ev.length && ev[this.ei].t <= upTo) { apply(ev[this.ei++]); dirty(); }
    for (const svc of ['hardhat', 'gnark', 'relayer']) {
      const lines = RECORDING.services[svc];
      while (this.si[svc] < lines.length && lines[this.si[svc]].t <= upTo) term(svc, lines[this.si[svc]++].line);
    }
  },
  toEnd() {
    if (!this.playing && this.ei === 0) this.start();
    this.skipping = true; this.advance(Infinity); this.skipping = false; this.playing = false;
    hopQ.length = 0; dirty();
  },
};

// ---- rendering ------------------------------------------------------------

let raf = 0;
function dirty() { if (!raf) raf = setTimeout(() => { raf = 0; S.variant?.render(S.state); }, 16); }

const esc = s => String(s ?? '').replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
const short = (h, n = 6) => h && h.length > 2 * n + 4 ? h.slice(0, n + 2) + '…' + h.slice(-n) : (h || '');
const fmt = n => Number(n).toLocaleString('en-US');
const progress = st => st.steps.length ? st.steps.filter(s => s.status === 'done').length / st.steps.length : 0;

// Content blocks the variants place where they like (not layouts).
function auditTable(st) {
  const a = st.audit;
  if (!a) return '<p class="hint">The auditor recovers every view key after registration, then decrypts each transaction.</p>';
  const rows = a.users.map(u => {
    const ok = String(a.balances?.[u]) === String(a.own?.[u]) && String(a.usdrBalances?.[u]) === String(a.ownUsdr?.[u]);
    return `<tr><td>${esc(u)}</td><td class="num">${a.balances?.[u] ?? '–'}</td><td class="num">${a.usdrBalances?.[u] ?? '–'}</td><td class="${ok ? 'ok' : 'pending'}">${ok ? '✓ matches' : '…'}</td></tr>`;
  }).join('');
  return `<table class="audit"><thead><tr><th>User</th><th>RaylsERC20</th><th>USDr</th><th>vs. own view</th></tr></thead><tbody>${rows}</tbody></table>
    <p class="hint">${(a.entries || []).length} transactions decrypted</p>`;
}
function totalsLine(st) {
  const t = st.totals;
  return t ? `${t.txs} transactions · ${fmt(t.gas)} gas · ${(t.proveMs / 1000).toFixed(1)} s proving · ${(t.relayMs / 1000).toFixed(1)} s relaying` : '';
}
function sendForm() {
  const opts = NAMES.slice(1).map(n => `<option>${n}</option>`).join('');
  return `<form class="send" onsubmit="return false" title="Replay only — no live stack behind this prototype">
    <select disabled>${opts}</select><span>→</span><select disabled>${opts}</select>
    <input disabled value="25"><button disabled>Send</button></form>`;
}
function controls() {
  return {
    run: `<button data-act="run">RUN</button>`,
    end: `<button data-act="end" title="Jump to the end of the recorded run">⏭</button>`,
    speed: `<select data-act="speed" title="Replay speed">${[1, 2, 4].map(x => `<option value="${x}" ${S.speed == x ? 'selected' : ''}>${x}×</option>`).join('')}</select>`,
    theme: `<button data-act="theme" title="Light / dark">◐</button>`,
    options: [
      `<label>Shield total <input value="${OPTIONS.shieldTotal}" disabled></label>`,
      `<label>Payouts <input value="${OPTIONS.payouts.join(', ')}" disabled></label>`,
      `<label><input type="checkbox" checked disabled> USDr fee</label>`,
      `<label><input type="checkbox" checked disabled> with tag</label>`,
    ],
  };
}

// nodeInfo is the hover detail for a diagram node.
function nodeInfo(id) {
  const n = NODE[id], st = S.state, rows = [];
  if (/^a\d$/.test(id)) {
    const f = st.actors[+id.slice(1)].f;
    for (const k of ['role', 'ethAddr', 'pkSpend', 'pkView', 'status', 'balance', 'usdr', 'note', 'scan', 'channels'])
      if (f[k]) rows.push([k, k.startsWith('pk') || k === 'note' || k === 'ethAddr' ? short(f[k], 8) : f[k]]);
  }
  for (const k of NODE_FIELDS[id] || []) if (st.proto[k]) rows.push([k, /^0x/.test(st.proto[k]) ? short(st.proto[k], 8) : st.proto[k]]);
  if (id === 'dvp' && st.lastTransfer) rows.push(['last settle', `block ${st.lastTransfer.block} · gas ${fmt(st.lastTransfer.gas)}`]);
  if (id === 'registry') rows.push(['stores', 'spend key, 1184-byte ML-KEM view key, audit ciphertexts']);
  if (!rows.length) rows.push(['', 'run the scenario to fill this in']);
  return `<b>${esc(n.label)}</b><span class="tt-group">${n.group}</span>` +
    rows.map(([k, v]) => `<div><i>${esc(k)}</i> <code>${esc(v)}</code></div>`).join('');
}

// ---- terminal drawer (shared: the same dark drawer in every variant) -------

const TABS = ['engine', 'hardhat', 'gnark', 'relayer'];
const MAXLINES = 1000;
const termBuf = Object.fromEntries(TABS.map(t => [t, []]));
let hhWrite = false, termTab = 'engine', hideReads = true, drawerH = 230, drawerOpen = true;

function ansi(s) {
  let out = '', open = 0;
  for (const part of esc(s).split(/(\x1b\[[0-9;]*m)/)) {
    const m = /^\x1b\[([0-9;]*)m$/.exec(part);
    if (!m) { out += part; continue; }
    const codes = m[1].split(';').map(Number);
    if (codes.includes(0) || m[1] === '') { out += '</span>'.repeat(open); open = 0; continue; }
    out += `<span class="${codes.map(c => 'c' + c).join(' ')}">`; open++;
  }
  return out + '</span>'.repeat(open);
}

function term(tab, line) {
  // Hardhat prints one block per request: the unindented line names the RPC
  // method, so a block is a read unless that method sends a transaction.
  let read = false;
  if (tab === 'hardhat') {
    const plain = line.replace(/\x1b\[[0-9;]*m/g, '');
    if (plain && !plain.startsWith(' ')) hhWrite = /eth_send/.test(plain);
    read = !hhWrite;
  }
  const buf = termBuf[tab];
  buf.push({ line, read });
  if (buf.length > MAXLINES) buf.shift();
  if (tab === termTab && !(read && hideReads)) termAppend({ line, read });
  document.querySelector(`[data-tab="${tab}"] .count`)?.replaceChildren(String(buf.length));
}
function termReset() { for (const t of TABS) termBuf[t].length = 0; hhWrite = false; termRender(); }
function termAppend(l) {
  const el = document.getElementById('termBody');
  const follow = el.scrollTop + el.clientHeight >= el.scrollHeight - 6;
  const div = document.createElement('div');
  div.innerHTML = ansi(l.line) || '&nbsp;';
  el.appendChild(div);
  while (el.childElementCount > MAXLINES) el.firstChild.remove();
  if (follow) el.scrollTop = el.scrollHeight;
}
function termRender() {
  const el = document.getElementById('termBody');
  const lines = termBuf[termTab].filter(l => !(l.read && hideReads));
  el.innerHTML = lines.length ? lines.map(l => `<div>${ansi(l.line) || '&nbsp;'}</div>`).join('')
    : `<div class="c90">${termTab === 'engine' ? 'Press RUN to replay the recorded run.' : 'Waiting for ' + termTab + ' output…'}</div>`;
  el.scrollTop = el.scrollHeight;
  document.querySelectorAll('#drawer [data-tab]').forEach(b => b.classList.toggle('on', b.dataset.tab === termTab));
  document.getElementById('hideReads').parentElement.hidden = termTab !== 'hardhat';
  layoutDrawer();
}
function layoutDrawer() {
  const h = drawerOpen ? drawerH : 34;
  document.getElementById('drawer').style.height = h + 'px';
  document.documentElement.style.setProperty('--drawer-h', h + 'px');
  document.getElementById('drawerToggle').textContent = drawerOpen ? '▾' : '▴';
}
function initDrawer() {
  const d = document.getElementById('drawer');
  d.querySelector('.tabs').innerHTML = TABS.map(t => `<button data-tab="${t}">${t} <span class="count">0</span></button>`).join('');
  d.addEventListener('click', e => {
    const b = e.target.closest('[data-tab]');
    if (b) { termTab = b.dataset.tab; drawerOpen = true; termRender(); }
  });
  document.getElementById('hideReads').onchange = e => { hideReads = e.target.checked; termRender(); };
  document.getElementById('drawerToggle').onclick = () => { drawerOpen = !drawerOpen; layoutDrawer(); };
  document.getElementById('drawerGrip').onpointerdown = e => {
    e.preventDefault();
    const move = ev => { drawerOpen = true; drawerH = Math.max(80, Math.min(innerHeight - 120, innerHeight - ev.clientY)); layoutDrawer(); };
    const up = () => { removeEventListener('pointermove', move); removeEventListener('pointerup', up); };
    addEventListener('pointermove', move); addEventListener('pointerup', up);
  };
  termRender();
}

// ---- tooltip, theme, controls ---------------------------------------------

function initTooltip() {
  const tt = document.getElementById('tooltip');
  document.addEventListener('pointerover', e => {
    const n = e.target.closest('[data-node]');
    if (!n) { tt.hidden = true; return; }
    tt.innerHTML = nodeInfo(n.dataset.node); tt.hidden = false;
  });
  document.addEventListener('pointermove', e => {
    if (tt.hidden) return;
    const x = Math.min(e.clientX + 14, innerWidth - tt.offsetWidth - 8), y = e.clientY + 16 + tt.offsetHeight > innerHeight ? e.clientY - tt.offsetHeight - 10 : e.clientY + 16;
    tt.style.transform = `translate(${x}px,${y}px)`;
  });
}

const THEME_KEY = 'shieldDemoTheme';
function setTheme(t) {
  document.documentElement.dataset.theme = t;
  try { localStorage.setItem(THEME_KEY, t); } catch {}
}
function initTheme() {
  let t = new URL(location.href).searchParams.get('theme');
  try { t ||= localStorage.getItem(THEME_KEY); } catch {}
  document.documentElement.dataset.theme = t === 'dark' ? 'dark' : 'light';
}

document.addEventListener('click', e => {
  const a = e.target.closest('[data-act]')?.dataset.act;
  if (a === 'run') replay.start();
  if (a === 'end') replay.toEnd();
  if (a === 'theme') setTheme(document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark');
});
document.addEventListener('change', e => { if (e.target.dataset.act === 'speed') S.speed = +e.target.value; });

// ---- variant switcher (prototype only) ------------------------------------

function mountVariant(key) {
  const v = VARIANTS[key] ? key : Object.keys(VARIANTS)[0];
  const url = new URL(location.href);
  url.searchParams.set('variant', v);
  history.replaceState(null, '', url);
  const app = document.getElementById('app');
  app.className = 'v-' + v;
  app.innerHTML = '';
  S.variant = VARIANTS[v].mount(app);
  S.variant.render(S.state);
  document.getElementById('switchLabel').textContent = `${v} · ${VARIANTS[v].name}`;
}
function cycle(d) {
  const keys = Object.keys(VARIANTS);
  const cur = new URL(location.href).searchParams.get('variant');
  mountVariant(keys[(keys.indexOf(cur) + d + keys.length) % keys.length]);
}
function initSwitcher() {
  document.getElementById('prevV').onclick = () => cycle(-1);
  document.getElementById('nextV').onclick = () => cycle(1);
  addEventListener('keydown', e => {
    if (e.target.closest('input,textarea,select,[contenteditable]')) return;
    if (e.key === 'ArrowLeft') cycle(-1);
    if (e.key === 'ArrowRight') cycle(1);
  });
}

addEventListener('DOMContentLoaded', () => {
  initTheme(); initDrawer(); initTooltip(); initSwitcher();
  mountVariant(new URL(location.href).searchParams.get('variant'));
  if (new URL(location.href).searchParams.has('autoplay')) replay.start();
});
