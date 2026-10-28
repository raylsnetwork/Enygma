// PROTOTYPE (#131) — throwaway. Three hop detail panel + tree view variants,
// one render(el, view) each; view is { entry, history, owners, nav }.
//
//   A · Inspector   a sidebar right of the diagram: sentence, the vault in
//                   front with its pruned tree and a 2-row leaf strip, the
//                   other vault as one thin strip, real values folded away.
//   B · Tree stage  half the width: both vaults side by side as big trees
//                   with values written on the nodes, the vault not touched
//                   dimmed, a public | private table under them.
//   C · Feed        a column left of the diagram: a dock with a sideways tree
//                   and 16×16 leaf tiles for both vaults, then a feed of
//                   detail cards, the newest open, older ones one line each.

// ---- shared drawing ----------------------------------------------------------

// idleEntry is the view before the first hop detail: the vaults as the run's
// tree snapshot left them.
const idleEntry = () => ({ hop: null, details: [], focus: {}, front: run.front, trees: structuredClone(trees), stepLabel: '' });

const ownerStyle = o => o ? ` style="--o:var(--own-${o})"` : '';

// treeOf is the tree a vault shows: the one the hop touches, else its current one.
function treeOf(en, v) {
  const t = en.trees[v];
  if (!t) return null;
  const num = en.focus[v]?.tree ?? t.current;
  return { num, ...t.byNum[num], current: t.current, fullTrees: Object.entries(t.byNum).filter(([n, x]) => x.full).map(([n, x]) => ({ n: +n, ...x })) };
}

// treeSvg draws the pruned tree above a vault's touched leaves. orient is
// 'down' (root on top) or 'right' (root on the left); labels writes the
// hashes on the nodes instead of only on hover.
function treeSvg(en, v, { w, h, orient = 'down', labels = false, owners }) {
  const f = en.focus[v], t = treeOf(en, v);
  const { nodes, edges } = prunedTree(f ? f.touched : []);
  const pad = labels ? 34 : 14;
  const xy = n => orient === 'down'
    ? [pad + n.x * (w - 2 * pad), 14 + (DEPTH - n.level) / DEPTH * (h - 30)]
    : [16 + (DEPTH - n.level) / DEPTH * (w - (labels ? 110 : 40)), 10 + n.x * (h - 20)];
  const hashOf = n => n.level === DEPTH ? f?.root || t?.root
    : n.kind === 'sibling' ? f?.siblings[`${n.level}:${n.index}`]
    : n.level === 0 ? t?.leaves?.[n.index]?.commitment : null;
  const out = [`<svg class="tree" viewBox="0 0 ${w} ${h}" width="100%" preserveAspectRatio="xMidYMid meet">`];
  for (const [a, b] of edges) {
    const [x1, y1] = xy(a), [x2, y2] = xy(b);
    out.push(`<line class="e ${b.kind === 'sibling' ? 'sib' : f?.kind || ''}" x1="${x1}" y1="${y1}" x2="${x2}" y2="${y2}"/>`);
  }
  for (const n of nodes) {
    const [x, y] = xy(n), hv = hashOf(n), tip = hv ? `<title>${esc(hv)}</title>` : '';
    const leaf = n.level === 0 ? t?.leaves?.[n.index] : null;
    const cls = n.level === DEPTH ? 'root' : n.kind === 'leaf' ? `leaf ${f?.kind || ''}` : n.kind;
    const copy = hv ? ` data-copy="${esc(hv)}"` : '';
    if (n.kind === 'leaf') out.push(`<rect class="n ${cls}${owners && leaf?.owner ? ' owned' : ''}"${owners && leaf ? ownerStyle(leaf.owner) : ''}${copy} x="${x - 6}" y="${y - 6}" width="12" height="12" rx="2">${tip}</rect>`);
    else out.push(`<circle class="n ${cls}"${copy} cx="${x}" cy="${y}" r="${n.level === DEPTH ? 7 : n.kind === 'sibling' ? 3.5 : 4.5}">${tip}</circle>`);
    const below = orient === 'down';
    if (n.level === DEPTH) out.push(`<text class="t root" x="${x + (below ? 0 : -4)}" y="${below ? y - 10 + 2 : y - 11}" text-anchor="${below ? 'middle' : 'start'}">root ${labels && hv ? shortHex(hv) : ''}</text>`);
    if (n.kind === 'leaf') out.push(`<text class="t" x="${below ? x : x + 10}" y="${below ? y + 18 : y + 3}" text-anchor="${below ? 'middle' : 'start'}">#${n.index}${labels && hv ? ' ' + shortHex(hv) : ''}${owners && leaf?.owner ? ' · ' + leaf.owner : ''}</text>`);
    if (labels && n.kind === 'sibling' && hv) out.push(`<text class="t sib" x="${below ? x : x + 7}" y="${below ? y + 12 : y + 3}" text-anchor="${below ? 'middle' : 'start'}">${shortHex(hv).slice(0, 7)}</text>`);
  }
  if (!f) out.push(`<text class="t idle" x="${w / 2}" y="${h / 2 + 24}" text-anchor="middle">not touched by this hop</text>`);
  out.push('</svg>');
  return out.join('');
}

