// Every tab names itself for assistive tech through the shared PageTitle
// component - an sr-only h1 - rather than a visible heading of its own, and
// none of them carries a subtitle paragraph any more. Each page renders
// correctly on its own, so neither the type checker nor a render test would
// notice one quietly bringing its old heading back. A source scan does.
import { readdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const HERE = dirname(fileURLToPath(import.meta.url));
const PAGES = join(HERE, "..", "pages");
const COMPONENTS = join(HERE, "..", "components");

// Glyphs.tsx is a developer sheet reachable only by typing its route, and
// Login.tsx and OAuthConsent.tsx are standalone cards with no tab.
const NOT_A_TAB = new Set(["Glyphs.tsx", "Login.tsx", "OAuthConsent.tsx"]);

function pageFiles(): string[] {
  return readdirSync(PAGES).filter((f) => /^[A-Z].*\.tsx$/.test(f) && !/\.test\.tsx$/.test(f) && !NOT_A_TAB.has(f));
}

describe("PageTitle", () => {
  it("keeps the h1 in the DOM for screen readers, sr-only", () => {
    const src = readFileSync(join(COMPONENTS, "PageTitle.tsx"), "utf8");
    expect(src).toMatch(/<h1[^>]*\bsr-only\b/);
  });
});

describe("every tab's heading", () => {
  it("finds the pages at all (so an empty scan cannot pass)", () => {
    expect(pageFiles().length).toBeGreaterThan(6);
  });

  it.each(pageFiles())("%s names itself through PageTitle and shows no heading or subtitle", (file) => {
    const src = readFileSync(join(PAGES, file), "utf8");

    expect(src, `${file} does not import PageTitle`).toMatch(/import \{ PageTitle \} from "\.\.\/components\/PageTitle"/);
    expect(src, `${file} never renders <PageTitle>`).toMatch(/<PageTitle>/);

    expect(
      src,
      `${file} has a raw <h1>; every tab's title goes through PageTitle instead of repeating the markup`,
    ).not.toMatch(/<h1[ >]/);

    // The removed subtitle paragraph, under whatever key it read: a page's
    // own title/subtitle/intro key called directly inside a <p>.
    expect(
      src,
      `${file} still renders a page subtitle; the decision was to remove it, not just hide it`,
    ).not.toMatch(/<p[^>]*>\s*\{t(?:Ltr\(t,)?\("[a-zA-Z]+\.(subtitle|pageSubtitle|intro)"\)\)?\}/);
  });
});
