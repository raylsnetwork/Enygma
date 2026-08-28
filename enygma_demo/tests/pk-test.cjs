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
  await pg.click('#tabPay'); await w(pg,700);

  console.log('--- the compose form offers both kinds per leg ---');
  const opts = await pg.evaluate(()=>{
    const s=document.querySelector('.pay-users');
    return { vals:[...s.options].map(o=>o.value), labs:[...s.options].map(o=>o.textContent.trim()), def:s.value };
  });
  pass(opts.vals.includes('settlement'), `a settlement option exists: ${opts.labs.join(' | ')}`);
  pass(opts.vals.filter(v=>v!=='settlement').length===3, 'plus 1/2/3 client options');
  pass(opts.def==='settlement', `interbank settlement is the default ("${opts.def}")`);
  const trunc = await pg.evaluate(()=>{
    const s=document.querySelector('.pay-users');
    const probe=document.createElement('span');
    probe.style.cssText='position:absolute;visibility:hidden;white-space:nowrap;font:'+getComputedStyle(s).font;
    probe.textContent=[...s.options].map(o=>o.textContent).sort((a,b)=>b.length-a.length)[0];
    document.body.appendChild(probe);
    const need=probe.getBoundingClientRect().width; probe.remove();
    return { need: Math.ceil(need), have: Math.floor(s.getBoundingClientRect().width) };
  });
  pass(trunc.have >= trunc.need + 18, `the kind select fits its longest label (${trunc.have}px for ${trunc.need}px + arrow)`);
  const legend = await pg.evaluate(()=>document.querySelector('.pay-head').textContent.replace(/\s+/g,' ').trim());
  pass(/Payload carries/i.test(legend), `a column legend explains the third column ("${legend}")`);

  // one settlement leg + one client leg in the SAME payment
  await pg.evaluate(()=>{
    const bx=[...document.querySelectorAll('.pay-check input')];
    if(!bx[0].checked) bx[0].click();
    if(!bx[1].checked) bx[1].click();
  });
  await w(pg,300);
  const ids = await pg.evaluate(()=>[...document.querySelectorAll('input[data-pay]:checked')].map(c=>c.dataset.pay));
  await pg.evaluate(i=>{ const s=document.querySelector(`[data-users="${i}"]`); s.value='settlement'; s.dispatchEvent(new Event('change')); }, ids[0]);
  await pg.evaluate(i=>{ const s=document.querySelector(`[data-users="${i}"]`); s.value='3'; s.dispatchEvent(new Event('change')); }, ids[1]);
  await w(pg,400);
  const tot = await pg.evaluate(()=>document.querySelector('#payTotal').innerText.replace(/\s+/g,' '));
  pass(/1 settlement/.test(tot) && /1 client leg covering 3 customer/.test(tot), `the total narrates the mix: "${tot}"`);
  pass(/k stays/.test(tot), 'and states k is unchanged');

  await pg.click('#tfBtn'); await w(pg,9000);

  console.log('\n--- both kinds ride the same envelope ---');
  const tx = await pg.evaluate(()=>{
    const t=window.__T.state.txs.find(x=>x.type==='payment');
    return { k:t.k, kinds:t.legKinds, ctLens:t.ctLens, ctBytes:t.ctBytes,
             plainKinds: t.cts.map(c=>c.data.kind),
             stl: t.cts.find(c=>c.data.kind==='settlement'),
             cli: t.cts.find(c=>c.data.kind==='client'),
             tagLens:[...new Set(t.tags.map(x=>x.length))],
             sums:t.sumsToIdentity, allVerified:t.cts.every(c=>c.verified) };
  });
  pass(tx.kinds.length===2 && tx.kinds.some(l=>l.kind==='settlement') && tx.kinds.some(l=>l.kind==='client'),
       `one payment carried both kinds: ${tx.kinds.map(l=>l.kind+'/'+l.users).join(', ')}`);
  pass(tx.ctLens.length===1, `all ${tx.k} ciphertexts are the same length (${tx.ctLens[0]/2} bytes) — the kind does not leak`);
  pass(tx.tagLens.length===1, 'tags still equal length too');
  pass(tx.sums===true, 'the commitment vector still sums to the identity');
  pass(tx.allVerified===true, 'every padded payload still round-trips through AES-256-GCM');
  pass(!!tx.stl && tx.stl.data.ref && !tx.stl.data.beneficiaries,
       `the settlement payload carries a reference and NO beneficiaries (${tx.stl && tx.stl.data.ref})`);
  pass(!!tx.cli && Array.isArray(tx.cli.data.beneficiaries) && tx.cli.data.beneficiaries.length===3,
       `the client payload carries 3 beneficiaries`);
  const sum = tx.cli ? tx.cli.data.beneficiaries.reduce((a,x)=>a+x.amount,0) : -1;
  pass(sum===tx.cli.data.amount, `the beneficiary split sums to the leg amount (${sum} == ${tx.cli.data.amount})`);
  pass(new Set(tx.plainKinds).size>=3, `slot payloads span several kinds: ${[...new Set(tx.plainKinds)].join(', ')}`);

  console.log('\n--- padding is real: same bytes for decoy, settlement and client ---');
  const lens = await pg.evaluate(async ()=>{
    const F=window.__T, t=F.state.txs.find(x=>x.type==='payment');
    const out=[];
    for(const c of t.cts){
      try { const p = await F.aesDecrypt(F.fromHex(c.keyHex), c.iv, c.ct);
            out.push({ kind:p.kind, ctLen:c.ct.length, ok:JSON.stringify(p)===JSON.stringify(c.data) }); }
      catch(e){ out.push({ kind:'ERR', ctLen:c.ct.length, ok:false }); }
    }
    return out;
  });
  pass(lens.every(o=>o.ok), 'every slot decrypts to exactly the object that was encrypted');
  pass(new Set(lens.map(o=>o.ctLen)).size===1,
       `decoy / settlement / client all ${lens[0].ctLen/2} bytes: ${lens.map(o=>o.kind+':'+o.ctLen/2).join(' ')}`);

  console.log('\n--- the recipient of a settlement sees no client detail ---');
  const rd = await pg.evaluate(async ()=>{
    const S=window.__T.state, F=window.__T, t=S.txs.find(x=>x.type==='payment');
    const stl = t.cts.find(c=>c.data.kind==='settlement');
    const cli = t.cts.find(c=>c.data.kind==='client');
    const rs = await F.trialTags(t, stl.to), rc = await F.trialTags(t, cli.to);
    return { stl:{ amount:rs.amount, opens:rs.opens, bf:!!(rs.plain&&rs.plain.beneficiaries), ref:rs.plain&&rs.plain.ref },
             cli:{ amount:rc.amount, opens:rc.opens, bf:rc.plain&&rc.plain.beneficiaries&&rc.plain.beneficiaries.length } };
  });
  pass(rd.stl.opens===true && rd.stl.bf===false && !!rd.stl.ref,
       `the settlement recipient opens its commitment and gets a reference only (${rd.stl.ref})`);
  pass(rd.cli.opens===true && rd.cli.bf===3, 'the client recipient gets its 3 beneficiaries');

  console.log('\n--- and the UI renders each kind for what it is ---');
  const ui = await pg.evaluate(()=>{
    // open every payload card we are allowed to open
    document.querySelectorAll('[data-open-pl]').forEach(b=>b.click());
    return true; });
  await w(pg,900);
  const cards = await pg.evaluate(()=>[...document.querySelectorAll('.pl-plain-hd')].map(h=>h.innerText.replace(/\s+/g,' ')));
  const env = await pg.evaluate(()=>document.querySelector('#envBody .env-foot').innerText.replace(/\s+/g,' '));
  pass(ui && cards.length>0, `${cards.length} payload card(s) decrypted in the UI`);
  pass(cards.some(c=>/interbank settlement/i.test(c)) || cards.length===0,
       `a settlement card names itself: "${cards.find(c=>/interbank/i.test(c))||'(none openable from this persona)'}"`);
  pass(/padded to a fixed 256 bytes/i.test(env) && /272 B each/.test(env),
       `the envelope footer states the padding (256 B plaintext) and the wire size (272 B ciphertext)`);

  pass(errs.length===0, 'no console/page errors'+(errs.length?': '+errs.slice(0,3).join(' | '):''));
  await b.close();
  console.log('\n'+(fails? fails+' CHECK(S) FAILED':'ALL CHECKS PASSED'));
  process.exit(fails?1:0);
})();
