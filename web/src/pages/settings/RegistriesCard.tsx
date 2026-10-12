// The sign-ins for private container registries. A registry is added in a
// window: pick its kind from the tiles, then fill in what is missing.
import { useState, type CSSProperties } from "react";
import { Button } from "../../components/Button";
import { IconBack, IconSave } from "../../components/glyphs";
import { IconAdd, IconClose, IconDatabase, IconPencil, IconTrash } from "../../components/navGlyphs";
import { PickerGroup } from "../../components/Picker";
import { ReadmeButton } from "../../components/ReadmeButton";
import { RevealInput } from "../../components/RevealInput";
import { Rows, SettingRow } from "../../components/storage/rows";
import type { RegistryAuthEntry } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { REGISTRY_KINDS, registryKind, type RegistryGroup, type RegistryKind } from "../../lib/registryMarks";
import { useReveal } from "../../lib/useReveal";
import { InstanceWindow } from "../instances/InstanceWindow";
import { fold } from "./SettingsSearch";
import { Card, type SaveState } from "./shared";
import { useSettings } from "./settingsStore";

const GROUPS: { id: RegistryGroup; title: TranslationKey }[] = [
  { id: "public", title: "tile.registry.public" },
  { id: "selfHosted", title: "tile.registry.selfHosted" },
  { id: "cloud", title: "tile.registry.cloud" },
];

const FIELD = "rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus w-64 max-w-full";

function Logo({ kind }: { kind: RegistryKind }) {
  return <img src={`data:image/svg+xml;utf8,${encodeURIComponent(kind.mark)}`} alt="" />;
}

/** RegistryMark is a registry's logo on a tile of its colour, a plain glyph
 *  for a registry of no known kind. */
function RegistryMark({ kind, large = false }: { kind?: RegistryKind; large?: boolean }) {
  const box = large ? "h-12 w-12" : "h-(--btn-h) w-(--btn-h)";
  if (!kind) {
    return (
      <span className={`flex ${box} shrink-0 items-center justify-center rounded-control bg-carbon-surface2 text-carbon-textSub`}>
        <IconDatabase />
      </span>
    );
  }
  return (
    <span
      className={`glim-registry-mark flex ${box} shrink-0 items-center justify-center rounded-control`}
      style={{ backgroundColor: kind.tile }}
    >
      <Logo kind={kind} />
    </span>
  );
}

/** What the window shows: the tiles, or the form for a kind or for the entry
 *  at `index`. */
type Step = { at: "pick" } | { at: "form"; kind: string; index?: number };

function Picker({ onPick, onClose }: { onPick: (kind: string) => void; onClose: () => void }) {
  const { t } = useT();
  const [query, setQuery] = useState("");
  const q = fold(query.trim());
  const hit = (name: string) => fold(name).includes(q);
  const other = hit(t("tile.registry.other"));
  const groups = GROUPS.map((g, at) => ({
    ...g,
    kinds: REGISTRY_KINDS.filter((k) => k.group === g.id && hit(k.name)),
    other: other && at === GROUPS.length - 1,
  })).filter((g) => g.kinds.length > 0 || g.other);

  return (
    <InstanceWindow
      title={t("settings.registryAdd")}
      hint={t("settings.registriesHint")}
      picker
      search={{ value: query, onChange: setQuery, label: t("tile.registry.search") }}
      onClose={onClose}
      footer={<Button label={t("common.cancel")} labelKey="common.cancel" glyph={<IconClose />} onClick={onClose} />}
    >
      {groups.length === 0 && <p className="text-sm text-carbon-textMuted">{t("tile.registry.noMatch")}</p>}
      {groups.map((g) => (
        <PickerGroup key={g.id} title={t(g.title)}>
          <ul className="glim-provider-pick glim-registry-pick">
            {g.kinds.map((k) => (
              <li
                key={k.id}
                className={k.dark ? "glim-registry-dark" : undefined}
                style={{ "--provider-tile": k.tile, "--provider-ink": "#ffffff" } as CSSProperties}
              >
                <ReadmeButton
                  tile="glim-tile-provider"
                  parts={[{ name: k.name, sub: k.host ?? t("tile.registry.ownAddress"), onClick: () => onPick(k.id) }]}
                  mark={<Logo kind={k} />}
                />
              </li>
            ))}
            {g.other && (
              <li>
                <ReadmeButton
                  tile="glim-tile-provider"
                  parts={[{ name: t("tile.registry.other"), sub: t("tile.registry.ownAddress"), onClick: () => onPick("") }]}
                  mark={<IconDatabase />}
                  markClass="text-carbon-textSub"
                />
              </li>
            )}
          </ul>
        </PickerGroup>
      ))}
    </InstanceWindow>
  );
}

