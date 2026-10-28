// PROTOTYPE (#124) — throwaway. Three light-theme variants of the agreed
// layout (control bar · diagram · timeline beside panels · terminal drawer).
// Each mount() owns its whole layout and drawing; shared.js only feeds it
// state (render) and paced hops (hop).

const SVGNS = 'http://www.w3.org/2000/svg';
function svg(tag, attrs = {}, parent) {
  const el = document.createElementNS(SVGNS, tag);
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v);
  if (parent) parent.appendChild(el);
  return el;
}
const ACTOR_IDS = NODES.filter(n => n.group === 'actor').map(n => n.id);
const CONTRACT_IDS = ['registry', 'vault', 'dvp', 'usdrVault', 'tagChan', 'tagReg'];
const statusText = st => ({ idle: 'ready', running: 'running', done: 'complete', failed: 'failed' }[st.status]);
const currentStep = st => st.steps.find(s => s.id === st.current);
const actorSub = (st, id) => {
  if (!/^a\d$/.test(id)) return '';
  const f = st.actors[+id.slice(1)].f;
  return f.balance ? f.balance.replace(' RaylsERC20', '') + (f.usdr ? ' · ' + f.usdr.replace(' USDr', 'U') : '')
    : f.status || (f.pkSpend ? 'keys' : '');
};
// Scrolls only `box` (which must be positioned) to show `el`, never the page.
function keepInView(box, el) {
  if (!el) return;
  if (el.offsetTop < box.scrollTop || el.offsetTop + el.offsetHeight > box.scrollTop + box.clientHeight)
    box.scrollTop = el.offsetTop - box.clientHeight / 3;
}
// Clears a node's or edge's highlight after `ms`, unless re-lit meanwhile.
function lightFor(el, cls, ms) {
  el.classList.add(cls);
  clearTimeout(el._t);
  el._t = setTimeout(() => el.classList.remove(cls), ms);
}

