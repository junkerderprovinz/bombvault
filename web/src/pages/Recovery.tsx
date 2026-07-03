import { useT } from "../lib/i18n";

export default function Recovery() {
  const { t } = useT();
  return (
    <div className="flex flex-col gap-5 p-1">
      <div>
        <h1 className="text-lg font-semibold text-carbon-text">{t("recovery.pageTitle")}</h1>
        <p className="text-sm text-carbon-textMuted mt-1 max-w-2xl">{t("recovery.intro")}</p>
      </div>
      {/* Step cards added in Tasks 2-6 */}
    </div>
  );
}
