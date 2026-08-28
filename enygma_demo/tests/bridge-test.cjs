/* The Bridge tab of the Institutional network: where a Pedersen account balance becomes a hash
 * commitment in the note vault, and comes back again.
 *
 * Swapping those notes is a different protocol with its own network — see swap-test.cjs. What is
 * unique here, and what these checks cover, is the join between the two commitment schemes: value
 * changes representation without the supply moving, and bringing a note back publishes a nullifier.
 */
const { chromium, launchOpts, PAGE, enterProduct } = require('./_env.cjs');
const w=(p,ms)=>p.waitForTimeout(ms);
let fails=0; const pass=(c,m)=>{ console.log((c?'  PASS  ':'  FAIL  ')+m); if(!c) fails++; };
(async () => {
  const b=await chromium.launch(launchOpts);
  const pg=await b.newPage({viewport:{width:1360,height:1000}});
  const errs=[]; pg.on('pageerror',e=>errs.push('pageerror: '+e.message));
  pg.on('console',m=>{ if(m.type()==='error') errs.push(m.text()); });
  await pg.goto(PAGE); await w(pg,2000);
  await enterProduct(pg, 'institutional');
  await pg.click('#tabDvp'); await w(pg,700);

  console.log('--- Bridge tab shape ---');
  let t = await pg.evaluate(()=>({
    tabs: [...document.querySelectorAll('.maintabs button')].map(e=>e.innerText.trim()),
    paneShown: !document.querySelector('#viewDvp').hidden,
    othersHidden: document.querySelector('#viewKeys').hidden && document.querySelector('#viewPay').hidden,
    tourHidden: document.querySelector('.tour').hidden,
    sections: [...document.querySelectorAll('#viewDvp section')].map(s=>s.getAttribute('aria-label')),
    swapUI: !!document.querySelector('#swapSec, #settleSec, #sfGo, #swapForm, #settleBody'),
    notes: window.__T.state.dvp.notes.length,
    cash: window.__T.state.dvp.notes.filter(n=>n.token==='CASH').length,
    bonds: window.__T.state.dvp.notes.filter(n=>n.token==='BOND').length,
    escrow: Number(window.__T.state.dvp.escrow.v),
  }));
  pass(t.tabs.length===4 && /Bridge/.test(t.tabs[2]), `four tabs, Bridge third: ${JSON.stringify(t.tabs)}`);
  pass(t.paneShown && t.othersHidden, 'only the Bridge pane renders');
  pass(t.tourHidden, 'no guided walkthrough on Bridge either');
  pass(!t.swapUI,
       `no swap or settlement UI here — that is the DvP protocol's job: ${JSON.stringify(t.sections)}`);
  pass(t.sections.some(s=>/DvP protocol/i.test(s||'')), 'and it hands off to that protocol explicitly');
  pass(t.bonds===3 && t.cash===0, `seeded with ${t.bonds} bond notes and no cash — cash arrives by bridging`);
  pass(t.escrow===0, 'escrow starts empty');

  console.log('\n--- BRIDGE IN: Pedersen point -> hash note, value conserved ---');
  const pre = await pg.evaluate(()=>({
    supply: window.__T.EC.compress(window.__T.totalSupplyC()),
    bankV: Number(window.__T.state.insts[0].bal.v),
    root: window.__T.state.dvp.root,
  }));
  await pg.fill('#brgAmt','1000000');
  await pg.click('#brgGo'); await w(pg,2000);
  const post = await pg.evaluate(()=>{
    const S=window.__T.state, F=window.__T;
    const ev = S.dvp.lastBridge, n = S.dvp.notes[ev.note];
    return { supply: F.EC.compress(F.totalSupplyC()), bankV: Number(S.insts[0].bal.v),
             escrow: Number(S.dvp.escrow.v), pairSums: ev.pairSums,
             root: S.dvp.root, leaf: n.leaf, amount: n.amount, token: n.token,
             saltLen: n.salt.length, cLen: n.C.length, leaves: S.dvp.leaves.length };
  });
  pass(post.supply===pre.supply, `total supply commitment UNCHANGED by the bridge (${pre.supply.slice(0,14)}…)`);
  pass(post.bankV===pre.bankV-1000000, `bank debited on the Pedersen side (${pre.bankV} → ${post.bankV})`);
  pass(post.escrow===1000000, `escrow credited the same amount (${post.escrow})`);
  pass(post.pairSums===true, 'the two Pedersen deltas sum to the identity');
  pass(post.amount===1000000 && post.token==='CASH', `a ${post.token} note of ${post.amount} now exists`);
  pass(post.saltLen===64 && post.cLen===64, 'note carries a 32-byte salt and a 32-byte hash commitment');
  pass(post.root!==pre.root, `Merkle root advanced (${pre.root.slice(0,10)}… → ${post.root.slice(0,10)}…)`);

  console.log('\n--- the note commitment is exactly H(pk_spend, salt, amount, token) ---');
  const rec = await pg.evaluate(async ()=>{
    const S=window.__T.state, F=window.__T;
    const n = S.dvp.notes[S.dvp.lastBridge.note];
    const again = await F.noteHash(S.insts[n.owner].pkSpend, n.salt, n.amount, n.token);
    const wrongAmt = await F.noteHash(S.insts[n.owner].pkSpend, n.salt, n.amount+1, n.token);
    const wrongOwner = await F.noteHash(S.insts[1].pkSpend, n.salt, n.amount, n.token);
    const root = await F.merkleRoot(S.dvp.leaves);
    return { same: again===n.C, wrongAmt: wrongAmt!==n.C, wrongOwner: wrongOwner!==n.C, rootOk: root===S.dvp.root };
  });
  pass(rec.same, 'recomputing the commitment from its opening reproduces it exactly');
  pass(rec.wrongAmt && rec.wrongOwner, 'changing the amount or the owner changes the commitment (binding)');
  pass(rec.rootOk, 'the stored root really is the Merkle root over the leaf list');

  console.log('\n--- a leaf hides its owner, not just its amount ---');
  const attr = async (label) => {
    await pg.evaluate(()=>document.querySelector('#personaBtn').click()); await w(pg,250);
    await pg.evaluate(l=>{ const x=[...document.querySelectorAll('#personaMenu button')].find(e=>e.innerText.includes(l)); x&&x.click(); }, label);
    await w(pg,700);
    return pg.evaluate(()=>{
      const rows=[...document.querySelectorAll('#notesBody tbody tr')];
      return { total: rows.length,
               unattributable: rows.filter(r=>/unattributable/.test(r.innerText)).length,
               named: rows.filter(r=>/Bank/.test(r.cells[1].innerText)).length,
               amounts: rows.filter(r=>!/hash only/.test(r.cells[4].innerText)).length };
    });
  };
  let a = await attr('Private Network');
  pass(a.unattributable===a.total && a.named===0, `chain: all ${a.total} leaves unattributable, 0 named`);
  pass(a.amounts===0, 'chain: no amount readable on any leaf');
  a = await attr('Bank 1');
  pass(a.named>0 && a.named<a.total, `Bank 1 recognises only its own ${a.named} of ${a.total} leaves`);
  pass(a.amounts===a.named, `and reads amounts on exactly those ${a.amounts}`);
  a = await attr('Regulator');
  pass(a.named===a.total, `Regulator attributes all ${a.total} leaves via delegated view keys`);
  await attr('Bank 1');

  console.log('\n--- BRIDGE BACK: spending a note is what publishes a nullifier ---');
  const before = await pg.evaluate(()=>({
    supply: window.__T.EC.compress(window.__T.totalSupplyC()),
    bankV: Number(window.__T.state.insts[0].bal.v),
    escrow: Number(window.__T.state.dvp.escrow.v),
    nfs: window.__T.state.dvp.nullifiers.length,
    leaves: window.__T.state.dvp.leaves.length,
  }));
  await pg.click('#brgBack'); await w(pg,2200);
  const back = await pg.evaluate(()=>{
    const S=window.__T.state, F=window.__T;
    const ev = S.dvp.events[0];
    const n = S.dvp.notes[ev.note];
    return { type: ev.type, supply: F.EC.compress(F.totalSupplyC()),
             bankV: Number(S.insts[0].bal.v), escrow: Number(S.dvp.escrow.v),
             pairSums: ev.pairSums, nf: ev.nf, nfs: S.dvp.nullifiers.length,
             noteStatus: n.status, spentHas: S.dvp.spent.has(ev.nf),
             leaves: S.dvp.leaves.length, amount: ev.amount };
  });
  pass(back.type==='unbridge', 'the return leg is recorded as its own event');
  pass(back.supply===before.supply, 'total supply commitment UNCHANGED by the return leg too');
  pass(back.bankV===before.bankV+back.amount, `bank credited back (${before.bankV} → ${back.bankV})`);
  pass(back.escrow===before.escrow-back.amount, `escrow debited to match (${before.escrow} → ${back.escrow})`);
  pass(back.pairSums===true, 'and again the two Pedersen deltas sum to the identity');
  pass(back.noteStatus==='spent', 'the note is spent, not deleted — the leaf stays in the tree');
  pass(back.leaves===before.leaves, `the tree is append-only: ${before.leaves} leaves before, ${back.leaves} after`);
  pass(back.nfs===before.nfs+1, `exactly one nullifier published (${before.nfs} → ${back.nfs})`);
  pass(back.spentHas, 'and it is in the spent set');

  console.log('\n--- the nullifier is H(sk_spend, leafIndex), and nothing else ---');
  const nfCheck = await pg.evaluate(async ()=>{
    const S=window.__T.state, F=window.__T;
    const ev = S.dvp.events.find(e=>e.type==='unbridge');
    const n = S.dvp.notes[ev.note];
    const mine = await F.nullifierOf(S.insts[0].skSpendHex, n.leaf);
    const otherKey = await F.nullifierOf(S.insts[1].skSpendHex, n.leaf);
    const otherLeaf = await F.nullifierOf(S.insts[0].skSpendHex, n.leaf+1);
    return { matches: mine===ev.nf, wrongKey: otherKey!==ev.nf, wrongLeaf: otherLeaf!==ev.nf,
             namesLeaf: JSON.stringify(S.dvp.nullifiers).includes('"leaf"') };
  });
  pass(nfCheck.matches, 'the holder can recompute the published nullifier from its own spend key');
  pass(nfCheck.wrongKey, 'another bank\'s spend key produces a different value — it cannot forge it');
  pass(nfCheck.wrongLeaf, 'and a different leaf produces a different value — it binds to exactly one note');
  pass(!nfCheck.namesLeaf, 'the published record carries no leaf index');

  console.log('\n--- a spent note cannot be spent twice ---');
  const dbl = await pg.evaluate(async ()=>{
    const S=window.__T.state, F=window.__T;
    const ev = S.dvp.events.find(e=>e.type==='unbridge');
    try { await F.bridgeOut(0, ev.note); return { blocked:false }; }
    catch(e){ return { blocked:true, msg:e.message }; }
  });
  pass(dbl.blocked, `spending it again is rejected ("${(dbl.msg||'').slice(0,44)}…")`);

  console.log('\n--- the two ledgers reconcile ---');
  const cons = await pg.evaluate(()=>{
    const S=window.__T.state, F=window.__T;
    const notesTotal = S.dvp.notes.filter(n=>n.status!=='spent' && n.token==='CASH').reduce((a,n)=>a+n.amount,0);
    return { escrow:Number(S.dvp.escrow.v), notesTotal, supply:F.EC.compress(F.totalSupplyC()) };
  });
  pass(cons.escrow===cons.notesTotal,
       `escrow (${cons.escrow}) equals the live CASH notes (${cons.notesTotal}) — the two ledgers reconcile`);

  console.log('\n--- a frozen contract blocks both legs ---');
  await pg.evaluate(()=>{ const S=window.__T.state; S.frozen = true; });
  const frz = await pg.evaluate(async ()=>{
    const F=window.__T, S=F.state;
    const out = {};
    try { await F.bridgeIn(0, 1000, 'CASH'); out.inBlocked=false; }
    catch(e){ out.inBlocked=true; out.inMsg=e.message; }
    const spendable = S.dvp.notes.find(n=>n.owner===0 && n.status==='unspent');
    try { await F.bridgeOut(0, spendable ? spendable.id : -1); out.outBlocked=false; }
    catch(e){ out.outBlocked=true; out.outMsg=e.message; }
    return out;
  });
  pass(frz.inBlocked, `bridging in reverts while frozen — it is a withdraw ("${(frz.inMsg||'').slice(0,38)}…")`);
  pass(frz.outBlocked, `bridging back reverts too — it is a deposit ("${(frz.outMsg||'').slice(0,38)}…")`);

  pass(errs.length===0, 'no console/page errors'+(errs.length?': '+errs.slice(0,3).join(' | '):''));
  await b.close();
  console.log('\n'+(fails? fails+' CHECK(S) FAILED':'ALL CHECKS PASSED'));
  process.exit(fails?1:0);
})();
