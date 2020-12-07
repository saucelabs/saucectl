/// <reference types="cypress" />
// ***********************************************************
// This example plugins/index.js can be used to load plugins
//
// You can change the location of this file or turn off loading
// the plugins file with the 'pluginsFile' configuration option.
//
// You can read more here:
// https://on.cypress.io/plugins-guide
// ***********************************************************

// This function is called when a project is opened or re-opened (e.g. due to
// the project's config changing)

/**
 * @type {Cypress.PluginConfig}
 */
module.exports = (on, config) => {
  // `on` is used to hook into various events Cypress emits
  // `config` is the resolved Cypress config
  //   on('before:browser:launch', (browser = {}, launchOptions) => {
  //       // `args` is an array of all the arguments that will
  //       // be passed to browsers when it launches
  //       console.log(launchOptions.args) // print all current args
  //       if (browser.name === 'chrome' && browser.isHeadless) {
  //           launchOptions.args.push('--window-size=1280,720')
  //           launchOptions.args.push('--force-device-scale-factor=1')
  //           return launchOptions
  //       }
  //
  //       if (browser.name === 'firefox' && browser.isHeadless) {
  //           // menubars take up height on the screen
  //           // so fullPage screenshot size is 1400x1126
  //           launchOptions.args.push('--width=1280')
  //           launchOptions.args.push('--height=720')
  //           return launchOptions
  //       }
  //   })
}
