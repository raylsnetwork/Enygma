const { chromium, launchOpts, PAGE, enterProduct } = require('./_env.cjs');
const w=(p,ms)=>p.waitForTimeout(ms);
let fails=0; const pass=(c,m)=>{ console.log((c?'  PASS  ':'  FAIL  ')+m); if(!c) fails++; };
const cellSel = (a,b) => `#matrix button.cell[data-i="${Math.max(a,b)}"][data-j="${Math.min(a,b)}"]`;
const clickCell = async (pg,a,b) => { await pg.click(cellSel(a,b)); await w(pg,600); };
const persona = async (pg,label) => {
  await pg.evaluate(()=>document.querySelector('#personaBtn').click()); await w(pg,250);
  await pg.evaluate(l=>{ const x=[...document.querySelectorAll('#personaMenu button')].find(e=>e.innerText.includes(l)); x&&x.click(); }, label);
  await w(pg,700);
};
(async () => {
  const b=await chromium.launch(launchOpts);
  const pg=await b.newPage({viewport:{width:1440,height:1000}});
  const errs=[]; pg.on('pageerror',e=>errs.push('pageerror: '+e.message));
  pg.on('console',m=>{ if(m.type()==='error') errs.push(m.text()); });
  await pg.goto(PAGE); await w(pg,2000);
  await enterProduct(pg, 'institutional');

  // Bank 1 discloses its channel with Bank 2 to the Regulator
  const sel = await pg.evaluate(()=>{
    const S=window.__T.state;
    S.sel = { i:0, j:1 };
    return { verifiedBefore: S.agr.get('0-1').verified };
  });
  pass(sel.verifiedBefore===false, 'pair B1⇄B2 starts unaudited');
  await clickCell(pg,0,1);
  const disc = await pg.evaluate(()=>{
    const btn=document.querySelector('[data-disc-ch]'); if(btn){ btn.click(); return true; } return false; });
  await w(pg,600);
  pass(disc===true, 'Bank 1 discloses the B1⇄B2 channel key to the Regulator');

  await persona(pg,'Regulator');
  await clickCell(pg,0,1);
  const pre = await pg.evaluate(()=>{
    const btn=document.querySelector('[data-open]');
    return { label: btn ? btn.textContent.trim() : null,
             verdict: document.querySelector('.verdict').className,
             kpi: document.querySelector('#kAud').textContent,
             cellCls: document.querySelector('#matrix button.cell[data-i="1"][data-j="0"]').className };
  });
  pass(/Decapsulate ctxt with delegated view key/.test(pre.label||''), `the regulator sees "${pre.label}"`);
  pass(/pending/.test(pre.verdict), 'and the verdict still reads Awaiting audit before decapsulating');
  const kpiBefore = Number(pre.kpi.split('/')[0]);
  // how many agreed-but-unaudited pairs the button offers right now — the registry size decides it
  const agreedBefore = await pg.evaluate(()=>
    Number((document.querySelector('#runAllBtn').innerText.match(/\((\d+)\)/)||[])[1]));

  console.log('\n--- decapsulating IS the audit for that index ---');
  await pg.evaluate(()=>document.querySelector('[data-open]').click()); await w(pg,900);
  const post = await pg.evaluate(()=>{
    const S=window.__T.state, r=S.agr.get('0-1');
    return { verified:r.verified, via:r.auditVia, recomputed:r.recomputed, cid:r.channelId,
             verdict: document.querySelector('.verdict').className,
             vtext: document.querySelector('.verdict').innerText.replace(/\s+/g,' '),
             vault: document.querySelector('.vault').innerText.replace(/\s+/g,' '),
             kpi: document.querySelector('#kAud').textContent,
             cellCls: document.querySelector('#matrix button.cell[data-i="1"][data-j="0"]').className,
             logRow: [...document.querySelectorAll('.audit-log .logrow')].map(r=>r.innerText.replace(/\s+/g,' ')).find(t=>/B1⇄B2/.test(t)),
             runAll: document.querySelector('#runAllBtn').innerText.trim() };
  });
  pass(post.verified===true, 'the pair is now marked verified in state');
  pass(post.via==='decaps', `recorded route is decapsulation (${post.via})`);
  pass(post.recomputed===post.cid, 'SHA256(0x01 ‖ ss) equals the on-chain channelId — a real equality, not a flag flip');
  pass(/ok/.test(post.verdict) && !/pending/.test(post.verdict), 'the verdict box flipped to Verified');
  pass(/by decapsulation/i.test(post.vtext), 'and names the route it took');
  pass(/SHA256\(0x01/i.test(post.vtext), 'showing the compared value');
  pass(/is the audit for this index/i.test(post.vault) || /now marked verified/i.test(post.vault),
       'the vault says the equality settled this index');
  pass(/ok|posted ok/.test(post.cellCls), `the matrix cell is now a verified cell ("${post.cellCls}")`);
  pass(Number(post.kpi.split('/')[0])===kpiBefore+1, `the audit KPI advanced ${pre.kpi} → ${post.kpi}`);
  pass(/decaps/.test(post.logRow||''), `the audit log records the route ("${post.logRow}")`);
  // the registry holds ten banks, so 45 pairs exist and 42 are agreed; one just got audited
  const remaining = Number((post.runAll.match(/\((\d+)\)/)||[])[1]);
  pass(remaining === agreedBefore - 1,
       `Run audit now offers the remaining ${agreedBefore - 1} ("${post.runAll}")`);

  console.log('\n--- a mismatch would NOT be marked audited ---');
  const bad = await pg.evaluate(async ()=>{
    const S=window.__T.state, F=window.__T;
    const r = S.agr.get('0-2');
    r.discReg = true;                 // disclosed
    r.channelId = '0'.repeat(64);     // but the matrix entry disagrees with the secret
    r.opened = false; r.verified = false; delete r.recomputed; delete r.auditVia;
    S.sel = { i:0, j:2 };
    return true;
  });
  await clickCell(pg,0,2);
  await pg.evaluate(()=>{ const b=document.querySelector('[data-open]'); if(b) b.click(); }); await w(pg,800);
  const mm = await pg.evaluate(()=>{
    const r=window.__T.state.agr.get('0-2');
    return { verified:r.verified, cls:document.querySelector('.verdict').className,
             txt:document.querySelector('.verdict').innerText.replace(/\s+/g,' ') };
  });
  pass(bad && mm.verified===false, 'a channel whose commitment disagrees is NOT marked audited');
  pass(/bad/.test(mm.cls), 'it renders as a mismatch verdict');
  pass(/not.{0,4} marked audited/i.test(mm.txt) && /Mismatch/i.test(mm.txt), 'and says so explicitly');

  console.log('\n--- a bank opening its own channel does not self-audit ---');
  await persona(pg,'Bank 3');
  const bankOpen = await pg.evaluate(async ()=>{
    const S=window.__T.state; const r=S.agr.get('2-3');
    r.opened=false; r.verified=false; delete r.recomputed; delete r.auditVia;
    S.sel={i:2,j:3}; return r.verified; });
  await clickCell(pg,2,3);
  await pg.evaluate(()=>{ const b=document.querySelector('[data-open]'); if(b) b.click(); }); await w(pg,700);
  const after = await pg.evaluate(()=>{
    const r=window.__T.state.agr.get('2-3');
    return { opened:r.opened, verified:r.verified, via:r.auditVia }; });
  pass(after.opened===true, 'Bank 3 can still open its own channel');
  pass(after.verified===false && after.via===undefined, 'but that is not an audit — only the regulator marks an index');

  pass(errs.length===0, 'no console/page errors'+(errs.length?': '+errs.slice(0,3).join(' | '):''));
  await b.close();
  console.log('\n'+(fails? fails+' CHECK(S) FAILED':'ALL CHECKS PASSED'));
  process.exit(fails?1:0);
})();
