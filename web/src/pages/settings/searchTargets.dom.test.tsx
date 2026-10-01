// @vitest-environment jsdom
// A settings search result opens its page and marks the card at once, so a
// card that loads its own settings has to be there before they arrive.
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, render } from "@testing-library/react";
import { I18nProvider, en, useT } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";
import { jumpTarget } from "./SettingsSearch";

let answer: "pending" | "failed" = "pending";
const reply = () =>
  answer === "pending" ? new Promise<never>(() => undefined) : Promise.resolve({ ok: false, error: "docker is not reachable" });

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return { ...actual, getIdle: () => reply(), getStreaming: () => reply() };
});

const { IdleCard } = await import("./IdleCard");
const { StreamingCard } = await import("./StreamingCard");

function Cards() {
  const { t } = useT();
  return (
    <>
      <IdleCard t={t} />
      <StreamingCard t={t} />
    </>
  );
}

async function renderCards() {
  let container!: HTMLElement;
  await act(async () => {
    ({ container } = render(
      <I18nProvider>
        <ToastProvider>
          <Cards />
        </ToastProvider>
      </I18nProvider>
    ));
  });
  return container;
}

afterEach(() => {
  cleanup();
  answer = "pending";
});

it("offers the idle and streaming cards as search targets while their settings load", async () => {
  const root = await renderCards();
  expect(jumpTarget(root, { card: en["idle.title"], row: en["idle.cpu"] })?.getAttribute("data-search-card")).toBe(en["idle.title"]);
  expect(jumpTarget(root, { card: en["streaming.title"], row: en["streaming.toggle"] })?.getAttribute("data-search-card")).toBe(
    en["streaming.title"]
  );
});

it("leaves the idle and streaming cards out when their settings cannot be read", async () => {
  answer = "failed";
  const root = await renderCards();
  expect(jumpTarget(root, { card: en["idle.title"] })).toBeNull();
  expect(jumpTarget(root, { card: en["streaming.title"] })).toBeNull();
});
