const { chromium, launchOpts, PAGE } = require('./_env.cjs');
const w=(p,ms)=>p.waitForTimeout(ms);
let fails=0; const pass=(c,m)=>{ console.log((c?'  PASS  ':'  FAIL  ')+m); if(!c) fails++; };
const persona = async (pg,label) => {
  await pg.evaluate(()=>document.querySelector('#personaBtn').click()); await w(pg,250);
  await pg.evaluate(l=>{ const x=[...document.querySelectorAll('#personaMenu button')].find(e=>e.innerText.includes(l)); x&&x.click(); }, l=label);
  await w(pg,700);
};
(async () => {
  const b=await chromium.launch(launchOpts);
  const pg=await b.newPage({viewport:{width:1360,height:1000}});
  const errs=[]; pg.on('pageerror',e=>errs.push('pageerror: '+e.message));
  pg.on('console',m=>{ if(m.type()==='error') errs.push(m.text()); });
  await pg.goto(PAGE); await w(pg,2000);
  await pg.click('#tabDvp'); await w(pg,800);

  console.log('--- the notes table exposes no per-leaf spend state ---');
  const cols = await pg.evaluate(()=>({
    heads: [...document.querySelectorAll('#notesBody thead th')].map(t=>t.innerText.trim()),
    cells: (document.querySelector('#notesBody tbody tr')||{cells:[]}).cells.length }));
  pass(cols.heads.length===5, `5 columns: ${cols.heads.join(' | ')}`);
  pass(!/state|status|spent/i.test(cols.heads.join(' ')), 'no State / Status / spent column header');
  pass(cols.cells===cols.heads.length, `every row has exactly ${cols.heads.length} cells`);

  const leak = await pg.evaluate(()=>{
    const tb = document.querySelector('#notesBody .notes-wrap');
    return { txt: tb.innerText, cls: tb.innerHTML.match(/st-(unspent|spent|locked)/g) || [],
             badge: tb.querySelectorAll('.nst').length };
  });
  pass(!/\bunspent\b/i.test(leak.txt), 'the word "unspent" appears nowhere in the leaf table');
  pass(!/\bspent\b/i.test(leak.txt), 'nor does "spent"');
  pass(leak.cls.length===0, 'no st-spent / st-unspent row classes remain');
  pass(leak.badge===0, 'no per-leaf state badge is rendered');
  const dim = await pg.evaluate(()=>{
    const rs=[...document.querySelectorAll('#notesBody tbody tr')];
    return [...new Set(rs.map(r=>getComputedStyle(r).opacity))]; });
  pass(dim.length===1 && dim[0]==='1', `no row is visually dimmed (opacities: ${dim.join(',')})`);

  console.log('\n--- the nullifier set is shown instead, with no leaf column ---');
  const nf0 = await pg.evaluate(()=>{
    const s=document.querySelector('.nfset');
    return { present: !!s, rows: s.querySelectorAll('.nfrow').length, txt: s.innerText };
  });
  pass(nf0.present, 'a "Nullifier set" block exists on the DvP tab');
  pass(nf0.rows===0 && /set is empty/i.test(nf0.txt), 'starts empty — nothing spent yet');
  pass(/have I seen this nullifier|already in the set/i.test(nf0.txt), 'states the contract check');
  pass(!/leaf/i.test(nf0.txt.split('There is no column')[0]), 'the set itself never names a leaf');

  console.log('\n--- open a swap: exactly one nullifier appears, unlinked to any leaf ---');
  const opened = await pg.evaluate(async ()=>{
    const S=window.__T.state, F=window.__T;
    const mine = S.dvp.notes.filter(n=>n.owner===0 && n.status==='unspent');
    if(!mine.length) await F.bridgeIn(0, 1000000, 'CASH');
    const m = S.dvp.notes.filter(n=>n.owner===0 && n.status==='unspent');
    const s = await F.openSwap(0, m[0].id, 1, 'BOND', 5000);
    await F.bobRetrieve(s);
    return { nfA: s.nfA, leaf: S.dvp.notes[s.inNote].leaf };
  });
  await pg.evaluate(()=>window.__T && document.querySelector('#tabDvp').click()); await w(pg,900);
  const nf1 = await pg.evaluate(()=>{
    const s=document.querySelector('.nfset');
    const rows=[...s.querySelectorAll('.nfrow')];
    return { n: rows.length, states: rows.map(r=>r.querySelector('.nst').textContent.trim().toLowerCase()),
             rowText: rows.map(r=>r.innerText.replace(/\s+/g,' ')),
             foot: document.querySelector('#notesBody .env-foot').innerText };
  });
  pass(nf1.n===1, `one nullifier published (${nf1.n})`);
  pass(nf1.states[0]==='locked', `its state is "locked", not tied to a note (${nf1.states[0]})`);
  pass(!new RegExp('leaf\\s*'+opened.leaf+'\\b','i').test(nf1.rowText[0]),
       `the row does not name leaf ${opened.leaf} (its actual source)`);
  pass(/1 nullifiers published|1 nullifier/i.test(nf1.foot) || /nullifiers published/.test(nf1.foot),
       'the footer counts published nullifiers, not spent notes');
  pass(/no leaf is ever marked spent/i.test(nf1.foot), 'the footer says plainly that no leaf is marked spent');

  console.log('\n--- the ZK statement and check order are stated on the settlement panel ---');
  const zk = await pg.evaluate(()=>{
    const z=document.querySelector('.zks'); return z ? z.innerText.replace(/\s+/g,' ') : null; });
  pass(!!zk, 'the swap panel states what the proof proves');
  pass(/Merkle path/i.test(zk) && /stay private/i.test(zk), 'private Merkle inclusion path is named');
  pass(/outputs add up to the amount committed in that leaf/i.test(zk), 'output amounts = input amount is named');
  pass(/already in the nullifier set/i.test(zk) && /reverts here/i.test(zk), 'nullifier-membership check comes first');
  pass(zk.indexOf('already in the nullifier set') < zk.indexOf('verify'), 'and it is stated BEFORE proof verification');
  pass(/cannot ask/i.test(zk) && /is that note spent/i.test(zk), 'says outright the contract cannot ask if a note is spent');

  console.log('\n--- settle, then both nullifiers are in the set and still unlinked ---');
  await pg.evaluate(async ()=>{ const S=window.__T.state; await window.__T.completeSwap(S.dvp.swaps[0]); });
  await pg.evaluate(()=>document.querySelector('#tabDvp').click()); await w(pg,900);
  const nf2 = await pg.evaluate(()=>{
    const rows=[...document.querySelectorAll('.nfset .nfrow')];
    return { n: rows.length, states: rows.map(r=>r.querySelector('.nst').textContent.trim().toLowerCase()),
             tableTxt: document.querySelector('#notesBody .notes-wrap').innerText };
  });
  pass(nf2.n===2, `two nullifiers after settlement (${nf2.n})`);
  pass(nf2.states.every(s=>s==='spent'), `both read "spent" (${nf2.states.join(',')})`);
  pass(!/\bspent\b/i.test(nf2.tableTxt), 'the leaf table STILL says nothing about spending after a settlement');

  console.log('\n--- the owner can still tell, because it recomputes its own nullifier ---');
  const own = await pg.evaluate(async ()=>{
    const S=window.__T.state, F=window.__T;
    const me = 0, mine = S.dvp.notes.filter(n=>n.owner===me);
    const out = [];
    for(const n of mine){
      const nf = await F.nullifierOf(S.insts[me].skSpendHex, n.leaf);
      out.push({ leaf:n.leaf, inSet: S.dvp.spent.has(nf), modelled: n.status==='spent' });
    }
    // a different bank's key must NOT reproduce those nullifiers
    const other = await F.nullifierOf(S.insts[1].skSpendHex, mine[0].leaf);
    const mineNf = await F.nullifierOf(S.insts[0].skSpendHex, mine[0].leaf);
    return { out, keyBound: other !== mineNf };
  });
  pass(own.out.every(o=>o.inSet===o.modelled),
       `every one of Bank 1's ${own.out.length} leaves: recomputed nullifier membership matches its true state`);
  pass(own.keyBound, 'another bank\'s key on the same leaf index yields a different nullifier');

  const hint = await pg.evaluate(()=>({ notes: document.querySelector('#notesHint').innerText,
                                        swap: document.querySelector('#swapHint').innerText }));
  pass(!/unspent/i.test(hint.notes), `panel header counts leaves, not unspent notes ("${hint.notes}")`);
  pass(/spendable by you/i.test(hint.swap), `the swap composer scopes it to the viewer ("${hint.swap}")`);

  console.log('\n--- the FAQ answers it too ---');
  await pg.click('#tabProtocol'); await w(pg,700);
  const faq = await pg.evaluate(()=>{
    const ds=[...document.querySelectorAll('#faqBlock details.faq')];
    const d=ds.find(x=>/which notes have been spent/i.test(x.querySelector('summary').innerText));
    return { n: ds.length, found: !!d, txt: d ? d.querySelector('.faq-a').textContent.replace(/\s+/g,' ') : '',
             count: document.querySelector('#faqCount').textContent };
  });
  pass(faq.found, 'a FAQ entry asks "Can anyone tell which notes have been spent?"');
  pass(/structurally incapable|No —/.test(faq.txt), 'and answers no');
  pass(/no status column/i.test(faq.txt), 'and explains why the table has no status column');
  pass(faq.count===faq.n+' questions · 6 sections', `FAQ counter updated: "${faq.count}"`);

  pass(errs.length===0, 'no console/page errors'+(errs.length?': '+errs.slice(0,3).join(' | '):''));
  await b.close();
  console.log('\n'+(fails? fails+' CHECK(S) FAILED':'ALL CHECKS PASSED'));
  process.exit(fails?1:0);
})();
