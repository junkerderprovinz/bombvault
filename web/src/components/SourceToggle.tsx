import { useT } from "../lib/i18n";
import type { OffsiteDomain } from "../lib/useOffsiteTargets";
import {
  offsiteTargetLabel,
  offsiteTargetSource,
  useOffsiteTargets,
} from "../lib/useOffsiteTargets";

/**
 * A repo source: the local repo, the domain's PRIMARY off-site target ("offsite"),
 * or one SPECIFIC off-site target ("offsite:<id>"). The templated third form is
 * what the backend has always parsed (normalizeSource / offsiteTargetForSource);
 * the type used to stop at the binary toggle, so a domain with several off-site
 * copies could only ever be restored from the primary one (issue #138).
 */
export type RepoSource = "local" | "offsite" | `offsite:${string}`;

/**
 * Whether a source string addresses an off-site repo (either form). Takes a
 * plain string so it also classifies a source read back from the API (a
 * recorded drill's `source`, say), not just the toggle's own state.
 */
export function isOffsiteSource(source: string): boolean {
  return source === "offsite" || source.startsWith("offsite:");
}

/**
 * Local | Off-site segmented toggle. Lets the restore browser and the
 * integrity/maintenance card operate on either the primary local repo or the
 * off-site replica.
 *
 * Pass `domain` to make it multi-off-site aware: when that domain has TWO OR
 * MORE enabled off-site targets, a picker appears next to the toggle so the
 * operator chooses WHICH copy to browse/restore from. With one target (the
 * common case) or none, nothing changes — the picker never renders.
 */
export function SourceToggle({
  source,
  onChange,
  disabled,
  domain,
}: {
  source: RepoSource;
  onChange: (s: RepoSource) => void;
  disabled?: boolean;
  domain?: OffsiteDomain;
}) {
  const { t } = useT();
  const targets = useOffsiteTargets(domain);
  const multi = targets.length > 1;

  const opt = (val: RepoSource, label: string, active: boolean) => (
    <button
      type="button"
      onClick={() => onChange(val)}
      disabled={disabled}
      className={`px-2.5 py-1 text-xs transition-colors disabled:opacity-50 ${
        active
          ? "bg-accent text-accentContrast"
          : "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
      }`}
    >
      {label}
    </button>
  );

  return (
    <span className="inline-flex items-center gap-2 flex-wrap">
      <span className="inline-flex rounded-control bg-carbon-surface2 overflow-hidden">
        {opt("local", t("source.local"), source === "local")}
        {/* Switching to off-site always lands on the PRIMARY target first; the
            picker below then narrows it down. */}
        {opt("offsite", t("source.offsite"), isOffsiteSource(source))}
      </span>
      {multi && isOffsiteSource(source) && (
        <select
          aria-label={t("source.offsiteTarget")}
          value={source}
          disabled={disabled}
          onChange={(e) => onChange(e.target.value as RepoSource)}
          className="rounded-control bg-carbon-surface2 text-carbon-text text-xs px-2 py-1 disabled:opacity-50 focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
        >
          {targets.map((target, i) => (
            <option key={target.id} value={offsiteTargetSource(target, i)}>
              {offsiteTargetLabel(target)}
            </option>
          ))}
        </select>
      )}
    </span>
  );
}
