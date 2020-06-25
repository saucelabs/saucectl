function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

describe('Herokuapp Login Page', () => {
  describe('Login page is constructed correctly', () => {
    test('Page is available', async () => {
      await page.goto('https://the-internet.herokuapp.com/login');
      expect(await page.url()).toContain('login');
      await sleep(2000)
    });

    test('Username is available', async () => {
      const usernameElement = await page.$('input#uername');
      expect(usernameElement).not.toBeNull();
      await sleep(2000)
    });

    test('Password is available', async () => {
      const passwordElement = await page.$('input#password');
      expect(passwordElement).not.toBeNull();
      await sleep(2000)
    });

    test('Login button is available', async () => {
      const loginButtonElement = await page.$('button[type="submit"]');
      expect(loginButtonElement).not.toBeNull();
      await sleep(2000)
    });
  });


  describe.skip('Login scenarios', () => {
    test('Bad credentials fail', async () => {
      await page.goto('https://the-internet.herokuapp.com/login');
      await page.type('input#username', 'junk');
      await page.type('input#password', 'junk');
      await page.click('button[type="submit"]');
      const divAlert = await page.$('div#flash');
      const alertText = await page.evaluate(divAlert => divAlert.textContent, divAlert);
      expect(alertText).toContain("Your username is invalid!");
      await sleep(2000)
    });

    test('Good credentials pass', async () => {
      await page.goto('https://the-internet.herokuapp.com/login');
      await page.type('input#username', 'tomsmith');
      await page.type('input#password', 'SuperSecretPassword!');
      await page.click('button[type="submit"]');
      const divAlert = await page.$('div#flash');
      expect(await page.url()).toContain('secure');
      expect(divAlert).not.toBeNull();
      await sleep(2000)
    });
  });

  describe.skip('Logout scenario', () => {
    test('Can logout successfully', async () => {
      await page.goto('https://the-internet.herokuapp.com/login');
      await page.type('input#username', 'tomsmith');
      await page.type('input#password', 'SuperSecretPassword!');
      await page.click('button[type="submit"]');
      await page.click('a[href="/logout"]');
      const divAlert = await page.$('div#flash');
      const alertText = await page.evaluate(divAlert => divAlert.textContent, divAlert);
      expect(await page.url()).toContain('login');
      expect(alertText).toContain("You logged out of the secure area!");
      await sleep(2000)
    });
  });
});
