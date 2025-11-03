require('./utils/mock')()
const promiseAjax = require('../src/promiseAjax')

const chai = require('chai')
chai.should()

function checkCompilersourceContent(compilersource) {
  compilersource.should.be.a('string')
  compilersource.substring(0, 11).should.be.equal('var Module;')
}

function getOpts(caching) {
  const compilerURL =
    'https://solc-bin.ethereum.org/bin/soljson-v0.4.25+commit.59dbf8f1.js'
  const opts = { url: compilerURL, caching }
  return opts
}

describe.only('promiseAjax', () => {
  it('cache 1', async () => {
    const opts = getOpts(true)
    let compilersource = await promiseAjax(opts)
    checkCompilersourceContent(compilersource)
  })

  it('cache 2', async () => {
    const opts = getOpts(true)
    let compilersource = await promiseAjax(opts)
    checkCompilersourceContent(compilersource)
  })

  it('no cache', async () => {
    const opts = getOpts(false)
    let compilersource = await promiseAjax(opts)
    checkCompilersourceContent(compilersource)
  })

  it('parse error', async () => {
    const compilerURL = 'https://xxx.com/a.json'
    const opts = {
      url: compilerURL,
      caching: false,
      transform: function(data) {
        try {
          JSON.parse(data)
        } catch (error) {
          throw error
        }
      },
    }
    try {
      await promiseAjax(opts)
    } catch (error) {
      error.name.should.be.eq('SyntaxError')
    }
  })

  it('fetch error', async () => {
    const compilerURL = 'https://ereree.com/a.json'
    try {
      await promiseAjax(compilerURL)
    } catch (error) {
      error.name.should.be.eq('FetchError')
    }
  })

  it('waiting', async () => {
    const opts = getOpts(false)
    let compilersources = await Promise.all([
      promiseAjax(opts),
      promiseAjax(opts),
      promiseAjax(opts),
    ])
    compilersources.should.be.a('array')
    checkCompilersourceContent(compilersources[0])
    checkCompilersourceContent(compilersources[1])
    checkCompilersourceContent(compilersources[2])
  })
})
