module.exports = groupByVersion;

function removeAllZeroPointFiveVersion(select) {
  select.nightly = select.nightly.filter(x => !~x.indexOf('v0.5.'));
  select.all = select.all.filter(x => !~x.indexOf('v0.5.'));
  select.releases = select.releases.filter(x => !~x.indexOf('v0.5.'));
}

function groupByVersion(data, skip5 = true) {
  const { releases, nightly, all } = data;
  let select = {};
  select.nightly = Object.keys(nightly).reverse();
  select.all = Object.keys(all).reverse();
  select.releases = Object.keys(releases).reverse();
  if (skip5) removeAllZeroPointFiveVersion(select);
  return select;
}