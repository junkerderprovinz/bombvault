// The card a new install keeps on its overview until the first backup has
// finished: the three steps, and what the next one asks for.
import { useCallback, useEffect, useState, type CSSProperties, type ReactNode } from "react";
import { Link, useNavigate } from "react-router-dom";

import { KIND_LABEL, KindGlyph, entryName } from "../../components/backups/kinds";
import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { IconForward } from "../../components/glyphs";
import { InfoBubble } from "../../components/InfoBubble";
import { IconAdd, IconPencil } from "../../components/navGlyphs";
import { StepMark } from "../../components/StepMark";
import { AddLocationWindow } from "../../components/storage/AddLocationWindow";
import { Rows } from "../../components/storage/rows";
import { Toggle } from "../../components/Toggle";
import { getSettings, putSettings, type Settings, type StorageLocation } from "../../lib/api";
import { hueVars } from "../../lib/appearance";
import type { EntryKind } from "../../lib/backupEntry";
import { entryTarget, protection, tilePath, waitingLabel } from "../../lib/backupList";
import {
  FOUND_KINDS,
  INCLUDE_FOUND,
  SECTIONS,
  kindsToInclude,
  scheduleChosen,
  switchSection,
  type FoundKind,
  type Section,
} from "../../lib/firstSections";
import { foundSentence, includedKinds, rememberIncluded, type Found, type StepState } from "../../lib/firstSteps";
import type { useT } from "../../lib/i18n";
import { formatList } from "../../lib/placement";
import type { Tally } from "../../lib/protectionTally";
import { flowDomains } from "../../lib/storageFlow";
import { useToast } from "../../lib/toast";
import type { BackupItems } from "../../lib/useBackupItems";
import { useFirstSteps } from "../../lib/useFirstSteps";
import { useIsDesktop } from "../../lib/useMediaQuery";
import { usePlacementDefaults } from "../../lib/usePlacementDefaults";
import { Card } from "./Card";

type T = ReturnType<typeof useT>["t"];

const SHOWN_WAITING = 4;

const HINT = "text-sm text-carbon-textSub";

/**
 * useSections reads which sections are switched on and saves what the first
 * step changes. A refused switch, such as VMs without a connection to the
 * host, goes back and says why.
 */
function useSections(t: T, items: BackupItems) {
  const { push } = useToast();
  const [settings, setSettings] = useState<Settings | null>(null);
  const [saving, setSaving] = useState(false);
  const [refused, setRefused] = useState<Partial<Record<EntryKind, number>>>({});

  const read = useCallback(() => {
    getSettings()
      .then((res) => {
        if (res.ok) setSettings(res.settings);
      })
      .catch(() => undefined);
  }, []);
  useEffect(() => {
    read();
    window.addEventListener("bv:settings-changed", read);
    return () => window.removeEventListener("bv:settings-changed", read);
  }, [read]);

  async function attempt(work: () => Promise<unknown>): Promise<boolean> {
    setSaving(true);
    try {
      await work();
      return true;
    } catch (err) {
      push((err instanceof Error && err.message) || t("settings.error"), "fail");
      return false;
    } finally {
      setSaving(false);
    }
  }

  // The whole object is written, so it is read first: a setting saved
  // elsewhere since the card was drawn must not be put back.
  async function write(change: (now: Settings) => Partial<Settings>): Promise<Settings> {
    const now = await getSettings();
    if (!now.ok) throw new Error();
    const patch = change(now.settings);
    if (Object.keys(patch).length === 0) return now.settings;
    const next = { ...now.settings, ...patch };
    const res = await putSettings(next);
    if (!res.ok) throw new Error(res.error);
    window.dispatchEvent(new Event("bv:settings-changed"));
    return next;
  }

  async function flip(section: Section, on: boolean) {
    setSettings((s) => s && { ...s, [section.enabled]: on });
    if (await attempt(() => write((now) => switchSection(now, section, on)))) return;
    setSettings((s) => s && { ...s, [section.enabled]: !on });
    setRefused((n) => ({ ...n, [section.kind]: (n[section.kind] ?? 0) + 1 }));
  }

  /** Gives the sections that are on their schedule and takes what was found
   *  into it, so the first backup and every later one hold it. */
  const prepare = () =>
    attempt(async () => {
      const now = await write(scheduleChosen);
      const kinds = kindsToInclude(now, items.reported, includedKinds());
      for (const kind of kinds) {
        const res = await INCLUDE_FOUND[kind]();
        if (!res.ok) throw new Error(res.error);
        rememberIncluded(kind);
      }
      if (kinds.length > 0) items.reload();
    });

  return { settings, saving, refused, flip, prepare };
}

