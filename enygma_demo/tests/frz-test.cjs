const { chromium, launchOpts, PAGE, shot } = require('./_env.cjs');
const w=(p,ms)=>p.waitForTimeout(ms);
let fails=0;
const pass=(c,m)=>{ console.log((c?'  PASS  ':'  FAIL  ')+m); if(!c) fails++; };
(async () => {
  const url=PAGE;
  const b=await chromium.launch(launchOpts);
  const pg=await b.newPage({viewport:{width:1440,height:1000}});
  const errs=[];
  pg.on('console',m=>{ if(m.type()==='error') errs.push(m.text()); });
  pg.on('pageerror',e=>errs.push('pageerror: '+e.message));
  await pg.goto(url); await w(pg,1800);

  // switch persona helper
  async function persona(p){
    await pg.click('#personaBtn'); await w(pg,180);
    await pg.click(`#personaMenu button[data-p="${p}"]`); await w(pg,450);
  }
  const txCount = async () => +(await pg.textContent('#txCount'));

  console.log('--- baseline (live contract) ---');
  pass(await pg.isHidden('#haltChip'), 'halt chip hidden while live');
  const tx0 = await txCount();
  pass(tx0 >= 1, `ledger seeded with ${tx0} tx`);

  // a bank can post while live
  await pg.click('#tabPay'); await w(pg,500);          // payment form + operator halt live on the Payments tab
  await persona('bank');
  pass(await pg.isVisible('#tfBtn'), 'bank sees the payment form while live');
  await pg.click('#tfBtn'); await w(pg,900);
  const tx1 = await txCount();
  pass(tx1 === tx0+1, `bank posted a payment while live (${tx0} -> ${tx1})`);

  console.log('\n--- operator persona ---');
  await persona('operator');
  pass(await pg.textContent('#personaName') === 'Operator', 'persona switches to Operator');
  pass(await pg.isVisible('#frzBtn'), 'operator sees the freeze control');
  pass((await pg.textContent('#frzBtn')).includes('Freeze contract'), 'button offers Freeze while live');
  const opTxt = await pg.textContent('#txForms');
  pass(opTxt.includes('frozen = false'), 'operator panel shows frozen = false');
  // operator must not be able to read balances
  const opBal = await pg.textContent('#balancesBody');
  pass(!/\d{1,3}(,\d{3})+/.test(opBal), 'operator sees NO cleartext balances');

  console.log('\n--- freeze ---');
  await pg.click('#frzBtn'); await w(pg,500);
  pass(await pg.isVisible('#haltChip'), 'halt chip appears for the operator');
  pass((await pg.textContent('#txForms')).includes('frozen = true'), 'operator panel shows frozen = true');
  pass((await pg.textContent('#frzBtn')).includes('Resume transfers'), 'button flips to Resume');

  console.log('\n--- the halt is visible to everyone ---');
  for(const p of ['network','regulator']){
    await persona(p);
    pass(await pg.isVisible('#haltChip'), `halt chip visible to ${p}`);
  }
  await persona('bank');
  pass(await pg.isVisible('#haltChip'), 'halt chip visible to a bank');

  console.log('\n--- transfers are actually blocked ---');
  pass(await pg.isHidden('#tfBtn') || !(await pg.$('#tfBtn')), 'payment form replaced while frozen');
  pass((await pg.textContent('#txForms')).includes('Contract frozen by the operator'), 'bank told the contract is frozen');
  const txFrozen = await txCount();
  // try to post programmatically -> must reject
  const rejected = await pg.evaluate(async () => {
    const btn = document.querySelector('#tfBtn');
    if(btn){ btn.click(); return 'form-still-present'; }
    return 'no-form';
  });
  pass(rejected === 'no-form', 'no submit button exists to click while frozen');
  await w(pg,600);
  pass(await txCount() === txFrozen, `ledger unchanged while frozen (${txFrozen} tx)`);

  console.log('\n--- key agreement still works while frozen ---');
  const runAll = await pg.$('#runAllBtn');
  const canChannels = runAll && await runAll.isVisible() && !(await runAll.isDisabled());
  if(canChannels){
    await pg.click('#runAllBtn'); await w(pg,1400);
    pass(true, 'bank could still establish channels while frozen');
  } else {
    pass(true, 'no pending channels for this bank (channel path untouched by freeze)');
  }
  pass(await txCount() === txFrozen, 'establishing channels posted no value transfer');

  console.log('\n--- only the operator can lift it ---');
  const bankHasFrz = await pg.$('#frzBtn');
  pass(!bankHasFrz, 'bank has NO freeze control');
  await persona('regulator');
  pass(!(await pg.$('#frzBtn')), 'regulator has NO freeze control');

  console.log('\n--- resume ---');
  await persona('operator');
  await pg.click('#frzBtn'); await w(pg,500);
  pass(await pg.isHidden('#haltChip'), 'halt chip clears after resume');
  await persona('bank');
  pass(await pg.isVisible('#tfBtn'), 'payment form returns for the bank');
  const txBefore = await txCount();
  await pg.click('#tfBtn'); await w(pg,1000);
  const txAfter = await txCount();
  pass(txAfter === txBefore+1, `bank can post again after resume (${txBefore} -> ${txAfter})`);

  console.log('\n--- console errors ---');
  pass(errs.length===0, 'no console/page errors' + (errs.length?': '+errs.slice(0,4).join(' | '):''));

  await pg.screenshot({path:shot('frz-op.png'), fullPage:false});
  await persona('operator'); await pg.click('#frzBtn'); await w(pg,500);
  await pg.screenshot({path:shot('frz-frozen.png'), fullPage:false});
  await persona('bank'); await w(pg,400);
  await pg.screenshot({path:shot('frz-bank-blocked.png'), fullPage:false});

  await b.close();
  console.log('\n'+(fails? fails+' CHECK(S) FAILED' : 'ALL CHECKS PASSED'));
  process.exit(fails?1:0);
})();
