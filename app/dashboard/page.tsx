import Link from "next/link";
import { logout } from "../actions/auth";
import { requireSession } from "../../lib/auth-server";
import { getTranslator } from "../../lib/i18n/server";

// Protected by middleware (first gate) AND requireSession() (defense-in-depth,
// SEC-005): re-verifies the token and checks the session_version server-side.
export const dynamic = "force-dynamic";

export default async function DashboardPage() {
  await requireSession();
  const { t } = await getTranslator();
  return (
    <main style={{ padding: "2rem" }}>
      <h1>{t("dashboard.title")}</h1>
      <p>{t("dashboard.body")}</p>
      <p>
        <Link href="/spike">{t("dashboard.spikeLink")} →</Link>
      </p>
      <form action={logout}>
        <button type="submit">{t("dashboard.signOut")}</button>
      </form>
    </main>
  );
}
