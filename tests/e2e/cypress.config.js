/* eslint-disable no-unused-vars */
const { defineConfig } = require('cypress');

module.exports = defineConfig({
  e2e: {
    experimentalWebKitSupport: true,
    setupNodeEvents (on, config) {
      // implement node event listeners here
    },
  },
});