// ===========================================================================
// A · Lanes — swimlanes (actors / services / contracts), straight edges with
// label pills, vertical timeline, actor cards.
// ===========================================================================
function mountA(root) {
  const c = controls();
  root.innerHTML = `
  <header class="bar">
    <div class="brand">ENYGMA<span>Retail shield demo</span></div>
    <div class="grp">${c.run}${c.end}${c.speed}</div>
    <div class="grp opts">${c.options.join('')}</div>
    <div class="status"><i class="dot"></i><span id="aStatus"></span></div>
    ${c.theme}
  </header>
  <section class="diagram"><svg viewBox="0 0 1200 340" id="aSvg"></svg></section>
  <div class="progress"><div id="aProg"></div></div>
  <main>
    <aside class="timeline"><h2>Steps</h2><ol id="aSteps"></ol></aside>
    <section class="panels">
      <h2>Actors</h2><div class="cards" id="aCards"></div>
      <div class="row2">
        <div class="box"><h2>Auditor</h2><div id="aAudit"></div></div>
        <div class="box"><h2>Send</h2>${sendForm()}<h2>Totals</h2><p id="aTotals" class="hint">—</p></div>
      </div>
    </section>
  </main>`;
  const s = root.querySelector('#aSvg');
  const pos = {};
  ACTOR_IDS.forEach((id, i) => pos[id] = { x: 160 + i * 110, y: 52, w: 96, h: 40 });
  pos.gnark = { x: 470, y: 172, w: 170, h: 46 };
  pos.relayer = { x: 830, y: 172, w: 170, h: 46 };
  CONTRACT_IDS.forEach((id, i) => pos[id] = { x: 150 + i * 190, y: 292, w: 160, h: 40 });
  [['Actors', 6, 96], ['Services', 124, 96], ['Contracts', 244, 96]].forEach(([t, y, h], i) => {
    svg('rect', { x: 4, y, width: 1192, height: h, rx: 10, class: 'lane l' + i }, s);
    svg('text', { x: 16, y: y + h / 2 + 4, class: 'lane-label' }, s).textContent = t.toUpperCase();
  });
  const gE = svg('g', {}, s), gN = svg('g', {}, s), gL = svg('g', {}, s);
  const nodeEl = {}, subEl = {};
  for (const n of NODES) {
    const p = pos[n.id];
    const g = svg('g', { class: 'node ' + n.group, 'data-node': n.id, transform: `translate(${p.x},${p.y})` }, gN);
    svg('rect', { x: -p.w / 2, y: -p.h / 2, width: p.w, height: p.h, rx: 7 }, g);
    svg('text', { y: -2, class: 'nl' }, g).textContent = n.label;
    subEl[n.id] = svg('text', { y: 12, class: 'ns' }, g);
    nodeEl[n.id] = g;
  }
  const edges = {};
  function edge(a, b) {
    const k = a + '>' + b;
    if (edges[k]) return edges[k];
    const p = pos[a], q = pos[b];
    let d;
    if (p.y === q.y) d = `M${p.x},${p.y + p.h / 2} Q${(p.x + q.x) / 2},${p.y + p.h / 2 + 44} ${q.x},${q.y + q.h / 2}`;
    else {
      const down = q.y > p.y;
      d = `M${p.x},${p.y + (down ? p.h / 2 : -p.h / 2)} L${q.x},${q.y + (down ? -q.h / 2 : q.h / 2)}`;
    }
    return edges[k] = svg('path', { d, class: 'edge', 'marker-end': 'url(#aArrow)' }, gE);
  }
  const defs = svg('defs', {}, s);
  const mk = svg('marker', { id: 'aArrow', viewBox: '0 0 10 10', refX: 9, refY: 5, markerWidth: 7, markerHeight: 7, orient: 'auto-start-reverse' }, defs);
  svg('path', { d: 'M0,0 L10,5 L0,10 z', class: 'arrow' }, mk);

  function pill(x, y, text, ms, failed) {
    const g = svg('g', { class: 'pill' + (failed ? ' failed' : ''), transform: `translate(${x},${y})` }, gL);
    const t = svg('text', { y: 4 }, g); t.textContent = text;
    const w = t.getComputedTextLength() + 14;
    g.insertBefore(svg('rect', { x: -w / 2, y: -10, width: w, height: 20, rx: 10 }), t);
    setTimeout(() => g.remove(), ms);
  }
  return {
    reset() { gE.innerHTML = ''; gL.innerHTML = ''; for (const k in edges) delete edges[k]; },
    hop(batch, hold) {
      batch.forEach((h, i) => {
        lightFor(nodeEl[h.from], 'lit', hold * 1.4);
        if (h.from === h.to) { pill(pos[h.from].x, pos[h.from].y - 34, h.label, hold * 1.4, h.failed); return; }
        lightFor(nodeEl[h.to], 'lit', hold * 1.4);
        const e = edge(h.from, h.to);
        e.classList.add('used'); e.classList.toggle('failed', !!h.failed);
        lightFor(e, 'active', hold * 1.4);
        const p = pos[h.from], q = pos[h.to];
        pill((p.x + q.x) / 2, p.y === q.y ? p.y + 52 : (p.y + q.y) / 2 + i * 22, h.label, hold * 1.4, h.failed);
      });
    },
    render(st) {
      root.querySelector('#aStatus').textContent = statusText(st);
      root.querySelector('.status').dataset.s = st.status;
      root.querySelector('#aProg').style.width = progress(st) * 100 + '%';
      root.querySelector('#aSteps').innerHTML = st.steps.map(x =>
        `<li class="${x.status}"><b>${esc(x.label)}</b>${x.msg && x.status !== 'pending' ? `<small>${esc(x.msg)}</small>` : ''}</li>`).join('');
      keepInView(root.querySelector('.timeline'), root.querySelector('#aSteps .running'));
      root.querySelector('#aCards').innerHTML = st.actors.map(a => `
        <div class="card" data-node="a${a.id}"><div class="nm">${esc(a.name)} <em>${esc(a.f.role || '')}</em></div>
          <dl><dt>Balance</dt><dd>${esc(a.f.balance || a.f.public || '—')}</dd><dt>USDr</dt><dd>${esc(a.f.usdr || '—')}</dd>
          <dt>Status</dt><dd>${esc(a.f.status || (a.f.pkSpend ? 'keys made' : '—'))}</dd><dt>Scan</dt><dd>${esc(a.f.scan || a.f.channels || '—')}</dd></dl></div>`).join('');
      root.querySelector('#aAudit').innerHTML = auditTable(st);
      root.querySelector('#aTotals').textContent = totalsLine(st) || '—';
      for (const id in subEl) subEl[id].textContent = actorSub(st, id);
    },
  };
}

