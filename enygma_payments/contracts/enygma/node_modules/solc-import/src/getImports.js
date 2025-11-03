module.exports = getImports;

function getImports(source) {
  let matches = [];
  let ir = /^(.*import){1}(.+){0,1}\s['"](.+)['"];/gm;
  let match = null;
  while ((match = ir.exec(source))) {
    matches.push(match[3]);
  }
  return matches;
}