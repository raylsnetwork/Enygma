/* The suite shell: one identity, generated once, carried into all four protocols.
 *
 * These checks re-derive the generated keys with their own code rather than trusting what the
 * page renders, and then follow that same public key into each product's on-chain registry.
 */
const { chromium, launchOpts, PAGE, enterProduct } = require('./_env.cjs');
const w=(p,ms)=>p.waitForTimeout(ms);
let fails=0; const pass=(c,m)=>{ console.log((c?'  PASS  ':'  FAIL  ')+m); if(!c) fails++; };

const REGISTER_BTN = { payment:'#onboardBtn', retail:'#btnRegisterTen', dvp:'#btnRegisterTen', auctions:'#btnRegisterTen' };

const PRODUCTS = [
  { key:'payment',  route:'institutional', label:'Institutional Payments' },
  { key:'retail',   route:'retail',        label:'Retail Payments' },
  { key:'dvp',      route:'dvp',           label:'DvP' },
  { key:'auctions', route:'auctions',      label:'Auctions' },
];

// how each product exposes its party 0 — the payment app keeps its original __T hook
const party0 = (pg, key) => pg.evaluate((k)=>{
  const E = window.__ENYGMA;
  const p = k === 'payment' ? E.payment.state.insts[0] : E[k].users()[0];
  return p ? { pkSpend: p.pkSpend, pkView: p.pkView || null, isYou: !!p.isYou } : null;
}, key);

