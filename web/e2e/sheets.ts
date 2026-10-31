// A window on a phone is a sheet: it is parked below the screen for a frame,
// then slides up. A tap aimed at it before it is up can land on the scrim
// around it, which closes the sheet, so a test lets it come to rest first.
import { expect, type Locator } from "@playwright/test";

/** sheetUp waits until the whole sheet stands on screen, which it only does
 *  once its slide has ended. */
export async function sheetUp(sheet: Locator): Promise<void> {
  await expect(sheet).toBeInViewport({ ratio: 1 });
}
