const { chromium, launchOpts, PAGE, shot } = require('./_env.cjs');
const w=(p,ms)=>p.waitForTimeout(ms);
let fails=0; const pass=(c,m)=>{ console.log((c?'  PASS  ':'  FAIL  ')+m); if(!c) fails++; };

// drive a flow to completion by clicking Next until the button reads Complete
async function runFlow(pg, maxStages=6){
  for(let i=0;i<maxStages;i++){
    const txt = await pg.textContent('#tourNext');
    if(/Complete/.test(txt)) break;
    if(await pg.isDisabled('#tourNext')) { await w(pg,400); i--; continue; }
    await pg.click('#tourNext');
    // wait for the stage to finish
    for(let t=0;t<200;t++){
      await w(pg,150);
      const busy = await pg.evaluate(()=>document.querySelector('#tourNext').textContent.includes('Running'));
      if(!busy) break;
    }
  }
}
const snap = pg => pg.evaluate(()=>{
  const st = window.__st || null;
  return {
    phase: document.querySelector('#tourPhase').textContent,
    count: document.querySelector('#tourCount').textContent,
    stages: [...document.querySelectorAll('.tstage .tslab')].map(e=>e.textContent),
    stepper: [...document.querySelectorAll('.tstage')].map(e=>e.className),
    narr: document.querySelector('#tourNarr').textContent.slice(0,90),
    kInst: document.querySelector('#kInst').textContent,
    kAgr: document.querySelector('#kAgr').textContent,
    kAud: document.querySelector('#kAud').textContent,
    txCount: document.querySelector('#txCount').textContent,
    balRows: [...document.querySelectorAll('#balancesBody tr')].map(r=>r.cells[1].innerText.trim()),
    balAmts: [...document.querySelectorAll('#balancesBody tr')].map(r=>r.cells[2].innerText.trim()),
    txKinds: [...document.querySelectorAll('#txList .txrow, #txList > *')].length,
    persona: document.querySelector('#personaName').textContent,
  };
});

