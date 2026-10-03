// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { I18nProvider, useT } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";

vi.mock("../../lib/useCloudCredSets", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/useCloudCredSets")>()),
  useCloudCredSets: () => [
    { id: "s1", name: "NAS direct", keptFor: "r1", s3KeyId: "", s3Region: "", restUser: "bv", s3StorageClass: "", s3SecretSet: false, restPasswordSet: true },
  ],
}));

const { CloudCredSetsCard } = await import("./CloudCredSetsCard");

function Harness() {
  const { t } = useT();
  return <CloudCredSetsCard t={t} hueIndex={0} />;
}

afterEach(cleanup);

it("lists a kept set under a translated name", async () => {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <Harness />
        </ToastProvider>
      </I18nProvider>
    );
  });
  expect(screen.getByText("NAS direct (kept credentials)")).toBeTruthy();
});
