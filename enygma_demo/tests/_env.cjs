/* Shared test environment.
 *
 * Every suite drives the demo page in headless Chromium through Playwright. The three things that
 * differ between machines — where Playwright lives, which Chromium binary it should launch, and
 * which page is under test — are resolved here so the suites themselves stay portable.
 *
 *   PLAYWRIGHT_PATH   override the module path (default: the local "playwright" dependency)
 *   CHROMIUM_PATH     launch a specific Chromium binary instead of Playwright's own
 *   DEMO_PAGE         point the suites at a different HTML file (default: ../index.html)
 */
const path = require("path");

function loadPlaywright(){
  const candidates = [
    process.env.PLAYWRIGHT_PATH,
    "playwright",
    "/opt/node22/lib/node_modules/playwright",   // the container image used to build this demo
  ].filter(Boolean);
  const tried = [];
  for(const c of candidates){
    try { return require(c); } catch(e){ tried.push(`${c} (${e.code || e.message})`); }
  }
  throw new Error(
    "Playwright not found. Run `npm install` in enygma_demo/, or set PLAYWRIGHT_PATH.\nTried:\n  " +
    tried.join("\n  "));
}

const { chromium } = loadPlaywright();

// launch options: honour CHROMIUM_PATH, otherwise let Playwright pick its bundled browser
const launchOpts = process.env.CHROMIUM_PATH ? { executablePath: process.env.CHROMIUM_PATH } : {};

// the page under test, resolved from this file so suites run from any working directory
const PAGE = process.env.DEMO_PAGE
  ? (/^\w+:/.test(process.env.DEMO_PAGE) ? process.env.DEMO_PAGE
                                         : "file://" + path.resolve(process.env.DEMO_PAGE))
  : "file://" + path.join(__dirname, "..", "index.html");

// screenshots written by the suites land here, outside version control
const SHOTS = path.join(__dirname, "screenshots");
try { require("fs").mkdirSync(SHOTS, { recursive: true }); } catch(e){}
const shot = (name) => path.join(SHOTS, name);

/* The page opens on the suite shell, not on a product: step 1 generates one identity, step 2
 * chooses which of the four networks to join. Every suite that exercises a product has to walk
 * that path first, so it lives here rather than being repeated eleven times.
 *
 * A network can be arrived at two ways, and both are real user paths:
 *
 *   mode: "join"       (default) — the network already exists; you publish the keys you hold and
 *                       are on its registry in about two seconds. This is what a returning visitor
 *                       sees, and it is what a suite wanting a populated registry should use.
 *   mode: "formation"  — the registry starts empty and fills in front of you, ten members, yours
 *                       first. Slower (~20s) and covered explicitly by keys-test.cjs.
 *   mode: "empty"      — stop at the empty registry and do nothing.
 *
 * Returns the generated identity, so a suite can assert that the keys it saw in step 1 are the
 * ones the network actually registered.
 */
const PRODUCT_ALIAS = { institutional: "payment", payments: "payment", auction: "auctions" };

// how each product starts its formation run
const FORM_BUTTON = {
  payment:  "#onboardBtn",
  retail:   "#btnRegisterTen",
  dvp:      "#btnRegisterTen",
  auctions: "#btnRegisterTen",
};
const REGISTRY_SIZE = 10;

const partyCount = (name) =>
  name === "payment" ? window.__T.state.insts.length : window.__ENYGMA[name].users().length;

async function enterProduct(pg, product, opts){
  const name = PRODUCT_ALIAS[product] || product;
  const settle = (opts && opts.settle != null) ? opts.settle : 2500;
  // `register: false` is the old spelling of `mode: "empty"`, kept so older suites still read
  const mode = (opts && opts.mode) || (opts && opts.register === false ? "empty" : "join");

  await pg.click("#btnGenKeys");
  await pg.waitForFunction(() => !!(window.__ENYGMA && window.__ENYGMA.identity), null, { timeout: 20000 });
  const identity = await pg.evaluate(() => window.__ENYGMA.identity);

  // take the join path unless this suite specifically wants to watch a network form
  if(mode === "join") await pg.evaluate(() => window.__ENYGMA.skipFormation());

  await pg.click("#btnKeysNext");
  await pg.click(`.pick[data-product="${name}"]`);
  await pg.waitForFunction(
    (n) => window.__ENYGMA.product === n && window.__ENYGMA.booted().indexOf(n) >= 0,
    name, { timeout: 20000 });

  if(mode === "formation"){
    await pg.click(`#app-${name} ${FORM_BUTTON[name]}`);
  }
  if(mode !== "empty"){
    // joining settles in ~2s; a formation paces itself at ~1.2s a member plus channels and balances
    await pg.waitForFunction(
      (n) => (n === "payment" ? window.__T.state.insts.length
                              : window.__ENYGMA[n].users().length) >= 10,
      name, { timeout: 120000 });
    await pg.waitForTimeout(mode === "formation" ? 3000 : 1500);
  }

  await pg.waitForTimeout(settle);
  return identity;
}

module.exports = { chromium, launchOpts, PAGE, shot, enterProduct, REGISTRY_SIZE, partyCount };
