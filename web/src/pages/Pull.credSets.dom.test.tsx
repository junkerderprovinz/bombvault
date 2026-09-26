// @vitest-environment jsdom
// The credential sets live on the Pull page, where pull sources choose from
// them. A set that belongs to a storage place goes by the place's name and is
// changed in the place's details, never here.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { CloudCredSet, CloudCredSetInfo } from "../lib/api";
import type { Place } from "../lib/places";

const credsKept = { code: "direct-creds-kept", targetId: "t-b2", targetName: "B2", items: 2 };
const credsKeptText = "B2 direct cannot be opened with the new key and keeps the old one.";

function set(id: string, name: string): CloudCredSetInfo {
  return { id, name, s3KeyId: `key-${id}`, s3Region: "", restUser: "", s3StorageClass: "", s3SecretSet: true, restPasswordSet: false };
}

let sets: CloudCredSetInfo[] = [];
let places: Place[] = [];
const saved: CloudCredSet[][] = [];

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  listPullSources: () => Promise.resolve({ ok: true, sources: [] }),
  getCloudCredSets: () => Promise.resolve({ ok: true, sets }),
  setCloudCredSets: (next: CloudCredSet[]) => {
    saved.push(next);
    return Promise.resolve({ ok: true, warnings: [credsKept] });
  },
}));

vi.mock("../lib/places", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/places")>()),
  listPlaces: () => Promise.resolve({ ok: true, places, unplaced: [] }),
}));

vi.mock("../components/placeMarks", () => ({
  PlaceMark: ({ provider }: { provider: string }) => <span data-testid="mark" data-provider={provider} />,
}));

const { Pull } = await import("./Pull");

function b2Place(): Place {
  return { id: "p-b2", name: "B2 Backups", provider: "b2", credsRef: "set-b2" } as Place;
}

async function renderPull() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Pull />
        </ToastProvider>
      </I18nProvider>
    );
  });
}

const setsCard = () => screen.getByRole("heading", { name: new RegExp(`^${en["cloud.credSets.title"]}`) }).parentElement!;

beforeEach(() => {
  sets = [set("set-b2", "B2 key"), set("set-tower", "Tower")];
  places = [b2Place()];
  saved.length = 0;
});
afterEach(cleanup);

describe("credential sets on the Pull page", () => {
  it("names a place's set after the place and leaves its changes to the place", async () => {
    await renderPull();
    const card = setsCard();
    const placeRow = within(card).getByText("B2 Backups").closest("div.rounded-card") as HTMLElement;
    expect(within(placeRow).getByTestId("mark").getAttribute("data-provider")).toBe("b2");
    expect(within(placeRow).queryByRole("button")).toBeNull();
    expect(within(card).queryByText("B2 key")).toBeNull();
    const towerRow = within(card).getByText("Tower").closest("div.rounded-card") as HTMLElement;
    expect(within(towerRow).getByRole("button", { name: en["offsite.targets.edit"] })).toBeTruthy();
    expect(within(towerRow).getByRole("button", { name: en["offsite.targets.remove"] })).toBeTruthy();
  });

  it("offers every set in the source window, a place's set by the place's name", async () => {
    await renderPull();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: en["pull.addSource"] }));
    });
    fireEvent.click(screen.getByRole("combobox", { name: en["offsite.targets.credsLabel"] }));
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual([en["pull.credsNone"], "B2 Backups", "Tower"]);
  });

  it("shows the warnings a save of a credential set answered with", async () => {
    await renderPull();
    fireEvent.click(within(setsCard()).getByRole("button", { name: en["cloud.credSets.add"] }));
    await act(async () => {
      fireEvent.click(within(setsCard()).getByRole("button", { name: en["settings.save"] }));
    });
    await screen.findByText(credsKeptText);
    expect(saved).toHaveLength(1);
  });
});