// ===========================================================================
// B · Flow — left-to-right columns (off-chain → services → on-chain), curved
// edges with a travelling packet, a "now" caption; stepper + ledger table.
// ===========================================================================
function mountB(root) {
  const c = controls();
  root.innerHTML = `
  <header class="bar">
    <h1>Enygma <span>retail shield · live</span></h1>
    <div class="seg">${c.run}${c.end}${c.speed}</div>
    <details class="opts"><summary>Options</summary><div>${c.options.join('')}</div></details>
    <span class="chip" id="bStatus"></span>${c.theme}
  </header>
  <section class="diagram">
    <div class="cols"><span>Off-chain parties</span><span>Services</span><span>On-chain contracts</span></div>
    <svg viewBox="0 0 1200 420" id="bSvg"></svg>
    <div class="caption"><span class="now" id="bNow">The diagram follows the run — press RUN.</span><span class="q" id="bQ"></span></div>
  </section>
  <main>
    <aside class="stepper" id="bSteps"></aside>
    <section class="ledger">
      <table><thead><tr><th>Actor</th><th>Role</th><th>Status</th><th class="num">RaylsERC20</th><th class="num">USDr</th><th>Discovery</th></tr></thead><tbody id="bRows"></tbody></table>
      <div class="split"><div><h3>Auditor’s view</h3><div id="bAudit"></div></div><div><h3>Send</h3>${sendForm()}<h3>Run totals</h3><p id="bTotals" class="hint">—</p></div></div>
    </section>
  </main>`;
  const s = root.querySelector('#bSvg');
  const pos = {};
  ACTOR_IDS.forEach((id, i) => pos[id] = { x: 120, y: 30 + i * 40, w: 170, h: 30 });
  pos.gnark = { x: 600, y: 130, w: 200, h: 64 };
  pos.relayer = { x: 600, y: 290, w: 200, h: 64 };
  CONTRACT_IDS.forEach((id, i) => pos[id] = { x: 1070, y: 45 + i * 66, w: 210, h: 40 });
  const gE = svg('g', {}, s), gN = svg('g', {}, s), gP = svg('g', {}, s);
  const nodeEl = {}, subEl = {};
  for (const n of NODES) {
    const p = pos[n.id];
    const g = svg('g', { class: 'node ' + n.group, 'data-node': n.id, transform: `translate(${p.x},${p.y})` }, gN);
    svg('rect', { x: -p.w / 2, y: -p.h / 2, width: p.w, height: p.h, rx: n.group === 'service' ? 32 : 6 }, g);
    svg('text', { x: n.group === 'actor' ? -p.w / 2 + 12 : 0, y: n.group === 'service' ? -2 : 4, class: 'nl', 'text-anchor': n.group === 'actor' ? 'start' : 'middle' }, g).textContent = n.label;
    subEl[n.id] = svg('text', { x: n.group === 'actor' ? p.w / 2 - 10 : 0, y: n.group === 'service' ? 16 : 4, class: 'ns', 'text-anchor': n.group === 'actor' ? 'end' : 'middle' }, g);
    nodeEl[n.id] = g;
  }
  const edges = {};
  function path(a, b) {
    const p = pos[a], q = pos[b];
    if (a === b) {
      const x = p.x + p.w / 2, y = p.y;
      return `M${x},${y - 8} C${x + 46},${y - 30} ${x + 46},${y + 30} ${x},${y + 8}`;
    }
    const right = q.x > p.x, same = q.x === p.x;
    const x1 = p.x + (same ? p.w / 2 : right ? p.w / 2 : -p.w / 2), x2 = q.x + (same ? q.w / 2 : right ? -q.w / 2 : q.w / 2);
    const bend = same ? 80 : Math.abs(x2 - x1) / 2;
    return `M${x1},${p.y} C${x1 + (right || same ? bend : -bend)},${p.y} ${x2 + (right ? -bend : bend)},${q.y} ${x2},${q.y}`;
  }
  function edge(a, b) {
    const k = a + '>' + b;
    return edges[k] ||= svg('path', { d: path(a, b), class: 'edge' }, gE);
  }
  function packet(d, ms, failed) {
    const dot = svg('circle', { r: 6, class: 'packet' + (failed ? ' failed' : '') }, gP);
    const am = svg('animateMotion', { dur: Math.max(ms, 120) + 'ms', path: d, fill: 'freeze', begin: 'indefinite' }, dot);
    am.beginElement();
    setTimeout(() => dot.remove(), ms + 250);
  }
  return {
    reset() { gE.innerHTML = ''; for (const k in edges) delete edges[k]; },
    hop(batch, hold) {
      for (const h of batch) {
        const e = edge(h.from, h.to);
        e.classList.add('used'); e.classList.toggle('failed', !!h.failed);
        lightFor(e, 'active', hold * 1.3);
        lightFor(nodeEl[h.from], 'lit', hold * 1.3);
        lightFor(nodeEl[h.to], 'lit', hold * 1.3);
        packet(e.getAttribute('d'), hold * 0.9, h.failed);
      }
      const h = batch[0];
      root.querySelector('#bNow').innerHTML = batch.map(h =>
        `<b>${esc(NODE[h.from].label)}</b> ${h.from === h.to ? '' : `→ <b>${esc(NODE[h.to].label)}</b>`} <span class="lbl ${h.failed ? 'failed' : ''}">${esc(h.label)}</span> <small>${esc(h.detail)}</small>`).join('<br>');
      root.querySelector('#bQ').textContent = S.state.queued ? `+${S.state.queued} queued` : '';
    },
    render(st) {
      root.querySelector('#bStatus').textContent = statusText(st);
      root.querySelector('#bStatus').dataset.s = st.status;
      const done = st.steps.filter(x => x.status === 'done').length;
      root.querySelector('#bSteps').innerHTML = `<div class="count">${done} / ${st.steps.length}</div>` + st.steps.map((x, i) =>
        `<div class="st ${x.status}"><span class="n">${x.status === 'done' ? '✓' : x.status === 'failed' ? '✗' : i + 1}</span><div><b>${esc(x.label)}</b>${x.status === 'running' || x.status === 'failed' ? `<p>${esc(x.msg)}</p>` : ''}</div></div>`).join('');
      keepInView(root.querySelector('#bSteps'), root.querySelector('#bSteps .running'));
      root.querySelector('#bRows').innerHTML = st.actors.map(a => `<tr data-node="a${a.id}"><td><b>${esc(a.name)}</b></td><td>${esc(a.f.role || '')}</td>
        <td>${esc(a.f.status || (a.f.pkSpend ? 'keys made' : '—'))}</td><td class="num">${esc((a.f.balance || a.f.public || '—').replace(' RaylsERC20', ''))}</td>
        <td class="num">${esc((a.f.usdr || '—').replace(' USDr', ''))}</td><td>${esc(a.f.scan || a.f.channels || '')}</td></tr>`).join('');
      root.querySelector('#bAudit').innerHTML = auditTable(st);
      root.querySelector('#bTotals').textContent = totalsLine(st) || '—';
      for (const id in subEl) subEl[id].textContent = id === 'relayer' && st.proto.relayerUsdrIncome ? 'earned ' + st.proto.relayerUsdrIncome : actorSub(st, id);
    },
  };
}

