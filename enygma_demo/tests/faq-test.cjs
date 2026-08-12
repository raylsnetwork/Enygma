const { chromium, launchOpts, PAGE } = require('./_env.cjs');
const w=(p,ms)=>p.waitForTimeout(ms);
let fails=0; const pass=(c,m)=>{ console.log((c?'  PASS  ':'  FAIL  ')+m); if(!c) fails++; };
(async () => {
  const b=await chromium.launch(launchOpts);
  const pg=await b.newPage({viewport:{width:1440,height:1000}});
  const errs=[]; pg.on('pageerror',e=>errs.push('pageerror: '+e.message));
  pg.on('console',m=>{ if(m.type()==='error') errs.push(m.text()); });
  await pg.goto(PAGE); await w(pg,1800);

  console.log('--- FAQ lives in the Protocol tab ---');
  pass(await pg.isHidden('#faqBlock'), 'FAQ hidden while the Key Setup tab is showing');
  await pg.click('#tabProtocol'); await w(pg,900);
  pass(await pg.isVisible('#faqBlock'), 'FAQ visible on the Protocol tab');

  const shape = await pg.evaluate(()=>{
    const blk = document.querySelector('#faqBlock');
    const ds = [...blk.querySelectorAll('details.faq')];
    return { inProtocol: !!blk.closest('#protocol'),
             last: document.querySelector('#protocol').lastElementChild.id,
             groups: [...blk.querySelectorAll('.faq-gh')].map(g=>g.textContent.trim()),
             n: ds.length,
             anyOpen: ds.some(d=>d.open),
             count: document.querySelector('#faqCount').textContent,
             btn: document.querySelector('#faqAll').textContent,
             emptyA: ds.filter(d=>!d.querySelector('.faq-a') || d.querySelector('.faq-a').textContent.trim().length < 80).length,
             qs: ds.map(d=>d.querySelector('summary.faq-q').textContent.trim()),
             chevs: ds.filter(d=>!d.querySelector('summary .chev')).length,
             pline09: (()=>{ const steps=[...document.querySelectorAll('#protocol .pstep')];
               const nine=steps[steps.length-2]; const l=nine.querySelector('.pline');
               return getComputedStyle(l).display; })(),
             plineFaq: getComputedStyle(blk.querySelector('.pline')).display };
  });
  pass(shape.inProtocol, 'FAQ is inside #protocol');
  pass(shape.last==='faqBlock', 'FAQ is the last block of the Protocol tab');
  pass(shape.n>=25, `${shape.n} questions`);
  pass(shape.groups.length===6, `6 sections: ${shape.groups.join(' | ')}`);
  pass(!shape.anyOpen, 'all answers start collapsed');
  pass(shape.emptyA===0, 'every question has a substantive answer');
  pass(shape.chevs===0, 'every summary has a chevron');
  pass(shape.count===shape.n+' questions · 6 sections', `counter reflects reality: "${shape.count}"`);
  pass(shape.btn==='Expand all', 'button starts as "Expand all"');
  pass(shape.plineFaq==='none', 'FAQ rail line suppressed (last step)');
  pass(shape.pline09!=='none', 'step 09 rail line restored now that FAQ follows it');

  console.log('\n--- the two explicitly requested questions are answered ---');
  const qlist = shape.qs.map(s=>s.toLowerCase());
  pass(qlist.some(q=>q.includes('quantum')), 'a "how is this quantum private?" entry exists');
  pass(qlist.some(q=>q.includes('cryptography')), 'a "what cryptography does it use?" entry exists');

  console.log('\n--- answers are grounded in the spec, not invented ---');
  const txt = (await pg.evaluate(()=>document.querySelector('#faqBlock').textContent)).replace(/\s+/g,' ');
  for(const t of ['ML-KEM-768','FIPS 203','Groth16','BN254','Poseidon','secp256k1','HKDF','AES-256-GCM',
                  'Hash-To-Curve','nullifier','baby-step giant-step','Merkle','tokenId','deadline',
                  'Harvest now, decrypt later']){
    pass(txt.includes(t), `mentions ${t}`);
  }
  pass(/perfectly.{0,3} hiding/i.test(txt) && /binding/i.test(txt), 'distinguishes hiding from binding');
  pass(/Not post-quantum/i.test(txt), 'states plainly what is NOT post-quantum');
  pass(/Substituted/i.test(txt), 'discloses what this build substitutes');

  console.log('\n--- expand / collapse ---');
  await pg.click('#faqAll'); await w(pg,400);
  const afterExpand = await pg.evaluate(()=>({
    open: [...document.querySelectorAll('#faqBlock details.faq')].filter(d=>d.open).length,
    btn: document.querySelector('#faqAll').textContent }));
  pass(afterExpand.open===shape.n, `expand all opened ${afterExpand.open}/${shape.n}`);
  pass(afterExpand.btn==='Collapse all', 'button flipped to "Collapse all"');
  await pg.click('#faqAll'); await w(pg,400);
  const afterCollapse = await pg.evaluate(()=>({
    open: [...document.querySelectorAll('#faqBlock details.faq')].filter(d=>d.open).length,
    btn: document.querySelector('#faqAll').textContent }));
  pass(afterCollapse.open===0, 'collapse all closed everything');
  pass(afterCollapse.btn==='Expand all', 'button flipped back');

  // clicking one summary opens exactly one, and the button label follows
  await pg.evaluate(()=>document.querySelectorAll('#faqBlock summary.faq-q')[1].click()); await w(pg,300);
  const one = await pg.evaluate(()=>({
    open: [...document.querySelectorAll('#faqBlock details.faq')].filter(d=>d.open).length,
    btn: document.querySelector('#faqAll').textContent }));
  pass(one.open===1, 'clicking one question opens exactly that one');
  pass(one.btn==='Expand all', 'button still offers "Expand all" while one is open');

  console.log('\n--- layout holds at narrow widths ---');
  await pg.click('#faqAll'); await w(pg,500);
  for(const width of [1440, 820, 430]){
    await pg.setViewportSize({width, height:1000}); await w(pg,400);
    // the header nav scrolls horizontally by design at narrow widths, so compare the
    // Protocol tab's page width against a tab that has no FAQ rather than against the viewport
    const base = await pg.evaluate(async ()=>{
      const scrollW = () => document.documentElement.scrollWidth;
      document.querySelector('#tabKeys').click(); await new Promise(r=>setTimeout(r,250));
      const keys = scrollW();
      document.querySelector('#tabProtocol').click(); await new Promise(r=>setTimeout(r,250));
      const proto = scrollW();
      const blk=document.querySelector('#faqBlock'); const r=blk.getBoundingClientRect();
      let worst=0;
      blk.querySelectorAll('*').forEach(el=>{ worst=Math.max(worst, el.getBoundingClientRect().right - r.right); });
      return { keys, proto, worst: Math.round(worst) };
    });
    pass(base.proto<=base.keys, `${width}px: FAQ adds no page width (protocol ${base.proto} ≤ baseline ${base.keys})`);
    pass(base.worst<=2, `${width}px: nothing overflows the FAQ box (worst +${base.worst}px)`);
  }
  await pg.setViewportSize({width:1440,height:1000}); await w(pg,300);

  // other tabs still work with the FAQ present
  for(const t of ['#tabKeys','#tabPay','#tabDvp','#tabProtocol']){ await pg.click(t); await w(pg,500); }
  pass(errs.length===0, 'no console/page errors'+(errs.length?': '+errs.slice(0,3).join(' | '):''));
  await b.close();
  console.log('\n'+(fails? fails+' CHECK(S) FAILED':'ALL CHECKS PASSED'));
  process.exit(fails?1:0);
})();
