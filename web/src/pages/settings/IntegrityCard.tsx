// IntegrityCard is a row per section with the four checks the overview
// measures it by, and a menu with what can be run on its backups by hand: a
// check of the repositories here and off site, a trial restore, the test of
// the delete protection, unlocking and pruning.
import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { ActionMenu, type MenuAction } from "../../components/backups/ActionMenu";
import { KIND_LABEL, KindGlyph } from "../../components/backups/kinds";
import { Button } from "../../components/Button";
import { IconPrune, IconShieldOn, IconUnlock } from "../../components/glyphs";
import { IconCheck } from "../../components/Sidebar";
import { IconCloud, IconMore, IconRestore } from "../../components/navGlyphs";
import { ProgressBar } from "../../components/ProgressBar";
import { Rows } from "../../components/storage/rows";
import { countProgressText, type ResolveName } from "../../lib/activityLog";
import { heldTagLabels } from "../../lib/anomalies";
import {
  checkDomain,
  getStatus,
  listItems,
  pruneDomain,
  runDrill,
  tamperTest,
  unlockDomain,
  type BackupItem,
  type DomainStatus,
} from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { STALE_MS, useProgress, type ProgressState } from "../../lib/progress";
import { deleteSentence } from "../../lib/secretsRefusal";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useOwnLogin } from "../../lib/useOwnLogin";
import { useTipBubble } from "../../lib/useTipBubble";
import { CHECK_LABEL, canDrill, drilledText, sectionsOf, type Check, type Section } from "../overview/protectionChecks";
import { TONE_DOT } from "../overview/ProtectionCard";
import { Card } from "./shared";

type T = ReturnType<typeof useT>["t"];
type Action = "verify" | "verifyOffsite" | "drill" | "tamper" | "unlock" | "prune";

type Domain = "containers" | "vms" | "flash" | "files" | "zfs";

// The self-backup has no repository of its own to check here.
const CHECKED: ReadonlySet<string> = new Set<Domain>(["containers", "vms", "flash", "files", "zfs"]);

function CheckDot({ check, t }: { check: Check; t: T }) {
  const label = t(CHECK_LABEL[check.key]);
  const say = t(check.say);
  const tip = useTipBubble(say);
  return (
    <span className="relative flex">
      <span
        ref={tip.ref}
        tabIndex={0}
        aria-label={say}
        aria-describedby={tip.describedBy}
        data-check={check.key}
        data-tone={check.tone}
        {...tip.handlers}
        className="inline-flex items-center gap-1.5 rounded-control text-xs text-carbon-textSub focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-(--focus-ring)"
      >
        <i aria-hidden="true" className={`block h-2 w-2 rounded-full ${TONE_DOT[check.tone]}`} />
        {label}
      </span>
      {tip.bubble}
    </span>
  );
}

function RowMenu({ label, actions }: { label: string; actions: MenuAction[] }) {
  const { t } = useT();
  const anchor = useRef<HTMLSpanElement>(null);
  const [open, setOpen] = useState(false);
  return (
    <>
      <span ref={anchor} className="flex">
        <Button
          label={t("nav.more")}
          labelKey="nav.more"
          glyph={<IconMore />}
          ariaExpanded={open}
          onClick={() => setOpen((was) => !was)}
        />
      </span>
      {open && anchor.current && (
        <ActionMenu label={label} actions={actions} place={{ anchor: anchor.current }} onClose={() => setOpen(false)} />
      )}
    </>
  );
}

// unlockBoth clears the stale locks here and, when there is one, off site,
// where an interrupted copy leaves its lock.
async function unlockBoth(domain: Domain, offsite: boolean) {
  const here = await unlockDomain(domain, "local");
  if (!here.ok || !offsite) return here;
  const away = await unlockDomain(domain, "offsite");
  return { ...away, skipped: [...(here.skipped ?? []), ...(away.skipped ?? [])] };
}

