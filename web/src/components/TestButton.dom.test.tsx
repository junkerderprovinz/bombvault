// @vitest-environment jsdom
// A test's verdict belongs to the inputs it tested. Once one of them changes,
// the button goes back to offering the test instead of vouching for values it
// never saw.
import { useState } from "react";
import { afterEach, expect, it } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../lib/i18n";
import { useTestVerdict } from "../lib/useTestVerdict";
import { TestButton, VerdictLine } from "./TestButton";

let answer = { ok: true, reason: undefined as string | undefined };

function Harness() {
  const [host, setHost] = useState("tower");
  const [port, setPort] = useState(22);
  const test = useTestVerdict({ host, port }, "fallback");
  return (
    <>
      <input aria-label="host" value={host} onChange={(e) => setHost(e.target.value)} />
      {/* Sets the same port again, which rebuilds the inputs without changing them. */}
      <button type="button" onClick={() => setPort(22)}>
        same
      </button>
      <VerdictLine verdict={test.verdict} />
      <TestButton label="Test connection" labelKey="vm.ssh.test" test={test} onClick={() => void test.run(async () => answer)} />
    </>
  );
}

async function renderAndTest() {
  render(
    <I18nProvider>
      <Harness />
    </I18nProvider>,
  );
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Test connection" }));
  });
}

afterEach(() => {
  cleanup();
  answer = { ok: true, reason: undefined };
});

it("drops a pass when an input it tested changes", async () => {
  await renderAndTest();
  expect(screen.getByRole("button", { name: en["verdict.connected"] })).toBeTruthy();
  fireEvent.change(screen.getByLabelText("host"), { target: { value: "tower2" } });
  const button = screen.getByRole("button", { name: "Test connection" });
  expect(button.className).not.toContain("bg-statusOkSolid");
});

it("drops a failure and its reason when an input changes", async () => {
  answer = { ok: false, reason: "Connection refused" };
  await renderAndTest();
  expect(screen.getByText("Connection refused")).toBeTruthy();
  fireEvent.change(screen.getByLabelText("host"), { target: { value: "tower2" } });
  expect(screen.queryByText("Connection refused")).toBeNull();
  expect(screen.getByRole("button", { name: "Test connection" }).className).not.toContain("glim-shake");
});

it("keeps the verdict when the inputs are rebuilt with the same values", async () => {
  await renderAndTest();
  fireEvent.click(screen.getByRole("button", { name: "same" }));
  expect(screen.getByRole("button", { name: en["verdict.connected"] })).toBeTruthy();
});

it("keeps the dropped verdict gone when an input changes back", async () => {
  await renderAndTest();
  fireEvent.change(screen.getByLabelText("host"), { target: { value: "tower2" } });
  fireEvent.change(screen.getByLabelText("host"), { target: { value: "tower" } });
  expect(screen.getByRole("button", { name: "Test connection" })).toBeTruthy();
});
