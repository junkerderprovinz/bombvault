// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import type { Provider } from "../../lib/api";
import { I18nProvider } from "../../lib/i18n";

const ONEDRIVE = { id: "onedrive", name: "OneDrive", backend: "onedrive", route: "rclone", group: "cloud", auth: "token" } as Provider;
const QUESTION = { name: "config_driveid", choices: [{ value: "a", help: "drive a" }, { value: "b", help: "drive b" }] };

const checkDraft = vi.fn();
vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  getProviders: () => Promise.resolve({ ok: true, providers: [ONEDRIVE], backends: {} }),
  checkDraft: (...args: unknown[]) => checkDraft(...args),
  draftFolders: () => Promise.resolve({ ok: true, folders: [] }),
}));

const { useDestinationDraft } = await import("./useDestinationDraft");

const wrapper = ({ children }: { children: ReactNode }) => <I18nProvider>{children}</I18nProvider>;

afterEach(() => {
  cleanup();
  checkDraft.mockReset();
});

async function onedriveDraft() {
  const hook = renderHook(() => useDestinationDraft(), { wrapper });
  await act(async () => {
    hook.result.current.pick(ONEDRIVE);
  });
  await act(async () => {
    hook.result.current.setSetting("token", "{}");
  });
  checkDraft.mockResolvedValueOnce({ ok: true, reachable: false, question: QUESTION });
  await act(async () => {
    await hook.result.current.runTest();
  });
  return hook;
}

it("holds the choice a test left open and sends the answer with the next test", async () => {
  const hook = await onedriveDraft();
  expect(hook.result.current.question).toEqual(QUESTION);
  expect(hook.result.current.reached).toBe(false);

  await act(async () => {
    hook.result.current.setSetting("config_driveid", "b");
  });
  checkDraft.mockResolvedValueOnce({ ok: true, reachable: true });
  await act(async () => {
    await hook.result.current.runTest();
  });
  expect(checkDraft.mock.calls[1][0].settings).toEqual({ token: "{}", config_driveid: "b" });
  expect(hook.result.current.reached).toBe(true);
});

it("asks the choice again once the sign-in changes", async () => {
  const hook = await onedriveDraft();
  await act(async () => {
    hook.result.current.setSetting("config_driveid", "b");
  });

  await act(async () => {
    hook.result.current.setSetting("token", '{"other":1}');
  });

  expect(hook.result.current.question).toBeNull();
  expect(hook.result.current.settings).toEqual({ token: '{"other":1}' });
});