export function IntegrityCard({ t, hueIndex }: { t: T; hueIndex?: number }) {
  const progressMap = useProgress();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const ownLogin = useOwnLogin();
  const [domains, setDomains] = useState<DomainStatus[]>([]);
  const [items, setItems] = useState<BackupItem[]>([]);
  const [busy, setBusy] = useState<Record<string, boolean>>({});

  const resolveName: ResolveName = (key, params, count) => {
    let s = t(key as TranslationKey, count);
    for (const [name, value] of Object.entries(params ?? {})) s = s.split(`{${name}}`).join(value);
    return s;
  };

  const reload = useCallback(() => {
    getStatus()
      .then((r) => {
        if (r.ok && r.domains) setDomains(r.domains);
      })
      .catch(() => undefined);
    listItems()
      .then((r) => {
        if (r.ok) setItems(r.items ?? []);
      })
      .catch(() => undefined);
  }, []);
  useEffect(reload, [reload]);

  // A run keeps sending events, if only a repeat of its last one, so a bar
  // that has heard nothing for STALE_MS belongs to a run that died, as when
  // BombVault restarted under it. The clock ticks while a bar shows, because a
  // dead run sends nothing that would re-render the card.
  const [now, setNow] = useState(() => Date.now());
  const liveKeys = Object.keys(progressMap)
    .filter((k) => /^(verify|prune|drill):/.test(k) && progressMap[k].active && !progressMap[k].finished)
    .sort()
    .join("|");
  useEffect(() => {
    if (!liveKeys) return;
    const id = window.setInterval(() => setNow(Date.now()), 5000);
    return () => window.clearInterval(id);
  }, [liveKeys]);

  // A run that ends while the card is open, started here or anywhere else,
  // has just changed what the dots say.
  const [prevLive, setPrevLive] = useState(liveKeys);
  const [ended, setEnded] = useState(0);
  if (prevLive !== liveKeys) {
    const gone = prevLive.split("|").some((k) => k && !liveKeys.split("|").includes(k));
    setPrevLive(liveKeys);
    if (gone) setEnded((n) => n + 1);
  }
  useEffect(() => {
    if (ended > 0) reload();
  }, [ended, reload]);

  function runningOf(domain: string): { state: ProgressState; label: string } | null {
    const ops: [string, TranslationKey][] = [
      [`verify:${domain}`, "integrity.checking"],
      [`prune:${domain}`, "integrity.pruning"],
      [`drill:${domain}`, "verify.running"],
    ];
    for (const [key, labelKey] of ops) {
      const state = progressMap[key];
      if (state?.active && !state.finished && Math.max(now, Date.now()) - state.lastSeen <= STALE_MS) {
        return { state, label: t(labelKey) };
      }
    }
    return null;
  }

  async function run(section: Section, action: Action) {
    const domain = section.status.domain as Domain;
    const name = t(KIND_LABEL[section.kind]);
    const say = (key: TranslationKey) => t(key).replace("{section}", name);
    if (action === "prune" && !(await confirm(say("integrity.pruneAsk"), { confirmKey: "integrity.prune" }))) return;
    if (action === "drill" && !(await confirm(t("verify.trialConfirm"), { confirmKey: "verify.trialRestore" }))) return;
    const key = `${domain}:${action}`;
    setBusy((b) => ({ ...b, [key]: true }));
    try {
      if (action === "tamper") {
        const r = await tamperTest(domain);
        if (!r.ok) push(r.error ?? t("offsite.tamperError"), "fail");
        else if (!r.testable) push(`${name}: ${t("offsite.tamperUnverifiable")}`, "warn");
        else if (r.protected) push(say("integrity.guardHeld"), "success");
        else push(r.detail ? `${say("integrity.guardFailed")} ${r.detail}` : say("integrity.guardFailed"), "fail");
      } else if (action === "drill") {
        const r = await runDrill(domain, "offsite", "dr");
        if (r.ok && r.drill?.ok) {
          const drilled = drilledText(t, r.drill.item, r.drill.snapshotId);
          push(drilled ? `${say("integrity.trialPassed")} (${drilled})` : say("integrity.trialPassed"), "success");
        }
        else push(r.drill?.detail || r.error || t("verify.failed"), "fail");
      } else {
        const r =
          action === "verify" ? await checkDomain(domain, "local")
          : action === "verifyOffsite" ? await checkDomain(domain, "offsite")
          : action === "unlock" ? await unlockBoth(domain, section.status.offsiteConfigured)
          : await pruneDomain(domain, "local");
        // A repository shared with another domain gets only its stale locks
        // cleared, so unlock can report either outcome and still have changed
        // nothing there.
        const skipped: string[] = "skipped" in r && Array.isArray(r.skipped) ? r.skipped : [];
        if (skipped.length) push(t("integrity.unlockPartial").replace("{list}", skipped.join(", ")), "warn");
        const paused: string[] = "paused" in r && Array.isArray(r.paused) ? r.paused : [];
        if (r.ok && paused.length) push(t("anomaly.prunePaused").replace("{names}", heldTagLabels(paused, t).join(", ")), "warn");
        if (!r.ok) {
          if (!ownLogin.handled(r)) push(deleteSentence(t, r.code) ?? r.error ?? t("integrity.failed"), "fail");
        }
        else if (action === "unlock") push(say("integrity.unlocked"), "success");
        else if (action === "prune") push(say("integrity.pruned"), "success");
        else push(say("integrity.passed"), "success");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("integrity.failed"), "fail");
    } finally {
      setBusy((b) => ({ ...b, [key]: false }));
      // The verdicts land in /api/status, which the overview reads too.
      window.dispatchEvent(new Event("bv:settings-changed"));
      reload();
    }
  }

  function actionsOf(section: Section): MenuAction[] {
    const d = section.status;
    const item = (id: Action, label: TranslationKey, glyph: ReactNode): MenuAction => ({
      id,
      label: t(label),
      glyph,
      onSelect: () => {
        if (!busy[`${d.domain}:${id}`]) void run(section, id);
      },
    });
    const list = [item("verify", "integrity.checkReadable", <IconCheck />)];
    if (d.offsiteConfigured) list.push(item("verifyOffsite", "integrity.checkOffsite", <IconCloud />));
    if (d.offsiteConfigured && canDrill(d.domain)) list.push(item("drill", "verify.trialRestore", <IconRestore />));
    if (d.offsiteConfigured && d.offsiteImmutable) list.push(item("tamper", "verify.tamper", <IconShieldOn />));
    list.push(item("unlock", "integrity.unlock", <IconUnlock />), item("prune", "integrity.prune", <IconPrune />));
    return list;
  }

  const sections = sectionsOf(domains, items, progressMap, null).filter((s) => CHECKED.has(s.status.domain));

  return (
    <Card title={t("integrity.checkTitle")} hint={t("integrity.checkHint")} hueIndex={hueIndex}>
      <Rows as="ul">
        {sections.map((section) => {
          const live = runningOf(section.status.domain);
          const name = t(KIND_LABEL[section.kind]);
          return (
            <li key={section.kind} data-domain={section.status.domain} className="flex flex-col gap-2 py-2.5 first:pt-0 last:pb-0">
              <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
                <span className="flex min-w-0 flex-1 items-center gap-2.5 text-sm font-semibold text-carbon-text">
                  <span aria-hidden="true" className="inline-flex text-accentText [&_svg]:h-4 [&_svg]:w-4">
                    <KindGlyph kind={section.kind} />
                  </span>
                  {name}
                </span>
                <span className="order-last flex w-full flex-wrap items-center gap-x-4 gap-y-1.5 md:order-none md:w-auto">
                  {section.checks.map((check) => (
                    <CheckDot key={check.key} check={check} t={t} />
                  ))}
                </span>
                <RowMenu label={name} actions={actionsOf(section)} />
              </div>
              {live && (
                <ProgressBar
                  inline
                  active
                  percent={live.state.total ? live.state.percent : 0}
                  label={[live.label, countProgressText(resolveName, live.state)].filter(Boolean).join(" ")}
                />
              )}
            </li>
          );
        })}
      </Rows>
      {confirmDialog}
      {ownLogin.dialog}
    </Card>
  );
}
