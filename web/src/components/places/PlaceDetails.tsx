import { useEffect, useRef, useState, type ReactNode } from "react";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { FolderBrowser } from "../FolderBrowser";
import { InfoBubble } from "../InfoBubble";
import { NumberField } from "../NumberField";
import { RevealInput } from "../RevealInput";
import { SelectField } from "../SelectField";
import { HUE_OFFSET, Selector } from "../Selector";
import { Toggle } from "../Toggle";
import { fieldLabelKey, folderRoots } from "./PlaceForm";
import { primaryRemoteTamperTest, tamperTest, type OffsiteDomain } from "../../lib/api";
import { retentionLowered } from "../../lib/directRepo";
import { useT, type TranslationKey } from "../../lib/i18n";
import { pushSaveWarnings } from "../../lib/placementCodes";
import { domainName, placeErrorText } from "../../lib/placeText";
import {
  PLACE_DOMAINS,
  patchPlace,
  placesChanged,
  type CatalogProvider,
  type PatchPlaceBody,
  type Place,
  type PlaceDomain,
} from "../../lib/places";
import { STORAGE_CLASSES } from "../../lib/storageClasses";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useReveal } from "../../lib/useReveal";

// A place's details save themselves: a switch or a choice at once, rolled back
// with a shake when the server refuses, a typed field 800 ms after the last
// key. A field that is still waiting when the details close is saved then.

const DEBOUNCE_MS = 800;

/** The credential fields a kind stores in its set; the others go by the address. */
const CRED_KEYS: Partial<Record<Place["kind"], string[]>> = {
  s3: ["keyId", "secret", "region"],
  rest: ["user", "password"],
  webdav: ["url", "user", "password"],
  azure: ["account", "secret"],
};

const DEFAULT_FOLDERS: Record<PlaceDomain, string> = {
  containers: "container",
  vms: "vms",
  flash: "flash",
  config: "config",
  files: "files",
};

type RetentionKey = "retentionKeepLast" | "retentionKeepDaily" | "retentionKeepWeekly" | "retentionKeepMonthly";

const RETENTION: { key: RetentionKey; label: TranslationKey }[] = [
  { key: "retentionKeepLast", label: "places.details.keepLast" },
  { key: "retentionKeepDaily", label: "places.details.keepDaily" },
  { key: "retentionKeepWeekly", label: "places.details.keepWeekly" },
  { key: "retentionKeepMonthly", label: "places.details.keepMonthly" },
];
const RETENTION_KEYS = RETENTION.map((r) => r.key);

const LIMITS: { key: "limitUpload" | "limitDownload" | "growthBudgetGb"; label: TranslationKey; hint: TranslationKey }[] = [
  { key: "limitUpload", label: "places.details.limitUpload", hint: "places.details.limitsHint" },
  { key: "limitDownload", label: "places.details.limitDownload", hint: "places.details.limitsHint" },
  { key: "growthBudgetGb", label: "places.details.growthBudget", hint: "places.details.growthBudgetHint" },
];

const FIELD_CLASS = "w-full rounded-control bg-carbon-surface3 text-carbon-text text-sm px-3 py-1.5 glim-field-focus-well";

type Verdict = { domain: string; text: string };

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

function SecretInput({ id, value, placeholder, onChange }: { id: string; value: string; placeholder: string; onChange: (v: string) => void }) {
  const reveal = useReveal();
  return (
    <RevealInput
      {...reveal}
      id={id}
      value={value}
      placeholder={placeholder}
      onChange={(e) => onChange(e.target.value)}
      autoComplete="off"
      spellCheck={false}
      wrapperClassName="w-full"
      className={`${FIELD_CLASS} font-mono`}
    />
  );
}

