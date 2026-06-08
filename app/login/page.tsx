import { redirect } from "next/navigation";
import { getDb } from "../../server/db";
import { isOnboarded } from "../../lib/auth";
import { login } from "../actions/auth";
import { getTranslator } from "../../lib/i18n/server";

export const dynamic = "force-dynamic";

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string }>;
}) {
  if (!isOnboarded(getDb())) redirect("/onboarding");
  const { t } = await getTranslator();
  const params = await searchParams;
  return (
    <main style={{ padding: "2rem", maxWidth: 420 }}>
      <h1>{t("login.title")}</h1>
      {params.error ? (
        <p style={{ color: "var(--error)" }}>{t("login.error")}</p>
      ) : null}
      <form action={login}>
        <input
          type="password"
          name="password"
          placeholder={t("login.passwordPlaceholder")}
          required
          style={{ display: "block", width: "100%", padding: 8, marginBottom: 12 }}
        />
        <button type="submit">{t("login.submit")}</button>
      </form>
    </main>
  );
}
