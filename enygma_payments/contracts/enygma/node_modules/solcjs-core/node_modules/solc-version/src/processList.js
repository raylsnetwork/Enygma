module.exports = processList;

function processList(json) {
  const data = JSON.parse(json);
  const lists = Object.values(data.builds).reduce(({ agg, d }, x, i, arr) => {
    const { path, prerelease, version } = x;
    if (prerelease) {
      d = prerelease.split('nightly.')[1];
      var [year0, month0, day0] = d.split('.').map(Number);
      if ((month0 + '').length < 2) month0 = '0' + month0;
      if ((day0 + '').length < 2) day0 = '0' + day0;
      d = [year0, month0, day0].join('.');
      const entry = [`v${version}-nightly-${d}`, path];
      agg.nightly.push(entry);
      agg.all.push(entry);
    } else {
      for (var j = i + 1, ahead; j < arr.length && !(ahead = arr[j].prerelease); j++) { }
      if (ahead) ahead = ahead.split('nightly.')[1];
      else ahead = d;
      if (!d) d = ahead;
      if (ahead !== d) {
        var [year1, month1, day1] = d.split('.').map(Number);
        var [year2, month2, day2] = ahead.split('.').map(Number);
        var d1 = new Date(year1, month1 - 1, day1);
        var d2 = new Date(year2, month2 - 1, day2);
        var diffDays = parseInt((d2 - d1) / (1000 * 60 * 60 * 24));
        var d3 = new Date(d1);
        d3.setDate(d3.getDate() + diffDays / 2);
        var month = d3.getUTCMonth() + 1;
        var day = d3.getDate();
        var year = d3.getUTCFullYear();
        var current = [year, month, day].join('.');
      } else {
        var current = ahead;
      }
      var [year0, month0, day0] = current.split('.').map(Number);
      if ((month0 + '').length < 2) month0 = '0' + month0;
      if ((day0 + '').length < 2) day0 = '0' + day0;
      current = [year0, month0, day0].join('.');
      const entry = [`v${version}-stable-${current}`, path];
      agg.releases.push(entry);
      agg.all.push(entry);
    }
    return { agg, d };
  }, { agg: { releases: [], nightly: [], all: [] }, d: null }).agg;
  const { releases, nightly, all } = lists;
  lists.releases = releases.reduce((o, x) => ((o[x[0]] = x[1]), o), {});
  lists.nightly = nightly.reduce((o, x) => ((o[x[0]] = x[1]), o), {});
  lists.all = all.reduce((o, x) => ((o[x[0]] = x[1]), o), {});
  return lists;
}