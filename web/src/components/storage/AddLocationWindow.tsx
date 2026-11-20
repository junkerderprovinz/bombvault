import { useEffect, useState, type CSSProperties } from "react";
import { catalogPicker, type PickGroup } from "@glimstone/core";
import { STORAGE_PROVIDERS, type StorageProviderId } from "@glimstone/core/storageProviders";
import { PickerWindow, type PickerItem } from "@glimstone/react/PickerWindow";
import type { WindowAction } from "@glimstone/react/Window";
import { useNavigate } from "react-router-dom";
import {
  destinationFolder,
  destinationForDomain,
  getNewTargetPreview,
  previewPlacementDefault,
  putPlacementDefault,
  type DefaultRow,
  type OffsiteDomain,
  type PlacementDomain,
  type PlacementOptions,
  type Provider,
  type TargetPreview,
} from "../../lib/api";
import { hueVars } from "../../lib/appearance";
import { useT, type TranslationKey } from "../../lib/i18n";
import { defaultChangeOf, defaultView, formatList, isPlacementDomain, lockHint, stepForNewTarget, viewHomeLabel } from "../../lib/placement";
import { tickDestination } from "../../lib/placementButtons";
import { placementErrorText, type PlacementRefusal } from "../../lib/placementCodes";
import { placementChanged } from "../../lib/placementEvents";
import { DOMAIN_LABEL, locationRoute } from "../../lib/storageLocations";
import { useToast } from "../../lib/toast";
import { destinationsChanged } from "../../lib/useDestinations";
import { useLocalNames } from "../../lib/useLocalName";
import { offsiteTargetsChanged } from "../../lib/useOffsiteTargets";
import { usePlacementOptions } from "../../lib/usePlacementOptions";
import { useRcloneRemotes } from "../../lib/useRcloneRemotes";
import { Button } from "../Button";
import { RCLONE_MARK, providerName, storageEntry, storageGlyph } from "../destinations/providers";
import { FolderBody, ProtectBody, RecoveryBody, SignInBody, type DestinationWords } from "../destinations/steps";
import { useDestinationDraft } from "../destinations/useDestinationDraft";
import { receiverTile, useGroupOffers } from "../destinations/useGroupOffers";
import { IconForward, IconSave } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { IconCheck, IconCloud, IconPencil } from "../navGlyphs";
import { itemName } from "../placement/NewTargetQuestion";
import { Selector } from "../Selector";
import { verdictKey } from "../TestButton";
import { Toggle } from "../Toggle";
import { OwnRcloneWindow } from "./OwnRcloneWindow";
import { DomainGlyph } from "./rows";
import { StepCard } from "./StepCard";

const WORDS: DestinationWords = {
  folderTip: "storage.add.folderTip",
  lockNone: "storage.add.lockNone",
  recovery: "storage.add.recovery",
};

const STEPS: TranslationKey[] = [
  "storage.add.stepProvider",
  "storage.add.stepSignIn",
  "storage.folder",
  "storage.add.stepProtect",
  "storage.add.stepSections",
];

// A folder on this host is a provider like the others to the server, and the
// one tile that needs no sign-in, so it stands apart and first.
const LOCAL = "local";

// The tile for remotes from an rclone.conf of one's own.
const OWN_RCLONE = "rclone.conf";

// Where a provider the catalog does not know yet stands, by the server's group.
const UNKNOWN_GROUP = { storage: "storage", selfs3: "storage", server: "protocol", cloud: "cloud" } as const;

interface SectionChoice {
  domain: OffsiteDomain;
  /** Why the new place cannot be switched on for the section. */
  locked?: string;
  preview?: TargetPreview;
}

/** switchOnFor switches a new location on for a section and answers why it
 *  could not, or null once it is on. */
export async function switchOnFor(
  id: string,
  domain: OffsiteDomain,
  row: DefaultRow | undefined,
  offered: PlacementOptions | null,
  t: ReturnType<typeof useT>["t"],
  lang: string
): Promise<string | null> {
  const why = (res: PlacementRefusal) => (res.ok ? null : placementErrorText(t, lang, res, "settings.error"));
  if (!isPlacementDomain(domain)) return why(await destinationForDomain(id, domain));
  if (!row || !offered) return t("placement.unreadable");
  const ticked = await tickDestination(id, domain);
  if ("error" in ticked) return why(ticked.error);
  const change = defaultChangeOf(stepForNewTarget(ticked.targetId, defaultView(row, offered), offered));
  if (!change) return t("settings.error");
  const preview = await previewPlacementDefault(domain, change);
  if (!preview.ok || !preview.impact) return why({ ...preview, ok: false });
  return why(await putPlacementDefault(domain, change, preview.impact));
}

