module.exports = getStandardError;

function getStandardError(errors) {
  let result = [];
  for (let error of errors) {
    result.push({
      component: error.component,
      formattedMessage: error.formattedMessage,
      message: error.message,
      type: error.type
    });
  }
  return result;
}