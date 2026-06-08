import { getTranslator } from "../../lib/i18n/server";
import { getDb } from "../../server/db";
import { getConfig } from "../../lib/config";
import { createRepo } from "../../lib/backup-repo";
import { saveDestinationAction } from "./actions";

// Reads cookies + DB on every request — must be dynamic.
export const dynamic = "force-dynamic";

export default async function DestinationsPage() {
  const { t } = await getTranslator();

  const repo = createRepo(getDb(), getConfig().APP_KEY);
  const destinations = repo.listDestinations();

  return (
    <main style={{ padding: "2rem" }}>
      <h1>{t("destinations.title")}</h1>

      {destinations.length > 0 && (
        <section style={{ marginBottom: "2rem" }}>
          <table style={{ borderCollapse: "collapse", width: "100%" }}>
            <thead>
              <tr>
                <th style={{ textAlign: "left", padding: 6, borderBottom: "1px solid var(--border)" }}>
                  {t("containers.colName")}
                </th>
                <th style={{ textAlign: "left", padding: 6, borderBottom: "1px solid var(--border)" }}>
                  {t("destinations.localPath")}
                </th>
              </tr>
            </thead>
            <tbody>
              {destinations.map((d) => (
                <tr key={d.id}>
                  <td style={{ padding: 6, borderBottom: "1px solid var(--border)" }}>{d.name}</td>
                  <td style={{ padding: 6, borderBottom: "1px solid var(--border)" }}>{d.repo_path}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      )}

      <section
        style={{
          background: "var(--surface)",
          border: "1px solid var(--border)",
          borderRadius: 4,
          padding: "1.5rem",
          maxWidth: 480,
        }}
      >
        <form action={saveDestinationAction} style={{ display: "flex", flexDirection: "column", gap: "1rem" }}>
          <div style={{ display: "flex", flexDirection: "column", gap: "0.25rem" }}>
            <label htmlFor="dest-name" style={{ color: "var(--fg-muted)", fontSize: "0.875rem" }}>
              {t("containers.colName")}
            </label>
            <input
              id="dest-name"
              name="name"
              type="text"
              required
              style={{
                background: "var(--bg)",
                color: "var(--fg)",
                border: "1px solid var(--border)",
                borderRadius: 2,
                padding: "0.5rem 0.75rem",
                fontFamily: "inherit",
                fontSize: "1rem",
              }}
            />
          </div>

          <div style={{ display: "flex", flexDirection: "column", gap: "0.25rem" }}>
            <label htmlFor="dest-path" style={{ color: "var(--fg-muted)", fontSize: "0.875rem" }}>
              {t("destinations.localPath")}
            </label>
            <input
              id="dest-path"
              name="repoPath"
              type="text"
              required
              style={{
                background: "var(--bg)",
                color: "var(--fg)",
                border: "1px solid var(--border)",
                borderRadius: 2,
                padding: "0.5rem 0.75rem",
                fontFamily: "inherit",
                fontSize: "1rem",
              }}
            />
          </div>

          <div style={{ display: "flex", flexDirection: "column", gap: "0.25rem" }}>
            <label htmlFor="dest-password" style={{ color: "var(--fg-muted)", fontSize: "0.875rem" }}>
              {t("destinations.password")}
            </label>
            {/* Password is write-only — never rendered back to the browser. */}
            <input
              id="dest-password"
              name="password"
              type="password"
              required
              autoComplete="new-password"
              style={{
                background: "var(--bg)",
                color: "var(--fg)",
                border: "1px solid var(--border)",
                borderRadius: 2,
                padding: "0.5rem 0.75rem",
                fontFamily: "inherit",
                fontSize: "1rem",
              }}
            />
          </div>

          <p style={{ margin: 0, fontSize: "0.8rem", color: "var(--fg-muted)" }}>
            {t("destinations.initOnSave")}
          </p>

          <div>
            <button
              type="submit"
              style={{
                background: "var(--accent)",
                color: "#fff",
                border: "none",
                borderRadius: 2,
                padding: "0.5rem 1.5rem",
                fontSize: "1rem",
                cursor: "pointer",
              }}
            >
              {t("destinations.save")}
            </button>
          </div>
        </form>
      </section>
    </main>
  );
}
