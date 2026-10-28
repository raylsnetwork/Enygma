// PROTOTYPE (#131) — throwaway. The plumbing shared by the hop detail
// variants: the replay of recording.js into the page's onEvent, the vaults'
// tree state as the diagram plays it, the pruned tree layout, the stepper's
// "show this step again", value formatting and the variant switcher. Each
// variant's layout and drawing is in variants.js.

const VAULT_LABEL = { Erc20CoinVault: 'RaylsERC20 vault', UsdrCoinVault: 'USDr vault' };
const VAULT_IDS = Object.keys(VAULT_LABEL);
const DEPTH = 8, TREE_LEAVES = 256;
const OWNERS = ['Sender', 'Ana', 'Bruno', 'Carla', 'Diego', 'Elena', 'Felipe', 'Relayer'];

// Each detail kind's fixed plain-language sentence.
const SENTENCE = {
  commitment: v => `A new note is hidden as one hash, its commitment, and added as a leaf to the ${VAULT_LABEL[v]}'s Merkle tree. The chain stores the hash, never what is inside.`,
  merklePath: () => `To spend a note, the payer proves its leaf is in the tree: the hashes beside its path climb to a root the vault knows, without saying which leaf it is.`,
  publicInputs: () => `The proof makes public only roots, nullifiers and the new commitments. The spend key, amounts and salts never leave the payer.`,
  settleChecks: () => `Before it settles, EnygmaDvp checks each root is one its vault has had, each nullifier is unused and the proofs verify, then publishes the nullifiers so the notes can't be spent twice.`,
  leavesIn: v => `The new notes land as leaves in the ${VAULT_LABEL[v]}'s tree, all in this one transaction, and the tree gets a new root.`,
};

// ---- values ---------------------------------------------------------------

const shortHex = v => { v = String(v); return v.startsWith('0x') && v.length > 14 ? v.slice(0, 8) + '…' + v.slice(-4) : v; };
// hex is a field element in short hex, the full value on hover, copied on click.
const hex = v => `<span class="hex" data-copy="${esc(v)}" title="${esc(v)} (click to copy)">${esc(shortHex(v))}</span>`;
document.addEventListener('click', e => {
  const el = e.target.closest('[data-copy]');
  if (!el) return;
  navigator.clipboard?.writeText(el.dataset.copy).catch(() => {});
  el.classList.add('copied');
  setTimeout(() => el.classList.remove('copied'), 700);
});

// ---- tree state -------------------------------------------------------------

// trees[vault] = { current, byNum: { n: { leaves: [{commitment, owner, before}], leafCount, root, full } } },
// changed only as the diagram plays a hop, so the tree view matches the diagram.
let trees = {};

function snapshotTrees(snap) {
  trees = {};
  for (const v of snap) {
    const byNum = {};
    for (const f of v.fullTrees) byNum[f.tree] = { leaves: null, leafCount: f.leafCount, root: f.root, full: true };
    byNum[v.tree] = { leaves: v.leaves.map(l => ({ ...l, before: true })), leafCount: v.leafCount, root: v.root, full: false };
    trees[v.vault] = { current: v.tree, byNum };
  }
}

function landLeaves(vault, tree, leaves, root) {
  const t = trees[vault];
  if (tree > t.current) {
    t.byNum[t.current].full = true;
    t.byNum[tree] = { leaves: [], leafCount: 0, root: null, full: false };
    t.current = tree;
  }
  const tr = t.byNum[tree];
  for (const l of leaves) tr.leaves[l.index] = { commitment: l.commitment, owner: l.owner, before: false };
  tr.leafCount = tr.leaves.length;
  if (root) tr.root = root;
}

function applyDetail(d) {
  if (d.kind === 'commitment') landLeaves(d.vault, d.tree, [{ index: d.leaf, commitment: d.commitment, owner: d.owner }], d.rootAfter);
  if (d.kind === 'leavesIn') landLeaves(d.vault, d.tree, d.leaves, d.rootAfter);
}