// strip is a vault tree's 256 leaves as cells: empty, before this run, this
// run, and touched by this hop.
function strip(en, v, { cols = 128, owners, cls = '' }) {
  const t = treeOf(en, v), f = en.focus[v];
  const touched = new Set(f ? f.touched : []);
  const cells = [];
  for (let i = 0; i < TREE_LEAVES; i++) {
    const l = t.leaves ? t.leaves[i] : i < t.leafCount ? { before: true } : null;
    const c = !l ? 'empty' : touched.has(i) ? 'hot' : l.before ? 'before' : 'run';
    cells.push(`<i class="${c}${owners && l?.owner ? ' owned' : ''}"${owners && l?.owner ? ownerStyle(l.owner) : ''} title="leaf #${i}${l?.commitment ? ' · ' + esc(l.commitment) : l ? ' · before this run' : ' · empty'}${owners && l?.owner ? ' · demo: ' + l.owner : ''}"></i>`);
  }
  return `<div class="strip ${cls}" style="--cols:${cols}">${cells.join('')}</div>`;
}

function vaultHead(en, v) {
  const t = treeOf(en, v), filled = t.leaves ? t.leaves.length : t.leafCount;
  const chips = t.fullTrees.filter(x => x.n !== t.num).map(x => `<span class="tchip" title="tree ${x.n} is full · root ${esc(x.root || '')}">tree ${x.n} · full</span>`).join('');
  const note = t.num !== t.current ? ` <span class="older">a full tree; new leaves go to tree ${t.current}</span>` : '';
  return `<div class="vhead"><b>${VAULT_LABEL[v]}</b> <span class="tnum">tree ${t.num}</span> <span class="fill">${filled} / ${TREE_LEAVES}</span>${chips}${note}</div>`;
}

function ownersToggle(owners) {
  return `<label class="ownersTgl"><input type="checkbox" data-act="owners"${owners ? ' checked' : ''}> demo's view: show owners</label>` +
    (owners ? `<div class="ownersNote">Owners are known only to the demo, which holds every actor's keys. The chain sees only hashes.</div>` : '');
}

function navBar(nav) {
  if (!nav) return '';
  return `<div class="pnav"><button data-act="prev">‹</button><span>${esc(nav.label)} · detail ${nav.i + 1} / ${nav.n}</span><button data-act="next">›</button><button class="live" data-act="live">● back to live</button></div>`;
}

const hopTitle = h => h ? `<span class="hlbl">${esc(h.label)}</span> <span class="hstep">${esc(NODE[h.from].label)} → ${esc(NODE[h.to].label)}</span>` : '';
const sentences = en => en.details.map(d => SENTENCE[d.kind](d.vault || d.paths?.[0]?.vault));

const priv = s => `<span class="priv">${esc(s)}</span>`;
const pub = s => `<span class="pub">${s}</span>`;

