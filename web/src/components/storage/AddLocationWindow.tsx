import { useEffect, useState, type CSSProperties } from "react";
import { useNavigate } from "react-router-dom";
import {
  destinationFolder,
  destinationForDomain,
  getNewTargetPreview,
  previewPlacementDefault,
  putPlacementDefault,
  type DefaultRow,
  type Destination,
  type OffsiteDomain,
  type PlacementDomain,
  type PlacementOptions,
  type Provider,
  type ProviderGroup,
  type TargetPreview,
} from "../../lib/api";
import { hueVars } from "../../lib/appearance";
import { useT, type TranslationKey } from "../../lib/i18n";
import { defaultChangeOf, defaultView, formatList, isPlacementDomain, lockHint, stepForNewTarget, viewHomeLabel } from "../../lib/placement";
import { tickDestination } from "../../lib/placementButtons";
import { placementChanged } from "../../lib/placementEvents";
import { DOMAIN_LABEL, locationRoute } from "../../lib/storageLocations";
import { useToast } from "../../lib/toast";
import { destinationsChanged } from "../../lib/useDestinations";
import { useLocalNames } from "../../lib/useLocalName";
import { offsiteTargetsChanged } from "../../lib/useOffsiteTargets";
import { usePlacementOptions } from "../../lib/usePlacementOptions";
import { useRcloneRemotes } from "../../lib/useRcloneRemotes";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Button } from "../Button";
import { GroupReceiverPicker } from "../destinations/GroupReceiverPicker";
import { ProviderMark, ProviderTiles, providerName, providerTileStyle } from "../destinations/ProviderPicker";
import { FolderBody, ProtectBody, RecoveryBody, SignInBody, type DestinationWords } from "../destinations/steps";
import { useDestinationDraft } from "../destinations/useDestinationDraft";
import { IconBack, IconForward, IconSave } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { IconCheck, IconClose, IconPencil } from "../navGlyphs";
import { PickerGroup, PickerNoMatch } from "../Picker";
import { itemName } from "../placement/NewTargetQuestion";
import { ReadmeButton } from "../ReadmeButton";
import { Selector } from "../Selector";
import { TestButton } from "../TestButton";
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

interface TileSection {
  id: string;
  title: TranslationKey;
  groups: ProviderGroup[];
  fit?: { label: TranslationKey; tone: "ok" | "warn" };
  tips: TranslationKey[];
}

// A folder on this host is a provider like the others to the server, and the
// one tile that needs no sign-in, so it stands apart and first.
const LOCAL: TileSection = { id: "local", title: "placement.segLocal", groups: [], tips: ["storage.add.localTip"] };
const SECTIONS: TileSection[] = [
  {
    id: "server",
    title: "storage.add.groupServer",
    groups: ["server", "selfs3"],
    fit: { label: "dest.fit.good", tone: "ok" },
    tips: ["dest.groupTip.server", "dest.groupTip.selfs3"],
  },
  {
    id: "s3",
    title: "storage.add.groupS3",
    groups: ["storage"],
    fit: { label: "dest.fit.best", tone: "ok" },
    tips: ["dest.groupTip.storage"],
  },
  {
    id: "cloud",
    title: "dest.group.cloud",
    groups: ["cloud"],
    fit: { label: "dest.fit.slower", tone: "warn" },
    tips: ["dest.groupTip.cloud"],
  },
];

function fold(text: string): string {
  return text.normalize("NFD").replace(/\p{Diacritic}/gu, "").toLowerCase();
}

interface SectionChoice {
  domain: OffsiteDomain;
  /** Why the new place cannot be switched on for the section. */
  locked?: string;
  preview?: TargetPreview;
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
  const [query, setQuery] = useState("");
  const [off, setOff] = useState<OffsiteDomain[]>([]);
  const [previews, setPreviews] = useState<Partial<Record<PlacementDomain, TargetPreview>>>({});
  const [placing, setPlacing] = useState(false);
  const [ownConf, setOwnConf] = useState(false);
  const rclone = useRcloneRemotes();
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

  async function place(made: Destination, domain: OffsiteDomain): Promise<boolean> {
    if (!isPlacementDomain(domain)) return (await destinationForDomain(made.id, domain)).ok;
    const row = defaults?.find((r) => r.domain === domain);
    const offered = options[domain];
    if (!row || !offered) return false;
    const ticked = await tickDestination(made.id, domain);
    if ("error" in ticked) return false;
    const change = defaultChangeOf(stepForNewTarget(ticked.targetId, defaultView(row, offered), offered));
    if (!change) return false;
    const preview = await previewPlacementDefault(domain, change);
    if (!preview.ok || !preview.impact) return false;
    return (await putPlacementDefault(domain, change, preview.impact)).ok;
  }

