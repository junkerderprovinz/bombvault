import { assembleReport } from "../../server/spike-report";
import { DEFAULT_PROBES } from "../../server/host-probes";
import { requireSession } from "../../lib/auth-server";

// Server component: runs the probes on each request and renders the report.
// Protected by middleware (first gate) AND requireSession() (defense-in-depth,
// SEC-005). This is the artifact the user opens on the real Unraid host to
// confirm every mount/CLI is reachable.
export const dynamic = "force-dynamic";

export default async function SpikePage() {
  await requireSession();
  const report = await assembleReport(DEFAULT_PROBES);

  // SEC-006: never render raw probe error text to the UI (a future probe error
  // could echo a repo path, host detail, or secret). Keep the detail in the
  // server log; show a generic message in the browser.
  for (const c of report.checks) {
    if (!c.ok && c.error) {
      // eslint-disable-next-line no-console
      console.error(`spike check failed: ${c.name}: ${c.error}`);
    }
  }
  return (
    <main style={{ padding: "2rem" }}>
      <h1>Host Integration Spike</h1>
      <p>
        Overall:{" "}
        <strong style={{ color: report.overall ? "#42be65" : "#fa4d56" }}>
          {report.overall ? "ALL OK" : "DEGRADED"}
        </strong>
      </p>
      <table style={{ borderCollapse: "collapse", width: "100%" }}>
        <thead>
          <tr>
            <th style={{ textAlign: "left", padding: 6 }}>Check</th>
            <th style={{ textAlign: "left", padding: 6 }}>Status</th>
            <th style={{ textAlign: "left", padding: 6 }}>Detail</th>
          </tr>
        </thead>
        <tbody>
          {report.checks.map((c, i) => (
            <tr key={i}>
              <td style={{ padding: 6 }}>{c.name}</td>
              <td style={{ padding: 6, color: c.ok ? "#42be65" : "#fa4d56" }}>
                {c.ok ? "OK" : "FAIL"}
              </td>
              <td style={{ padding: 6 }}>{c.ok ? c.detail : "probe failed (see server logs)"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </main>
  );
}
