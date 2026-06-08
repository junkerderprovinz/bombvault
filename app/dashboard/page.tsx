import Link from "next/link";
import { getTranslator } from "../../lib/i18n/server";

export const dynamic = "force-dynamic";

export default async function DashboardPage() {
  const { t } = await getTranslator();
  return (
    <main style={{ padding: "2rem" }}>
      <h1>{t("dashboard.title")}</h1>
      <p>{t("dashboard.body")}</p>
      <p>
        <Link href="/spike">{t("dashboard.spikeLink")} →</Link>
      </p>
      <p>
        <Link href="/containers">{t("nav.containers")} →</Link>
      </p>
      <p>
        <Link href="/destinations">{t("nav.destinations")} →</Link>
      </p>
    </main>
  );
}
