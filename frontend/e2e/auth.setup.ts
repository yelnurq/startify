import { test as setup } from '@playwright/test';

import { signIn, storageState, USERS, type SeededUser } from './support/accounts';

for (const user of Object.keys(USERS) as SeededUser[]) {
  setup(`sign in as ${user}`, async ({ page }) => {
    await signIn(page, USERS[user]);
    await page.context().storageState({ path: storageState(user) });
  });
}
