// @vitest-environment jsdom
/**
 * The diagnostics button.
 *
 * The one behaviour worth pinning here is the failure: the server refuses the
 * bundle outright when no login password is set, and that refusal is the
 * message the user needs ("set a login password before downloading the
 * diagnostics bundle"). Swallowing it, or replacing it with a generic
 * "download failed", would leave someone staring at a button that does nothing
 * and never says why.
 */
import { render, screen, cleanup, waitFor, fireEvent } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const downloadDiagnostics = vi.fn();
const pushed: { message: string; severity?: string }[] = [];

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  downloadDiagnostics: (...a: unknown[]) => downloadDiagnostics(...a),
}));

vi.mock("../../lib/toast", () => ({
  useToast: () => ({
    push: (message: string, severity?: string) => pushed.push({ message, severity }),
    quiet: false,
    setQuiet: () => {},
  }),
}));

import { SettingsPortabilityCard } from "./SettingsPortabilityCard";
import { en } from "../../lib/i18n";

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as unknown as Parameters<
  typeof SettingsPortabilityCard
>[0]["t"];

function renderCard() {
  return render(
    <SettingsPortabilityCard t={t} applyImport={async () => ({ ok: true })} />
  );
}

afterEach(() => {
  cleanup();
  downloadDiagnostics.mockReset();
  pushed.length = 0;
});

describe("the diagnostics button", () => {
  it("downloads the bundle when pressed", async () => {
    downloadDiagnostics.mockResolvedValue(null);

    renderCard();
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["diagnostics.button"], "i") }));

    await waitFor(() => expect(downloadDiagnostics).toHaveBeenCalledTimes(1));
    expect(pushed).toHaveLength(0);
  });

  it("shows the server's refusal instead of failing silently", async () => {
    downloadDiagnostics.mockResolvedValue(
      "set a login password before downloading the diagnostics bundle"
    );

    renderCard();
    fireEvent.click(screen.getByRole("button", { name: new RegExp(en["diagnostics.button"], "i") }));

    await waitFor(() => expect(pushed).toHaveLength(1));
    expect(pushed[0].message).toContain("login password");
    expect(pushed[0].severity).toBe("fail");
  });
});
