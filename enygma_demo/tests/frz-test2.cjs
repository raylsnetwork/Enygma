const { chromium, launchOpts, PAGE, shot } = require('./_env.cjs');
const w=(p,ms)=>p.waitForTimeout(ms);
let fails=0;
const pass=(c,m)=>{ console.log((c?'  PASS  ':'  FAIL  ')+m); if(!c) fails++; };
(async () => {
  const url=PAGE;
  const b=await chromium.launch(launchOpts);

  for(const scheme of ['light','dark']){
    const pg=await b.newPage({viewport:{width:1440,height:1000},colorScheme:scheme});
    const errs=[]; pg.on('pageerror',e=>errs.push(e.message));
    pg.on('console',m=>{ if(m.type()==='error') errs.push(m.text()); });
    await pg.goto(url); await w(pg,1600);
    async function persona(p){ await pg.click('#personaBtn'); await w(pg,180); await pg.click(`#personaMenu button[data-p="${p}"]`); await w(pg,420); }

    console.log(`--- ${scheme}: freeze then run the walkthrough ---`);
    await pg.click('#tabPay'); await w(pg,500);        // the halt control lives on the Payments tab
    await persona('operator');
    await pg.click('#frzBtn'); await w(pg,400);
    pass(await pg.isVisible('#haltChip'), `[${scheme}] frozen before walkthrough`);
    await pg.screenshot({path:shot(`frz-op-${scheme}.png`)});

    // the walkthrough lives on the Key material tab; starting it must reset to a live contract
    await pg.click('#tabKeys'); await w(pg,500);
    await pg.click('#tourStart'); await w(pg,1600);
    pass(await pg.isHidden('#haltChip'), `[${scheme}] walkthrough reset lifted the freeze`);
    // autoplay all stages, poll until it reports Complete
    await pg.click('#tourPlay'); 
    let phase='';
    for(let i=0;i<80;i++){
      await w(pg,1500);
      phase = await pg.textContent('#tourPhase');
      if(/Complete/i.test(phase)) break;
    }
    pass(/Complete/i.test(phase), `[${scheme}] walkthrough completed after a freeze (phase: ${phase})`);
    // a payment is composed by hand afterwards — that is what a freeze would have blocked
    await pg.click('#tourExit').catch(()=>{}); await w(pg,300);
    await pg.click('#tabPay'); await w(pg,600);
    await pg.click('#mintBtn').catch(()=>{}); await w(pg,3000);
    await persona('bank'); await w(pg,500);
    await pg.click('#tfBtn'); await w(pg,9000);
    const txn = +(await pg.textContent('#txCount'));
    pass(txn > 0, `[${scheme}] a hand-composed payment went through on the live contract (${txn} ledger entries)`);
    pass(errs.length===0, `[${scheme}] no errors`+(errs.length?': '+errs.slice(0,3).join(' | '):''));

    // freeze again post-tour and screenshot the bank-blocked state
    await pg.click('#tourExit').catch(()=>{}); await w(pg,300);
    await persona('operator'); await pg.click('#frzBtn'); await w(pg,400);
    await pg.screenshot({path:shot(`frz-frozen-${scheme}.png`)});
    await persona('bank'); await w(pg,400);
    await pg.screenshot({path:shot(`frz-blocked-${scheme}.png`)});
    await pg.close();
  }
  await b.close();
  console.log('\n'+(fails? fails+' CHECK(S) FAILED' : 'ALL CHECKS PASSED'));
  process.exit(fails?1:0);
})();
