/* Shared test environment.
 *
 * Every suite drives the demo page in headless Chromium through Playwright. The two things that
 * differ between machines — where Playwright lives and which Chromium binary it should launch —
 * are resolved here so the suites themselves stay portable.
 *
 *   PLAYWRIGHT_PATH   override the module path (default: the local "playwright" dependency)
 *   CHROMIUM_PATH     launch a specific Chromium binary instead of Playwright's own
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
const PAGE = "file://" + path.join(__dirname, "..", "index.html");

// screenshots written by the suites land here, outside version control
const SHOTS = path.join(__dirname, "screenshots");
try { require("fs").mkdirSync(SHOTS, { recursive: true }); } catch(e){}
const shot = (name) => path.join(SHOTS, name);

module.exports = { chromium, launchOpts, PAGE, shot };
