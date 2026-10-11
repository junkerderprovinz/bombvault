// @vitest-environment jsdom
// The check card is a row per section with the overview's four dots and a
// menu of what can be run by hand, offering only what the section has.
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { DomainStatus } from "../../lib/api";
import { domain, item } from "../overview/overview.testsupport";

const push = vi.fn();
const checkDomain = vi.fn();
const runDrill = vi.fn();
let domains: DomainStatus[] = [];

vi.mock("../../lib/api", () => ({
  checkDomain: (...a: unknown[]) => checkDomain(...a),
  unlockDomain: vi.fn().mockResolvedValue({ ok: true }),
  pruneDomain: vi.fn().mockResolvedValue({ ok: true }),
  runDrill: (...a: unknown[]) => runDrill(...a),
  tamperTest: vi.fn().mockResolvedValue({ ok: true, testable: true, protected: true }),
  getStatus: () => Promise.resolve({ ok: true, domains }),
  listItems: () => Promise.resolve({ ok: true, items: [item()] }),
}));

vi.mock("../../lib/toast", () => ({
  useToast: () => ({ push, quiet: false, setQuiet: () => {} }),
}));

import { IntegrityCard } from "./IntegrityCard";
import { en } from "../../lib/i18n";

// The card follows live progress, and jsdom has no EventSource.
vi.stubGlobal(
  "EventSource",
  class {
    onmessage = null;
    close() {}
  }
);

const t = ((key: string) => (en as Record<string, string>)[key] ?? key) as unknown as Parameters<typeof IntegrityCard>[0]["t"];

async function renderCard() {
  await act(async () => {
    render(<IntegrityCard t={t} />);
  });
}

function row(domainName: string): HTMLElement {
  return document.querySelector<HTMLElement>(`[data-domain="${domainName}"]`)!;
}

async function openMenu(domainName: string): Promise<HTMLElement> {
  await act(async () => {
    fireEvent.click(within(row(domainName)).getByRole("button", { name: en["nav.more"] }));
  });
  return screen.getByRole("menu");
}

beforeEach(() => {
  domains = [
    domain({ domain: "containers" }),
    domain({ domain: "vms", offsiteConfigured: false, offsiteImmutable: false }),
    domain({ domain: "config" }),
  ];
  checkDomain.mockResolvedValue({ ok: true });
  runDrill.mockResolvedValue({ ok: true, drill: { ok: true } });
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

it("is named after checking the backups", async () => {
  await renderCard();
  expect(screen.getByRole("heading", { name: new RegExp(`^${en["integrity.checkTitle"]}`) })).toBeTruthy();
});

it("lists every switched-on section but the self-backup, each with the four checks", async () => {
  await renderCard();
  expect(row("containers")).toBeTruthy();
  expect(row("vms")).toBeTruthy();
  expect(row("config")).toBeNull();
  const dots = row("containers").querySelectorAll("[data-check]");
  expect([...dots].map((d) => d.getAttribute("data-check"))).toEqual(["local", "offsite", "lock", "drill"]);
  expect(dots[0].getAttribute("aria-label")).toBe(en["overview.check.localCurrent"]);
  expect(dots[0].textContent).toBe(en["settings.pathMode.local"]);
});

it("offers the off-site checks only where there is an off-site copy", async () => {
  await renderCard();
  const full = within(await openMenu("containers"))
    .getAllByRole("menuitem")
    .map((m) => m.textContent);
  expect(full).toEqual([
    en["integrity.checkReadable"],
    en["integrity.checkOffsite"],
    en["verify.trialRestore"],
    en["verify.tamper"],
    en["integrity.unlock"],
    en["integrity.prune"],
  ]);
  fireEvent.keyDown(document.activeElement ?? document.body, { key: "Escape" });
  await waitFor(() => expect(screen.queryByRole("menu")).toBeNull());
  const local = within(await openMenu("vms"))
    .getAllByRole("menuitem")
    .map((m) => m.textContent);
  expect(local).toEqual([en["integrity.checkReadable"], en["integrity.unlock"], en["integrity.prune"]]);
});

it("checks the repository here and says that it passed", async () => {
  await renderCard();
  const menu = await openMenu("containers");
  await act(async () => {
    fireEvent.click(within(menu).getByRole("menuitem", { name: en["integrity.checkReadable"] }));
  });
  expect(checkDomain).toHaveBeenCalledWith("containers", "local");
  expect(push).toHaveBeenCalledWith(en["integrity.passed"].replace("{section}", en["nav.containers"]), "success");
});

it("asks before a trial restore and fetches it from off site", async () => {
  await renderCard();
  const menu = await openMenu("containers");
  await act(async () => {
    fireEvent.click(within(menu).getByRole("menuitem", { name: en["verify.trialRestore"] }));
  });
  const dialog = await screen.findByRole("dialog");
  expect(dialog.textContent).toContain(en["verify.trialConfirm"]);
  await act(async () => {
    fireEvent.click(within(dialog).getByRole("button", { name: en["verify.trialRestore"] }));
  });
  expect(runDrill).toHaveBeenCalledWith("containers", "offsite", "dr");
});