function Step({
  n,
  state,
  title,
  summary,
  children,
  t,
}: {
  n: number;
  state: StepState;
  title: string;
  /** What a finished step came to. */
  summary?: string;
  /** What the step asks for, shown while it is the next one. */
  children: ReactNode;
  t: T;
}) {
  return (
    <li
      data-state={state}
      aria-current={state === "next" ? "step" : undefined}
      className="flex gap-3.5 border-t border-carbon-border py-3.5 first:border-t-0 first:pt-0 last:pb-0"
    >
      <StepMark n={n} state={state} />
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        <span className={`text-base font-semibold leading-7 ${state === "open" ? "text-carbon-textMuted" : "text-carbon-text"}`}>
          {title}
          {state === "done" && <span className="sr-only">{` (${t("common.done")})`}</span>}
        </span>
        {state === "done" && summary && <p className={HINT}>{summary}</p>}
        {state === "next" && children}
      </div>
    </li>
  );
}

function SectionSwitches({
  sections,
  settings,
  found,
  t,
}: {
  sections: ReturnType<typeof useSections>;
  settings: Settings;
  found: Found;
  t: T;
}) {
  const navigate = useNavigate();
  // A phone has no room for the word beside the switch.
  const isDesktop = useIsDesktop();
  const { saving, refused, flip, prepare } = sections;
  // The list can leave an entry out only once it is in the schedule.
  const choose = async (kind: FoundKind) => {
    if (await prepare()) navigate(tilePath(kind));
  };
  const pause = t("firstSteps.chooseHint").replace("{action}", () => t("schedule.pause"));
  const line: Partial<Record<EntryKind, string | false>> = {
    container: found.container > 0 && t("firstSteps.foundCount", found.container),
    vm: found.vm > 0 && t("firstSteps.foundCount", found.vm),
    config: t("firstSteps.ownSettings"),
  };

  return (
    <Rows>
      {SECTIONS.map((section, i) => {
        const choosable =
          settings[section.enabled] && FOUND_KINDS.find((kind) => kind === section.kind && found[kind] > 0);
        return (
          <div
            key={section.kind}
            data-section={section.kind}
            style={hueVars(i) as CSSProperties}
            className="glim-hue flex items-center gap-3 py-2.5 first:pt-0 last:pb-0"
          >
            <span className="glim-row-glyph flex h-[34px] w-[34px] flex-none items-center justify-center rounded-control bg-carbon-surface2 [&_svg]:h-[18px] [&_svg]:w-[18px]">
              <KindGlyph kind={section.kind} />
            </span>
            <div className="flex min-w-0 flex-1 flex-col">
              <span className="text-sm font-semibold text-carbon-text">{t(KIND_LABEL[section.kind])}</span>
              {line[section.kind] && <span className="text-subline text-carbon-textMuted">{line[section.kind]}</span>}
            </div>
            {choosable && (
              <Button
                label={t("firstSteps.choose")}
                labelKey="firstSteps.choose"
                title={pause}
                glyph={<IconPencil />}
                tone="subtle"
                variant={isDesktop ? "default" : "icon"}
                glyphOnly={!isDesktop}
                disabled={saving}
                onClick={() => void choose(choosable)}
              />
            )}
            <Toggle
              key={refused[section.kind]}
              hideLabel
              label={t(KIND_LABEL[section.kind])}
              checked={settings[section.enabled]}
              onChange={(on) => void flip(section, on)}
              disabled={saving}
              className={refused[section.kind] ? "glim-shake" : undefined}
            />
          </div>
        );
      })}
    </Rows>
  );
}

