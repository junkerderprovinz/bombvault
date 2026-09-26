// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";

let stored: string[] = [];
const saved: string[] = [];
let saveAnswer: { ok: boolean; error?: string } = { ok: true };

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  getRclone: () => Promise.resolve({ ok: true, remotes: stored }),
  setRclone: (conf: string) => {
    saved.push(conf);
    if (saveAnswer.ok) stored = ["b2", "gdrive"];
    return Promise.resolve(saveAnswer);
  },
}));

const { RcloneConfig } = await import("./RcloneConfig");

beforeEach(() => {
  stored = ["b2"];
  saved.length = 0;
  saveAnswer = { ok: true };
});
afterEach(cleanup);

async function show(onRemotes = vi.fn()) {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <RcloneConfig onRemotes={onRemotes} />
        </ToastProvider>
      </I18nProvider>
    );
  });
  return onRemotes;
}

const config = () => screen.getByLabelText(en["places.rclone.config"]);
const save = () => screen.getByRole("button", { name: en["places.rclone.save"] });

describe("RcloneConfig", () => {
  it("reports the remotes the stored config names", async () => {
    const onRemotes = await show();
    expect(onRemotes).toHaveBeenLastCalledWith(["b2"]);
    expect(save()).toHaveProperty("disabled", true);
  });

  it("saves a pasted config at once and reports its remotes", async () => {
    const onRemotes = await show();
    fireEvent.change(config(), { target: { value: "[gdrive]\ntype = drive\n" } });
    await act(async () => {
      fireEvent.click(save());
    });
    expect(saved).toEqual(["[gdrive]\ntype = drive\n"]);
    expect(onRemotes).toHaveBeenLastCalledWith(["b2", "gdrive"]);
    expect(config()).toHaveProperty("value", "");
    expect(await screen.findByText(en["places.rclone.saved"])).toBeTruthy();
  });

  it("keeps a refused config, says why and shakes", async () => {
    saveAnswer = { ok: false, error: "the rclone config does not parse" };
    await show();
    fireEvent.change(config(), { target: { value: "nonsense" } });
    await act(async () => {
      fireEvent.click(save());
    });
    expect(await screen.findByText("the rclone config does not parse")).toBeTruthy();
    expect(config()).toHaveProperty("value", "nonsense");
    expect(save().className).toContain("glim-shake");
  });
});