function Form({
  step,
  entry,
  onBack,
  onClose,
  onSave,
}: {
  step: Extract<Step, { at: "form" }>;
  /** The entry being edited. */
  entry?: RegistryAuthEntry;
  onBack: () => void;
  onClose: () => void;
  onSave: (entry: RegistryAuthEntry) => Promise<boolean>;
}) {
  const { t } = useT();
  const kind = entry ? registryKind(entry.host, entry.kind) : REGISTRY_KINDS.find((k) => k.id === step.kind);
  const [host, setHost] = useState(entry?.host ?? kind?.host ?? "");
  const [username, setUsername] = useState(entry?.username ?? "");
  const [token, setToken] = useState("");
  const [busy, setBusy] = useState(false);
  const [missing, setMissing] = useState(0);
  const reveal = useReveal();
  const name = kind?.name ?? (entry ? entry.host : t("tile.registry.other"));

  async function submit() {
    if (!host.trim()) {
      setMissing((n) => n + 1);
      return;
    }
    setBusy(true);
    const ok = await onSave({
      host: host.trim(),
      username: username.trim(),
      token,
      tokenSet: entry?.tokenSet ?? false,
      kind: entry ? entry.kind : step.kind,
    });
    setBusy(false);
    if (ok) onClose();
  }

  return (
    <InstanceWindow
      title={t(entry ? "tile.registry.editTitle" : "tile.registry.addTitle").replace("{name}", () => name)}
      facts={kind?.host ?? t("tile.registry.withOwnAddress")}
      picker
      onClose={onClose}
      footer={
        <>
          {entry ? (
            <Button label={t("common.cancel")} labelKey="common.cancel" onClick={onClose} />
          ) : (
            <Button label={t("common.back")} labelKey="common.back" glyph={<IconBack />} onClick={onBack} />
          )}
          <Button
            label={t("settings.save")}
            labelKey="settings.save"
            glyph={<IconSave />}
            tone="accent"
            onClick={() => void submit()}
            disabled={busy}
            busy={busy}
          />
        </>
      }
    >
      <div className="flex items-center gap-3">
        <RegistryMark kind={kind} large />
        <span className="min-w-0 text-sm font-semibold text-carbon-text wrap-anywhere">{name}</span>
      </div>
      <Rows>
        <SettingRow label={t("settings.registryHost")}>
          <input
            type="text"
            value={host}
            onChange={(e) => setHost(e.target.value)}
            placeholder={kind?.placeholder ?? "registry.example.org"}
            aria-label={t("settings.registryHost")}
            spellCheck={false}
            dir="ltr"
            key={missing}
            className={`${FIELD}${missing ? " glim-shake" : ""}`}
          />
        </SettingRow>
        <SettingRow label={t("settings.registryUser")}>
          <input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            aria-label={t("settings.registryUser")}
            autoComplete="off"
            spellCheck={false}
            className={FIELD}
          />
        </SettingRow>
        <SettingRow label={t("settings.registryToken")} note={entry ? t("tile.registry.keepToken") : undefined}>
          <RevealInput
            {...reveal}
            value={token}
            onChange={(e) => setToken(e.target.value)}
            aria-label={t("settings.registryToken")}
            autoComplete="new-password"
            placeholder={entry?.tokenSet ? t("cloud.secretSet") : ""}
            wrapperClassName="w-64 max-w-full"
            className="rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus"
          />
        </SettingRow>
      </Rows>
      {missing > 0 && !host.trim() && <p className="text-xs text-statusFail">{t("tile.registry.hostFirst")}</p>}
    </InstanceWindow>
  );
}

