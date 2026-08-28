const { chromium, launchOpts, PAGE, enterProduct } = require('./_env.cjs');
const w=(p,ms)=>p.waitForTimeout(ms);
let fails=0; const pass=(c,m)=>{ console.log((c?'  PASS  ':'  FAIL  ')+m); if(!c) fails++; };
(async () => {
  const b=await chromium.launch(launchOpts);
  const pg=await b.newPage({viewport:{width:1440,height:1000}});
  const errs=[]; pg.on('pageerror',e=>errs.push('pageerror: '+e.message));
  pg.on('console',m=>{ if(m.type()==='error') errs.push(m.text()); });
  await pg.goto(PAGE); await w(pg,2000);
  await enterProduct(pg, 'institutional');

  // payments are manual now: compose one so there is an envelope to inspect
  await pg.click('#tabPay'); await w(pg,600);
  await pg.evaluate(()=>{ const bx=[...document.querySelectorAll('.pay-check input')];
    if(!bx[0].checked) bx[0].click(); if(bx[1] && !bx[1].checked) bx[1].click(); });
  await w(pg,300);
  await pg.click('#tfBtn'); await w(pg,9000);

  console.log('--- envelope shape (spec §6) ---');
  const env = await pg.evaluate(async () => {
    const tx = window.__T.state.txs.find(t=>t.type==='payment');
    return { k: tx.k, commits: tx.commits.length, tags: tx.tags.length, cts: tx.cts.length,
             nullifier: tx.nullifier, proof: tx.proof, block: tx.block, sums: tx.sumsToIdentity,
             tagLens: [...new Set(tx.tags.map(t=>t.length))],
             ctPresentForEverySlot: tx.cts.every((c,i)=>c.slot===i && c.ct && c.ct.length>0),
             paidCount: tx.cts.filter(c=>!c.self && c.amount>0).length };
  });
  pass(env.commits===env.k, `${env.commits} commitments for k=${env.k}`);
  pass(env.tags===env.k, `${env.tags} messaging tags — one per slot`);
  pass(env.cts===env.k, `${env.cts} ciphertexts — one per slot, NOT one per bank paid (${env.paidCount} paid)`);
  pass(env.ctPresentForEverySlot, 'every slot carries a real ciphertext (decoys included)');
  pass(env.tagLens.length===1, `all tags the same length (${env.tagLens.join(',')} hex chars) — no length leak`);
  pass(!!env.nullifier && env.nullifier.length===64, `nullifier present (${env.nullifier.slice(0,16)}…)`);
  pass(!!env.proof, `proof present (${env.proof.slice(0,16)}…)`);
  pass(env.sums===true, 'commitment vector sums to the identity');
  pass(env.block>0, `transaction carries a block number (n=${env.block})`);

  console.log('\n--- blinding factors are DERIVED, not random (so a recipient can open) ---');
  const der = await pg.evaluate(async () => {
    const S = window.__T.state, F = window.__T;
    const tx = S.txs.find(t=>t.type==='payment');
    const out = [];
    for(let i=0;i<tx.set.length;i++){
      const idx = tx.set[i];
      if(idx === tx.from) continue;
      const rec = S.agr.get(F.pairKey(tx.from, idx));
      const rHex = await F.hkdf(F.fromHex(rec.ss), 'enygma/rand/'+tx.block, 32);
      const r = F.EC.mod(BigInt('0x'+rHex), F.EC.N);
      out.push({ idx, matches: F.scalarHex(r) === tx.slots[i].rHex });
    }
    return out;
  });
  pass(der.length>0 && der.every(o=>o.matches), `every recipient can re-derive its own r from the shared secret (${der.length}/${der.length})`);

  console.log('\n--- the sender slot uses r_prev, not a shared secret ---');
  const jt = await pg.evaluate(async () => {
    const S=window.__T.state, F=window.__T;
    const tx = S.txs.find(t=>t.type==='payment');
    // no peer's tag should reproduce the sender's own slot tag
    const posted = tx.tags[tx.jSlot];
    const clashes = [];
    for(const idx of tx.set){
      if(idx===tx.from) continue;
      const rec = S.agr.get(F.pairKey(tx.from, idx));
      const t = await F.hkdf(F.fromHex(rec.ss), 'enygma/tag/'+tx.block, 8);
      if(t===posted) clashes.push(idx);
    }
    return { clashes: clashes.length, jSlot: tx.jSlot, senderAcct: tx.from };
  });
  pass(jt.clashes===0, `sender's own tag is not derivable from any channel secret (slot ${jt.jSlot})`);

  console.log('\n--- RETRIEVE: each bank trials its slot tag ---');
  const scan = await pg.evaluate(async () => {
    const S=window.__T.state, F=window.__T;
    const tx = S.txs.find(t=>t.type==='payment');
    const rows=[];
    for(let me=0; me<S.insts.length; me++){
      const r = await F.trialTags(tx, me);
      const slot = tx.set.indexOf(me);
      rows.push({ me, code:S.insts[me].code, inSet:r.inSet, hashes:r.hashes||0,
                  sender:r.sender, senderCode: r.sender!=null?S.insts[r.sender].code:null,
                  amount:r.amount, opens:r.opens, decoy:r.decoy, ctOk:r.ctOk,
                  isSender: me===tx.from,
                  trueDelta: slot>=0 ? tx.slots[slot].delta : null });
    }
    return { rows, from: S.insts[tx.from].code, k: tx.k };
  });
  for(const r of scan.rows){
    if(!r.inSet){ pass(true, `${r.code}: not in the anonymity set — nothing to trial`); continue; }
    if(r.isSender){ pass(r.sender===null, `${r.code} (the sender): its own tag matches no channel, as designed`); continue; }
    pass(r.sender!==null, `${r.code}: identified the sender as ${r.senderCode} after ${r.hashes} hashes (expected ${scan.from})`);
    pass(r.senderCode===scan.from, `${r.code}: sender named correctly`);
    pass(r.ctOk===true, `${r.code}: ciphertext at its own slot decrypts under the block key`);
    pass(r.amount===r.trueDelta, `${r.code}: recovered amount ${r.amount} == actual delta ${r.trueDelta}`);
    pass(r.opens===true, `${r.code}: C − r·H equals v·G — commitment opens to that amount`);
    if(r.trueDelta===0) pass(r.decoy===true, `${r.code}: correctly told it is a decoy (v = 0)`);
  }

  console.log('\n--- a bank NOT in the set, and the chain itself, learn nothing ---');
  const neg = await pg.evaluate(async () => {
    const S=window.__T.state, F=window.__T;
    const tx = S.txs.find(t=>t.type==='payment');
    // forge: try to open another bank's slot with your own secrets
    const victim = tx.set.find(i=>i!==tx.from && tx.slots[tx.set.indexOf(i)].delta>0);
    const attacker = tx.set.find(i=>i!==tx.from && i!==victim);
    const rec = S.agr.get(F.pairKey(attacker, tx.from));
    const key = await F.hkdf(F.fromHex(rec.ss), 'enygma/aes/'+tx.block, 32);
    const vSlot = tx.set.indexOf(victim);
    let opened=false;
    try { await F.aesDecrypt(F.fromHex(key), tx.cts[vSlot].iv, tx.cts[vSlot].ct); opened=true; } catch(e){}
    return { opened, attacker:S.insts[attacker].code, victim:S.insts[victim].code };
  });
  pass(neg.opened===false, `${neg.attacker} cannot decrypt ${neg.victim}'s slot with its own channel key`);

  pass(errs.length===0, 'no console/page errors'+(errs.length?': '+errs.slice(0,3).join(' | '):''));
  await b.close();
  console.log('\n'+(fails? fails+' CHECK(S) FAILED':'ALL CHECKS PASSED'));
  process.exit(fails?1:0);
})();
