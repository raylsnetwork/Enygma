const { chromium, launchOpts, PAGE } = require('./_env.cjs');
const w=(p,ms)=>p.waitForTimeout(ms);
let fails=0; const pass=(c,m)=>{ console.log((c?'  PASS  ':'  FAIL  ')+m); if(!c) fails++; };
(async () => {
  const b=await chromium.launch(launchOpts);
  const pg=await b.newPage({viewport:{width:1360,height:1000}});
  const errs=[]; pg.on('pageerror',e=>errs.push('pageerror: '+e.message));
  pg.on('console',m=>{ if(m.type()==='error') errs.push(m.text()); });
  await pg.goto(PAGE); await w(pg,2000);
  await pg.click('#tabDvp'); await w(pg,700);

  console.log('--- DvP tab shape ---');
  let t = await pg.evaluate(()=>({
    tabs: [...document.querySelectorAll('.maintabs button')].map(e=>e.innerText.trim()),
    paneShown: !document.querySelector('#viewDvp').hidden,
    othersHidden: document.querySelector('#viewKeys').hidden && document.querySelector('#viewPay').hidden,
    tourHidden: document.querySelector('.tour').hidden,
    notes: window.__T.state.dvp.notes.length,
    cash: window.__T.state.dvp.notes.filter(n=>n.token==='CASH').length,
    bonds: window.__T.state.dvp.notes.filter(n=>n.token==='BOND').length,
    escrow: Number(window.__T.state.dvp.escrow.v),
  }));
  pass(t.tabs.length===4 && /DvP/.test(t.tabs[2]), `four tabs, DvP third: ${JSON.stringify(t.tabs)}`);
  pass(t.paneShown && t.othersHidden, 'only the DvP pane renders');
  pass(t.tourHidden, 'no guided walkthrough on DvP either');
  pass(t.bonds===3 && t.cash===0, `seeded with ${t.bonds} bond notes and no cash — cash arrives by bridging`);
  pass(t.escrow===0, 'escrow starts empty');

  console.log('\n--- BRIDGE: Pedersen point -> hash note, value conserved ---');
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

  console.log('\n--- OPEN a swap: Bank 1 cash for Bank 2 bonds ---');
  await pg.evaluate(()=>{ document.querySelector('#sfBob').value='1'; });
  await pg.click('#sfGo'); await w(pg,2200);
  const sw = await pg.evaluate(()=>{
    const S=window.__T.state; const s=S.dvp.swaps[0];
    return { alice:s.alice, bob:s.bob, status:s.status, inStatus:S.dvp.notes[s.inNote].status,
             nfA:!!s.nfA, nfB:s.nfB, deadline:s.deadline, now:S.block,
             accept:s.bobCheck.accept, steps:s.bobCheck.steps.length,
             allOk:s.bobCheck.steps.every(x=>x.ok), sees:s.bobCheck.sees,
             give:s.giveAmount, want:s.wantAmount,
             distinct: new Set([s.cOutA,s.cOutB,s.cRevA]).size };
  });
  pass(sw.status==='active' && sw.inStatus==='locked', `swap active, Alice's note LOCKED (not spent)`);
  pass(sw.nfA && sw.nfB===null, 'only Alice\'s nullifier exists so far');
  pass(sw.deadline>sw.now, `deadline is in the future (block ${sw.deadline}, now ${sw.now})`);
  pass(sw.distinct===3, 'the three output commitments are all distinct');
  pass(sw.allOk && sw.accept, `Bank 2 verified all ${sw.steps} retrieval steps and accepts`);
  pass(sw.sees && sw.sees.amount===sw.give, `Bank 2 decrypted what it will receive (${sw.sees.amount} ${sw.sees.tokenId})`);

  console.log('\n--- a third bank cannot read the swap payload ---');
  const out = await pg.evaluate(async ()=>{
    const S=window.__T.state, F=window.__T; const s=S.dvp.swaps[0];
    const other = S.insts.findIndex((_,i)=>i!==s.alice && i!==s.bob);
    const r = S.agr.get(F.pairKey(s.alice, other));
    if(!r) return { noChannel:true };
    const k = await F.hkdf(F.fromHex(r.ss), 'encryption key', 32);
    let opened=false; try { await F.aesDecrypt(F.fromHex(k), s.encTx.iv, s.encTx.ct); opened=true; } catch(e){}
    return { opened, other:S.insts[other].code };
  });
  pass(out.noChannel || out.opened===false, `${out.other||'a third bank'} cannot decrypt ENC_TX_DATA with its own channel key`);

  console.log('\n--- COMPLETE: both legs settle at once ---');
  const settled = await pg.evaluate(async ()=>{
    const S=window.__T.state, F=window.__T; const s=S.dvp.swaps[0];
    const leavesBefore = S.dvp.leaves.length;
    await F.completeSwap(s);
    const nA=S.dvp.notes[s.outNotes[0]], nB=S.dvp.notes[s.outNotes[1]];
    return { status:s.status, inSpent:S.dvp.notes[s.inNote].status, bobSpent:S.dvp.notes[s.bobNote].status,
             nfBoth: S.dvp.spent.has(s.nfA) && S.dvp.spent.has(s.nfB),
             newLeaves: S.dvp.leaves.length - leavesBefore,
             aliceGot:{owner:nA.owner, token:nA.token, amount:nA.amount},
             bobGot:{owner:nB.owner, token:nB.token, amount:nB.amount},
             alice:s.alice, bob:s.bob, give:s.giveAmount, want:s.wantAmount };
  });
  pass(settled.status==='settled', 'swap reports settled');
  pass(settled.inSpent==='spent' && settled.bobSpent==='spent', 'BOTH input notes are now spent');
  pass(settled.nfBoth, 'both nullifiers recorded — neither note can be respent');
  pass(settled.newLeaves===2, `exactly two new leaves inserted (${settled.newLeaves})`);
  pass(settled.aliceGot.owner===settled.alice && settled.aliceGot.amount===settled.want,
       `Alice received ${settled.aliceGot.amount} ${settled.aliceGot.token}`);
  pass(settled.bobGot.owner===settled.bob && settled.bobGot.amount===settled.give,
       `Bob received ${settled.bobGot.amount} ${settled.bobGot.token}`);

  console.log('\n--- REVERT: the timeout path returns Alice her own asset ---');
  await w(pg,600);
  const rev = await pg.evaluate(async ()=>{
    const S=window.__T.state, F=window.__T;
    // bridge again so Alice has a fresh note, then open and abandon a swap
    await F.bridgeIn(0, 250000, 'CASH');
    const mine = S.dvp.notes.filter(n=>n.owner===0 && n.status==='unspent' && n.token==='CASH');
    const s = await F.openSwap(0, mine[mine.length-1].id, 2, 'BOND', 5000);
    const bobNotesBefore = S.dvp.notes.filter(n=>n.owner===2 && n.status==='unspent').length;
    await F.revertSwap(s);
    const nR = S.dvp.notes[s.outNotes[0]];
    return { status:s.status, outCount:s.outNotes.length, nfA:S.dvp.spent.has(s.nfA), nfB:s.nfB,
             refundOwner:nR.owner, refundToken:nR.token, refundAmount:nR.amount,
             inSpent:S.dvp.notes[s.inNote].status,
             bobUnchanged: S.dvp.notes.filter(n=>n.owner===2 && n.status==='unspent').length===bobNotesBefore,
             usedRevertSalt: nR.salt===s.saltArev };
  });
  pass(rev.status==='reverted', 'swap reports reverted');
  pass(rev.outCount===1 && rev.refundOwner===0, 'exactly one new note, owned by Alice');
  pass(rev.refundToken==='CASH' && rev.refundAmount===250000, `she got her own asset back (${rev.refundAmount} ${rev.refundToken})`);
  pass(rev.usedRevertSalt, 'the refund used the revert commitment\'s salt, not the swap payout salt');
  pass(rev.nfA===true && rev.nfB===null, 'only Alice\'s nullifier was spent — Bob never moved');
  pass(rev.bobUnchanged, 'Bob\'s notes are untouched: he gave nothing and got nothing');

  console.log('\n--- value is still conserved across both ledgers ---');
  const cons = await pg.evaluate(()=>{
    const S=window.__T.state, F=window.__T;
    const notesTotal = S.dvp.notes.filter(n=>n.status!=='spent' && n.token==='CASH').reduce((a,n)=>a+n.amount,0);
    return { escrow:Number(S.dvp.escrow.v), notesTotal,
             supply:F.EC.compress(F.totalSupplyC()) };
  });
  pass(cons.escrow===cons.notesTotal,
       `escrow (${cons.escrow}) equals the live CASH notes (${cons.notesTotal}) — the two ledgers reconcile`);

  console.log('\n--- a frozen contract blocks bridging (it is a withdraw) ---');
  await pg.evaluate(()=>{ const S=window.__T.state; S.frozen = true; });
  const frz = await pg.evaluate(async ()=>{
    try { await window.__T.bridgeIn(0, 1000, 'CASH'); return { blocked:false }; }
    catch(e){ return { blocked:true, msg:e.message }; }
  });
  pass(frz.blocked, `bridging reverts while frozen ("${(frz.msg||'').slice(0,42)}…")`);

  pass(errs.length===0, 'no console/page errors'+(errs.length?': '+errs.slice(0,3).join(' | '):''));
  await b.close();
  console.log('\n'+(fails? fails+' CHECK(S) FAILED':'ALL CHECKS PASSED'));
  process.exit(fails?1:0);
})();