// rowsFor is a detail's real values as [label, value html] rows.
function rowsFor(d, owners) {
  const who = o => owners && o ? ` <span class="who"${ownerStyle(o)}>demo: ${esc(o)}</span>` : '';
  switch (d.kind) {
  case 'commitment': return [
    ['leaf', `${VAULT_LABEL[d.vault]} · tree ${d.tree} · #${d.leaf}`],
    ['commitment · public', hex(d.commitment) + who(d.owner)],
    ['opening · private', d.opening.map(priv).join(' ')],
    ['root after', hex(d.rootAfter)]];
  case 'merklePath': return d.paths.flatMap((p, i) => [
    [`path ${d.paths.length > 1 ? i + 1 + ' ' : ''}leaf`, `${VAULT_LABEL[p.vault]} · tree ${p.tree} · #${p.leaf} ${hex(p.commitment)}${who(p.owner)}`],
    ['siblings, leaf → root', `<span class="sibs">${p.siblings.map(hex).join(' ')}</span>`],
    ['root', hex(p.root)]]);
  case 'publicInputs': return [
    ...d.public.map(x => [x.name, pub(String(x.value).startsWith('0x') ? hex(x.value) : esc(x.value))]),
    ['private witness', d.private.map(priv).join(' ')]];
  case 'settleChecks': return [
    ...d.roots.map(r => [`root · ${VAULT_LABEL[r.vault]} tree ${r.tree}`, `${hex(r.root)} <span class="ok">✓ a root the vault has had</span>`]),
    ...d.nullifiers.map(n => [`nullifier · ${VAULT_LABEL[n.vault]}`, `${hex(n.nullifier)} <span class="ok">✓ unused → published</span>`]),
    ['proofs', d.proofs.map(p => `${esc(p)} <span class="ok">✓ verifies</span>`).join(' · ')]];
  case 'leavesIn': return [
    ...d.leaves.map(l => [`new leaf · tree ${d.tree} #${l.index}`, hex(l.commitment) + who(l.owner)]),
    ['root before', hex(d.rootBefore)], ['root after', hex(d.rootAfter)],
    ...(d.treeStarted ? [['tree', `<span class="ok">tree ${d.tree} started: the batch would have filled tree ${d.tree - 1}</span>`]] : [])];
  }
  return [];
}
const rowsHtml = rows => `<div class="rows">${rows.map(([k, v]) => `<div class="row"><span class="k">${esc(k)}</span><span class="v">${v}</span></div>`).join('')}</div>`;

// ---- A · Inspector -------------------------------------------------------------

let aValuesOpen = false;
document.addEventListener('toggle', e => { if (e.target.id === 'aVals') aValuesOpen = e.target.open; }, true);

