import { useT } from "../lib/i18n";
import type { OffsiteDomain } from "../lib/useOffsiteTargets";
import {
  offsiteTargetLabel,
  offsiteTargetSource,
  useOffsiteTargets,
} from "../lib/useOffsiteTargets";
import { Selector } from "./Selector";
import { IconFolder, IconCloud } from "./Sidebar";

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

        UPDATED (jdp, live-review, Flash tab: "Lokal und Offsite Button
        sollen quadratische Badges mit Glyphen sein" — icon-badge standing
        rule): icon-only, same conversion PathModeSwitch.tsx's own
        Local/Remote pair already went through (that file's own "GlimStone
        follow-up round, Paths & Storage tab rework, points 2/5" comment).
        This is the ONE shared component behind EVERY "Local/Off-site"
        source picker in the app (Flash.tsx, RestorePanel.tsx,
        Containers.tsx, Config.tsx, Files.tsx, VMs.tsx, Recovery.tsx,
        Settings.tsx's integrity Card) — fixing it here once, rather than
        forking a second icon-only copy for Flash alone, covers every one of
        those call sites in the same pass (verified: all eight sit their own
        `t("source.label")`/`t("recovery.configSourceLabel")` caption
        directly beside this component already, so losing the segments' own
        text loses nothing — the caption still names the control for a
        sighted user, `SelectorItem.label` still becomes each segment's
        `aria-label` for everyone else). IconFolder/IconCloud reused
        verbatim from Sidebar.tsx's shared icon set — the SAME "local
        disk" vs "remote/off-site" glyph pair PathModeSwitch's own
        Local/Remote segments already draw, so a user learns the glyphs
        once and reads them the same way in both places. Engine + tooltip
        pairing (standing rule, icon-badges-need-engine-and-tooltip) comes
        free from Selector itself, not bolted on here: `hue` defaults to
        `true` (this file never opts out), so each segment already resolves
        its own rainbow position via `hueVars(rainbowAt(i))` — the same
        mechanism that already painted the OLD text chips before this
        change, now painting the icon-only squares instead; `tip` is
        Selector's own hover/focus InfoBubble-style bubble (SelectorTab),
        carrying each segment's PRE-iconOnly text ("Local"/"Off-site") so
        the meaning that used to sit in the visible label survives as a
        tooltip. Square footprint (`h-8 w-8`) is `iconOnly`'s own existing
        Selector styling, not new here.
      */}
      <Selector
        items={[
          {
            id: "local",
            label: t("source.local"),
            icon: <IconFolder />,
            tip: t("source.localTip"),
          },
          {
            id: "offsite",
            label: t("source.offsite"),
            icon: <IconCloud />,
            tip: t("source.offsiteTip"),
          },
        ]}
        label={t("source.label")}
        select="one"
        equalWidth
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
