module.exports = {
  type: 'github',
  parser: require('./parser'),
  resolver: require('./resolver'),
  match: /^(https?:\/\/)?(www.)?github.com\/([^/]*\/[^/]*)\/(.*)/
};