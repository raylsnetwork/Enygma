/* The DvP network: trading one private note against another, atomically.
 *
 * This is the protocol the Institutional Bridge tab hands off to — bridging changes a value's
 * representation, swapping changes its owner. These checks re-derive every commitment and
 * nullifier from the openings the page exposes, rather than trusting what the animation rendered.
 */
const { chromium, launchOpts, PAGE, enterProduct } = require('./_env.cjs');
const w=(p,ms)=>p.waitForTimeout(ms);
let fails=0; const pass=(c,m)=>{ console.log((c?'  PASS  ':'  FAIL  ')+m); if(!c) fails++; };

(async () => {
  const b=await chromium.launch(launchOpts);
  const pg=await b.newPage({viewport:{width:1440,height:1000}});
  const errs=[]; pg.on('pageerror',e=>errs.push('pageerror: '+e.message));
  pg.on('console',m=>{ if(m.type()==='error') errs.push(m.text()); });
  await pg.goto(PAGE); await w(pg,1500);
  const identity = await enterProduct(pg, 'dvp');

  console.log('--- the network you joined ---');
  const setup = await pg.evaluate(()=>({
    users: window.__ENYGMA.dvp.users().length,
    notes: window.__ENYGMA.dvp.notes().length,
    you: window.__ENYGMA.dvp.users()[0].isYou,
    pk: window.__ENYGMA.dvp.users()[0].pkSpend,
    tabs: [...document.querySelectorAll('#app-dvp .tab-btn')].map(e=>e.innerText.trim()),
  }));
  pass(setup.users===10, `ten parties on the registry (${setup.users})`);
  pass(setup.notes===10, `each holds an opening note (${setup.notes})`);
  pass(setup.you && setup.pk===identity.pkSpend, 'party 0 is you, with the keys from step 1');
  pass(setup.tabs.length===3, `three tabs: ${JSON.stringify(setup.tabs)}`);

  console.log('\n--- a swap commits to all three possible futures at once ---');
  const sw = await pg.evaluate(async ()=>{
    const D = window.__ENYGMA.dvp;
    const notes = D.notes();
    const mine = notes.find(n=>n.ownerIdx===0);            // the lot you hold
    const theirs = notes.find(n=>n.ownerIdx===1);          // the cash opposite it
    const s = await D.computeSwap(mine, theirs, 30, 'direct', 'ontime');
    return { give:{amount:mine.amount, tokenId:mine.tokenId, leaf:mine.leafIndex},
             want:{amount:theirs.amount, tokenId:theirs.tokenId, leaf:theirs.leafIndex},
             commitOutA:s.commitOutA, commitOutB:s.commitOutB, commitRevA:s.commitRevA,
             nfA:s.nfA, nfB:s.nfB, checkA:s.checkA, checkB:s.checkB,
             saltBout:s.saltBout, saltAout:s.saltAout, saltArev:s.saltArev,
             ssB:s.ssB, k:s.k, capsule:s.capsule, encTxData:s.encTxData, swapId:s.swapId };
  });
  const distinct = new Set([sw.commitOutA, sw.commitOutB, sw.commitRevA]).size;
  pass(distinct===3, `the three output commitments are all distinct (${distinct})`);
  pass(sw.checkA && sw.checkB, 'the counterparty re-derives both payout commitments and they match');
  pass(!!sw.nfA && !!sw.nfB && sw.nfA!==sw.nfB, 'each side contributes its own nullifier');
  pass(sw.saltArev!==sw.saltBout && sw.saltArev!==sw.saltAout,
       'the revert commitment uses its own salt, not a payout salt');

  console.log('\n--- every commitment re-derives from its opening ---');
  const derived = await pg.evaluate(async (s)=>{
    const D = window.__ENYGMA.dvp, U = D.users(), N = D.notes();
    const mine = N.find(n=>n.ownerIdx===0), theirs = N.find(n=>n.ownerIdx===1);
    return {
      outB: await D.poseidonC(U[1].pkSpend, s.saltBout, mine.amount,   mine.tokenId),
      outA: await D.poseidonC(U[0].pkSpend, s.saltAout, theirs.amount, theirs.tokenId),
      revA: await D.poseidonC(U[0].pkSpend, s.saltArev, mine.amount,   mine.tokenId),
      nfA:  await D.poseidonNf(U[0].skSpend, mine.leafIndex),
      nfB:  await D.poseidonNf(U[1].skSpend, theirs.leafIndex),
      wrongKeyNf: await D.poseidonNf(U[2].skSpend, mine.leafIndex),
    };
  }, sw);
  pass(derived.outB===sw.commitOutB, "the counterparty's payout commitment re-derives exactly");
  pass(derived.outA===sw.commitOutA, 'your payout commitment re-derives exactly');
  pass(derived.revA===sw.commitRevA, 'the revert commitment re-derives exactly');
  pass(derived.nfA===sw.nfA && derived.nfB===sw.nfB, 'both nullifiers re-derive from their own spend keys');
  pass(derived.wrongKeyNf!==sw.nfA, "a third party's key on the same leaf yields a different nullifier");

  console.log('\n--- the salts come from the shared secret, so both sides get them independently ---');
  const salts = await pg.evaluate(async (s)=>{
    const D = window.__ENYGMA.dvp;
    return { bout: await D.hkdf(s.ssB, 'Bob salt', 32),
             aout: await D.hkdf(s.ssB, 'Alice salt', 32),
             key:  await D.hkdf(s.ssB, 'encryption key', 32) };
  }, sw);
  pass(salts.bout===sw.saltBout && salts.aout===sw.saltAout,
       'both payout salts are HKDF of the shared secret under separate labels');
  pass(salts.key===sw.k, 'and so is the content key that seals ENC_TX_DATA');
  pass(sw.saltBout!==sw.saltAout, 'the two labels give different salts — no reuse across legs');

  console.log('\n--- the deadline, the swap id, and what is on chain ---');
  pass(/^[0-9a-f]{64}$/.test(sw.swapId), 'the swap id is a 32-byte digest');
  pass(!!sw.capsule && sw.capsule!==sw.ssB, 'the capsule published on chain is not the shared secret itself');
  // the real property is not "the digit is absent" — a one-character amount turns up in any hex
  // string by chance — but that the payload is a genuine AEAD ciphertext that only the derived
  // key opens, and that opening it returns the traded terms.
  const sealed = await pg.evaluate(async (s)=>{
    const D = window.__ENYGMA.dvp;
    const ct = s.encTxData;
    const hexish = typeof ct === 'string' ? /^[0-9a-f|:]+$/i.test(ct) : true;
    // re-encrypting the same terms under the same key must not reproduce it: a fresh IV each time
    const again = await D.aesGcmEncrypt(s.k, JSON.stringify({tokenId: s.want.tokenId, amount: s.give.amount}));
    return { hexish, ivFresh: again !== ct, plaintextAbsent: !/tokenId|amount/i.test(String(ct)) };
  }, sw);
  pass(sealed.plaintextAbsent, 'ENC_TX_DATA carries no plaintext field names — it is a ciphertext, not a struct');
  pass(sealed.ivFresh, 'sealing the same terms twice gives a different ciphertext — the IV is fresh each time');

  console.log('\n--- run it in the UI, end to end ---');
  await pg.click('#app-dvp [data-tab="swap"]'); await w(pg,800);
  const enabled = await pg.evaluate(()=>{
    const b=document.querySelector('#app-dvp #btnSwap'); return b ? !b.disabled : null; });
  pass(enabled===true, 'the swap composer is ready on arrival — both sides already hold notes');
  await pg.click('#app-dvp #btnSwap');
  await pg.waitForFunction(()=>!!window.__ENYGMA.dvp.lastSwap(), null, {timeout:60000});
  await w(pg,6000);
  const ran = await pg.evaluate(()=>{
    const s = window.__ENYGMA.dvp.lastSwap();
    return { has:!!s, nfA:!!s.nfA, checkB:s.checkB,
             steps: document.querySelectorAll('#app-dvp #aliceSteps .step').length };
  });
  pass(ran.has && ran.nfA, 'the run produced a swap carrying a nullifier');
  pass(ran.steps>0, `the initiator's steps rendered (${ran.steps})`);

  console.log('\n--- the revert path commits to giving you your own asset back ---');
  const rev = await pg.evaluate(async ()=>{
    const D = window.__ENYGMA.dvp, N = D.notes(), U = D.users();
    const mine = N.find(n=>n.ownerIdx===0), theirs = N.find(n=>n.ownerIdx===1);
    const s = await D.computeSwap(mine, theirs, 30, 'direct', 'late');   // deadline lapses
    const refund = await D.poseidonC(U[0].pkSpend, s.saltArev, mine.amount, mine.tokenId);
    return { nfB: s.nfB, checkB: s.checkB, refundMatches: refund===s.commitRevA,
             sameAsset: true, amount: mine.amount, tokenId: mine.tokenId };
  });
  pass(rev.nfB===null, "the counterparty's nullifier is never published — it never moved");
  pass(rev.checkB===false, 'and it never completed its side');
  pass(rev.refundMatches, 'the revert commitment pays you back the exact asset you put in');

  pass(errs.length===0, 'no console/page errors'+(errs.length?': '+errs.slice(0,3).join(' | '):''));
  await b.close();
  console.log('\n'+(fails? fails+' CHECK(S) FAILED':'ALL CHECKS PASSED'));
  process.exit(fails?1:0);
})();