const VARIANTS = {
  A: {
    name: 'Inspector',
    render(el, { entry, owners, nav }) {
      const en = entry || idleEntry();
      const back = VAULT_IDS.find(v => v !== en.front);
      el.innerHTML = `<div class="inner a">
        ${navBar(nav)}
        <div class="ahead">${entry ? hopTitle(en.hop) : '<span class="hstep">Hop detail</span>'}</div>
        <p class="sentence">${entry ? sentences(en).map(esc).join(' ') : 'Press RUN. Each hop that changes the protocol state explains itself here, over both vaults’ real Merkle trees.'}</p>
        <div class="vcard front">${vaultHead(en, en.front)}${treeSvg(en, en.front, { w: 380, h: 170, owners })}${strip(en, en.front, { cols: 64, owners })}</div>
        <div class="vcard back">${vaultHead(en, back)}${strip(en, back, { cols: 256, owners, cls: 'thin' })}</div>
        ${ownersToggle(owners)}
        ${entry ? `<details id="aVals"${aValuesOpen ? ' open' : ''}><summary>Real values</summary>${en.details.map(d => rowsHtml(rowsFor(d, owners))).join('')}</details>` : ''}
      </div>`;
    },
  },

  // ---- B · Tree stage ------------------------------------------------------------
  B: {
    name: 'Tree stage',
    values: true,
    render(el, { entry, owners, nav }) {
      const en = entry || idleEntry();
      const pi = en.details.find(d => d.kind === 'publicInputs');
      const col = v => `<div class="bcol${en.focus[v] || !entry ? '' : ' dim'}">${vaultHead(en, v)}
          ${treeSvg(en, v, { w: 340, h: 180, owners, labels: this.values })}${strip(en, v, { cols: 32, owners, cls: 'band' })}</div>`;
      const other = en.details.filter(d => d.kind !== 'publicInputs' && d.kind !== 'merklePath');
      el.innerHTML = `<div class="inner b">
        ${navBar(nav)}
        <div class="bbanner">${entry ? `<div>${hopTitle(en.hop)}</div>` : ''}<p class="sentence">${entry ? sentences(en).map(esc).join(' ') : 'Press RUN: both vaults’ Merkle trees, with the vault each hop touches lit.'}</p></div>
        <div class="btools">${ownersToggle(owners)}<label class="ownersTgl"><input type="checkbox" data-act="bvals"${this.values ? ' checked' : ''}> values on the tree</label></div>
        <div class="bcols">${VAULT_IDS.map(col).join('')}</div>
        ${pi ? `<div class="bpp"><div><h4>the chain sees · public inputs of ${esc(pi.circuit)}</h4>${rowsHtml(rowsFor({ ...pi, private: [] }, owners).slice(0, -1))}</div>
                <div><h4>stays with the payer · private</h4>${pi.private.map(priv).join(' ')}</div></div>` : ''}
        ${other.map(d => rowsHtml(rowsFor(d, owners))).join('')}
      </div>`;
    },
    act(a) { if (a === 'bvals') { this.values = !this.values; panel.render(); } },
  },

  // ---- C · Feed ------------------------------------------------------------------
  C: {
    name: 'Feed',
    open: null, // the history index of an older card opened by hand
    render(el, { entry, history, owners, nav }) {
      const en = entry || idleEntry();
      const back = VAULT_IDS.find(v => v !== en.front);
      const list = nav ? history.filter(e => e.step === entry.step) : history.slice(-12);
      const card = (e, latest) => {
        const idx = history.indexOf(e);
        const open = latest || this.open === idx;
        if (!open) return `<div class="ccard shut" data-act="c:${idx}">▸ <b>${esc(e.hop.label)}</b> <span class="hstep">${esc(e.stepLabel)}</span></div>`;
        return `<div class="ccard${latest ? ' latest' : ''}"><div>${hopTitle(e.hop)} <span class="hstep">· ${esc(e.stepLabel)}</span></div>
          <p class="sentence">${sentences(e).map(esc).join(' ')}</p>
          <div class="chips">${e.details.flatMap(d => rowsFor(d, owners)).map(([k, v]) => `<span class="vchip"><span class="k">${esc(k)}</span> ${v}</span>`).join('')}</div></div>`;
      };
      const feed = list.slice().reverse().map(e => card(e, e === entry)).join('');
      el.innerHTML = `<div class="inner c">
        ${navBar(nav)}
        <div class="dock">
          <div class="dfront">${vaultHead(en, en.front)}<div class="dgrid">${treeSvg(en, en.front, { w: 230, h: 150, orient: 'right', owners })}${strip(en, en.front, { cols: 16, owners, cls: 'tile' })}</div></div>
          <div class="dback">${vaultHead(en, back)}${strip(en, back, { cols: 16, owners, cls: 'tile small' })}</div>
        </div>
        ${ownersToggle(owners)}
        <div class="feed">${feed || '<p class="sentence">Press RUN. Every hop that changes the protocol state adds a card here, newest on top.</p>'}</div>
      </div>`;
    },
    act(a) {
      if (!a?.startsWith('c:')) return;
      const i = Number(a.slice(2));
      this.open = this.open === i ? null : i;
      panel.render();
    },
  },
};

document.addEventListener('click', e => {
  const a = e.target.closest('[data-act]')?.dataset.act;
  if (a) VARIANTS[variant].act?.(a);
});

setVariant(VARIANTS[params.get('variant')] ? params.get('variant') : 'A');
if (params.has('owners')) run.owners = true;
if (params.get('speed')) speedSel.value = params.get('speed');
if (params.has('autoplay') || params.has('from')) replay.start();
if (stepStart(params.get('from')) !== undefined) replay.skipTo(stepStart(params.get('from')) - 1);
// ?revisit=<step id> opens a finished step's details, as clicking it in the stepper does.
if (params.get('revisit')) panel.revisit(params.get('revisit'));
