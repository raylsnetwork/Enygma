require('./utils/mock')();

const chai = require('chai');
chai.should();

const v = require('../src/index');

describe('version2url', () => {

  it('0.4.25', async () => {
    let version = 'v0.4.25-stable-2018.09.13';
    let url = await v.version2url(version);
    url.should.be.a('string');
    url.should.match(/solc-bin.ethereum.org/);
  });

  it('0.4.25 passed list', async () => {
    let version = 'v0.4.25-stable-2018.09.13';
    const list = JSON.stringify(require('./utils/list.json'));
    let url = await v.version2url(version, list);
    url.should.be.a('string');
    url.should.match(/solc-bin.ethereum.org/);
  });

  it('nightly', async () => {
    let version = 'nightly';
    let url = await v.version2url(version);
    url.should.be.a('string');
    url.should.match(/solc-bin.ethereum.org/);
  });

  it('latest', async () => {
    let version = 'latest';
    let url = await v.version2url(version);
    // console.log(url);
    
    url.should.be.a('string');
    url.should.match(/solc-bin.ethereum.org/);
  });

});