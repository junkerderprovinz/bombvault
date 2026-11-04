// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import type { DomainStatus } from "../../lib/api";
import { I18nProvider, useT } from "../../lib/i18n";
import { worstRpoStatus } from "./rpo";
import { WorstRpoHealthLine } from "./WorstRpoHealthLine";

function domain(status: string): DomainStatus {
  return { domain: "containers", enabled: true, status } as DomainStatus;
}

function Line({ domains }: { domains: DomainStatus[] }) {
  const { t } = useT();
  return <WorstRpoHealthLine t={t} domains={domains} loading={false} />;
}

afterEach(cleanup);

describe("the recovery point", () => {
  it("reads a section waiting for its first run as neutral, without a failure badge", () => {
    render(
      <I18nProvider>
        <Line domains={[domain("ok"), domain("never")]} />
      </I18nProvider>
    );

    expect(screen.getByText("No backup yet")).toBeTruthy();
    expect(screen.queryByText("Overdue")).toBeNull();
    expect(screen.queryByText("FAIL")).toBeNull();
  });

  it("ranks overdue and due soon above a section with no backup yet", () => {
    expect(worstRpoStatus([domain("never"), domain("overdue")])).toBe("overdue");
    expect(worstRpoStatus([domain("never"), domain("warn")])).toBe("warn");
    expect(worstRpoStatus([domain("ok"), domain("never")])).toBe("never");
    expect(worstRpoStatus([domain("ok")])).toBe("ok");
    expect(worstRpoStatus([domain("off")])).toBe("off");
  });
});
