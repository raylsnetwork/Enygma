module.exports = {
  type: 'http',
  parser: require('./parser'),
  resolver: require('./resolver'),
  match: /^(http|https?:\/\/?(.*))$/
};

// const match = /^(http?:\/\/?(.*))$/;