function Waiting({ tally, t }: { tally: Tally; t: T }) {
  const left = tally.entries.filter((item) => protection(item) !== "ok");
  const more = left.length - SHOWN_WAITING;
  if (left.length === 0) return null;
  return (
    <>
      <p className={`flex flex-wrap gap-x-3 gap-y-1 ${HINT}`}>
        {tally.pending > tally.replicaPending && (
          <span>{t("firstSteps.waiting", tally.pending - tally.replicaPending)}</span>
        )}
        {tally.replicaPending > 0 && <span>{t("firstSteps.waitingReplica", tally.replicaPending)}</span>}
        {tally.none > 0 && <span>{t("firstSteps.notProtected", tally.none)}</span>}
      </p>
      <Rows as="ul">
        {left.slice(0, SHOWN_WAITING).map((item, i) => (
          <li
            key={`${item.kind}:${item.key}`}
            data-testid="first-steps-entry"
            style={hueVars(i) as CSSProperties}
            className="glim-hue flex flex-wrap items-center gap-x-3 gap-y-1.5 py-2"
          >
            <span className="glim-row-glyph flex h-[34px] w-[34px] flex-none items-center justify-center rounded-control bg-carbon-surface2 [&_svg]:h-[18px] [&_svg]:w-[18px]">
              <KindGlyph kind={item.kind} />
            </span>
            <Link
              to={entryTarget(item)}
              className="min-w-0 flex-1 basis-24 truncate rounded-sm text-sm font-semibold text-carbon-text hover:underline focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-(--focus-ring)"
            >
              {entryName(item, t)}
            </Link>
            {protection(item) === "pending" ? (
              <Badge tone="neutral">{t(waitingLabel(item))}</Badge>
            ) : (
              <Badge tone="warn">{t("backups.status.notProtected")}</Badge>
            )}
          </li>
        ))}
      </Rows>
      {more > 0 && <p className={HINT}>{t("firstSteps.more", more)}</p>}
    </>
  );
}

function AddLocation({ locations, onClose }: { locations: StorageLocation[]; onClose: () => void }) {
  const defaults = usePlacementDefaults();
  return <AddLocationWindow domains={flowDomains(locations)} defaults={defaults.rows} onClose={onClose} />;
}

/**
 * FirstStepsCard lists the three steps with where each stands. The next one
 * shows what it asks for: the sections to switch on, the window that adds a
 * storage location, or what still waits for the first backup, which the
 * page's own action starts.
 */
export function FirstStepsCard({ tally, t, lang, hueIndex }: { tally: Tally; t: T; lang: string; hueIndex: number }) {
  const { states, found, items, locations, added, chooseSections } = useFirstSteps();
  const sections = useSections(t, items);
  const [adding, setAdding] = useState(false);
  const sentence = foundSentence(found, t, lang);
  const { settings } = sections;
  const switchedOn = SECTIONS.filter(({ enabled }) => settings?.[enabled]).map(({ kind }) => t(KIND_LABEL[kind]));
  const where = `${t("nav.settings")} › ${t("settings.schedule")}`;
  const next = async () => {
    if (await sections.prepare()) chooseSections();
  };

  return (
    <div data-testid="first-steps" className="flex">
      <Card title={t("firstSteps.title")} hint={t("firstSteps.hint")} hueIndex={hueIndex}>
        <ol className="flex flex-col">
          <Step n={1} state={states.sections} title={t("firstSteps.sections")} summary={formatList(lang, switchedOn)} t={t}>
            <p className={HINT}>
              {[sentence, t("firstSteps.sectionsHint")].filter(Boolean).join(" ")}{" "}
              <span className="inline-flex align-middle">
                <InfoBubble tip={t("firstSteps.nightly").replace("{where}", () => where)} />
              </span>
            </p>
            {settings && <SectionSwitches sections={sections} settings={settings} found={found} t={t} />}
            <div className="flex justify-end">
              <Button
                label={t("zfs.replica.add.next")}
                labelKey="zfs.replica.add.next"
                glyph={<IconForward />}
                disabled={sections.saving}
                onClick={() => void next()}
              />
            </div>
          </Step>
          <Step
            n={2}
            state={states.location}
            title={t("storage.add.title")}
            summary={formatList(lang, added.map((location) => location.name))}
            t={t}
          >
            <p className={HINT}>{t("firstSteps.locationHint")}</p>
            <div className="flex justify-end">
              <Button
                label={t("storage.add.title")}
                labelKey="storage.add.title"
                glyph={<IconAdd />}
                onClick={() => setAdding(true)}
              />
            </div>
          </Step>
          <Step n={3} state={states.backup} title={t("firstSteps.backup")} t={t}>
            <Waiting tally={tally} t={t} />
          </Step>
        </ol>
      </Card>
      {adding && <AddLocation locations={locations} onClose={() => setAdding(false)} />}
    </div>
  );
}