export function PlaceDetails({
  place,
  provider,
  hueIndex,
  hostMountRoot,
  onSaved,
}: {
  place: Place;
  /** The catalog entry, for the field labels, the folder roots and whether to ask where it stands. */
  provider?: CatalogProvider;
  hueIndex?: number;
  hostMountRoot: string;
  onSaved: (place: Place) => void;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [draft, setDraft] = useState<Place>(place);
  const [access, setAccess] = useState<Record<string, string>>(() => ({ ...place.creds.fields }));
  const [shake, setShake] = useState<Record<string, number>>({});
  const [verdicts, setVerdicts] = useState<Verdict[] | null>(null);
  const [testing, setTesting] = useState(false);
  const timers = useRef(new Map<string, { timer: ReturnType<typeof setTimeout>; run: () => void }>());

  // A saved answer replaces the draft, except for the fields still waiting to
  // be saved, which keep what is being typed.
  useEffect(() => {
    setDraft((d) => {
      const next = { ...place } as Record<string, unknown>;
      const waiting = [...timers.current.keys()].flatMap((key) => (key === "retention" ? RETENTION_KEYS : [key]));
      for (const key of waiting) if (key in d) next[key] = (d as unknown as Record<string, unknown>)[key];
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

  // The server builds a new base from the provider's form, and a local
  // provider's form is the folder alone.
  function editBase(value: string) {
    setDraft((d) => ({ ...d, base: value }));
    later("base", () => {
      const path = value.trim();
      if (path !== "" && path !== place.base) void save({ address: { path } }, "base");
    });
  }

  // The four numbers make one rule, so they wait, ask and save together.
  function editRetention(key: RetentionKey, value: number) {
    const next = { ...draft, [key]: value };
    setDraft(next);
    later("retention", async () => {
      const patch: Partial<PatchPlaceBody> = {};
      for (const k of RETENTION_KEYS) if (next[k] !== place[k]) patch[k] = next[k];
      if (Object.keys(patch).length === 0) return;
      if (retentionLowered(place, next) && place.usage.items > 0 && !(await confirm(t("places.details.retentionLowerAsk", place.usage.items)))) {
        setDraft((d) => {
          const back = { ...d };
          for (const k of RETENTION_KEYS) back[k] = place[k];
          return back;
        });
        return;
      }
      void save(patch, "retention");
    });
  }

  function editLimit(key: (typeof LIMITS)[number]["key"], value: number) {
    setDraft((d) => ({ ...d, [key]: value }));
    later(key, () => void save({ [key]: value }, key));
  }

  async function setAppendOnly(on: boolean) {
    const items = place.usage.items;
    if (!on && items > 0 && !(await confirm(t("places.details.appendOnlyOffAsk", items)))) return;
    saveAtOnce("immutable", on);
  }

  const credKeys = CRED_KEYS[place.kind] ?? [];
  const secretKeys = new Set(provider?.fields.filter((f) => f.secret).map((f) => f.key) ?? ["secret", "password"]);

  function editAccess(key: string, value: string) {
    const next = { ...access, [key]: value };
    setAccess(next);
    later("access", async () => {
      const fields: Record<string, string> = {};
      for (const k of credKeys) {
        const v = next[k] ?? "";
        if (secretKeys.has(k) ? v !== "" : v !== (place.creds.fields[k] ?? "")) fields[k] = v;
      }
      if (Object.keys(fields).length === 0) return;
      if (await save({ fields }, "access")) {
        setAccess((a) => Object.fromEntries(Object.entries(a).filter(([k]) => !secretKeys.has(k))));
      }
    });
  }

  function setOffered(domain: PlaceDomain, on: boolean) {
    // This save carries a folder name still being typed, and the waiting one
    // would otherwise follow it without this domain.
    const typing = timers.current.get("folders");
    if (typing) {
      clearTimeout(typing.timer);
      timers.current.delete("folders");
    }
    const was = draft.folders[domain];
    const folders = { ...draft.folders };
    if (on) folders[domain] = DEFAULT_FOLDERS[domain];
    else delete folders[domain];
    setDraft((d) => ({ ...d, folders }));
    void save({ folders }, `offer-${domain}`, () =>
      setDraft((d) => {
        const back = { ...d.folders };
        if (was === undefined) delete back[domain];
        else back[domain] = was;
        return { ...d, folders: back };
      })
    );
  }

  function editFolder(domain: PlaceDomain, value: string) {
    const folders = { ...draft.folders, [domain]: value };
    setDraft((d) => ({ ...d, folders }));
    later("folders", () => void save({ folders }, "folders"));
  }

  // A verdict stays in the section, "deletes accepted" included, since the
  // test ran; a test that could not run is a toast and a shake.
  async function runTamperTest() {
    setTesting(true);
    const found: Verdict[] = [];
    let failed = false;
    const domains = [
      ...place.usage.copyDomains.map((d) => ({ domain: d, run: () => tamperTest(d as OffsiteDomain) })),
      ...place.usage.homeDomains.map((d) => ({ domain: d, run: () => primaryRemoteTamperTest(d as OffsiteDomain) })),
    ];
    for (const { domain, run } of domains) {
      try {
        const r = await run();
        if (!r.ok) {
          push(`${domainName(t, domain)}: ${r.error ?? t("common.actionFailed")}`, "fail");
          failed = true;
          continue;
        }
        const text = !r.testable
          ? t("places.details.tamperUntestable")
          : r.protected
            ? t("places.details.tamperProtected")
            : `${t("places.details.tamperOpen")}${r.detail ? `: ${r.detail}` : ""}`;
        found.push({ domain, text });
      } catch (err) {
        push(`${domainName(t, domain)}: ${err instanceof Error ? err.message : t("common.actionFailed")}`, "fail");
        failed = true;
      }
    }
    setVerdicts(found.length > 0 ? found : null);
    if (failed) bump("tamper");
    setTesting(false);
  }

  const labelsOf: CatalogProvider = provider ?? { id: place.provider, group: "cloud", kind: place.kind, fields: [] };
  const asks = provider !== undefined && provider.offPremises === undefined;
  const tamperDomains = place.usage.copyDomains.length + place.usage.homeDomains.length;
  const fieldId = (key: string) => `place-${place.id}-${key}`;

  return (
    <div className="flex flex-col gap-6 pt-2">
      {confirmDialog}

      <Section title={t("places.details.general")} hueIndex={hueIndex}>
        <div className="flex flex-wrap items-end gap-4">
          <div key={`name-${shake.name ?? 0}`} className={`flex min-w-[12rem] flex-1 flex-col gap-1.5 ${shaken("name")}`}>
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
          <span key={`enabled-${shake.enabled ?? 0}`} className={shaken("enabled")}>
            <Toggle label={t("places.details.enabled")} checked={draft.enabled} onChange={(v) => saveAtOnce("enabled", v)} />
          </span>
        </div>
        {place.kind === "local" ? (
          <div key={`base-${shake.base ?? 0}`} className={shaken("base")}>
            <FolderBrowser
              label={t("places.form.address")}
              hint={t("places.details.baseHint")}
              value={draft.base}
              onChange={editBase}
              hostMountRoot={hostMountRoot}
              roots={provider ? folderRoots(provider) : undefined}
            />
          </div>
        ) : (
          <div className="flex flex-col gap-1.5">
            <span className="flex items-center gap-1 text-xs text-carbon-textSub">
              {t("places.form.address")}
              <InfoBubble tip={t("places.details.baseRemoteHint")} />
            </span>
            <span dir="ltr" className="break-all text-start font-mono text-sm text-carbon-text">
              {place.base}
            </span>
          </div>
        )}
        {asks && (
          <div key={`offPremises-${shake.offPremises ?? 0}`} className={`flex flex-col gap-1.5 ${shaken("offPremises")}`}>
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
        <div key={`retention-${shake.retention ?? 0}`} className={`grid grid-cols-2 gap-3 sm:grid-cols-4 ${shaken("retention")}`}>
          {RETENTION.map((r) => (
            <div key={r.key} className="flex flex-col gap-1.5">
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

      {place.kind !== "local" && (
        <Section title={t("places.details.protection")} hueIndex={hueIndex}>
          <div className="flex flex-wrap items-center gap-4">
            <span key={`immutable-${shake.immutable ?? 0}`} className={`flex items-center gap-1.5 ${shaken("immutable")}`}>
              <Toggle label={t("places.details.appendOnly")} checked={draft.immutable} onChange={(v) => void setAppendOnly(v)} />
              <InfoBubble tip={t("places.details.appendOnlyHint")} />
            </span>
            {place.kind === "rest" && draft.immutable && tamperDomains > 0 && (
              <Button
                key={`tamper-${shake.tamper ?? 0}`}
                label={t("places.details.tamperTest")}
                labelKey="places.details.tamperTest"
                tone="neutral"
                busy={testing}
                disabled={testing}
                onClick={() => void runTamperTest()}
                className={shaken("tamper")}
              />
            )}
          </div>
          {verdicts && (
            <ul className="flex flex-col gap-1 text-sm text-carbon-text" aria-live="polite">
              {verdicts.map((v) => (
                <li key={v.domain} className="flex flex-wrap gap-x-2">
                  <span className="text-carbon-textSub">{domainName(t, v.domain)}</span>
                  <span>{v.text}</span>
                </li>
              ))}
            </ul>
          )}
        </Section>
      )}

      {credKeys.length > 0 && (
        <Section
          title={t("places.details.access")}
          hint={place.creds.shared ? t("places.details.sharedCreds") : undefined}
          hueIndex={hueIndex}
        >
          <div key={`access-${shake.access ?? 0}`} className={`grid gap-3 sm:grid-cols-2 ${shaken("access")}`}>
            {credKeys.map((key) => (
              <div key={key} className="flex flex-col gap-1.5">
                <label htmlFor={fieldId(key)} className="text-xs text-carbon-textSub">
                  {t(fieldLabelKey(labelsOf, key))}
                </label>
                {secretKeys.has(key) ? (
                  <SecretInput
                    id={fieldId(key)}
                    value={access[key] ?? ""}
                    placeholder={place.creds.set.includes(key) ? t("places.details.secretKept") : ""}
                    onChange={(v) => editAccess(key, v)}
                  />
                ) : (
                  <input
                    id={fieldId(key)}
                    type="text"
                    dir="ltr"
                    value={access[key] ?? ""}
                    onChange={(e) => editAccess(key, e.target.value)}
                    autoComplete="off"
                    spellCheck={false}
                    className={`${FIELD_CLASS} text-start`}
                  />
                )}
              </div>
            ))}
          </div>
          {place.kind === "s3" && (
            <div key={`storageClass-${shake.storageClass ?? 0}`} className={`flex max-w-xs flex-col gap-1.5 ${shaken("storageClass")}`}>
              <label htmlFor={fieldId("storageClass")} className="text-xs text-carbon-textSub">
                {t("places.details.storageClass")}
              </label>
              <SelectField
                id={fieldId("storageClass")}
                label={t("places.details.storageClass")}
                value={draft.storageClass}
                onChange={(v) => saveAtOnce("storageClass", v)}
                options={[
                  { value: "", label: t("places.details.storageClassDefault") },
                  ...STORAGE_CLASSES.map((sc) => ({ value: sc, label: sc })),
                ]}
                className={FIELD_CLASS}
              />
            </div>
          )}
        </Section>
      )}

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

      <Section
        title={t("places.details.folders")}
        hint={place.repository ? t("places.details.isRepository") : t("places.details.foldersHint")}
        hueIndex={hueIndex}
      >
        <div key={`folders-${shake.folders ?? 0}`} className={`flex flex-col gap-2 ${shaken("folders")}`}>
          {PLACE_DOMAINS.map((d) => {
            const offered = d in draft.folders;
            const locked = place.repository || place.locked[d] === true;
            return (
              <div key={`${d}-${shake[`offer-${d}`] ?? 0}`} className={`flex flex-wrap items-center gap-3 ${shaken(`offer-${d}`)}`}>
                <span className="w-28 text-sm text-carbon-text">{domainName(t, d)}</span>
                <Toggle
                  label={t("places.details.offered").replace("{domain}", domainName(t, d))}
                  hideLabel
                  checked={offered}
                  disabled={locked}
                  onChange={(v) => setOffered(d, v)}
                />
                <input
                  type="text"
                  dir="ltr"
                  aria-label={t("places.details.folderOf").replace("{domain}", domainName(t, d))}
                  value={draft.folders[d] ?? ""}
                  disabled={!offered || locked}
                  onChange={(e) => editFolder(d, e.target.value)}
                  autoComplete="off"
                  spellCheck={false}
                  className={`${FIELD_CLASS} min-w-0 flex-1 font-mono text-start disabled:opacity-50`}
                />
                {place.locked[d] && !place.repository && <InfoBubble tip={t("places.details.folderLocked")} />}
              </div>
            );
          })}
        </div>
      </Section>
    </div>
  );
}
