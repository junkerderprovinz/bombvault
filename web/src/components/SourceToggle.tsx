import { useT } from "../lib/i18n";
import { SelectField } from "./SelectField";
import type { OffsiteDomain } from "../lib/useOffsiteTargets";
import {
  offsiteTargetLabel,
  offsiteTargetSource,
  useOffsiteTargets,
} from "../lib/useOffsiteTargets";
import { Selector } from "./Selector";
import { IconLocal, IconCloud } from "./Sidebar";

/**
 * A repo source: the local repo, the domain's primary off-site target
 * ("offsite"), or one specific off-site target ("offsite:<id>"), matching what
 * the backend's normalizeSource parses.
 */
export type RepoSource = "local" | "offsite" | `offsite:${string}`;

/**
 * Reports whether a source addresses an off-site repo. It takes a plain string
 * so it also works on sources read back from the API.
 */
export function isOffsiteSource(source: string): boolean {
  return source === "offsite" || source.startsWith("offsite:");
}

/**
 * Local / off-site toggle for the restore browser and the integrity card. With
 * `domain` set and more than one enabled off-site target, a picker next to it
 * chooses which copy to use.
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
        `active` collapses "offsite:<id>" to "offsite", which would otherwise
        match neither item and leave both unselected. Choosing "offsite" picks
        the primary target; the picker below narrows it to a specific one.
      */}
      <Selector
        items={[
          {
            id: "local",
            label: t("source.local"),
            icon: <IconLocal />,
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
        <SelectField
          label={t("source.offsiteTarget")}
          value={source}
          disabled={disabled}
          onChange={(v) => onChange(v as RepoSource)}
          options={targets.map((target, i) => ({
            value: offsiteTargetSource(target, i),
            label: offsiteTargetLabel(target),
          }))}
          className="rounded-control bg-carbon-surface2 text-carbon-text text-xs px-2 py-1 disabled:opacity-50 glim-field-focus"
        />
      )}
    </span>
  );
}
