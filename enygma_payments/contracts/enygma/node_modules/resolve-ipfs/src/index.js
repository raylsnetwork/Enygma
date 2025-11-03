module.exports = {
  type: 'ipfs',
  parser: require('./parser'),
  resolver: require('./resolver'),
  match: /^(ipfs:\/\/?.+)/
};