import { useRef, useState } from "react";
import { Button } from "../Button";
import { excludeFromTarget, type NewTargetExclusion, type OffsiteDomain } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { isPlacementDomain } from "../../lib/placement";
import { placementErrorText } from "../../lib/placementCodes";
import { placementChanged } from "../../lib/placementEvents";
import { useToast } from "../../lib/toast";
import { useNewTargetQuestion } from "./NewTargetQuestion";

/** OffsiteLocationInput edits a domain's off-site field. A change is saved on
 *  Enter, on leaving the field or with Save; a new location asks first, because
 *  its target receives the whole history. A stored value that changes
 *  underneath (an import, a save from elsewhere) only pulls the draft along
 *  while the user has not diverged from it, the same rule HomeSelect follows. */
export function OffsiteLocationInput({
  domain,
  value,
  targetId,
  targetName,
  placeholder,
  className,
  onSave,
}: {
  domain: OffsiteDomain;
  value: string;
  targetId?: string;
  targetName?: string;
  placeholder: string;
  className: string;
  onSave: (next: string) => Promise<boolean>;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { ask, dialog } = useNewTargetQuestion();
  const [draft, setDraft] = useState(value);
  const [synced, setSynced] = useState(value);
  if (value !== synced) {
    setSynced(value);
    if (draft === synced) setDraft(value);
  }
  // Save takes the focus from the field, and so does the question; both would
  // start a second save while the first is still asking.
  const committing = useRef(false);

  async function commit() {
    const next = draft.trim();
    if (committing.current || next === value.trim()) return;
    committing.current = true;
    try {
      let alsoExclude: NewTargetExclusion | null = null;
      if (next !== "") {
        const answer = await ask({ domain, location: next, targetId, name: targetName || next, moved: value.trim() !== "" });
        if (!answer.go) {
          setDraft(value);
          return;
        }
        alsoExclude = answer.alsoExclude;
      }
      if (!(await onSave(next)) || !alsoExclude || !isPlacementDomain(domain)) return;
      const r = await excludeFromTarget({ domain, field: true, ...alsoExclude });
      if (r.ok) placementChanged();
      else push(placementErrorText(t, lang, r, "settings.error"), "fail");
    } finally {
      committing.current = false;
    }
  }

  return (
    <div className="flex items-center gap-2">
      {dialog}
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
  );
}
