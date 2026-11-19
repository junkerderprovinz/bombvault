// The sign-ins for private container registries. A registry is added in a
// window: pick its kind from the tiles, then fill in what is missing.
import { useState, type CSSProperties, type ReactNode } from "react";
import { catalogPicker } from "@glimstone/core";
import { REGISTRY_PROVIDERS } from "@glimstone/core/registryProviders";
import { PickerWindow } from "@glimstone/react/PickerWindow";
import { ConnectionRow } from "@glimstone/react/ConnectionRow";
import { BrandMark } from "@glimstone/react/ReadmeButton";
import { Window, type WindowAction } from "@glimstone/react/Window";
import { Button } from "../../components/Button";
import { IconSave } from "../../components/glyphs";
import { IconAdd, IconDatabase } from "../../components/navGlyphs";
import { RevealInput } from "../../components/RevealInput";
import { Rows, SettingRow } from "../../components/storage/rows";
import type { RegistryAuthEntry } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { REGISTRY_KINDS, registryKind, type RegistryKind } from "../../lib/registries";
import { useReveal } from "../../lib/useReveal";
import { Card, type SaveState } from "./shared";
import { useSettings } from "./settingsStore";

// The tile that takes any address. Its entries are stored without a kind.
const OTHER = "other";

const FIELD = "rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus w-64 max-w-full";

/** RegistryMark is a registry's logo on a tile of its colour, as the picker
 *  lights it, and a plain glyph for a registry of no known kind. */
function RegistryMark({ kind, large = false }: { kind?: RegistryKind; large?: boolean }) {
  const box = large ? "h-12 w-12 [&_svg]:h-7 [&_svg]:w-7" : "h-(--btn-h) w-(--btn-h) [&_svg]:h-5 [&_svg]:w-5";
  if (!kind) {
    return (
      <span className={`flex ${box} shrink-0 items-center justify-center rounded-control bg-carbon-surface2 text-carbon-textSub`}>
        <IconDatabase />
      </span>
    );
  }
  const { svg, hover, tile, ink } = kind.mark;
  return (
    <span
      aria-hidden="true"
      className={`flex ${box} shrink-0 items-center justify-center rounded-control`}
      style={{ backgroundColor: tile, color: ink, "--mark-ink": ink } as CSSProperties}
    >
      <BrandMark svg={hover ?? svg} />
    </span>
  );
}

/** What the card has open: the add window, or the edit window for the entry
 *  at `index`. */
type Open = { at: "add" } | { at: "edit"; index: number };

interface Draft {
  host: string;
  username: string;
  token: string;
}

type SaveEntry = (entry: RegistryAuthEntry) => Promise<boolean>;

/** The form's fields and its Save, which the add window and the edit window
 *  both put in their foot. */
function useRegistryForm(onSave: SaveEntry, onClose: () => void, initial: Draft) {
  const { t } = useT();
  const [draft, setDraft] = useState(initial);
  const [busy, setBusy] = useState(false);
  const [missing, setMissing] = useState(0);
  const reveal = useReveal();
  const edit = (patch: Partial<Draft>) => setDraft((d) => ({ ...d, ...patch }));

  function start(host: string) {
    setDraft({ host, username: "", token: "" });
    setMissing(0);
  }

  async function submit(kind: string, entry?: RegistryAuthEntry) {
    if (!draft.host.trim()) {
      setMissing((n) => n + 1);
      return;
    }
    setBusy(true);
    const ok = await onSave({
      host: draft.host.trim(),
      username: draft.username.trim(),
      token: draft.token,
      tokenSet: entry?.tokenSet ?? false,
      kind,
    });
    setBusy(false);
    if (ok) onClose();
  }

  function save(kind: string, entry?: RegistryAuthEntry): WindowAction {
    return {
      label: t("settings.save"),
      labelKey: "settings.save",
      glyph: <IconSave />,
      primary: true,
      busy,
      disabled: busy,
      onClick: () => {
        void submit(kind, entry);
        return false;
      },
    };
  }

  function fields(kind: RegistryKind | undefined, name: string, entry?: RegistryAuthEntry): ReactNode {
    return (
      <>
        <div className="flex items-center gap-3">
          <RegistryMark kind={kind} large />
          <div className="flex min-w-0 flex-col gap-0.5">
            <span className="text-sm font-semibold text-carbon-text wrap-anywhere">{name}</span>
            <span className="text-xs text-carbon-textMuted wrap-anywhere">{kind?.host ?? t("tile.registry.withOwnAddress")}</span>
          </div>
        </div>
        <Rows>
          <SettingRow label={t("settings.registryHost")}>
            <input
              type="text"
              value={draft.host}
              onChange={(e) => edit({ host: e.target.value })}
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
              value={draft.username}
              onChange={(e) => edit({ username: e.target.value })}
              aria-label={t("settings.registryUser")}
              autoComplete="off"
              spellCheck={false}
              className={FIELD}
            />
          </SettingRow>
          <SettingRow label={t("settings.registryToken")} note={entry ? t("tile.registry.keepToken") : undefined}>
            <RevealInput
              {...reveal}
              value={draft.token}
              onChange={(e) => edit({ token: e.target.value })}
              aria-label={t("settings.registryToken")}
              autoComplete="new-password"
              placeholder={entry?.tokenSet ? t("cloud.secretSet") : ""}
              wrapperClassName="w-64 max-w-full"
              className="rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus"
            />
          </SettingRow>
        </Rows>
        {missing > 0 && !draft.host.trim() && <p className="text-xs text-statusFail">{t("tile.registry.hostFirst")}</p>}
      </>
    );
  }

  return { start, save, fields };
}

