module.exports = pretest;

async function pretest(compile) {
  try {
    let content = `
    contract NewContract {
      function f() public {}
    }`;
    await compile(content);
  } catch (error) {
    console.error('pretest failed');
    throw error;
  }
}