import { useId, useState } from "react";

import { listOffsiteTargets, previewRetention, type OffsiteDomain, type RetentionPreview as Preview } from "../lib/api";
import { dbDumpNameOf, isDbDumpIdentity } from "../lib/dbdump";
import { useT } from "../lib/i18n";

import { Button } from "./Button";
import { SelectField } from "./SelectField";
import { Rows, SettingRow } from "./storage/rows";

type Domain = OffsiteDomain;

// `as const` so each labelKey keeps its literal type: t() only accepts known
// keys, which is what stops a typo here from reaching a user as a raw key.
const DOMAINS = [
  { key: "containers", labelKey: "settings.containersEnabled" },
  { key: "vms", labelKey: "settings.vmsEnabled" },
  { key: "flash", labelKey: "settings.flashEnabled" },
  { key: "config", labelKey: "settings.configEnabled" },
  { key: "files", labelKey: "settings.filesEnabled" },
  { key: "zfs", labelKey: "settings.zfsEnabled" },
] as const satisfies readonly { key: Domain; labelKey: string }[];

/** The group's name: its identity tag, the dumps of a container in words, or
 *  the whole-repository fallback the legacy pass has no tag for. */
function itemLabel(tag: string, t: ReturnType<typeof useT>["t"]): string {
  if (isDbDumpIdentity(tag)) return t("dbdump.retentionItem").replace("{name}", dbDumpNameOf(tag));
  return tag || t("retentionPreview.wholeRepo");
}

const RULES = [
  ["keepLast", "settings.retentionLast"],
  ["keepDaily", "settings.retentionDaily"],
  ["keepWeekly", "settings.retentionWeekly"],
  ["keepMonthly", "settings.retentionMonthly"],
  ["keepYearly", "settings.retentionYearly"],
] as const;

/** The policy the answer was worked out with and the rules it sets, so a
 *  source's own policy cannot be mistaken for the shared one. */
function policyLine(policy: Preview["policy"], t: ReturnType<typeof useT>["t"]): string {
  const name = policy.own ? t("retentionPreview.ownPolicy") : t("retentionPreview.sharedPolicy");
  const rules = RULES.filter(([key]) => policy[key] > 0).map(([key, label]) => `${t(label)} ${policy[key]}`);
  return `${name}: ${rules.join(" · ")}`;
}

/** A short, readable stamp: the date and time, without the timezone tail. */
function stamp(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString();
}

type Source = "local" | "offsite";

/** One copy's answer: the preview, the server's refusal, or no off-site copy
 *  to ask about at all. A copy goes by its name, as a section can have several. */
type Part = { preview?: Preview; error?: string; none?: boolean; name?: string };

const SOURCES = [
  ["local", "source.local"],
  ["offsite", "source.offsite"],
] as const;

/**
 * Shows what the next retention run would remove from a source's local folder
 * and from each of its off-site copies, each beside the policy that decides it.
 *
 * It asks only when the button is pressed, for two reasons. The cheap one: the
 * Settings page's own tests mock the API module by spreading the real one, so a
 * fetch fired during render would escape to real fetch under jsdom. The real
 * one: the answer costs one restic invocation per item per repository, which on
 * a large domain over a cloud backend is minutes. That is a question an
 * operator asks on purpose, not something a page polls.
 */
