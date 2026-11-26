// @vitest-environment jsdom
// The pictures of the restore targets. On a picked tile the ground is the
// accent, so a word on a fresh box takes the picture's ink there, the colour
// that reads on the accent in every theme and hue.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { I18nProvider } from "../../lib/i18n";
import type { RestoreWhere } from "../../lib/restorePlan";
import { TargetScene } from "./TargetScene";

const CSS = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "../../index.css"), "utf8");

afterEach(cleanup);

function scene(where: RestoreWhere) {
  return render(
    <I18nProvider>
      <span className="glim-active">
        <TargetScene where={where} playing />
      </span>
    </I18nProvider>,
  ).container;
}

describe("the target pictures on a picked tile", () => {
  it.each<RestoreWhere>(["replace", "beside", "newDataset"])("write the word on a fresh box in the picture's ink for %s", (where) => {
    const boxes = [...scene(where).querySelectorAll(".glim-target-box-fresh")];
    expect(boxes.length).toBeGreaterThan(0);
    for (const box of boxes) expect(box.nextElementSibling?.classList.contains("glim-target-word-fresh")).toBe(true);
    expect(CSS).toMatch(/^\.glim-active \.glim-pic \.glim-target-word-fresh \{ fill: var\(--pic\); \}$/m);
  });
});
