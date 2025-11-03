require('./utils/mock')();

const chai = require('chai');
chai.should();

const v = require('../src/index');

describe('version', () => {

  it('versions', async() => {
    let select = await v.versions();
    const { releases, nightly, all } = select;
    // console.log(releases[0]);
    releases.should.be.a('Array');
    nightly.should.be.a('Array');
    all.should.be.a('Array');
    releases[0].should.be.a('string');
    releases[0].indexOf('stable').should.be.above(-1);
  });

  it('versions passed list.json', async () => {
    const list = JSON.stringify(require('./utils/list.json'));
    let select = await v.versions(list);
    const { releases, nightly, all } = select;
    // console.log(releases[0]);
    releases.should.be.a('Array');
    nightly.should.be.a('Array');
    all.should.be.a('Array');
    releases[0].should.be.a('string');
    releases[0].should.be.equal('v0.5.2-stable-2018.12.19');
  });

  it('versionsSkipVersion5', async () => {
    let select = await v.versionsSkipVersion5();
    const { releases, nightly, all } = select;
    // console.log(releases[0]);
    releases.should.be.a('Array');
    nightly.should.be.a('Array');
    all.should.be.a('Array');
    releases[0].should.be.a('string');
    releases[0].should.be.equal('v0.4.25-stable-2018.09.13');
  });

});