export function RegistriesCard({ hueIndex }: { hueIndex: number }) {
  const { t } = useT();
  const { settings, save } = useSettings();
  const [step, setStep] = useState<Step | null>(null);
  const [, setSaveState] = useState<SaveState>("idle");
  const [, setSaveError] = useState<string | null>(null);
  const entries = settings.registryAuths;

  // A token is sent once and never read back, so the list on screen keeps the
  // rows without it.
  function store(next: RegistryAuthEntry[]): Promise<boolean> {
    return save({ registryAuths: next }, setSaveState, setSaveError, () => ({
      registryAuths: next.map((a) => ({ ...a, token: "", tokenSet: a.tokenSet || a.token !== "" })),
    }));
  }

  return (
    <Card title={t("settings.registriesTitle")} hint={t("settings.registriesHint")} hueIndex={hueIndex}>
      {entries.length === 0 ? (
        <p className="text-sm text-carbon-textMuted">{t("settings.registriesEmpty")}</p>
      ) : (
        <ul className="flex flex-col divide-y divide-carbon-border/60">
          {entries.map((entry, i) => {
            const kind = registryKind(entry.host, entry.kind);
            return (
              <li key={entry.host} className="flex flex-wrap items-center gap-x-3 gap-y-2 py-3 first:pt-0 last:pb-0">
                <RegistryMark kind={kind} />
                <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="truncate text-sm font-semibold text-carbon-text">{kind?.name ?? entry.host}</span>
                  <span className="flex flex-wrap gap-x-1.5 text-xs text-carbon-textMuted">
                    <bdi dir="ltr">{entry.host}</bdi>
                    {entry.username && (
                      <span className="before:me-1.5 before:content-['·']">
                        {t("tile.registry.userLine").replace("{user}", () => entry.username)}
                      </span>
                    )}
                    {entry.tokenSet && (
                      <span className="before:me-1.5 before:content-['·']">{t("tile.registry.tokenSaved")}</span>
                    )}
                  </span>
                </div>
                <div className="ms-auto flex max-w-full flex-wrap items-center justify-end gap-2">
                  <Button
                    label={t("common.edit")}
                    labelKey="common.edit"
                    glyph={<IconPencil />}
                    onClick={() => setStep({ at: "form", kind: entry.kind, index: i })}
                  />
                  <Button
                    label={t("settings.registryRemove")}
                    labelKey="settings.registryRemove"
                    glyph={<IconTrash />}
                    onClick={() => void store(entries.filter((_, j) => j !== i))}
                  />
                </div>
              </li>
            );
          })}
        </ul>
      )}
      <div className="flex justify-end">
        <Button
          label={t("settings.registryAdd")}
          labelKey="settings.registryAdd"
          glyph={<IconAdd />}
          onClick={() => setStep({ at: "pick" })}
        />
      </div>
      {step?.at === "pick" && (
        <Picker onPick={(kind) => setStep({ at: "form", kind })} onClose={() => setStep(null)} />
      )}
      {step?.at === "form" && (
        <Form
          key={step.index ?? step.kind}
          step={step}
          entry={step.index === undefined ? undefined : entries[step.index]}
          onBack={() => setStep({ at: "pick" })}
          onClose={() => setStep(null)}
          onSave={(entry) =>
            store(step.index === undefined ? [...entries, entry] : entries.map((e, j) => (j === step.index ? entry : e)))
          }
        />
      )}
    </Card>
  );
}
