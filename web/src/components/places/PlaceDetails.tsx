import { useEffect, useRef, useState, type ReactNode } from "react";
import { Badge } from "../Badge";
import { InfoBubble } from "../InfoBubble";
import { NumberField } from "../NumberField";
import { HUE_OFFSET, Selector } from "../Selector";
import { Toggle } from "../Toggle";
import { retentionLowered } from "../../lib/directRepo";
import { useT, type TranslationKey } from "../../lib/i18n";
import { pushSaveWarnings } from "../../lib/placementCodes";
import { placeErrorText } from "../../lib/placeText";
import { patchPlace, placesChanged, type CatalogProvider, type PatchPlaceBody, type Place } from "../../lib/places";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";

// A place's details save themselves: a switch or a choice at once, rolled back
// with a shake when the server refuses, a typed field 800 ms after the last
// key. A field that is still waiting when the details close is saved then.

const DEBOUNCE_MS = 800;

const RETENTION: { key: "retentionKeepLast" | "retentionKeepDaily" | "retentionKeepWeekly" | "retentionKeepMonthly"; label: TranslationKey }[] = [
  { key: "retentionKeepLast", label: "places.details.keepLast" },
  { key: "retentionKeepDaily", label: "places.details.keepDaily" },
  { key: "retentionKeepWeekly", label: "places.details.keepWeekly" },
  { key: "retentionKeepMonthly", label: "places.details.keepMonthly" },
];

const LIMITS: { key: "limitUpload" | "limitDownload" | "growthBudgetGb"; label: TranslationKey; hint: TranslationKey }[] = [
  { key: "limitUpload", label: "places.details.limitUpload", hint: "places.details.limitsHint" },
  { key: "limitDownload", label: "places.details.limitDownload", hint: "places.details.limitsHint" },
  { key: "growthBudgetGb", label: "places.details.growthBudget", hint: "places.details.growthBudgetHint" },
];

const FIELD_CLASS = "w-full rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 glim-field-focus-well";

function Section({ title, hint, hueIndex, children }: { title: string; hint?: string; hueIndex?: number; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-3">
      <h4 className="flex items-center">
        <Badge tone="heading" size="heading" inFlow hueIndex={hueIndex}>
          {title}
          {hint && <InfoBubble tip={hint} onAccent />}
        </Badge>
      </h4>
      {children}
    </section>
  );
}

