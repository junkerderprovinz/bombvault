// @vitest-environment jsdom
// The mark beside a container that was recreated since its last backup. The
// server words each change from the restore's side; the notice speaks from now.
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import { ContainerChangeNotice } from "./ContainerChangeNotice";
import { countText, en } from "../lib/i18n";
import type { ContainerChange } from "../lib/api";

const t = ((key: string, n?: number) =>
  countText((en as Record<string, string>)[key] ?? key, "en", n)) as unknown as Parameters<
  typeof ContainerChangeNotice
>[0]["t"];

afterEach(cleanup);

function tip(): string {
  return screen.getByLabelText(/recreated with other settings/).getAttribute("aria-label") ?? "";
}

describe("ContainerChangeNotice", () => {
  it("shows nothing for a container that did not change", () => {
    const { container } = render(<ContainerChangeNotice changes={[]} t={t} />);
    expect(container.textContent).toBe("");
  });

  it("names what is new and what is gone from the point of view of now", () => {
    const changes: ContainerChange[] = [
      { field: "env", name: "APP_EXTRA", change: "removed" },
      { field: "port", change: "added", backup: "8080 -> 80/tcp" },
      { field: "env", name: "APP_SECRET", change: "changed" },
    ];
    render(<ContainerChangeNotice changes={changes} t={t} />);
    expect(screen.getByText("Changed since backup")).toBeTruthy();
    expect(tip()).toContain("Variable APP_EXTRA is new");
    expect(tip()).toContain("Port 8080 -> 80/tcp is gone");
    expect(tip()).toContain("Variable APP_SECRET has a new value");
  });

  it("tells a new tag from the same tag pulled again", () => {
    render(
      <ContainerChangeNotice
        changes={[
          { field: "image", change: "updated", now: "alpine:latest" },
          { field: "image", change: "changed", backup: "alpine:3.21", now: "alpine:3.22" },
        ]}
        t={t}
      />
    );
    expect(tip()).toContain("Image alpine:latest was pulled again");
    expect(tip()).toContain("Image is now alpine:3.22, was alpine:3.21");
  });

  it("counts what does not fit", () => {
    const changes: ContainerChange[] = Array.from({ length: 9 }, (_, i) => ({
      field: "env" as const,
      name: `V${i}`,
      change: "removed" as const,
    }));
    render(<ContainerChangeNotice changes={changes} t={t} />);
    expect(tip()).toContain("Variable V5 is new; 3 more.");
    expect(tip()).not.toContain("V6");
  });
});
