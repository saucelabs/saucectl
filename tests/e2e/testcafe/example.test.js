import { Selector } from 'testcafe';

fixture `Getting Started Sauce demo`
  .page `https://www.saucedemo.com/`;


const Users = {
  password: 'secret_sauce',
  standard: 'standard_user',
  locked: 'locked_out_user'
};

class Login {
  constructor () {
    this.usernameEl = Selector('#user-name');
    this.passwordEl = Selector('#password');
  }
}

const login = new Login();

test('SwagLabs username not set', async function (t) {
  await t
    .click('.btn_action')
    // Use the assertion to check if the actual header text is equal to the expected one
    .expect(Selector('h3, [data-test=error]').innerText).contains('Username is required')
    .expect(Selector('.error-button').visible).eql(true);
});