/**
 * AddLocationWindow sets up a storage location in one window: the provider,
 * its sign-in, the folder, the delete protection and, last, which sections
 * are backed up to the new place.
 */
export function AddLocationWindow({
  domains,
  defaults,
  onClose,
}: {
  /** The sections that are backed up at all. */
  domains: OffsiteDomain[];
  defaults: DefaultRow[] | null;
  onClose: () => void;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const localOf = useLocalNames();
  const navigate = useNavigate();
  const draft = useDestinationDraft();
  const { picked } = draft;
  const [step, setStep] = useState(1);
  const [furthest, setFurthest] = useState(1);
  const [off, setOff] = useState<OffsiteDomain[]>([]);
  const [previews, setPreviews] = useState<Partial<Record<PlacementDomain, TargetPreview>>>({});
  const [placing, setPlacing] = useState(false);
  const [ownConf, setOwnConf] = useState(false);
  const rclone = useRcloneRemotes();
  const offers = useGroupOffers({
    onPick: (r) => void draft.pickFromGroup(r).then((filled) => filled && go(2)),
    onAsked: onClose,
  });
  const options: Record<PlacementDomain, PlacementOptions | null> = {
    containers: usePlacementOptions("containers").options,
    vms: usePlacementOptions("vms").options,
    files: usePlacementOptions("files").options,
  };

  const nameOf = (p: Provider) => (p.route === "path" ? t("storage.localFolder") : providerName(p, t));
  const label = picked ? nameOf(picked) : "";
  const reach = draft.reached ? furthest : Math.min(furthest, 2);
  const shown = Math.min(step, reach);
  const { location } = draft;

  function go(next: number) {
    setStep(next);
    setFurthest((n) => Math.max(n, next));
  }

  function pick(p: Provider) {
    draft.pick(p);
    if (p.route === "path") draft.setName(t("storage.localFolder"));
    go(2);
  }

  useEffect(() => {
    if (shown !== 5 || location === "") return;
    let alive = true;
    for (const domain of domains) {
      if (!isPlacementDomain(domain)) continue;
      void getNewTargetPreview(domain, destinationFolder({ repo: location }, domain))
        .then((res) => {
          if (alive && res.ok && res.preview) setPreviews((seen) => ({ ...seen, [domain]: res.preview }));
        })
        .catch(() => undefined);
    }
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [shown, location]);

  const choices: SectionChoice[] = domains.map((domain) => {
    if (!isPlacementDomain(domain)) return { domain };
    const row = defaults?.find((r) => r.domain === domain);
    const offered = options[domain];
    if (!row || !offered || row.unreadable || offered.unreadable) return { domain, locked: t("placement.unreadable") };
    const view = defaultView(row, offered);
    if (view.repoKind === "remote") return { domain, locked: lockHint(t, "own-credentials", viewHomeLabel(t, localOf(domain), view, offered)) };
    return { domain, preview: previews[domain] };
  });

  async function finish() {
    setPlacing(true);
    try {
      const made = await draft.save();
      if (!made) return;
      const failed: { domain: OffsiteDomain; why: string }[] = [];
      for (const { domain, locked } of choices) {
        if (locked || off.includes(domain)) continue;
        const row = defaults?.find((r) => r.domain === domain);
        const offered = isPlacementDomain(domain) ? options[domain] : null;
        const why = await switchOnFor(made.id, domain, row, offered, t, lang).catch((err: unknown) =>
          err instanceof Error ? err.message : t("settings.error")
        );
        if (why !== null) failed.push({ domain, why });
      }
      destinationsChanged();
      offsiteTargetsChanged();
      placementChanged();
      if (failed.length > 0) {
        const list = formatList(lang, failed.map(({ domain }) => t(DOMAIN_LABEL[domain])));
        push(t("storage.add.sectionsFailed").replace("{name}", () => made.name).replace("{list}", () => list), "warn");
        for (const { domain, why } of failed) push(`${t(DOMAIN_LABEL[domain])}: ${why}`, "warn");
      }
      onClose();
      navigate(locationRoute(`destination:${made.id}`), { replace: true });
    } finally {
      setPlacing(false);
    }
  }

  const catalog = catalogPicker(STORAGE_PROVIDERS, {
    ids: draft.providers.filter((p) => storageEntry(p.id)).map((p) => p.id as StorageProviderId),
    titles: {
      cloud: { title: t("dest.group.cloud"), badge: { label: t("dest.fit.slower"), tone: "warn" }, hint: t("dest.groupTip.cloud") },
      storage: {
        title: t("storage.add.groupS3"),
        badge: { label: t("dest.fit.best"), tone: "ok" },
        hint: `${t("dest.groupTip.storage")} ${t("dest.groupTip.selfs3")}`,
      },
      protocol: { title: t("storage.add.groupServer"), badge: { label: t("dest.fit.good"), tone: "ok" }, hint: t("dest.groupTip.server") },
      // BombVault offers no target built on another one.
      layered: "",
    },
    glyph: (entry) => storageGlyph(entry.glyph),
    lang,
    own: (entry) => {
      if (entry.id === "path") return { label: t("storage.localFolder"), group: LOCAL };
      if (entry.id === "s3") return { label: t("dest.provider.s3"), terms: ["S3 compatible", t("dest.provider.s3Word")] };
      if (entry.id === "smb") return { sub: t("dest.provider.smbKind") };
      return undefined;
    },
  });
  const unknown: PickerItem[] = draft.providers
    .filter((p) => !storageEntry(p.id))
    .map((p) => ({ id: p.id, label: p.name, group: UNKNOWN_GROUP[p.group], mark: <IconCloud /> }));
  const own: PickerItem = {
    id: OWN_RCLONE,
    label: "Rclone",
    sub: rclone.remotes.join(", ") || undefined,
    group: OWN_RCLONE,
    mark: RCLONE_MARK,
  };
  const items = [...offers.items, ...catalog.items, ...unknown, own];
  const groups: PickGroup[] = [
    ...(offers.group ? [offers.group] : []),
    { id: LOCAL, title: t("placement.segLocal"), hint: t("storage.add.localTip") },
    ...catalog.groups,
    { id: OWN_RCLONE, title: t("storage.add.ownRclone"), hint: t("storage.add.ownRcloneTip") },
  ];

  function onPick(id: string) {
    if (offers.pick(id)) return;
    if (id === OWN_RCLONE) {
      setOwnConf(true);
      return;
    }
    const p = draft.providers.find((provider) => provider.id === id);
    if (p) pick(p);
  }

  if (ownConf) {
    return (
      <OwnRcloneWindow remotes={rclone.remotes} onSaved={rclone.refresh} onBack={() => setOwnConf(false)} onClose={onClose} />
    );
  }

  const next = (disabled = false): WindowAction => ({
    label: t("zfs.replica.add.next"),
    labelKey: "zfs.replica.add.next",
    glyph: <IconForward />,
    primary: true,
    disabled,
    onClick: () => {
      go(shown + 1);
      return false;
    },
  });
  const { verdict, running } = draft.test;
  const forward: WindowAction =
    shown === 2
      ? {
          label: t("offsite.test"),
          labelKey: "offsite.test",
          primary: true,
          busy: running,
          disabled: running,
          tone: verdict ? (verdict.ok ? "ok" : "danger") : undefined,
          title: verdict ? t(verdictKey(verdict, "connection")) : undefined,
          onClick: () => {
            void draft.runTest().then((ok) => ok && go(3));
            return false;
          },
        }
      : shown === 3
        ? next(draft.folderOpen)
        : shown === 4
          ? next()
          : {
              label: t("storage.add.save"),
              labelKey: "storage.add.save",
              glyph: <IconSave />,
              primary: true,
              busy: placing,
              disabled: placing || draft.folderOpen,
              onClick: () => {
                void finish();
                return false;
              },
            };

  return (
    <PickerWindow
      title={t("storage.add.title")}
      hint={t("storage.listHint")}
      items={items}
      groups={groups}
      text={{
        search: t("storage.add.search"),
        none: t("storage.add.noMatch"),
        clear: t("settings.search.clear"),
        close: t("common.cancel"),
        cancel: t("common.cancel"),
        back: t("common.back"),
      }}
      selected={draft.fromMember ? receiverTile(draft.fromMember) : picked?.id}
      start={shown === 1 && draft.loadError ? <span className="text-statusFail">{draft.loadError}</span> : undefined}
      onPick={onPick}
      actions={shown === 1 ? [] : [forward]}
      onBack={() => setStep(shown - 1)}
      onClose={onClose}
    >
      {shown > 1 && (
        <div className="glim-form-rows flex flex-col gap-4">
          <Selector
            items={STEPS.map((key, i) => ({
              id: String(i + 1),
              label: t(key),
              icon: i + 1 < shown ? <IconCheck /> : <span aria-hidden="true" className="tabular-nums">{i + 1}</span>,
              disabled: i + 1 > reach,
            }))}
            label={t("storage.add.title")}
            select="one"
            activation="manual"
            fit
            hue={false}
            active={String(shown)}
            onChange={(id) => setStep(Number(id))}
          />
          {shown > 2 && (
            <div className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-card bg-carbon-surface2 px-4 py-3">
              <span className="inline-flex text-statusOk">
                <IconCheck />
              </span>
              <span className="min-w-0 flex-1 text-sm text-carbon-text wrap-break-word">
                <b className="font-semibold">{draft.name.trim() || label}</b>
                {" · "}
                {t(picked?.route === "path" ? "storage.localFolder" : "storage.add.connected")}
              </span>
              <Button label={t("storage.add.change")} labelKey="storage.add.change" glyph={<IconPencil />} tone="neutral" onClick={() => setStep(2)} />
            </div>
          )}
          {shown === 2 && (
            <StepCard title={t("storage.add.signIn").replace("{name}", () => label)}>
              <SignInBody draft={draft} providerLabel={label} />
            </StepCard>
          )}
          {shown === 3 && (
            <StepCard title={t("storage.folder")} tip={t(WORDS.folderTip)}>
              <FolderBody draft={draft} />
            </StepCard>
          )}
          {shown === 4 && (
            <>
              <StepCard title={t("storage.add.protect")}>
                <ProtectBody draft={draft} providerLabel={label} words={WORDS} />
              </StepCard>
              <StepCard title={t("storage.add.emergency")}>
                <RecoveryBody draft={draft} providerLabel={label} words={WORDS} />
              </StepCard>
            </>
          )}
          {shown === 5 && (
            <StepCard title={t("storage.add.sections")} tip={t("storage.add.sectionsHint")}>
              {choices.map(({ domain, locked, preview }, i) => {
                const left = preview?.formerlyExcluded.map((e) => itemName(e.identity)) ?? [];
                const on = !locked && !off.includes(domain);
                return (
                  <div key={domain} className="glim-hue flex items-center justify-between gap-4" style={hueVars(i) as CSSProperties}>
                    <div className="flex min-w-0 flex-col gap-0.5">
                      <span className="flex items-center gap-2 text-sm text-carbon-text">
                        <DomainGlyph domain={domain} />
                        {t(DOMAIN_LABEL[domain])}
                        {locked && <InfoBubble tip={locked} />}
                      </span>
                      {preview && on && (
                        <span className="text-xs text-carbon-textMuted">
                          {t("backups.tile.count", preview.items)} · {t("storage.add.snapshots", preview.snapshots)}
                        </span>
                      )}
                      {left.length > 0 && on && (
                        <span className="flex items-center gap-1.5 text-xs text-carbon-textMuted">
                          <span className="min-w-0 wrap-break-word">{t("storage.add.leftOut").replace("{list}", () => formatList(lang, left))}</span>
                          <InfoBubble tip={t("storage.add.leftOutHint")} />
                        </span>
                      )}
                    </div>
                    <Toggle
                      hideLabel
                      label={t(DOMAIN_LABEL[domain])}
                      checked={on}
                      disabled={!!locked || placing}
                      onChange={(next) => setOff((list) => (next ? list.filter((d) => d !== domain) : [...list, domain]))}
                    />
                  </div>
                );
              })}
            </StepCard>
          )}
        </div>
      )}
    </PickerWindow>
  );
}
