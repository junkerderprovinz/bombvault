import { useRef, useState } from "react";
import { Button } from "../Button";
import { InfoBubble } from "../InfoBubble";
import {
  destinationFolder,
  excludeFromTarget,
  primaryFromDestination,
  type Destination,
  type NewTargetExclusion,
  type OffsiteDomain,
} from "../../lib/api";
import { useT } from "../../lib/i18n";
import { isPlacementDomain } from "../../lib/placement";
import { placementErrorText } from "../../lib/placementCodes";
import { placementChanged } from "../../lib/placementEvents";
import { useToast } from "../../lib/toast";
import { destinationsChanged, useDestinations } from "../../lib/useDestinations";
import { offsiteTargetsChanged } from "../../lib/useOffsiteTargets";
import { useNewTargetQuestion } from "./NewTargetQuestion";

/** OffsiteLocationInput edits a domain's off-site field. A change is saved on
 *  Enter, on leaving the field or with Save; a new location asks first, because
 *  its target receives the whole history. A stored value that changes
 *  underneath (an import, a save from elsewhere) only pulls the draft along
 *  while the user has not diverged from it, the same rule HomeSelect follows.
 *
 *  With onFromDestination, each destination the domain does not copy to yet
 *  gets a button that puts the copy into its folder instead, and the callback
 *  hears the field and append-only flag the server set. While the copy follows
 *  a destination, named by following, the location is shown locked until the
 *  user chooses to type one. */
export function OffsiteLocationInput({
  domain,
  value,
  targetId,
  targetName,
  following,
  placeholder,
  className,
  onSave,
  onFromDestination,
}: {
  domain: OffsiteDomain;
  value: string;
  targetId?: string;
  targetName?: string;
  following?: string;
  placeholder: string;
  className: string;
  onSave: (next: string) => Promise<boolean>;
  onFromDestination?: (location: string, immutable: boolean) => void;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { ask, dialog } = useNewTargetQuestion();
  const { destinations } = useDestinations();
  const [draft, setDraft] = useState(value);
  const [synced, setSynced] = useState(value);
  const [typing, setTyping] = useState(false);
  if (value !== synced) {
    setSynced(value);
    if (draft === synced) setDraft(value);
  }
  // Save takes the focus from the field, and so does the question; both would
  // start a second save while the first is still asking.
  const committing = useRef(false);
  const unused = onFromDestination ? destinations.filter((d) => !d.domains.includes(domain)) : [];

  async function excludeToo(alsoExclude: NewTargetExclusion | null) {
    if (!alsoExclude || !isPlacementDomain(domain)) return;
    const r = await excludeFromTarget({ domain, field: true, ...alsoExclude });
    if (r.ok) placementChanged();
    else push(placementErrorText(t, lang, r, "settings.error"), "fail");
  }

  async function commit() {
    const next = draft.trim();
    if (committing.current || next === value.trim()) return;
    committing.current = true;
    try {
      let alsoExclude: NewTargetExclusion | null = null;
      if (next !== "") {
        const answer = await ask({ domain, location: next, targetId, name: targetName || next, moved: value.trim() !== "" });
        if (!answer.go) return;
        alsoExclude = answer.alsoExclude;
      }
      if (await onSave(next)) await excludeToo(alsoExclude);
    } finally {
      committing.current = false;
    }
  }

  async function use(d: Destination) {
    if (committing.current) return;
    committing.current = true;
    try {
      const answer = await ask({ domain, location: destinationFolder(d, domain), targetId, name: d.name, moved: value.trim() !== "" });
      if (!answer.go) return;
      const r = await primaryFromDestination(d.id, domain).catch((e: unknown) => ({
        ok: false,
        error: e instanceof Error ? e.message : undefined,
        location: undefined,
        immutable: undefined,
      }));
      if (!r.ok || r.location === undefined) {
        push(placementErrorText(t, lang, r, "settings.error"), "fail");
        return;
      }
      onFromDestination?.(r.location, r.immutable ?? false);
      setTyping(false);
      push(t("offsite.primary.used").replace("{name}", () => d.name), "success");
      offsiteTargetsChanged();
      destinationsChanged();
      await excludeToo(answer.alsoExclude);
    } finally {
      committing.current = false;
    }
  }

  return (
    <div className="flex flex-col gap-2">
      {dialog}
      {following !== undefined && !typing ? (
        <>
          <div className="flex items-center gap-2">
            <span dir="ltr" className={`min-w-0 flex-1 break-all ${className}`}>{value}</span>
            <Button
              label={t("offsite.primary.retype")}
              labelKey="offsite.primary.retype"
              tone="neutral"
              onClick={() => setTyping(true)}
              className="glim-tile-raise"
            />
          </div>
          <span className="text-xs text-carbon-textMuted">{t("offsite.primary.follows").replace("{name}", () => following)}</span>
        </>
      ) : (
        <div className="flex items-center gap-2">
          <input
            value={draft}
            spellCheck={false}
            placeholder={placeholder}
            dir="ltr"
            className={`min-w-0 flex-1 ${className}`}
            onChange={(e) => setDraft(e.target.value)}
            onBlur={() => void commit()}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                void commit();
              } else if (e.key === "Escape") {
                setDraft(value);
              }
            }}
          />
          {draft.trim() !== value.trim() && (
            <Button label={t("settings.save")} labelKey="settings.save" tone="accent" onClick={() => void commit()} />
          )}
        </div>
      )}
      {unused.length > 0 && (
        <div className="flex flex-wrap items-center gap-2">
          {unused.map((d) => (
            <Button
              key={d.id}
              label={t("offsite.primary.use").replace("{name}", () => d.name)}
              labelKey="offsite.primary.use"
              tone="neutral"
              onClick={() => void use(d)}
              className="glim-btn-wrap glim-tile-raise"
            />
          ))}
          <InfoBubble tip={t("offsite.primary.useTip")} />
        </div>
      )}
    </div>
  );
}