export function PlaceDetails({
  place,
  provider,
  hueIndex,
  onSaved,
}: {
  place: Place;
  /** The catalog entry, for the field labels and whether to ask where it stands. */
  provider?: CatalogProvider;
  hueIndex?: number;
  onSaved: (place: Place) => void;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [draft, setDraft] = useState<Place>(place);
  const [shake, setShake] = useState<Record<string, number>>({});
  const timers = useRef(new Map<string, { timer: ReturnType<typeof setTimeout>; run: () => void }>());

  // A saved answer replaces the draft, except for the fields still waiting to
  // be saved, which keep what is being typed.
  useEffect(() => {
    setDraft((d) => {
      const next = { ...place } as Record<string, unknown>;
      for (const key of timers.current.keys()) if (key in d) next[key] = (d as unknown as Record<string, unknown>)[key];
      return next as unknown as Place;
    });
  }, [place]);

  useEffect(() => {
    const pending = timers.current;
    return () => {
      for (const { timer, run } of pending.values()) {
        clearTimeout(timer);
        run();
      }
      pending.clear();
    };
  }, []);

  const bump = (key: string) => setShake((s) => ({ ...s, [key]: (s[key] ?? 0) + 1 }));
  const shaken = (key: string) => (shake[key] ? "glim-shake" : "");

  async function save(patch: Partial<PatchPlaceBody>, key: string, rollback?: () => void): Promise<boolean> {
    try {
      const res = await patchPlace(place.id, patch);
      if (res.ok && res.place) {
        onSaved(res.place);
        pushSaveWarnings(push, t, res.warnings);
        placesChanged();
        return true;
      }
      push(placeErrorText(t, lang, res, "settings.error"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
    rollback?.();
    bump(key);
    return false;
  }

  function later(key: string, run: () => void) {
    const waiting = timers.current.get(key);
    if (waiting) clearTimeout(waiting.timer);
    const timer = setTimeout(() => {
      timers.current.delete(key);
      run();
    }, DEBOUNCE_MS);
    timers.current.set(key, { timer, run });
  }

  function saveAtOnce<K extends keyof PatchPlaceBody & keyof Place>(key: K, value: Place[K] & PatchPlaceBody[K]) {
    const before = place[key];
    setDraft((d) => ({ ...d, [key]: value }));
    void save({ [key]: value }, key, () => setDraft((d) => ({ ...d, [key]: before })));
  }

  function editName(value: string) {
    setDraft((d) => ({ ...d, name: value }));
    later("name", () => {
      if (value.trim() !== "") void save({ name: value.trim() }, "name");
    });
  }

  function editRetention(key: (typeof RETENTION)[number]["key"], value: number) {
    const next = { ...draft, [key]: value };
    setDraft(next);
    later(key, async () => {
      if (retentionLowered(place, next) && place.usage.items > 0 && !(await confirm(t("places.details.retentionLowerAsk", place.usage.items)))) {
        setDraft((d) => ({ ...d, [key]: place[key] }));
        return;
      }
      void save({ [key]: value }, key);
    });
  }

  function editLimit(key: (typeof LIMITS)[number]["key"], value: number) {
    setDraft((d) => ({ ...d, [key]: value }));
    later(key, () => void save({ [key]: value }, key));
  }

  const asks = provider !== undefined && provider.offPremises === undefined;
  const fieldId = (key: string) => `place-${place.id}-${key}`;

  return (
    <div className="flex flex-col gap-6 pt-2">
      {confirmDialog}

      <Section title={t("places.details.general")} hueIndex={hueIndex}>
        <div className="flex flex-wrap items-end gap-4">
          <div key={shake.name ?? 0} className={`flex min-w-[12rem] flex-1 flex-col gap-1.5 ${shaken("name")}`}>
            <label htmlFor={fieldId("name")} className="text-xs text-carbon-textSub">
              {t("places.form.name")}
            </label>
            <input
              id={fieldId("name")}
              type="text"
              value={draft.name}
              onChange={(e) => editName(e.target.value)}
              autoComplete="off"
              className={FIELD_CLASS}
            />
          </div>
          <span key={shake.enabled ?? 0} className={shaken("enabled")}>
            <Toggle label={t("places.details.enabled")} checked={draft.enabled} onChange={(v) => saveAtOnce("enabled", v)} />
          </span>
        </div>
        {asks && (
          <div key={shake.offPremises ?? 0} className={`flex flex-col gap-1.5 ${shaken("offPremises")}`}>
            <span className="flex items-center gap-1 text-xs text-carbon-textSub">
              {t("places.form.where")}
              <InfoBubble tip={t("places.form.whereHint")} />
            </span>
            <Selector
              label={t("places.form.where")}
              variant="well"
              hueOffset={HUE_OFFSET.placeWhere}
              items={[
                { id: "here", label: t("places.form.here") },
                { id: "away", label: t("places.form.away") },
              ]}
              active={draft.offPremises ? "away" : "here"}
              onChange={(id) => saveAtOnce("offPremises", id === "away")}
            />
          </div>
        )}
      </Section>

      <Section title={t("places.details.retention")} hint={t("places.details.retentionHint")} hueIndex={hueIndex}>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {RETENTION.map((r) => (
            <div key={`${r.key}-${shake[r.key] ?? 0}`} className={`flex flex-col gap-1.5 ${shaken(r.key)}`}>
              <label htmlFor={fieldId(r.key)} className="text-xs text-carbon-textSub">
                {t(r.label)}
              </label>
              <NumberField
                id={fieldId(r.key)}
                min={0}
                value={draft[r.key]}
                onChange={(e) => editRetention(r.key, Math.max(0, Number(e.target.value) || 0))}
                className={FIELD_CLASS}
              />
            </div>
          ))}
        </div>
      </Section>

      <Section title={t("places.details.limits")} hueIndex={hueIndex}>
        <div className="grid gap-3 sm:grid-cols-3">
          {LIMITS.map((l) => (
            <div key={`${l.key}-${shake[l.key] ?? 0}`} className={`flex flex-col gap-1.5 ${shaken(l.key)}`}>
              <label htmlFor={fieldId(l.key)} className="flex items-center gap-1 text-xs text-carbon-textSub">
                {t(l.label)}
                <InfoBubble tip={t(l.hint)} />
              </label>
              <NumberField
                id={fieldId(l.key)}
                min={0}
                value={draft[l.key]}
                onChange={(e) => editLimit(l.key, Math.max(0, Number(e.target.value) || 0))}
                className={FIELD_CLASS}
              />
            </div>
          ))}
        </div>
      </Section>
    </div>
  );
}