// ===========================================================================
// C · Sequence — a sequence diagram: every node is a lifeline, each hop an
// arrow added at the bottom (history scrolls up); narrative feed + tiles.
// ===========================================================================
function mountC(root) {
  const c = controls();
  const SHORT = { registry: 'User­Registry', vault: 'Rayls vault', usdrVault: 'USDr vault', dvp: 'EnygmaDvp', tagChan: 'Tag channels', tagReg: 'Tag registry', gnark: 'gnark', relayer: 'Relayer', funder: 'Gas funder' };
  const cols = [...ACTOR_IDS, 'gnark', 'relayer', ...CONTRACT_IDS];
  const W = 1200, CW = W / cols.length, ROW = 30, ROWS = 11;
  const cx = id => (cols.indexOf(id) + 0.5) * CW;
  root.innerHTML = `
  <header class="bar">
    ${c.run}
    <div class="mid"><div class="t"><b>Retail shield</b> <span id="cStep">ready</span></div><div class="progress"><div id="cProg"></div></div></div>
    <div class="right">${c.end}${c.speed}<details class="opts"><summary>⚙</summary><div>${c.options.join('')}</div></details>${c.theme}</div>
  </header>
  <section class="diagram">
    <div class="heads" style="grid-template-columns:repeat(${cols.length},1fr)">
      <div class="band actor" style="grid-column:1/${ACTOR_IDS.length + 1}">actors</div>
      <div class="band service" style="grid-column:${ACTOR_IDS.length + 1}/${ACTOR_IDS.length + 3}">services</div>
      <div class="band contract" style="grid-column:${ACTOR_IDS.length + 3}/-1">contracts</div>
      ${cols.map(id => `<div class="head ${NODE[id].group}" data-node="${id}"><b>${esc(SHORT[id] || NODE[id].label)}</b><small data-sub="${id}"></small></div>`).join('')}
    </div>
    <svg viewBox="0 0 ${W} ${ROW * ROWS + 10}" id="cSvg" preserveAspectRatio="none"></svg>
  </section>
  <main>
    <aside class="feed" id="cFeed"></aside>
    <section class="tiles-wrap"><div class="tiles" id="cTiles"></div>
      <div class="pair"><div class="box"><h3>Auditor</h3><div id="cAudit"></div></div><div class="box"><h3>Send</h3>${sendForm()}<h3>Totals</h3><p id="cTotals" class="hint">—</p></div></div>
    </section>
  </main>`;
  const s = root.querySelector('#cSvg');
  const defs = svg('defs', {}, s);
  const mk = svg('marker', { id: 'cArrow', viewBox: '0 0 10 10', refX: 9, refY: 5, markerWidth: 6, markerHeight: 6, orient: 'auto' }, defs);
  svg('path', { d: 'M0,0 L10,5 L0,10 z', class: 'arrow' }, mk);
  const gLife = svg('g', {}, s), gRows = svg('g', {}, s);
  for (const id of cols) svg('line', { x1: cx(id), x2: cx(id), y1: 0, y2: ROW * ROWS + 10, class: 'life ' + NODE[id].group }, gLife);
  let history = [], batchNo = 0;
  function drawRows() {
    gRows.innerHTML = '';
    history.slice(-ROWS).forEach((h, i, arr) => {
      const y = 18 + i * ROW, age = batchNo - h.batch;
      const g = svg('g', { class: 'msg' + (age === 0 ? ' now' : '') + (h.failed ? ' failed' : ''), opacity: Math.max(0.28, 1 - age * 0.12) }, gRows);
      const x1 = cx(h.from), x2 = cx(h.to);
      if (h.from === h.to) {
        svg('path', { d: `M${x1},${y - 6} h18 v12 h-16`, class: 'line', 'marker-end': 'url(#cArrow)' }, g);
        const t = svg('text', { x: x1 + 24, y: y + 4, class: 'lbl', 'text-anchor': 'start' }, g);
        t.textContent = h.label;
      } else {
        svg('line', { x1, x2: x2 + (x2 > x1 ? -4 : 4), y1: y, y2: y, class: 'line', 'marker-end': 'url(#cArrow)' }, g);
        svg('circle', { cx: x1, cy: y, r: 3, class: 'dotc' }, g);
        const t = svg('text', { x: (x1 + x2) / 2, y: y - 5, class: 'lbl' }, g);
        t.textContent = h.label;
      }
      svg('title', {}, g).textContent = `${NODE[h.from].label} → ${NODE[h.to].label}: ${h.label} — ${h.detail}`;
    });
  }
  return {
    reset() { history = []; batchNo = 0; drawRows(); },
    hop(batch) {
      batchNo++;
      for (const h of batch) history.push({ ...h, batch: batchNo });
      if (history.length > 200) history = history.slice(-ROWS);
      drawRows();
      for (const h of batch) for (const id of [h.from, h.to]) {
        const el = root.querySelector(`.head[data-node="${id}"]`);
        if (el) lightFor(el, 'lit', 900);
      }
    },
    render(st) {
      const cur = currentStep(st), i = st.steps.indexOf(cur);
      root.querySelector('#cStep').textContent = cur ? `· step ${i + 1}/${st.steps.length} · ${cur.label}` : '· ' + statusText(st);
      root.querySelector('#cProg').style.width = progress(st) * 100 + '%';
      const shown = st.steps.filter(x => x.status !== 'pending'), next = st.steps.filter(x => x.status === 'pending').slice(0, 3);
      root.querySelector('#cFeed').innerHTML = (shown.length ? '' : '<p class="hint">The story of the run appears here, step by step.</p>') +
        shown.map(x => `<article class="${x.status}"><header>${x.status === 'done' ? '✓' : x.status === 'failed' ? '✗' : '●'} ${esc(x.label)}</header><p>${esc(x.msg)}</p></article>`).join('') +
        (next.length ? `<div class="next">Up next: ${next.map(x => esc(x.label)).join(' · ')}</div>` : '');
      const feed = root.querySelector('#cFeed'); feed.scrollTop = feed.scrollHeight;
      root.querySelector('#cTiles').innerHTML = st.actors.map(a => {
        const bal = (a.f.balance || '').replace(' RaylsERC20', '');
        return `<div class="tile ${a.f.role || ''}" data-node="a${a.id}"><div class="av">${a.name[0]}</div><div class="who"><b>${esc(a.name)}</b><span>${esc(a.f.role || '')}</span></div>
          <div class="big">${bal || '—'}<small>${bal ? 'RaylsERC20' : ''}</small></div>
          <div class="chips">${a.f.usdr ? `<span>${esc(a.f.usdr)}</span>` : ''}${a.f.status ? `<span class="ok">${esc(a.f.status)}</span>` : ''}${a.f.scan ? `<span>${esc(a.f.scan)}</span>` : ''}</div></div>`;
      }).join('');
      root.querySelector('#cAudit').innerHTML = auditTable(st);
      root.querySelector('#cTotals').textContent = totalsLine(st) || '—';
      root.querySelectorAll('[data-sub]').forEach(el => el.textContent = actorSub(st, el.dataset.sub));
    },
  };
}

const VARIANTS = {
  A: { name: 'Lanes', mount: mountA },
  B: { name: 'Flow', mount: mountB },
  C: { name: 'Sequence', mount: mountC },
};
