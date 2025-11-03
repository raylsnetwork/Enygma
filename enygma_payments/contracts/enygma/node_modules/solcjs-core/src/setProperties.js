module.exports = setProperties;

function setProperties(oldSolc, newSolc) {
  Object.keys(oldSolc).forEach(key => {
    if (typeof oldSolc[key] == 'function') return;

    Object.defineProperty(newSolc, key, {
      get() {
        var currentValue = oldSolc[key];
        console.error(`compiler.${key} === `, currentValue);
        return currentValue;
      },
      set(newValue) {
        console.error(`compiler.${key} = `, newValue);
        return oldSolc[key] = newValue;
      },
      enumerable: true,
      configurable: true
    });
  });
}