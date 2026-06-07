import { assembleReport } from "../../server/spike-report";
import { DEFAULT_PROBES } from "../../server/host-probes";

// Server component: runs the probes on each request and renders the report.
// Protected by middleware (matcher includes /spike). This is the artifact the
// user opens on the real Unraid host to confirm every mount/CLI is reachable.
export const dynamic = "force-dynamic";

export default async function SpikePage() {
  const report = await assembleReport(DEFAULT_PROBES);
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
              <td style={{ padding: 6 }}>{c.ok ? c.detail : c.error}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </main>
  );
}