(async () => {
  const b=await chromium.launch(launchOpts);
  const pg=await b.newPage({viewport:{width:1440,height:1000}});
  const errs=[]; pg.on('pageerror',e=>errs.push('pageerror: '+e.message));
  pg.on('console',m=>{ if(m.type()==='error') errs.push(m.text()); });
  await pg.goto(PAGE); await w(pg,1800);

  console.log('--- tabs on load ---');
  let t = await pg.evaluate(()=>({
    keysSel: document.querySelector('#tabKeys').getAttribute('aria-selected'),
    paySel:  document.querySelector('#tabPay').getAttribute('aria-selected'),
    keysLab: document.querySelector('#tabKeys').innerText.trim(),
    payLab:  document.querySelector('#tabPay').innerText.trim(),
    keysPaneShown: !document.querySelector('#viewKeys').hidden,
    payPaneShown:  !document.querySelector('#viewPay').hidden,
    title:   document.querySelector('#tourLaunch .tl-text b').textContent,
    btn:     document.querySelector('#tourStart').textContent,
    launchHidden: document.querySelector('#tourLaunch').hidden,
  }));
  pass(t.keysSel==='true' && t.paySel==='false', `Key Setup tab selected on load (${t.keysSel}/${t.paySel})`);
  pass(t.keysLab.includes('Key Setup'), `tab 1 = "${t.keysLab}"`);
  pass(t.payLab.includes('Payments'), `tab 2 = "${t.payLab}"`);
  pass(t.title==='Guided walkthrough', `launch card titled "${t.title}"`);
  pass(t.keysPaneShown && !t.payPaneShown, `only the key-material pane is rendered (keys=${t.keysPaneShown}, pay=${t.payPaneShown})`);
  pass(/Replay/.test(t.btn), `seeded page offers "${t.btn}"`);

  console.log('\n--- switching to the Payments tab (no run active) ---');
  await pg.click('#tabPay'); await w(pg,300);
  t = await pg.evaluate(()=>({
    keysSel: document.querySelector('#tabKeys').getAttribute('aria-selected'),
    paySel:  document.querySelector('#tabPay').getAttribute('aria-selected'),
    title: document.querySelector('#tourLaunch .tl-text b').textContent,
    blurb: document.querySelector('#tourLaunchBlurb').textContent.slice(0,60),
    btn: document.querySelector('#tourStart').textContent,
    keysPaneShown: !document.querySelector('#viewKeys').hidden,
    payPaneShown:  !document.querySelector('#viewPay').hidden,
    kpiShown: !document.querySelector('#kpiStrip').hidden,
    regPanelVisible: !!document.querySelector('#registryBody')?.offsetParent,
    balPanelVisible: !!document.querySelector('#balancesBody')?.offsetParent,
  }));
  pass(t.paySel==='true' && t.keysSel==='false', 'selection moved to Payments');
  pass(/Replay/.test(t.btn), `card offers "${t.btn}"`);
  pass(t.payPaneShown && !t.keysPaneShown, `page swapped panes (keys=${t.keysPaneShown}, pay=${t.payPaneShown})`);
  pass(t.regPanelVisible===false && t.balPanelVisible===true, `registry table hidden, balances table shown (reg=${t.regPanelVisible}, bal=${t.balPanelVisible})`);
  pass(t.kpiShown===false, `channel/audit KPI strip hidden on the Payments tab (${t.kpiShown})`);

  console.log('\n--- KEY MATERIAL flow, run end to end ---');
  await pg.click('#tabKeys'); await w(pg,200);
  await pg.click('#tourStart'); await w(pg,600);
  let s = await snap(pg);
  pass(JSON.stringify(s.stages)===JSON.stringify(['Registration','Key agreement','Channel check']),
       `stepper = ${JSON.stringify(s.stages)}`);
  pass(/stage 1 of 3/.test(s.count), `counter = "${s.count}"`);
  pass(s.kInst==='0', `registry torn down for the replay (institutions=${s.kInst})`);
  await runFlow(pg);
  s = await snap(pg);
  pass(s.phase==='Complete', `flow completed (phase="${s.phase}")`);
  pass(s.kInst==='6', `six institutions registered (${s.kInst})`);
  pass(s.kAgr==='15', `all 15 channels agreed (${s.kAgr})`);
  pass(s.kAud==='15/15', `every channel checked (${s.kAud})`);
  pass(s.txCount==='0', `NO issuance or payment happened in this flow (ledger=${s.txCount})`);
  const allIdentity = s.balRows.slice(0,6).every(c=>/𝒪/.test(c));
  pass(allIdentity, `all six balances still at the identity point (${s.balRows[0]})`);
  pass(/𝒪/.test(s.balRows[6]||''), `supply row at 𝒪 too ("${s.balRows[6]}")`);
  pass(s.persona==='Regulator', `ends on the Regulator perspective (${s.persona})`);
  await pg.screenshot({path:shot('flow-keys-done.png')});

  console.log('\n--- PAYMENTS tab is manual: no walkthrough, no seeded payment ---');
  await pg.click('#tabPay'); await w(pg,500);
  let pv = await pg.evaluate(()=>({
    tourHidden: document.querySelector('.tour').hidden,
    txs: window.__T.state.txs.length,
    payments: window.__T.state.txs.filter(t=>t.type==='payment').length,
    derivEmpty: document.querySelector('#derivBody').innerText.trim(),
    tourActive: window.__T.tourActive(),
    anatomyGone: !document.querySelector('#anatomySec'),
  }));
  pass(pv.tourHidden===true, 'no guided-walkthrough card on the Payments tab');
  pass(pv.payments===0, `no payment is seeded (${pv.txs} ledger entries, all issuance)`);
  pass(/Compose a payment above|Issue opening balances/.test(pv.derivEmpty), `derivation waits for you: "${pv.derivEmpty.slice(0,50)}…"`);
  pass(pv.tourActive===false, 'leaving the Key Setup tab released the walkthrough (controls are live again)');
  pass(pv.anatomyGone, 'the separate Payment anatomy panel is gone (folded into the envelope)');

  console.log('\n--- compose ONE payment by hand and check the arithmetic shown ---');
  // the walkthrough ended on the Regulator lens; only a bank can compose a payment
  await pg.evaluate(()=>document.querySelector('#personaBtn').click()); await w(pg,250);
  await pg.evaluate(()=>{ const b=[...document.querySelectorAll('#personaMenu button')].find(x=>x.innerText.includes('Bank 1')); b&&b.click(); });
  await w(pg,600);
  pass(await pg.evaluate(()=>!!document.querySelector('#composeSec #tfBtn')), 'as a bank, the compose form sits above the derivation');
  // key material was just replayed, so balances are empty: issue them manually
  const mintShown = await pg.evaluate(()=>!document.querySelector('#mintBtn').hidden);
  pass(mintShown, 'an "Issue opening balances" control is offered while accounts are empty');
  await pg.click('#mintBtn'); await w(pg,3500);
  pass(await pg.evaluate(()=>document.querySelector('#mintBtn').hidden), 'the control retires once every account is funded');
  const before = await pg.evaluate(()=>{
    const rows=[...document.querySelectorAll('#balancesBody tr')];
    return rows[rows.length-1].cells[1].innerText.trim();
  });
  await pg.evaluate(()=>{ const bx=[...document.querySelectorAll('.pay-check input')]; if(!bx[0].checked) bx[0].click(); });
  await w(pg,300);
  await pg.click('#tfBtn'); await w(pg,9000);
  const d = await pg.evaluate(()=>{
    const tx = window.__T.state.txs.find(t=>t.type==='payment');
    const S = window.__T.state;
    const steps = [...document.querySelectorAll('#derivBody .dstep')];
    const rows=[...document.querySelectorAll('#balancesBody tr')];
    return {
      payments: S.txs.filter(t=>t.type==='payment').length,
      steps: steps.length,
      allShown: steps.every(e=>e.classList.contains('in')),
      titles: steps.map(e=>e.querySelector('.dhead .dt b').textContent),
      checks: [...document.querySelectorAll('#derivBody .dcheck')].map(e=>({ ok:e.classList.contains('ok'), t:e.textContent.trim() })),
      rDerived: tx.slots.filter(sl=>!sl.sender).every(sl=>/^[0-9a-f]{64}$/.test(sl.rHex)),
      rSumZero: tx.rSumZero, vSumZero: tx.vSumZero, sums: tx.sumsToIdentity,
      balArith: [...document.querySelectorAll('#derivBody .drow.b')].length,
      supplyAfter: rows[rows.length-1].cells[1].innerText.trim(),
      k: tx.k,
    };
  });
  pass(d.payments===1, `exactly one payment exists, the one composed by hand (${d.payments})`);
  pass(d.steps===6 && d.allShown, `derivation walked ${d.steps} steps, all revealed`);
  pass(/blinding factor/.test(d.titles[1]||''), `step 2 is the blinding-factor derivation ("${d.titles[1]}")`);
  pass(/closes the sum/.test(d.titles[2]||''), `step 3 closes the sum ("${d.titles[2]}")`);
  pass(/onto its account/.test(d.titles[5]||''), `step 6 adds commitments onto balances ("${d.titles[5]}")`);
  pass(d.rDerived, 'every recipient blinding factor is a full 32-byte derived scalar');
  pass(d.checks.length===3 && d.checks.every(c=>c.ok), `all three sum checks pass: ${d.checks.map(c=>c.t.replace(/\s+/g,' ')).join(' | ')}`);
  pass(d.rSumZero && d.vSumZero && d.sums, 'Σr = 0, Σv = 0 and ΣC = 𝒪 all hold');
  pass(d.balArith===d.k, `balance arithmetic shown for all ${d.k} touched accounts`);
  pass(before===d.supplyAfter, `total supply commitment unchanged by the payment (${before.slice(0,14)}…)`);

  console.log('\n--- conservation after the payments flow ---');
  const inv = await pg.evaluate(()=>{
    const rows=[...document.querySelectorAll('#balancesBody tr')];
    return { supply: rows[rows.length-1].cells[1].innerText.trim(), invLabel: rows[rows.length-1].cells[2].innerText.trim() };
  });
  pass(/^[0-9a-f]{2}/.test(inv.supply.replace(/^0x/,'')) || inv.supply.length>10, `supply row is a real commitment ("${inv.supply}")`);
  pass(/invariant/i.test(inv.invLabel), `supply amount cell reads "${inv.invLabel}"`);

  console.log('\n--- cold page with an empty registry: the Payments tab says so ---');
  const pg2 = await b.newPage({viewport:{width:1440,height:1000}});
  const errs2=[]; pg2.on('pageerror',e=>errs2.push('pageerror: '+e.message));
  pg2.on('console',m=>{ if(m.type()==='error') errs2.push(m.text()); });
  await pg2.goto(PAGE); await w(pg2,1800);
  await pg2.click('#tabKeys'); await w(pg2,150);
  await pg2.click('#tourStart'); await w(pg2,900);        // starts key material => registry emptied
  await pg2.click('#tourExit'); await w(pg2,300);
  pass(await pg2.evaluate(()=>document.querySelector('#kInst').textContent)==='0', 'registry emptied');
  await pg2.click('#tabPay'); await w(pg2,600);
  const cold = await pg2.evaluate(()=>({
    deriv: document.querySelector('#derivBody').innerText.trim(),
    env: document.querySelector('#envBody').innerText.trim(),
    mintOffered: !document.querySelector('#mintBtn').hidden,
    balRows: document.querySelectorAll('#balancesBody tr').length,
  }));
  pass(/Key Setup tab/.test(cold.deriv), `derivation points at the right tab: "${cold.deriv.slice(0,52)}…"`);
  pass(cold.mintOffered===false, 'no issuance control offered when there are no accounts to issue to');

  console.log('\n--- register + issue + pay, all by hand on a cold page ---');
  await pg2.click('#tabKeys'); await w(pg2,300);
  for(let i=0;i<3;i++){
    await pg2.click('#regBtn'); await w(pg2,250);
    const o = await pg2.$('#regMenu button[data-code]');
    await o.click(); await w(pg2,900);
  }
  pass(await pg2.evaluate(()=>document.querySelector('#kInst').textContent)==='3', 'three banks registered by hand');
  await pg2.click('#tabPay'); await w(pg2,500);
  pass(await pg2.evaluate(()=>!document.querySelector('#mintBtn').hidden), 'now issuance is offered');
  await pg2.click('#mintBtn'); await w(pg2,2500);
  await pg2.evaluate(()=>document.querySelector('#personaBtn').click()); await w(pg2,250);
  await pg2.evaluate(()=>{ const b=[...document.querySelectorAll('#personaMenu button')].find(x=>x.innerText.includes('Bank 1')); b&&b.click(); });
  await w(pg2,600);
  await pg2.evaluate(()=>{ const bx=[...document.querySelectorAll('.pay-check input')]; if(!bx[0].checked) bx[0].click(); });
  await w(pg2,300);
  await pg2.click('#tfBtn'); await w(pg2,9000);
  const small = await pg2.evaluate(()=>{
    const tx = window.__T.state.txs.find(t=>t.type==='payment');
    return { k: tx.k, commits: tx.commits.length, tags: tx.tags.length, cts: tx.cts.length,
             sums: tx.sumsToIdentity, steps: document.querySelectorAll('#derivBody .dstep.in').length };
  });
  pass(small.k===3, `k adapts to a 3-bank network (k=${small.k})`);
  pass(small.commits===3 && small.tags===3 && small.cts===3, `still ${small.k} commitments / ${small.tags} tags / ${small.cts} ciphertexts`);
  pass(small.sums===true && small.steps===6, 'conservation holds and the derivation walked all 6 steps');

  console.log('\n--- tabs are inert mid-stage ---');
  await pg2.click('#tabKeys'); await w(pg2,200);
  await pg2.click('#tourStart'); await w(pg2,600);
  await pg2.click('#tourNext'); await w(pg2,500);
  await pg2.evaluate(()=>document.querySelector('#tabPay').click());   // must be refused mid-stage
  const midStage = await pg2.evaluate(()=>({ busyDisabled: document.querySelector('#tabPay').disabled, next: document.querySelector('#tourNext').textContent, stillKeys: !document.querySelector('#viewKeys').hidden }));
  pass(midStage.busyDisabled===true, `tabs disabled while "${midStage.next.trim()}"`);
  pass(midStage.stillKeys===true, 'a mid-stage tab click did not swap the pane out from under the animation');

  pass(errs.length===0, 'no console/page errors (page 1)'+(errs.length?': '+errs.slice(0,3).join(' | '):''));
  pass(errs2.length===0, 'no console/page errors (page 2)'+(errs2.length?': '+errs2.slice(0,3).join(' | '):''));
  await b.close();
  console.log('\n'+(fails? fails+' CHECK(S) FAILED':'ALL CHECKS PASSED'));
  process.exit(fails?1:0);
})();
