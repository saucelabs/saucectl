import { Selector } from 'testcafe';

// DEMO ONLY: the test name carries a unique per-run id so each `saucectl run`
// produces NEW test FQNs → NEW TCM cases. Real tests have STABLE names, so
// re-runs of them add a new *result* to the same case (history), not a new case.
// This dynamic name is purely to demo case-creation on every run.
const RUN_ID = new Date().toISOString().replace(/[:.]/g, '-');

fixture `TCM Demo`
  .page `https://www.saucedemo.com/`;

test(`TCM demo - login form present [${RUN_ID}]`, async (t) => {
  await t.expect(Selector('#user-name').visible).ok();
});

test(`TCM demo - password field present [${RUN_ID}]`, async (t) => {
  await t.expect(Selector('#password').visible).ok();
});
