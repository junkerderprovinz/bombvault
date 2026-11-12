// Two bars share the page: BombVault's own under a card and GlimStone's, as
// its status window draws one for a member's running work. Each fill takes
// the width its value gives it.
import { expect, test } from "@playwright/test";

test("both progress bars fill as far as their value", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1280", "one desktop run is enough");
  await page.goto("/dashboard");
  await expect(page.locator("#bv-main h1").first()).toBeVisible();
  const widths = await page.evaluate(() => {
    const host = document.createElement("div");
    host.style.width = "400px";
    host.innerHTML =
      '<ul class="glim-status-list"><li class="glim-status-work"><span>plex</span>' +
      '<span data-glim="ProgressBar" role="progressbar" class="glim-meter glim-progress"><span class="glim-progress-fill" style="width: 40%"></span></span></li></ul>' +
      '<div class="glim-meter relative h-1 w-full overflow-hidden rounded-pill"><div class="glim-progress-fill h-full" style="width: 40%"></div></div>';
    document.body.append(host);
    const measure = (track: Element) => ({
      track: track.getBoundingClientRect().width,
      fill: track.querySelector(".glim-progress-fill")!.getBoundingClientRect().width,
      height: track.querySelector(".glim-progress-fill")!.getBoundingClientRect().height,
    });
    const [glim, own] = [...host.querySelectorAll(".glim-meter")].map(measure);
    host.remove();
    return { glim, own };
  });
  for (const bar of [widths.glim, widths.own]) {
    expect(bar.track).toBeGreaterThan(0);
    expect(bar.fill).toBeCloseTo(bar.track * 0.4, 0);
    expect(bar.height).toBeGreaterThan(0);
  }
});