(async () => {
  const b=await chromium.launch(launchOpts);
  const pg=await b.newPage({viewport:{width:1440,height:1000}});
  const errs=[]; pg.on('pageerror',e=>errs.push('pageerror: '+e.message));
  pg.on('console',m=>{ if(m.type()==='error') errs.push(m.text()); });
  await pg.goto(PAGE); await w(pg,1500);

  console.log('--- the page opens on step 1, not on a product ---');
  const start = await pg.evaluate(()=>({
    step: window.__ENYGMA.step,
    identity: window.__ENYGMA.identity,
    hash: location.hash,
    keysVisible: !document.getElementById('wizKeys').hidden,
    chooseVisible: !document.getElementById('wizChoose').hidden,
    productsVisible: [...document.querySelectorAll('[id^=app-]')].filter(e=>getComputedStyle(e).display!=='none').map(e=>e.id),
  }));
  pass(start.step==='keys', `opens on the key-generation step (step = ${start.step})`);
  pass(start.identity===null, 'no identity exists before you generate one');
  pass(start.keysVisible && !start.chooseVisible, 'step 1 is showing, step 2 is not');
  pass(start.productsVisible.length===0, `no product is rendered yet (${start.productsVisible.length} visible)`);
  pass(start.hash==='#/keys', `route reflects the step (${start.hash})`);

  console.log('\n--- you cannot skip ahead without keys ---');
  const gated = await pg.evaluate(()=>{
    location.hash = '#/dvp';
    return new Promise(r=>setTimeout(()=>r({ step: window.__ENYGMA.step, hash: location.hash }), 300));
  });
  pass(gated.step==='keys', 'asking for a product before generating keys lands back on step 1');
  pass(gated.hash==='#/keys', 'the route is corrected too');
  const nextDisabled = await pg.evaluate(()=>document.getElementById('btnKeysNext').disabled);
  pass(nextDisabled, 'the Continue button is disabled until keys exist');

  console.log('\n--- generation is real ---');
  await pg.click('#btnGenKeys');
  await pg.waitForFunction(()=>!!(window.__ENYGMA && window.__ENYGMA.identity), null, {timeout:20000});
  const id = await pg.evaluate(()=>window.__ENYGMA.identity);
  pass(/^[0-9a-f]{64}$/.test(id.skSpend), `sk_spend is 32 bytes (${id.skSpend.length/2} B)`);
  pass(/^[0-9a-f]{64}$/.test(id.pkSpend), `pk_spend is 32 bytes (${id.pkSpend.length/2} B)`);
  pass(/^[0-9a-f]{64}$/.test(id.viewSeed), `sk_view is 32 bytes (${id.viewSeed.length/2} B)`);
  pass(/^[0-9a-f]{64}$/.test(id.pkView), `pk_view is 32 bytes (${id.pkView.length/2} B)`);
  pass(id.skSpend!==id.viewSeed, 'the two secrets are independently drawn');

  // re-derive both public halves here, with our own code, from the secrets the page exposed
  const derived = await pg.evaluate(async ()=>{
    const id = window.__ENYGMA.identity, enc = new TextEncoder();
    const un = h => Uint8Array.from(h.match(/../g).map(x=>parseInt(x,16)));
    const hx = b => [...new Uint8Array(b)].map(x=>x.toString(16).padStart(2,'0')).join('');
    const cat = (a,b)=>{ const o=new Uint8Array(a.length+b.length); o.set(a,0); o.set(b,a.length); return o; };
    return {
      pkSpend: hx(await crypto.subtle.digest('SHA-256', cat(enc.encode('enygma/spend/v1'), un(id.skSpend)))),
      pkView:  hx(await crypto.subtle.digest('SHA-256', cat(enc.encode('enygma/mlkem768/ek/v1'), un(id.viewSeed)))),
    };
  });
  pass(derived.pkSpend===id.pkSpend, 'pk_spend re-derives from sk_spend under its own domain label');
  pass(derived.pkView===id.pkView,   'pk_view re-derives from sk_view under its own domain label');

  console.log('\n--- the secrets are not on display until asked for ---');
  const sealed = await pg.evaluate(()=>{
    const row = document.getElementById('rowSkSpend');
    return { sealedClass: row.classList.contains('sealed'), open: row.dataset.open || '0',
             text: document.getElementById('valSkSpend').textContent };
  });
  pass(sealed.sealedClass && sealed.open!=='1', 'sk_spend renders sealed by default');
  pass(sealed.text.length===64, 'the value is present in the DOM — sealed visually, not withheld');

  console.log('\n--- entropy: two generations never agree ---');
  const second = await pg.evaluate(async ()=>{
    const first = window.__ENYGMA.identity.skSpend;
    await window.__ENYGMA.generateKeys();
    return { first, second: window.__ENYGMA.identity.skSpend };
  });
  pass(second.first!==second.second, 'regenerating draws fresh randomness');

  console.log('\n--- the same identity reaches every product ---');
  await pg.close();
  const seen = [];
  for(const prod of PRODUCTS){
    const p2 = await b.newPage({viewport:{width:1440,height:1000}});
    const e2=[]; p2.on('pageerror',e=>e2.push('pageerror: '+e.message));
    p2.on('console',m=>{ if(m.type()==='error') e2.push(m.text()); });
    await p2.goto(PAGE); await w(p2,900);
    // arrive without registering, so the empty registry is observable
    const identity = await enterProduct(p2, prod.key, {register:false, settle:1200});
    const empty = await p2.evaluate((k)=> k==='payment' ? window.__T.state.insts.length
                                                        : window.__ENYGMA[k].users().length, prod.key);
    pass(empty===0, `${prod.label}: the registry starts empty (${empty} parties)`);
    pass(await p2.isVisible(`#app-${prod.key} ${REGISTER_BTN[prod.key]}`),
         `${prod.label}: the registration run is offered`);

    // now watch it fill
    await p2.click(`#app-${prod.key} ${REGISTER_BTN[prod.key]}`);
    const seenGrowing = await p2.evaluate((k)=> new Promise(res=>{
      const count = ()=> k==='payment' ? window.__T.state.insts.length : window.__ENYGMA[k].users().length;
      const marks = new Set(); const t0 = Date.now();
      const iv = setInterval(()=>{
        marks.add(count());
        if(count()>=10 || Date.now()-t0 > 120000){ clearInterval(iv); res([...marks].sort((a,b)=>a-b)); }
      }, 250);
    }), prod.key);
    pass(seenGrowing.length > 3,
         `${prod.label}: the table fills incrementally, not in one jump (saw ${seenGrowing.length} distinct counts)`);
    await w(p2, 3000);
    const total = await p2.evaluate((k)=> k==='payment' ? window.__T.state.insts.length
                                                        : window.__ENYGMA[k].users().length, prod.key);
    pass(total===10, `${prod.label}: 10 members registered (${total})`);

    const p0 = await party0(p2, prod.key);
    pass(!!p0, `${prod.label}: you are the first party registered`);
    pass(p0 && p0.pkSpend===identity.pkSpend, `${prod.label}: party 0 carries the pk_spend from step 1`);
    pass(p0 && p0.isYou===true, `${prod.label}: party 0 is flagged as you`);
    if(prod.key!=='auctions'){
      pass(p0 && p0.pkView===identity.pkView, `${prod.label}: party 0 carries the pk_view from step 1`);
    } else {
      pass(p0 && !p0.pkView, `${prod.label}: no per-user view key — auctions need the spend key only`);
    }
    const card = await p2.evaluate((k)=>{
      const el = document.querySelector(`#app-${k} .idcard`);
      return el ? el.textContent.replace(/\s+/g,' ').trim() : null;
    }, prod.key);
    pass(!!card && card.includes('Your identity'), `${prod.label}: the identity card is rendered in its setup pane`);
    const route = await p2.evaluate(()=>location.hash);
    pass(route===`#/${prod.route}`, `${prod.label}: deep-link route is ${route}`);
    pass(e2.length===0, `${prod.label}: no console/page errors` + (e2.length?`: ${e2[0]}`:''));
    seen.push(identity.pkSpend);
    await p2.close();
  }
  pass(new Set(seen).size===seen.length, 'each fresh visit generates its own distinct identity');

  console.log('\n--- one product at a time ---');
  const p3 = await b.newPage({viewport:{width:1440,height:1000}});
  await p3.goto(PAGE); await w(p3,900);
  await enterProduct(p3, 'dvp', {settle:1500});
  const onlyOne = await p3.evaluate(()=>
    [...document.querySelectorAll('[id^=app-]')].filter(e=>getComputedStyle(e).display!=='none').map(e=>e.id));
  pass(onlyOne.length===1 && onlyOne[0]==='app-dvp', `only the chosen product renders (${onlyOne.join(',') || 'none'})`);
  const booted = await p3.evaluate(()=>window.__ENYGMA.booted());
  pass(booted.length===1 && booted[0]==='dvp', `only the chosen product was booted (${booted.join(',')})`);

  // going back and forward again must not re-seed the product
  const stable = await p3.evaluate(async ()=>{
    const before = window.__ENYGMA.dvp.users().length;
    location.hash = '#/choose';
    await new Promise(r=>setTimeout(r,250));
    location.hash = '#/dvp';
    await new Promise(r=>setTimeout(r,600));
    return { before, after: window.__ENYGMA.dvp.users().length, booted: window.__ENYGMA.booted() };
  });
  pass(stable.before===stable.after, `re-entering does not re-seed (${stable.before} → ${stable.after} parties)`);
  pass(stable.booted.length===1, 'boot runs exactly once per product');
  await p3.close();

  pass(errs.length===0, 'no console/page errors' + (errs.length?`: ${errs[0]}`:''));
  await b.close();
  console.log(fails ? `\n${fails} CHECK(S) FAILED` : '\nALL CHECKS PASSED');
  process.exit(fails ? 1 : 0);
})();