// focusOf is what a hop's details touch, per vault: the tree, the touched
// leaves, the siblings by "level:index", the root(s) and the kind of touch.
function focusOf(details) {
  const f = {};
  const at = (v, tree, kind) => f[v] ||= { tree, kind, touched: [], siblings: {}, root: null };
  for (const d of details) {
    if (d.kind === 'commitment') { const x = at(d.vault, d.tree, 'insert'); x.touched.push(d.leaf); x.root = d.rootAfter; }
    if (d.kind === 'leavesIn') { const x = at(d.vault, d.tree, 'insert'); x.touched.push(...d.leaves.map(l => l.index)); x.root = d.rootAfter; x.rootBefore = d.rootBefore; }
    if (d.kind === 'merklePath') for (const p of d.paths) {
      const x = at(p.vault, p.tree, 'path');
      x.touched.push(p.leaf); x.root = p.root;
      p.siblings.forEach((h, l) => x.siblings[`${l}:${(p.leaf >> l) ^ 1}`] = h);
    }
    if (d.kind === 'settleChecks') for (const r of d.roots) { const x = at(r.vault, r.tree, 'check'); x.root = r.root; }
  }
  return f;
}

// prunedTree lays out the part of a depth-8 tree above the touched leaves:
// each touched leaf's path to the root, and the sibling beside each step of
// it. x runs 0..1 across the drawn leaves; level 0 is the leaves, 8 the root.
function prunedTree(touched) {
  const path = new Set(), drawn = new Map();
  for (const i of touched) for (let l = 0; l <= DEPTH; l++) path.add(`${l}:${i >> l}`);
  if (!touched.length) path.add(`${DEPTH}:0`);
  for (const k of path) {
    const [l, i] = k.split(':').map(Number);
    drawn.set(k, { level: l, index: i, kind: l === 0 ? 'leaf' : 'path' });
    if (l < DEPTH && touched.length && !path.has(`${l}:${i ^ 1}`)) drawn.set(`${l}:${i ^ 1}`, { level: l, index: i ^ 1, kind: 'sibling' });
  }
  let slot = 0;
  const edges = [];
  (function place(l, i) {
    const n = drawn.get(`${l}:${i}`);
    const kids = l > 0 ? [2 * i, 2 * i + 1].filter(c => drawn.has(`${l - 1}:${c}`)) : [];
    if (!kids.length) { n.x = slot++; return n.x; }
    n.x = kids.map(c => { edges.push([n, drawn.get(`${l - 1}:${c}`)]); return place(l - 1, c); }).reduce((a, b) => a + b) / kids.length;
    return n.x;
  })(DEPTH, 0);
  for (const n of drawn.values()) n.x = slot > 1 ? n.x / (slot - 1) : 0.5;
  return { nodes: [...drawn.values()], edges };
}

// ---- the panel: latest detail, or a finished step's details again ---------

const run = { history: [], front: VAULT_IDS[0], owners: false, revisit: null };

function entryFor(h) {
  const details = h.hopDetail;
  details.forEach(applyDetail);
  const focus = focusOf(details);
  const front = Object.keys(focus)[0] || run.front;
  return { hop: h, details, focus, front, trees: structuredClone(trees), step: h.step, stepLabel: steps.find(s => s.id === h.step)?.label || h.step };
}

const panel = {
  shown(h) {
    if (!h.hopDetail) return;
    const en = entryFor(h);
    run.front = en.front;
    run.history.push(en);
    if (!run.revisit) this.render();
  },
  render() {
    const r = run.revisit;
    const list = r ? run.history.filter(e => e.step === r.step) : run.history;
    const i = r ? r.i : list.length - 1;
    if (!trees[VAULT_IDS[0]]) {
      $('hopPanel').innerHTML = '<div class="inner"><p class="sentence">Press RUN to replay the recorded run. Each hop that changes the protocol state explains itself here, over both vaults’ Merkle trees.</p></div>';
      return;
    }
    VARIANTS[variant].render($('hopPanel'), {
      entry: list[i] || null, history: run.history, owners: run.owners,
      nav: r ? { i, n: list.length, label: list[0]?.stepLabel } : null,
    });
  },
  reset() { run.history = []; run.front = VAULT_IDS[0]; run.revisit = null; this.render(); },
  revisit(step) {
    if (!run.history.some(e => e.step === step)) return;
    run.revisit = { step, i: 0 };
    this.render();
  },
  move(d) {
    const r = run.revisit, n = run.history.filter(e => e.step === r.step).length;
    r.i = (r.i + d + n) % n;
    this.render();
  },
  live() { run.revisit = null; this.render(); },
  toggleOwners() { run.owners = !run.owners; this.render(); },
};
// Variants wire their controls with data-act, so a re-render keeps them working.
document.addEventListener('click', e => {
  const a = e.target.closest('[data-act]')?.dataset.act;
  if (a === 'owners') panel.toggleOwners();
  else if (a === 'prev') panel.move(-1);
  else if (a === 'next') panel.move(1);
  else if (a === 'live') panel.live();
});

