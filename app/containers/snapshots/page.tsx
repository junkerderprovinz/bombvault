import { getTranslator } from "../../../lib/i18n/server";
import { getDb } from "../../../server/db";
import { getConfig } from "../../../lib/config";
import { createRepo } from "../../../lib/backup-repo";
import { snapshots } from "../../../lib/restic";
import { toSnapshotRows } from "./view";
import { restoreAction } from "./actions";

// Reads live restic snapshots + DB on every request — must not be cached.
export const dynamic = "force-dynamic";

// ---------------------------------------------------------------------------
// Search params type (Next.js App Router — sync in 15+, async via await in 16)
// ---------------------------------------------------------------------------

interface SearchParams {
  targetId?: string;
}

export default async function SnapshotsPage({
  searchParams,
}: {
  searchParams: Promise<SearchParams>;
}) {
  const { t } = await getTranslator();
  const params = await searchParams;
  const targetId = params.targetId ?? "";

  // ── Guard: targetId is required ───────────────────────────────────────────
  if (!targetId) {
    return (
      <main style={{ padding: "2rem" }}>
        <h1>{t("snapshots.title")}</h1>
        <p style={{ color: "var(--error)" }}>No target selected.</p>
      </main>
    );
  }

  const cfg = getConfig();
  const repo = createRepo(getDb(), cfg.APP_KEY);

  // ── Load the backup target + its destination ──────────────────────────────
  const targetRow = repo.getTarget(targetId);
  if (!targetRow) {
    return (
      <main style={{ padding: "2rem" }}>
        <h1>{t("snapshots.title")}</h1>
        <p style={{ color: "var(--error)" }}>Backup target not found.</p>
      </main>
    );
  }

  const parsed = repo.parseTarget(targetRow);
  const destRow = repo.getDestinationRow(parsed.destination_id);
  if (!destRow) {
    return (
      <main style={{ padding: "2rem" }}>
        <h1>{t("snapshots.title")}</h1>
        <p style={{ color: "var(--error)" }}>{t("containers.noDestination")}</p>
      </main>
    );
  }

  // ── Load snapshots from the restic repository ─────────────────────────────
  let rows: ReturnType<typeof toSnapshotRows> = [];
  let snapshotError: string | null = null;

  try {
    const repoPassword = repo.getDestinationPassword(parsed.destination_id);
    const snaps = await snapshots(destRow.repo_path, repoPassword);
    // Show newest first.
    rows = toSnapshotRows([...snaps].reverse());
  } catch {
    // Log server-side; show only a generic message in the browser (SEC-006).
    snapshotError = "Could not load snapshots — see server logs.";
  }

  return (
    <main style={{ padding: "2rem" }}>
      <h1>{t("snapshots.title")}</h1>

      <p style={{ color: "var(--fg-muted)", fontSize: "0.875rem", marginTop: 0 }}>
        {parsed.container_name}
      </p>

      <a
        href="/containers"
        style={{ display: "inline-block", marginBottom: "1.5rem", color: "var(--accent)", fontSize: "0.875rem" }}
      >
        ← {t("containers.title")}
      </a>

      {snapshotError ? (
        <p style={{ color: "var(--error)" }}>{snapshotError}</p>
      ) : rows.length === 0 ? (
        <p style={{ color: "var(--fg-muted)" }}>{t("snapshots.none")}</p>
      ) : (
        <table style={{ borderCollapse: "collapse", width: "100%" }}>
          <thead>
            <tr>
              <th style={thStyle}>{t("snapshots.colId")}</th>
              <th style={thStyle}>{t("snapshots.colTime")}</th>
              <th style={thStyle}>{t("snapshots.colTags")}</th>
              <th style={thStyle}>{t("snapshots.restore")}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => {
              const doRestore = restoreAction.bind(null, targetId, row.id, true);
              return (
                <tr key={row.id} style={{ borderBottom: "1px solid var(--border)" }}>
                  {/* ID */}
                  <td style={tdStyle}>
                    <span
                      title={row.id}
                      style={{ fontFamily: "monospace", fontSize: "0.85rem" }}
                    >
                      {row.shortId}
                    </span>
                  </td>

                  {/* Time */}
                  <td style={{ ...tdStyle, fontSize: "0.875rem", color: "var(--fg-muted)" }}>
                    {new Date(row.time).toLocaleString()}
                  </td>

                  {/* Tags */}
                  <td style={{ ...tdStyle, fontSize: "0.8rem", color: "var(--fg-muted)" }}>
                    {row.tags.join(", ")}
                  </td>

                  {/* Restore — two-step confirm form (confirm checkbox must be checked) */}
                  <td style={tdStyle}>
                    <details style={{ display: "inline-block" }}>
                      <summary
                        style={{
                          cursor: "pointer",
                          color: "var(--accent)",
                          fontSize: "0.875rem",
                          listStyle: "none",
                          padding: "0.2rem 0.5rem",
                          border: "1px solid var(--accent)",
                          borderRadius: 2,
                        }}
                      >
                        {t("snapshots.restore")}
                      </summary>

                      {/* Confirm panel — only visible after clicking the summary */}
                      <div
                        style={{
                          position: "absolute",
                          background: "var(--surface)",
                          border: "1px solid var(--border)",
                          borderRadius: 4,
                          padding: "1rem",
                          zIndex: 10,
                          minWidth: 320,
                          marginTop: 4,
                        }}
                      >
                        <p
                          style={{
                            margin: "0 0 0.5rem",
                            fontWeight: "bold",
                            color: "var(--fg)",
                          }}
                        >
                          {t("restore.confirmTitle")}
                        </p>
                        <p
                          style={{
                            margin: "0 0 1rem",
                            fontSize: "0.875rem",
                            color: "var(--fg-muted)",
                          }}
                        >
                          {t("restore.confirmBody")}
                        </p>

                        <form action={doRestore}>
                          {/*
                           * Two-step confirm gate:
                           * The user must tick this checkbox before the submit
                           * button is enabled (required attribute). This prevents
                           * an accidental one-click restore.
                           */}
                          <label
                            style={{
                              display: "flex",
                              alignItems: "center",
                              gap: "0.5rem",
                              fontSize: "0.875rem",
                              marginBottom: "1rem",
                              cursor: "pointer",
                            }}
                          >
                            <input
                              type="checkbox"
                              name="confirmCheck"
                              required
                              style={{ accentColor: "var(--accent)" }}
                            />
                            {t("restore.confirm")}
                          </label>

                          <div style={{ display: "flex", gap: "0.5rem" }}>
                            <button
                              type="submit"
                              style={{
                                background: "var(--error)",
                                color: "#fff",
                                border: "none",
                                borderRadius: 2,
                                padding: "0.35rem 0.9rem",
                                fontSize: "0.875rem",
                                cursor: "pointer",
                                fontFamily: "inherit",
                              }}
                            >
                              {t("restore.confirm")}
                            </button>

                            {/*
                             * Cancel: close the <details> element via a reset
                             * that clears the checkbox (JS-optional progressive
                             * enhancement — without JS the user clicks elsewhere).
                             */}
                            <button
                              type="reset"
                              style={{
                                background: "transparent",
                                color: "var(--fg-muted)",
                                border: "1px solid var(--border)",
                                borderRadius: 2,
                                padding: "0.35rem 0.9rem",
                                fontSize: "0.875rem",
                                cursor: "pointer",
                                fontFamily: "inherit",
                              }}
                            >
                              {t("restore.cancel")}
                            </button>
                          </div>
                        </form>
                      </div>
                    </details>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
    </main>
  );
}

// ---------------------------------------------------------------------------
// Shared cell styles (reduce JSX noise)
// ---------------------------------------------------------------------------

const thStyle: React.CSSProperties = {
  textAlign: "left",
  padding: 8,
  borderBottom: "1px solid var(--border)",
  color: "var(--fg-muted)",
};

const tdStyle: React.CSSProperties = {
  padding: 8,
};
