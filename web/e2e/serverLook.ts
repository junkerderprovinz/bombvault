// Every worker shares one harness server, so a look one page stores there is
// the look of every page that boots after it.
import type { Page } from "@playwright/test";

/**
 * blockServerLook keeps the page from reading or storing the server's look.
 * It sits on the context because the afterEach unrouteAll clears the page's
 * routes while the page can still be booting: a test that ends on a Back to a
 * fresh document lets that boot find the empty server and store the test's
 * look there.
 */
export async function blockServerLook(page: Page): Promise<void> {
  // A boot cut short by the closing context has already lost its request.
  await page.context().route("**/api/display-prefs*", (route) => route.abort().catch(() => {}));
}