// A finished step in the stepper shows its hop details again.
$('steps').addEventListener('click', e => {
  const st = e.target.closest('.st.success, .st.error');
  if (st) panel.revisit(st.id.replace(/^step-/, ''));
});

// ---- replay -----------------------------------------------------------------

const speedSel = Object.assign(document.createElement('select'), { id: 'speed', title: 'Replay speed', innerHTML: '<option value="1">1×</option><option value="2">2×</option><option value="4">4×</option>' });
const skipBtn = Object.assign(document.createElement('button'), { id: 'skipBtn', textContent: '⏭', title: 'Jump to the end of the run' });
$('runBtn').before(speedSel, skipBtn);

const replay = {
  gen: 0,
  start() {
    const gen = ++this.gen;
    clearServices();
    onEvent({ kind: 'reset' });
    panel.reset();
    $('runBtn').disabled = true;
    Object.assign(this, { i: 0, clock: 0, last: performance.now() });
    const tick = () => {
      if (gen !== this.gen) return;
      const now = performance.now();
      this.clock += (now - this.last) * Number(speedSel.value); this.last = now;
      this.advance(this.clock);
      if (this.i < RECORDING.events.length) setTimeout(tick, 16);
    };
    tick();
  },
  // advance plays the events up to upTo; skipping, hops skip the diagram
  // and go straight to the panel.
  advance(upTo, skipping) {
    const ev = RECORDING.events;
    while (this.i < ev.length && ev[this.i].t <= upTo) {
      const e = ev[this.i++];
      const ts = '+' + (e.t / 1000).toFixed(3).padStart(7) + 's';
      if (e.kind === 'proto' && e.field === 'trees') { snapshotTrees(JSON.parse(e.value)); panel.render(); continue; }
      if (skipping && e.kind === 'hop') { panel.shown(e); continue; }
      onEvent({ ...e, ts });
    }
  },
  // skipTo jumps to time t (Infinity: the end) without playing the hops
  // before it, then plays on from there.
  skipTo(t) {
    if (!this.gen) this.start();
    diagram.reset();
    this.advance(t, true);
    this.clock = Math.max(this.clock, t);
    panel.render();
  },
};
// ?from=<step id> starts the replay at that step, e.g. ?from=transfer_6.
const stepStart = id => RECORDING.events.find(e => e.kind === 'step_started' && e.step === id)?.t;
$('runBtn').onclick = () => replay.start();
skipBtn.onclick = () => replay.skipTo(Infinity);

// ---- variant switcher ---------------------------------------------------------

const params = new URLSearchParams(location.search);
let variant = 'A';
function setVariant(v) {
  variant = v;
  document.body.dataset.variant = v;
  const u = new URL(location.href);
  u.searchParams.set('variant', v);
  history.replaceState(null, '', u);
  $('protoLabel').textContent = `${v} · ${VARIANTS[v].name}`;
  panel.render();
}
function cycle(d) { const ks = Object.keys(VARIANTS); setVariant(ks[(ks.indexOf(variant) + d + ks.length) % ks.length]); }
document.body.insertAdjacentHTML('beforeend',
  `<div id="protoBar"><span class="tag">PROTOTYPE #131</span><button data-cycle="-1" aria-label="Previous variant">←</button><span id="protoLabel"></span><button data-cycle="1" aria-label="Next variant">→</button></div>`);
$('protoBar').onclick = e => { const b = e.target.closest('[data-cycle]'); if (b) cycle(Number(b.dataset.cycle)); };
addEventListener('keydown', e => {
  if (e.target.closest('input, textarea, select, [contenteditable]')) return;
  if (e.key === 'ArrowLeft') cycle(-1);
  if (e.key === 'ArrowRight') cycle(1);
});
