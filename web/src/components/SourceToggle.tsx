import { useT } from "../lib/i18n";

export type RepoSource = "local" | "offsite";

/**
 * Local | Off-site segmented toggle. Lets the restore browser and the
 * integrity/maintenance card operate on either the primary local repo or the
 * off-site replica.
 */
export function SourceToggle({
  source,
  onChange,
  disabled,
}: {
  source: RepoSource;
  onChange: (s: RepoSource) => void;
  disabled?: boolean;
}) {
  const { t } = useT();
  const opt = (val: RepoSource, label: string) => (
    <button
      type="button"
      onClick={() => onChange(val)}
      disabled={disabled}
      className={`px-2.5 py-1 text-xs transition-colors disabled:opacity-50 ${
        source === val
          ? "bg-accent text-accentContrast"
          : "text-carbon-textSub hover:text-carbon-text"
      }`}
    >
      {label}
    </button>
  );
  return (
    <div className="inline-flex rounded-lg border border-carbon-border overflow-hidden">
      {opt("local", t("source.local"))}
      {opt("offsite", t("source.offsite"))}
    </div>
  );
}
