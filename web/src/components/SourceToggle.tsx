import { useT } from "../lib/i18n";
import type { OffsiteDomain } from "../lib/useOffsiteTargets";
import {
  offsiteTargetLabel,
  offsiteTargetSource,
  useOffsiteTargets,
} from "../lib/useOffsiteTargets";
import { Selector } from "./Selector";

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
  const offsite = isOffsiteSource(source);

  return (
    <span className="inline-flex items-center gap-2 flex-wrap">
      {/*
        Local | Off-site, on the shared Selector component (GlimStone
        form-engine Phase 2, Task 3). `active` is normalized to a plain
        "local"|"offsite" pair rather than the raw `source` string: a
        multi-target off-site source is stored as "offsite:<id>" (issue
        #138), which would never strictly equal either item's id and leave
        BOTH chips reading as unselected. Selecting "offsite" always lands
        on the PRIMARY target first (onChange("offsite")); the target
        picker below then narrows it down to a specific one.

        The two segments no longer share ONE bg-carbon-surface2 pill behind
        them (the "no wrapping bar" rule removes that wrapper) — Selector's
        default `plain={false}` chip treatment puts that same background on
        each segment individually instead, so removing the shared box
        doesn't wash the pair out to plain unstyled text.
      */}
      <Selector
        items={[
          { id: "local", label: t("source.local") },
          { id: "offsite", label: t("source.offsite") },
        ]}
        label={t("source.label")}
        select="one"
        active={offsite ? "offsite" : "local"}
        onChange={(id) => onChange(id as RepoSource)}
        disabled={disabled}
        size="md"
      />
      {multi && isOffsiteSource(source) && (
        <select
          aria-label={t("source.offsiteTarget")}
          value={source}
          disabled={disabled}
          onChange={(e) => onChange(e.target.value as RepoSource)}
          className="rounded-control bg-carbon-surface2 text-carbon-text text-xs px-2 py-1 disabled:opacity-50 bv-field-focus"
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