function AddRegistryWindow({ onSave, onClose }: { onSave: SaveEntry; onClose: () => void }) {
  const { t, lang } = useT();
  const form = useRegistryForm(onSave, onClose, { host: "", username: "", token: "" });
  const [picked, setPicked] = useState<string | null>(null);
  const kind = REGISTRY_KINDS.find((k) => k.id === picked);
  const name = kind?.name ?? t("tile.registry.other");
  const { items, groups } = catalogPicker(REGISTRY_PROVIDERS, {
    ids: [...REGISTRY_KINDS.map((k) => k.id), OTHER],
    titles: { public: t("tile.registry.public"), cloud: t("tile.registry.cloud"), selfHosted: t("tile.registry.selfHosted") },
    glyph: () => <IconDatabase />,
    lang,
    own: (entry) => {
      const sub = REGISTRY_KINDS.find((k) => k.id === entry.id)?.host ?? t("tile.registry.ownAddress");
      return entry.id === OTHER ? { label: t("tile.registry.other"), sub, last: true } : { sub };
    },
  });

  return (
    <PickerWindow
      title={picked === null ? t("settings.registryAdd") : t("tile.registry.addTitle").replace("{name}", () => name)}
      hint={t("settings.registriesHint")}
      items={items}
      groups={groups}
      text={{
        search: t("tile.registry.search"),
        none: t("tile.registry.noMatch"),
        clear: t("settings.search.clear"),
        close: t("common.close"),
        cancel: t("common.cancel"),
        back: t("common.back"),
      }}
      onPick={(id) => {
        setPicked(id);
        form.start(REGISTRY_KINDS.find((k) => k.id === id)?.host ?? "");
      }}
      actions={picked === null ? [] : [form.save(kind?.id ?? "")]}
      onBack={() => setPicked(null)}
      onClose={onClose}
    >
      {picked !== null && form.fields(kind, name)}
    </PickerWindow>
  );
}

function EditRegistryWindow({ entry, onSave, onClose }: { entry: RegistryAuthEntry; onSave: SaveEntry; onClose: () => void }) {
  const { t } = useT();
  const form = useRegistryForm(onSave, onClose, { host: entry.host, username: entry.username, token: "" });
  const kind = registryKind(entry.host, entry.kind);
  const name = kind?.name ?? entry.host;
  return (
    <Window
      title={t("common.editTitle").replace("{name}", () => name)}
      text={{ cancel: t("common.cancel"), close: t("common.close") }}
      actions={[form.save(entry.kind, entry)]}
      closeOnBackdrop={false}
      onClose={onClose}
    >
      {form.fields(kind, name, entry)}
    </Window>
  );
}

export function RegistriesCard({ hueIndex }: { hueIndex: number }) {
  const { t } = useT();
  const { settings, save } = useSettings();
  const [open, setOpen] = useState<Open | null>(null);
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
    <Card
      title={t("settings.registriesTitle")}
      hint={t("settings.registriesHint")}
      hueIndex={hueIndex}
      foot={
        <Button
          label={t("settings.registryAdd")}
          labelKey="settings.registryAdd"
          glyph={<IconAdd />}
          onClick={() => setOpen({ at: "add" })}
        />
      }
    >
      {entries.length === 0 ? (
        <p className="text-sm text-carbon-textMuted">{t("settings.registriesEmpty")}</p>
      ) : (
        <ul className="flex flex-col gap-2">
          {entries.map((entry, i) => {
            const kind = registryKind(entry.host, entry.kind);
            const facts = [
              entry.host,
              entry.username && t("tile.registry.userLine").replace("{user}", () => entry.username),
              entry.tokenSet && t("tile.registry.tokenSaved"),
            ];
            return (
              <li key={entry.host}>
                <ConnectionRow
                  name={kind?.name ?? entry.host}
                  mark={kind?.mark ?? <IconDatabase />}
                  status={facts.filter(Boolean).join(" · ")}
                  text={{ test: t("offsite.test"), edit: t("common.edit"), remove: t("settings.registryRemove") }}
                  labelKeys={{ edit: "common.edit", remove: "offsite.targets.remove" }}
                  onEdit={() => setOpen({ at: "edit", index: i })}
                  onRemove={() => void store(entries.filter((_, j) => j !== i))}
                />
              </li>
            );
          })}
        </ul>
      )}
      {open?.at === "add" && <AddRegistryWindow onSave={(entry) => store([...entries, entry])} onClose={() => setOpen(null)} />}
      {open?.at === "edit" && (
        <EditRegistryWindow
          key={open.index}
          entry={entries[open.index]}
          onSave={(entry) => store(entries.map((e, j) => (j === open.index ? entry : e)))}
          onClose={() => setOpen(null)}
        />
      )}
    </Card>
  );
}
