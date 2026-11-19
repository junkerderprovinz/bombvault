// The sign-ins for private container registries. A registry is added in a
// window: pick its kind from the tiles, then fill in what is missing.
import { useState, type ReactNode } from "react";
import { PickerWindow, type PickerItem, type TileMark } from "@glimstone/react/PickerWindow";
import { Window, type WindowAction } from "@glimstone/react/Window";
import { Button } from "../../components/Button";
import { IconSave } from "../../components/glyphs";
import { IconAdd, IconDatabase, IconPencil, IconTrash } from "../../components/navGlyphs";
import { RevealInput } from "../../components/RevealInput";
import { Rows, SettingRow } from "../../components/storage/rows";
import { textInkOn } from "../../lib/accent";
import type { RegistryAuthEntry } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { REGISTRY_KINDS, registryKind, type RegistryGroup, type RegistryKind } from "../../lib/registryMarks";
import { useReveal } from "../../lib/useReveal";
import { Card, type SaveState } from "./shared";
import { useSettings } from "./settingsStore";

const GROUPS: { id: RegistryGroup; title: TranslationKey }[] = [
  { id: "public", title: "tile.registry.public" },
  { id: "selfHosted", title: "tile.registry.selfHosted" },
  { id: "cloud", title: "tile.registry.cloud" },
];

// The tile that takes any address. Its entries are stored without a kind.
const OTHER = "other";

const FIELD = "rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus w-64 max-w-full";

function logoSrc(kind: RegistryKind): string {
  return `data:image/svg+xml;utf8,${encodeURIComponent(kind.mark)}`;
}

/** A registry's logo as a picker tile's mark. It is drawn as an image, so the
 *  ids inside one logo cannot meet another's, and the image carries the
 *  filter that turns it to the tile's ink when the tile lights up. A logo with
 *  a glyph shows the glyph there instead. */
function tileMark(kind: RegistryKind): TileMark {
  const ink = textInkOn(kind.tile);
  const lit = ink === "#FFFFFF" ? "brightness(0) invert(1)" : "brightness(0)";
  const dark = kind.dark ? " glim-registry-dark" : "";
  const glyph = kind.glyph
    ? `<img class="glim-registry-logo glim-registry-glyph" style="--logo-lit:${lit}" src="data:image/svg+xml;utf8,${encodeURIComponent(kind.glyph)}" alt="">`
    : "";
  return {
    svg: `<img class="glim-registry-logo${dark}${glyph ? " glim-registry-rest" : ""}" style="--logo-lit:${lit}" src="${logoSrc(kind)}" alt="">${glyph}`,
    tile: kind.tile,
    ink,
  };
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
      <img src={logoSrc(kind)} alt="" />
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
  const { t } = useT();
  const form = useRegistryForm(onSave, onClose, { host: "", username: "", token: "" });
  const [picked, setPicked] = useState<string | null>(null);
  const kind = REGISTRY_KINDS.find((k) => k.id === picked);
  const name = kind?.name ?? t("tile.registry.other");
  const items: PickerItem[] = [
    ...REGISTRY_KINDS.map((k) => ({
      id: k.id,
      label: k.name,
      sub: k.host ?? t("tile.registry.ownAddress"),
      group: k.group,
      mark: tileMark(k),
    })),
    {
      id: OTHER,
      label: t("tile.registry.other"),
      sub: t("tile.registry.ownAddress"),
      group: GROUPS[GROUPS.length - 1].id,
      mark: <IconDatabase />,
      last: true,
    },
  ];

  return (
    <PickerWindow
      title={picked === null ? t("settings.registryAdd") : t("tile.registry.addTitle").replace("{name}", () => name)}
      hint={t("settings.registriesHint")}
      items={items}
      groups={GROUPS.map((g) => ({ id: g.id, title: t(g.title) }))}
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
      title={t("tile.registry.editTitle").replace("{name}", () => name)}
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
        <Rows as="ul">
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
                    onClick={() => setOpen({ at: "edit", index: i })}
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
        </Rows>
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
