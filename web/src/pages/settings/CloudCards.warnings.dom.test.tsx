// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, useT } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";

const credsKept = { code: "direct-creds-kept", targetId: "t-b2", targetName: "B2", items: 2 };
const credsKeptText = "B2 direct cannot be opened with the new key and keeps the old one.";

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  getCloud: () =>
    Promise.resolve({
      ok: true, s3KeyId: "AKIAEXAMPLE", s3Region: "", restUser: "", s3StorageClass: "", s3SecretSet: true, restPasswordSet: false,
    }),
  setCloud: () => Promise.resolve({ ok: true, warnings: [credsKept] }),
}));

const { CloudCard } = await import("./CloudCard");

function Harness() {
  const { t } = useT();
  return <CloudCard t={t} hueIndex={0} />;
}

async function renderCard() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Harness />
        </ToastProvider>
      </I18nProvider>
    );
  });
}

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

it("shows the warnings a save of the shared credentials answered with", async () => {
  vi.useFakeTimers();
  await renderCard();
  fireEvent.change(screen.getByLabelText(/AWS_SECRET_ACCESS_KEY/), { target: { value: "new-secret" } });
  await act(async () => {
    await vi.advanceTimersByTimeAsync(900);
  });
  expect(screen.getByText(credsKeptText)).toBeTruthy();
});
