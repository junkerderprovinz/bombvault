import { useState } from "react";

import { previewRetention, type OffsiteDomain, type RetentionPreview as Preview } from "../lib/api";
import { dbDumpNameOf, isDbDumpIdentity } from "../lib/dbdump";
import { useT } from "../lib/i18n";

import { Button } from "./Button";
import { SelectField } from "./SelectField";

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
 *  to ask about at all. */
type Part = { preview?: Preview; error?: string; none?: boolean };

const SOURCES = [
  ["local", "source.local"],
  ["offsite", "source.offsite"],
] as const;

/**
 * Shows what the next retention run would remove from a source's local and
 * built-in off-site copies, each beside the policy that decides it.
 *
 * It asks only when the button is pressed, for two reasons. The cheap one: the
 * Settings page's own tests mock the API module by spreading the real one, so a
 * fetch fired during render would escape to real fetch under jsdom. The real
 * one: the answer costs one restic invocation per item per repository, which on
 * a large domain over a cloud backend is minutes. That is a question an
 * operator asks on purpose, not something a page polls.
 */
export function RetentionPreview({
  t,
  hasOffsite,
}: {
  t: ReturnType<typeof useT>["t"];
  /** Whether the domain has a built-in off-site repository to ask about. */
  hasOffsite: (domain: Domain) => boolean;
}) {
  const [domain, setDomain] = useState<Domain>("containers");
  const [busy, setBusy] = useState(false);
  const [parts, setParts] = useState<Record<Source, Part> | null>(null);

  async function ask(source: Source): Promise<Part> {
    if (source === "offsite" && !hasOffsite(domain)) return { none: true };
    try {
      const res = await previewRetention(domain, source === "offsite" ? "offsite" : undefined);
      // The server's own sentence, shown as it came. A refusal like "no
      // backups yet" is the answer, not a failure to report generically.
      if (!res.ok) return { error: res.error || t("retentionPreview.failed") };
      return { preview: res.preview };
    } catch (e) {
      return { error: e instanceof Error ? e.message : t("retentionPreview.failed") };
    }
  }

  async function run() {
    setBusy(true);
    setParts(null);
    const [local, offsite] = await Promise.all([ask("local"), ask("offsite")]);
    setParts({ local, offsite });
    setBusy(false);
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-end gap-2">
        <label className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textSub">{t("common.domain")}</span>
          <SelectField
            value={domain}
            label={t("common.domain")}
            options={DOMAINS.map((d) => ({ value: d.key, label: t(d.labelKey) }))}
            onChange={(next) => {
              setDomain(next);
              // A stale answer under a new domain name would be worse than no
              // answer: it would read as that domain's verdict.
              setParts(null);
            }}
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
          />
        </label>
        <Button
          label={t("retentionPreview.show")}
          labelKey="retentionPreview.show"
          tone="neutral"
          busy={busy}
          onClick={() => void run()}
          className="glim-btn-wrap"
        />
      </div>

      {parts &&
        SOURCES.map(([source, titleKey]) => (
          <section key={source} aria-label={t(titleKey)} className="flex flex-col gap-2">
            <h4 className="text-xs font-semibold text-carbon-textSub uppercase tracking-widest">{t(titleKey)}</h4>
            <PartView part={parts[source]} t={t} />
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
      {preview.policy.on && <p className="text-xs text-carbon-textSub">{policyLine(preview.policy, t)}</p>}

      {!preview.policy.on && <p className="text-sm text-carbon-textSub">{t("retentionPreview.off")}</p>}

      {preview.policy.on && removalCount === 0 && (
        <p className="text-sm text-carbon-textSub">{t("retentionPreview.nothing")}</p>
      )}

      {preview.repos.map((repo) => (
        <div key={repo.name} className="rounded-control bg-carbon-surface2 px-3 py-2">
          <div className="flex flex-wrap items-center justify-between gap-x-2">
            <span className="text-sm text-carbon-text">{repo.name}</span>
            {repo.appendOnly && (
              <span className="text-xs text-carbon-textSub">{t("retentionPreview.appendOnly")}</span>
            )}
          </div>

          {repo.error && <p className="text-sm text-statusFail">✗ {repo.error}</p>}

          {repo.items.map((item) => (
            <div key={item.tag || "__repo"} className="mt-2">
              <div className="text-xs text-carbon-textSub">
                {itemLabel(item.tag, t)} · {t("retentionPreview.keeps")}: {item.keep?.length ?? 0}
              </div>
              {item.paused && <p className="text-xs text-statusWarn">{t("retentionPreview.paused")}</p>}
              {(item.remove?.length ?? 0) > 0 && (
                <ul className="mt-1 flex flex-col gap-0.5">
                  {item.remove?.map((snap) => (
                    <li key={snap.id} className="text-sm text-carbon-text">
                      {snap.id.slice(0, 8)} · {stamp(snap.time)}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          ))}
        </div>
      ))}

      {preview.skipped?.length ? (
        <div className="rounded-control bg-carbon-surface2 px-3 py-2">
          <p className="text-xs text-statusWarn">{t("retentionPreview.skipped")}</p>
          <ul className="mt-1 flex flex-col gap-0.5">
            {preview.skipped.map((s) => (
              <li key={s} className="text-sm text-carbon-textSub">
                {s}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </>
  );
}
