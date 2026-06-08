import { getTranslator } from "../../lib/i18n/server";
import { getDb } from "../../server/db";
import { getConfig } from "../../lib/config";
import { createRepo } from "../../lib/backup-repo";
import { createDockerClient, listContainers } from "../../lib/docker";
import { toContainerRows } from "./view";
import { backupNowAction } from "./actions";

// Reads the Docker socket + DB on every request — must not be cached.
export const dynamic = "force-dynamic";

export default async function ContainersPage() {
  const { t } = await getTranslator();
  const cfg = getConfig();

  // ── Discover containers (graceful degradation) ────────────────────────────
  // If the Docker socket is absent (CI, dev machine without Docker, etc.) we
  // show a friendly message instead of crashing, matching the spike-page pattern.
  let rows: Awaited<ReturnType<typeof toContainerRows>> = [];
  let dockerError: string | null = null;

  try {
    const docker = createDockerClient();
    const containerList = await listContainers(docker);

    // ── Last-backup map from the DB ─────────────────────────────────────────
    // For each container we look up its target (if any) then its last
    // successful backup run. We do this inline to avoid N+1 queries by pulling
    // all targets in one shot.
    const repo = createRepo(getDb(), cfg.APP_KEY);
    const db = getDb();

    // One query: all targets (P1 — small scale, no pagination needed).
    const allTargets = db
      .prepare<[], { container_name: string; id: string }>(
        "SELECT id, container_name FROM backup_target",
      )
      .all();

    const lastBackupByName = new Map<string, string | null>();
    for (const target of allTargets) {
      const run = repo.lastBackupRun(target.id);
      // Convert epoch-ms timestamp to ISO string for display.
      lastBackupByName.set(
        target.container_name,
        run ? new Date(run.started_at).toISOString() : null,
      );
    }

    rows = toContainerRows(containerList, cfg.APPDATA_DIR, lastBackupByName);
  } catch (err) {
    // Log the full error server-side; show only a generic message in the browser
    // (no path/socket details leak to the client — avoids SEC-006 regression).
    // eslint-disable-next-line no-console
    console.error("containers: Docker socket unavailable:", err);
    dockerError = "Docker socket unavailable — see server logs.";
  }

  return (
    <main style={{ padding: "2rem" }}>
      <h1>{t("containers.title")}</h1>

      {dockerError ? (
        <p style={{ color: "var(--error)" }}>{dockerError}</p>
      ) : rows.length === 0 ? (
        <p style={{ color: "var(--fg-muted)" }}>{t("containers.discover")}</p>
      ) : (
        <table style={{ borderCollapse: "collapse", width: "100%" }}>
          <thead>
            <tr>
              <th
                style={{
                  textAlign: "left",
                  padding: 8,
                  borderBottom: "1px solid var(--border)",
                  color: "var(--fg-muted)",
                }}
              >
                {t("containers.colName")}
              </th>
              <th
                style={{
                  textAlign: "left",
                  padding: 8,
                  borderBottom: "1px solid var(--border)",
                  color: "var(--fg-muted)",
                }}
              >
                {t("containers.colImage")}
              </th>
              <th
                style={{
                  textAlign: "left",
                  padding: 8,
                  borderBottom: "1px solid var(--border)",
                  color: "var(--fg-muted)",
                }}
              >
                {t("containers.colStatus")}
              </th>
              <th
                style={{
                  textAlign: "left",
                  padding: 8,
                  borderBottom: "1px solid var(--border)",
                  color: "var(--fg-muted)",
                }}
              >
                {t("containers.colAppdata")}
              </th>
              <th
                style={{
                  textAlign: "left",
                  padding: 8,
                  borderBottom: "1px solid var(--border)",
                  color: "var(--fg-muted)",
                }}
              >
                {t("containers.lastBackup")}
              </th>
              <th
                style={{
                  textAlign: "left",
                  padding: 8,
                  borderBottom: "1px solid var(--border)",
                  color: "var(--fg-muted)",
                }}
              >
                {t("containers.colActions")}
              </th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr
                key={row.id}
                style={{ borderBottom: "1px solid var(--border)" }}
              >
                <td style={{ padding: 8 }}>{row.name}</td>
                <td style={{ padding: 8, color: "var(--fg-muted)", fontSize: "0.875rem" }}>
                  {row.image}
                </td>
                <td
                  style={{
                    padding: 8,
                    color:
                      row.state === "running"
                        ? "var(--success)"
                        : "var(--fg-muted)",
                  }}
                >
                  {row.state}
                </td>
                <td
                  style={{
                    padding: 8,
                    fontSize: "0.8rem",
                    color: "var(--fg-muted)",
                    maxWidth: 280,
                    wordBreak: "break-all",
                  }}
                >
                  {row.appdataPaths.join(", ")}
                </td>
                <td style={{ padding: 8, fontSize: "0.875rem", color: "var(--fg-muted)" }}>
                  {row.lastBackup
                    ? new Date(row.lastBackup).toLocaleString()
                    : t("containers.never")}
                </td>
                <td style={{ padding: 8 }}>
                  <form
                    action={async () => {
                      "use server";
                      await backupNowAction(row.name);
                    }}
                  >
                    <button
                      type="submit"
                      style={{
                        background: "var(--accent)",
                        color: "#fff",
                        border: "none",
                        borderRadius: 2,
                        padding: "0.35rem 0.9rem",
                        fontSize: "0.875rem",
                        cursor: "pointer",
                        fontFamily: "inherit",
                      }}
                    >
                      {t("containers.backupNow")}
                    </button>
                  </form>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </main>
  );
}