  async function finish() {
    setPlacing(true);
    try {
      const made = await draft.save();
      if (!made) return;
      const failed: OffsiteDomain[] = [];
      for (const { domain, locked } of choices) {
        if (locked || off.includes(domain)) continue;
        if (!(await place(made, domain).catch(() => false))) failed.push(domain);
      }
      destinationsChanged();
      offsiteTargetsChanged();
      placementChanged();
      if (failed.length > 0) {
        const list = formatList(lang, failed.map((domain) => t(DOMAIN_LABEL[domain])));
        push(t("storage.add.sectionsFailed").replace("{name}", () => made.name).replace("{list}", () => list), "warn");
      }
      onClose();
      navigate(locationRoute(`destination:${made.id}`), { replace: true });
    } finally {
      setPlacing(false);
    }
  }

  const q = fold(query.trim());
  const matches = (name: string) => q === "" || fold(name).includes(q);
  const tiles = [LOCAL, ...SECTIONS]
    .map((section) => ({
      section,
      providers: draft.providers
        .filter((p) => (section === LOCAL ? p.route === "path" : p.route !== "path" && section.groups.includes(p.group)))
        .filter((p) => matches(nameOf(p)))
        .sort((a, b) => section.groups.indexOf(a.group) - section.groups.indexOf(b.group)),
    }))
    .filter(({ providers }) => providers.length > 0);
  const ownShown = matches(t("storage.add.ownRclone")) || matches("rclone.conf");

  if (ownConf) {
    return (
      <OwnRcloneWindow remotes={rclone.remotes} onSaved={rclone.refresh} onBack={() => setOwnConf(false)} onClose={onClose} />
    );
  }

  const cancel = <Button label={t("common.cancel")} labelKey="common.cancel" glyph={<IconClose />} tone="neutral" onClick={onClose} />;
  const back = <Button label={t("common.back")} labelKey="common.back" glyph={<IconBack />} tone="neutral" onClick={() => setStep(shown - 1)} />;
  const next = (disabled = false) => (
    <Button
      label={t("zfs.replica.add.next")}
      labelKey="zfs.replica.add.next"
      glyph={<IconForward />}
      tone="accent"
      disabled={disabled}
      onClick={() => go(shown + 1)}
    />
  );
  const forward =
    shown === 2 ? (
      <TestButton
        label={t("offsite.test")}
        labelKey="offsite.test"
        tone="accent"
        test={draft.test}
        onClick={() => void draft.runTest().then((ok) => ok && go(3))}
      />
    ) : shown === 3 ? (
      next(draft.folderOpen)
    ) : shown === 4 ? (
      next()
    ) : (
      <Button
        label={t("storage.add.save")}
        labelKey="storage.add.save"
        glyph={<IconSave />}
        tone="accent"
        busy={placing}
        disabled={placing || draft.folderOpen}
        onClick={() => void finish()}
      />
    );

  return (
    <InstanceWindow
      title={t("storage.add.title")}
      hint={t("storage.listHint")}
      picker
      search={shown === 1 ? { value: query, onChange: setQuery, label: t("storage.add.search") } : undefined}
      onClose={onClose}
      footer={
        shown === 1 ? (
          cancel
        ) : (
          <>
            {cancel}
            {back}
            {forward}
          </>
        )
      }
    >
      {shown === 1 ? (
        <>
          {draft.loadError && <p className="text-xs text-statusFail">{draft.loadError}</p>}
          <GroupReceiverPicker
            picked={draft.fromMember}
            matches={matches}
            onPick={(r) => void draft.pickFromGroup(r).then((filled) => filled && go(2))}
          />
          {tiles.length > 0 && (
            <div data-new="provider-logos" className="flex flex-col gap-6">
              {tiles.map(({ section, providers }) => (
                <PickerGroup
                  key={section.id}
                  title={t(section.title)}
                  fit={section.fit && { label: t(section.fit.label), tone: section.fit.tone }}
                  tip={section.tips.map((tip) => t(tip)).join(" ")}
                >
                  <ProviderTiles providers={providers} picked={draft.fromMember ? undefined : picked?.id} onPick={pick} nameOf={nameOf} />
                </PickerGroup>
              ))}
            </div>
          )}
          {ownShown && (
            <PickerGroup title={t("storage.add.ownRclone")} tip={t("storage.add.ownRcloneTip")}>
              <ul className="glim-provider-pick">
                <li data-new="storage-add-own-rclone" style={providerTileStyle("rclone")}>
                  <ReadmeButton
                    tile="glim-tile-provider"
                    parts={[{ name: "Rclone", sub: rclone.remotes.join(", ") || undefined, onClick: () => setOwnConf(true) }]}
                    mark={<ProviderMark provider={{ mark: "rclone" }} />}
                    markClass="text-carbon-textSub"
                  />
                </li>
              </ul>
            </PickerGroup>
          )}
          <PickerNoMatch shown={tiles.length === 0 && !ownShown && q !== ""} text={t("storage.add.noMatch")} />
        </>
      ) : (
        <div className="flex flex-col gap-4">
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
    </InstanceWindow>
  );
}
