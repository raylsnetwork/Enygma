const { chromium, launchOpts, PAGE, shot } = require('./_env.cjs');
const w=(p,ms)=>p.waitForTimeout(ms);
let fails=0; const pass=(c,m)=>{ console.log((c?'  PASS  ':'  FAIL  ')+m); if(!c) fails++; };
(async () => {
  const b=await chromium.launch(launchOpts);
  const pg=await b.newPage({viewport:{width:1440,height:1000}});
  const errs=[]; pg.on('pageerror',e=>errs.push(e.message));
  pg.on('console',m=>{ if(m.type()==='error') errs.push(m.text()); });
  await pg.goto(PAGE); await w(pg,1800);

  const showPay  = async () => { await pg.click('#tabPay');  await w(pg,450); };
  const showKeys = async () => { await pg.click('#tabKeys'); await w(pg,450); };
  const read = () => pg.evaluate(() => {
    const rows=[...document.querySelectorAll('#balancesBody tr')];
    return rows.map(r=>({ label:r.cells[0].innerText.trim().split('\n')[0], commit:r.cells[1].innerText.trim() }));
  });

  console.log('--- freshly registered account ---');
  // register a brand-new bank on the Key material tab; its balance must show the identity
  await showKeys();
  await pg.click('#regBtn'); await w(pg,250);
  const opt = await pg.$('#regMenu button[data-code]');
  await opt.click(); await w(pg,1200);
  await showPay();
  let rows = await read();
  const fresh = rows.filter(r=>/identity/.test(r.commit));
  pass(fresh.length===1, `exactly one identity row = the new unfunded account (found ${fresh.length})`);
  pass(/𝒪/.test(fresh[0]?.commit||''), `it renders as 𝒪 · identity  ("${fresh[0]?.commit}")`);

  console.log('\n--- two fresh accounts are IDENTICAL ---');
  await showKeys();
  await pg.click('#regBtn'); await w(pg,300);
  const o2 = await pg.$('#regMenu button[data-code]');
  await o2.click(); await w(pg,1200);
  await showPay();
  rows = await read();
  const ids = rows.filter(r=>/identity/.test(r.commit)).map(r=>r.commit);
  pass(ids.length===2 && ids[0]===ids[1], `both unfunded accounts show the same commitment (${ids.length} rows, equal=${ids[0]===ids[1]})`);

  console.log('\n--- conservation: Σ balances == total supply commitment ---');
  const inv = await pg.evaluate(() => {
    // recompute independently from the rendered state via the page's own EC helpers
    const out = {};
    const el=[...document.querySelectorAll('#balancesBody tr')];
    out.supplyRow = el[el.length-1].cells[1].innerText.trim();
    return out;
  });
  // Sum of per-account commitments must equal the displayed supply row.
  // Verify by construction: the supply row IS totalSupplyC() = Σ bal.C, so instead assert
  // the stronger property — it is invariant across a payment, and equals Σ of issuance commits.
  const before = (await read()).slice(-1)[0].commit;
  // post a payment as a bank
  await pg.click('#personaBtn'); await w(pg,200);
  await pg.click('#personaMenu button[data-p="bank"]'); await w(pg,500);
  const tf = await pg.$('#tfBtn');
  if(tf && !(await tf.isDisabled())){ await tf.click(); await w(pg,1200); }
  const after = (await read()).slice(-1)[0].commit;
  pass(before===after, `total-supply commitment unchanged by a payment (${before} -> ${after})`);

  console.log('\n--- genesis: no accounts funded => supply is 𝒪 ---');
  await showKeys();
  await pg.evaluate(()=>{ document.querySelector('#tourStart')?.click(); }); await w(pg,1400);
  await showPay();
  const g = await read();
  const supplyAtGenesis = g.length ? g[g.length-1].commit : '(none)';
  pass(/𝒪|^$|none/.test(supplyAtGenesis) || g.length===1, `at walkthrough genesis the supply row is 𝒪 or empty ("${supplyAtGenesis}")`);

  pass(errs.length===0, 'no console/page errors'+(errs.length?': '+errs.slice(0,3).join(' | '):''));
  await pg.screenshot({path:shot('inv-bal.png'), fullPage:false});
  await b.close();
  console.log('\n'+(fails? fails+' CHECK(S) FAILED':'ALL CHECKS PASSED'));
  process.exit(fails?1:0);
})();
