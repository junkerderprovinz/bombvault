import { assembleReport } from "../../server/spike-report";
import { DEFAULT_PROBES } from "../../server/host-probes";
import { requireSession } from "../../lib/auth-server";
import { getTranslator } from "../../lib/i18n/server";

// Server component: runs the probes on each request and renders the report.
// Protected by middleware (first gate) AND requireSession() (defense-in-depth,
// SEC-005). This is the artifact the user opens on the real Unraid host to
// confirm every mount/CLI is reachable.
export const dynamic = "force-dynamic";

export default async function SpikePage() {
  await requireSession();
  const { t } = await getTranslator();
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
      <h1>{t("spike.title")}</h1>
      <p>
        {t("spike.overall")}{" "}
        <strong style={{ color: report.overall ? "var(--success)" : "var(--error)" }}>
          {report.overall ? t("spike.allOk") : t("spike.degraded")}
        </strong>
      </p>
      <table style={{ borderCollapse: "collapse", width: "100%" }}>
        <thead>
          <tr>
            <th style={{ textAlign: "left", padding: 6 }}>{t("spike.colCheck")}</th>
            <th style={{ textAlign: "left", padding: 6 }}>{t("spike.colStatus")}</th>
            <th style={{ textAlign: "left", padding: 6 }}>{t("spike.colDetail")}</th>
          </tr>
        </thead>
        <tbody>
          {report.checks.map((c, i) => (
            <tr key={i}>
              <td style={{ padding: 6 }}>{c.name}</td>
              <td style={{ padding: 6, color: c.ok ? "var(--success)" : "var(--error)" }}>
                {c.ok ? t("spike.ok") : t("spike.fail")}
              </td>
              <td style={{ padding: 6 }}>
                {c.ok ? c.detail : t("spike.probeFailed")}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </main>
  );
}
