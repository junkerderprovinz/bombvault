import { redirect } from "next/navigation";
import { getDb } from "../../server/db";
import { isOnboarded } from "../../lib/auth";
import { completeOnboarding } from "../actions/auth";
import { getTranslator } from "../../lib/i18n/server";
import { getConfig } from "../../lib/config";

export const dynamic = "force-dynamic";

export default async function OnboardingPage() {
  if (getConfig().DISABLE_AUTH) redirect("/dashboard");
  if (isOnboarded(getDb())) redirect("/login");
  const { t } = await getTranslator();
  return (
    <main style={{ padding: "2rem", maxWidth: 420 }}>
      <h1>{t("onboarding.title")}</h1>
      <p>{t("onboarding.subtitle")}</p>
      <form action={completeOnboarding}>
        <input
          type="password"
          name="password"
          placeholder={t("onboarding.passwordPlaceholder")}
          minLength={8}
          required
          style={{ display: "block", width: "100%", padding: 8, marginBottom: 12 }}
        />
        <button type="submit">{t("onboarding.submit")}</button>
      </form>
    </main>
  );
}
