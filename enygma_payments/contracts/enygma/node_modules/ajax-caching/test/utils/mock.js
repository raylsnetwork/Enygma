module.exports = mock;

function mock() {
  if (!global.window) {
    const mocks = require('mock-browser').mocks;
    let MockBrowser = mocks.MockBrowser;
    global.window = MockBrowser.createWindow();
    window.indexedDB = require('fake-indexeddb');
  }
  const fetch = require('cross-fetch');
  global.fetch = fetch;
}