export function RetentionPreview({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const sourceId = useId();
  const [domain, setDomain] = useState<Domain>("containers");
  const [busy, setBusy] = useState(false);
  const [parts, setParts] = useState<Record<Source, Part[]> | null>(null);

  async function ask(source?: string): Promise<Part> {
    try {
      const res = await previewRetention(domain, source);
      // The server's own sentence, shown as it came. A refusal like "no
      // backups yet" is the answer, not a failure to report generically.
      if (!res.ok) return { error: res.error || t("retentionPreview.failed") };
      return { preview: res.preview };
    } catch (e) {
      return { error: e instanceof Error ? e.message : t("retentionPreview.failed") };
    }
  }

  async function askCopies(): Promise<Part[]> {
    try {
      const res = await listOffsiteTargets(domain);
      if (!res.ok) return [{ error: res.error || t("retentionPreview.failed") }];
      const copies = (res.targets ?? []).filter((target) => target.enabled);
      if (copies.length === 0) return [{ none: true }];
      return await Promise.all(copies.map(async (target) => ({ ...(await ask(`offsite:${target.id}`)), name: target.name || target.repo })));
    } catch (e) {
      return [{ error: e instanceof Error ? e.message : t("retentionPreview.failed") }];
    }
  }

  async function run() {
    setBusy(true);
    setParts(null);
    const [local, offsite] = await Promise.all([ask(), askCopies()]);
    setParts({ local: [local], offsite });
    setBusy(false);
  }

  return (
    <div className="flex flex-col gap-3">
      <SettingRow label={t("retentionPreview.sourceLabel")} htmlFor={sourceId}>
        <SelectField
          id={sourceId}
          value={domain}
          label={t("retentionPreview.sourceLabel")}
          options={DOMAINS.map((d) => ({ value: d.key, label: t(d.labelKey) }))}
          onChange={(next) => {
            setDomain(next);
            // A stale answer under a new domain name would be worse than no
            // answer: it would read as that domain's verdict.
            setParts(null);
          }}
          className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
        />
        <Button
          label={t("retentionPreview.show")}
          labelKey="retentionPreview.show"
          tone="neutral"
          busy={busy}
          onClick={() => void run()}
          className="glim-btn-wrap"
        />
      </SettingRow>

      {parts &&
        SOURCES.map(([source, titleKey]) => (
          <section key={source} aria-label={t(titleKey)} className="flex flex-col gap-2">
            <h4 className="text-xs font-semibold text-carbon-textSub uppercase tracking-widest">{t(titleKey)}</h4>
            {parts[source].map((part, i) => (
              <PartView key={part.name ?? i} part={part} t={t} />
            ))}
          </section>
        ))}
    </div>
  );
}

function PartView({ part, t }: { part: Part; t: ReturnType<typeof useT>["t"] }) {
  if (part.none) return <p className="text-sm text-carbon-textSub">{t("dashboard.noOffsite")}</p>;
  if (part.error) return <p className="text-sm text-statusFail">✗ {part.error}</p>;
  const preview = part.preview;
  if (!preview) return null;

  const removalCount = preview.repos.reduce(
    (n, r) => n + r.items.reduce((m, i) => m + (i.remove?.length ?? 0), 0),
    0
  );

  return (
    <>
      {part.name && <p className="text-sm font-medium text-carbon-text">{part.name}</p>}
      {preview.policy.on && <p className="text-xs text-carbon-textSub">{policyLine(preview.policy, t)}</p>}

      {!preview.policy.on && <p className="text-sm text-carbon-textSub">{t(preview.policy.own ? "retentionPreview.offOwn" : "retentionPreview.off")}</p>}

      {preview.policy.on && removalCount === 0 && (
        <p className="text-sm text-carbon-textSub">{t("retentionPreview.nothing")}</p>
      )}

      <Rows>
        {preview.repos.map((repo) => (
          <SettingRow
            key={repo.name}
            label={repo.name}
            note={
              (repo.error || repo.items.length > 0) && (
                <>
                  {repo.error && <span className="text-sm text-statusFail">✗ {repo.error}</span>}
                  {repo.items.map((item) => (
                    <span key={item.tag || "__repo"} className="mt-1 flex flex-col gap-0.5">
                      <span className="text-carbon-textSub">
                        {itemLabel(item.tag, t)} · {t("retentionPreview.keeps")}: {item.keep?.length ?? 0}
                      </span>
                      {item.paused && <span className="text-statusWarn">{t("retentionPreview.paused")}</span>}
                      {(item.remove?.length ?? 0) > 0 && (
                        <ul className="flex flex-col gap-0.5">
                          {item.remove?.map((snap) => (
                            <li key={snap.id} className="text-sm text-carbon-text">
                              {snap.id.slice(0, 8)} · {stamp(snap.time)}
                            </li>
                          ))}
                        </ul>
                      )}
                    </span>
                  ))}
                </>
              )
            }
          >
            {repo.appendOnly && <span className="text-xs text-carbon-textSub">{t("retentionPreview.appendOnly")}</span>}
          </SettingRow>
        ))}

        {preview.skipped?.length ? (
          <SettingRow
            label={t("retentionPreview.skipped")}
            name={<span className="text-statusWarn">{t("retentionPreview.skipped")}</span>}
            note={
              <ul className="flex flex-col gap-0.5">
                {preview.skipped.map((s) => (
                  <li key={s} className="text-sm text-carbon-textSub">
                    {s}
                  </li>
                ))}
              </ul>
            }
          />
        ) : null}
      </Rows>
    </>
  );
}
