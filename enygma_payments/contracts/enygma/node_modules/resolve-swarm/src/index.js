module.exports = {
  type: 'swarm',
  parser: require('./parser'),
  resolver: require('./resolver'),
  match: /^(bzz[ri]?:\/\/?(.*))$/
};