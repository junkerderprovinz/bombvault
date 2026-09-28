// Pull fetches backups out of another BombVault's repository into this one. It
// follows Receiver.tsx, with one difference: a received repository is only
// read, while a pull writes to this disk. So a source card leads with its last
// result rather than a live probe, and removing a source touches neither
// repository.
import { useEffect, useState, type CSSProperties } from "react";
import { createPortal } from "react-dom";
import {
  listPullSources,
  createPullSource,
  updatePullSource,
  deletePullSource,
  testPullSource,
  runPullSource,
} from "../lib/api";
import type { PullSourceView, PullSourceInput } from "../lib/api";
import { useT } from "../lib/i18n";
import { PAGE_SHELL_RESPONSIVE, PAGE_SHELL_TABBED_RESPONSIVE } from "../lib/pageShell";
import { relativeTime } from "../lib/reltime";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { NumberField } from "../components/NumberField";
import { PageTitle } from "../components/PageTitle";
import { IconReceiver } from "../components/Sidebar";
import { Badge } from "../components/Badge";
import { InfoBubble } from "../components/InfoBubble";
import { MemberField } from "./instances/MemberField";
import { SelectField } from "../components/SelectField";
import { useCloudCredSets } from "../lib/useCloudCredSets";
import { useToast } from "../lib/toast";
import { hueVars } from "../lib/appearance";
import { Button } from "../components/Button";
import { TestButton, VerdictLine } from "../components/TestButton";
import { useTestVerdict } from "../lib/useTestVerdict";
import { ToggleRow } from "./settings/shared";

type T = ReturnType<typeof useT>["t"];

const inputCls =
  "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus";

/** Which local repository a source's snapshots land in. The same ones the
 *  backup side knows; the server rejects anything else. */
const DOMAINS = ["containers", "vms", "files", "zfs", "flash", "config"] as const;
type PullDomain = (typeof DOMAINS)[number];

function PullSourceCard({
  source,
  t,
  index,
  onRefresh,
  onEdit,
}: {
  source: PullSourceView;
  t: T;
  index: number;
  onRefresh: () => void;
  onEdit: () => void;
}) {
  const { push } = useToast();
  const [busy, setBusy] = useState(false);
  const [confirmRemove, setConfirmRemove] = useState(false);

  async function handlePull() {
    setBusy(true);
    try {
      const res = await runPullSource(source.id);
      if (res.ok) {
        const n = res.snapshots ?? 0;
        push(n === 0 ? t("pull.nothingNew") : t("pull.pulled", n), "success");
      } else {
        push(res.error ?? t("pull.pullFailed"), "fail");
      }
    } finally {
      setBusy(false);
      onRefresh();
    }
  }

  const test = useTestVerdict([source.repo, source.credsRef, source.domain], t("pull.pullFailed"));
  function handleTest() {
    void test.run(async () => {
      const res = await testPullSource(source.id);
      return res.ok ? { ok: true } : { ok: false, reason: res.error ?? t("pull.pullFailed") };
    });
  }

  async function handleRemove() {
    await deletePullSource(source.id).catch(() => undefined);
    onRefresh();
  }

  // A pull is something that happened, so the badge reports the last result
  // rather than a probe run now.
  const verdict = !source.enabled
    ? { label: t("pull.pullingOff"), tone: "neutral" as const }
    : source.lastPullOk === null
      ? { label: t("pull.neverPulled"), tone: "neutral" as const }
      : source.lastPullOk
        ? { label: t("pull.pullOk"), tone: "ok" as const }
        : { label: t("pull.pullFailed"), tone: "fail" as const };

  return (
    <div
      className="relative glim-notch-card glim-hue bg-carbon-surface rounded-card p-4 flex flex-col gap-3"
      style={hueVars(index) as CSSProperties}
    >
      <div className="flex items-start justify-between gap-3 flex-wrap">
        <div className="min-w-0 max-md:basis-full">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-medium text-carbon-text">{source.name}</span>
            <Badge tone={verdict.tone}>{verdict.label}</Badge>
            <Badge tone="neutral">{t(`nav.${source.domain}` as never)}</Badge>
            {source.needsPairing && (
              <Badge tone="warn">
                {t("pairing.pairAgain")}
                <InfoBubble tip={t("pairing.pairAgainTip")} onAccent />
              </Badge>
            )}
          </div>
          {/* A repository address is not prose; an RTL interface must not
              reorder it. */}
          <p className="mt-1 text-xs text-carbon-textMuted break-all" dir="ltr">
            {source.repo}
          </p>
        </div>
        <div className="text-end text-xs text-carbon-textMuted shrink-0 max-md:text-start">
          <div>
            {t("pull.lastPull")}:{" "}
            {source.lastPullAt === 0 ? "-" : relativeTime(t, source.lastPullAt)}
          </div>
          {source.snapshotsPulled > 0 && (
            <div className="glim-num">
              {t("pull.snapshotsPulled", source.snapshotsPulled)}
            </div>
          )}
        </div>
      </div>

      {source.lastPullOk === false && source.lastPullError !== "" && (
        <p className="text-xs text-statusFail wrap-break-word">{source.lastPullError}</p>
      )}

      <VerdictLine verdict={test.verdict} />

      <div className="flex items-center gap-2 flex-wrap">
        <Button
          label={t("pull.pullNow")}
          labelKey="pull.pullNow"
          tone="accent"
          onClick={() => void handlePull()}
          disabled={busy || test.running || !source.enabled}
          busy={busy}
          hueIndex={index}
        />
        <TestButton
          label={t("offsite.test")}
          labelKey="offsite.test"
          test={test}
          onClick={handleTest}
          disabled={busy}
          hueIndex={index}
        />
        <Button
          label={t("common.edit")}
          labelKey="common.edit"
          tone="neutral"
          onClick={onEdit}
          hueIndex={index}
        />
        {confirmRemove ? (
          <Button
            label={t("offsite.targets.confirmRemove")}
            labelKey="offsite.targets.confirmRemove"
            tone="neutral"
            onClick={() => void handleRemove()}
            hueIndex={index}
          />
        ) : (
          <Button
            label={t("receiver.remove")}
            labelKey="receiver.remove"
            tone="neutral"
            onClick={() => setConfirmRemove(true)}
            hueIndex={index}
          />
        )}
      </div>
    </div>
  );
}

function PullDialog({
  initial,
  t,
  onClose,
  onSaved,
}: {
  /** null = create; a row = edit that source. */
  initial: PullSourceView | null;
  t: T;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { push } = useToast();
  const [name, setName] = useState(initial?.name ?? "");
  const [repo, setRepo] = useState(initial?.repo ?? "");
  const [memberId, setMemberId] = useState("");
  const [domain, setDomain] = useState<PullDomain>((initial?.domain as PullDomain) ?? "containers");
  const [cadence, setCadence] = useState(initial?.cadence ?? "");
  // The login for the storage the source repository lies on; the restic
  // password only opens the repository itself. pull.go withholds this box's
  // own credentials, so an s3:, b2: or authenticated rest: source needs a set
  // here, and without one the refusal misleadingly names the password.
  const [credsRef, setCredsRef] = useState(initial?.credsRef ?? "");
  const credSets = useCloudCredSets();
  const [limitDownload, setLimitDownload] = useState(initial?.limitDownload ?? 0);
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [saving, setSaving] = useState(false);
  const [shake, setShake] = useState(0);

  const editing = initial !== null;
  // An edit of a paired source may keep its pairing; a new source, or one from
  // before pairing, needs its instance.
  const keepPairing = editing && !initial.needsPairing;
  const canSave = name.trim() !== "" && repo.trim() !== "" && (memberId !== "" || keepPairing) && !saving;

  async function handleSave() {
    if (!canSave) {
      setShake((n) => n + 1);
      return;
    }
    setSaving(true);
    const body: PullSourceInput = {
      name: name.trim(),
      repo: repo.trim(),
      memberId,
      credsRef,
      domain,
      cadence,
      limitDownload,
      limitUpload: initial?.limitUpload ?? 0,
      enabled,
      sortOrder: initial?.sortOrder ?? 0,
    };
    try {
      const res = editing ? await updatePullSource(initial.id, body) : await createPullSource(body);
      if (res.ok) {
        onSaved();
        return;
      }
      push(res.error ?? t("pull.saveError"), "fail");
      setShake((n) => n + 1);
    } catch {
      push(t("pull.saveError"), "fail");
      setShake((n) => n + 1);
    } finally {
      setSaving(false);
    }
  }

  // The title repeats the button that opened the dialog, so no new keys.
  const title = editing ? t("common.edit") : t("pull.addSource");

  return createPortal(
    <div
      className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4"
      onClick={onClose}
    >
      <div className="relative w-full max-w-lg">
        <h2 className="flex items-center px-5">
          <Badge tone="heading" size="heading" wrap>
            {title}
          </Badge>
        </h2>
        <div
          role="dialog"
          aria-modal="true"
          aria-label={title}
          onClick={(e) => e.stopPropagation()}
          className="w-full max-h-[90vh] overflow-y-auto rounded-card bg-carbon-surface p-5 flex flex-col gap-4 shadow-2xl"
        >
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-carbon-textSub">{t("pull.name")}</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              spellCheck={false}
              autoComplete="off"
              placeholder="tower next door"
              className={inputCls}
            />
          </div>

          <MemberField
            t={t}
            label={t("pull.member")}
            value={memberId}
            onChange={setMemberId}
            keepOption={keepPairing}
            onPickLocation={setRepo}
          />

          <div className="flex flex-col gap-1.5">
            <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
              {t("pull.repoLocation")}
              <InfoBubble tip={t("pull.repoLocationHint")} />
            </span>
            <input
              type="text"
              value={repo}
              onChange={(e) => setRepo(e.target.value)}
              spellCheck={false}
              autoComplete="off"
              dir="ltr"
              placeholder="rest:http://192.168.1.9:8000/their-containers"
              className={inputCls}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
              {t("offsite.targets.credsLabel")}
              <InfoBubble tip={t("pull.credsHint")} />
            </span>
            <SelectField
              value={credsRef}
              onChange={setCredsRef}
              label={t("offsite.targets.credsLabel")}
              options={[
                // Unlike on an off-site target, "" is not the shared default: a
                // pull never falls back to this box's credentials.
                { value: "", label: t("pull.credsNone") },
                ...credSets.map((c) => ({ value: c.id, label: c.name })),
              ]}
              className={inputCls}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
              {t("pull.domain")}
              <InfoBubble tip={t("pull.domainHint")} />
            </span>
            <SelectField
              value={domain}
              onChange={setDomain}
              label={t("pull.domain")}
              options={DOMAINS.map((d) => ({ value: d, label: t(`nav.${d}` as never) }))}
              className={inputCls}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
              {t("pull.cadence")}
              <InfoBubble tip={t("pull.cadenceHint")} />
            </span>
            <input
              type="text"
              value={cadence}
              onChange={(e) => setCadence(e.target.value)}
              spellCheck={false}
              autoComplete="off"
              placeholder="daily 04:00"
              className={inputCls}
            />
          </div>

          <div className="flex flex-col gap-1.5 max-w-48">
            <label className="text-xs text-carbon-textSub">{t("pull.limitDownload")}</label>
            <NumberField
              min={0}
              value={limitDownload}
              onChange={(e) => setLimitDownload(Math.max(0, parseInt(e.target.value, 10) || 0))}
              className={inputCls}
            />
          </div>

          <ToggleRow
            label={t("pull.pullFrom")}
            checked={enabled}
            onChange={setEnabled}
          />

          <div className="flex flex-wrap items-center justify-end gap-2 pt-1">
            <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={onClose} />
            <Button
              key={shake || 0}
              label={t("settings.save")}
              labelKey="settings.save"
              tone="accent"
              onClick={() => void handleSave()}
              disabled={!canSave}
              busy={saving}
              className={shake ? "glim-shake" : ""}
            />
          </div>
        </div>
      </div>
    </div>,
    document.body
  );
}

/** With `embedded` the page is a tab of Instances, which owns the shell and
 *  the heading; the subtitle stays. */
export function Pull({ embedded = false }: { embedded?: boolean } = {}) {
  const { t } = useT();
  const [sources, setSources] = useState<PullSourceView[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [dialog, setDialog] = useState<PullSourceView | "new" | null>(null);

  async function load() {
    setLoading(true);
    try {
      const res = await listPullSources();
      if (res.ok) {
        setSources(res.sources ?? []);
        setError(null);
      } else {
        setError(res.error ?? t("pull.saveError"));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("pull.saveError"));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  const showEmptyState = !loading && error === null && sources.length === 0;

  return (
    <div className={embedded ? PAGE_SHELL_TABBED_RESPONSIVE : PAGE_SHELL_RESPONSIVE}>
      {/* Nothing here is visible once the title is sr-only and the empty
          state has hidden the Add button, so this row goes sr-only too and
          leaves the flex layout: without it, an empty row still ate a gap
          above the empty-state card. */}
      <div
        className={`flex items-start justify-between gap-4 flex-wrap${
          showEmptyState ? " glim-page-title sr-only" : ""
        }`}
      >
        {!embedded && <PageTitle>{t("pull.title")}</PageTitle>}
        {!showEmptyState && (
          <Button
            label={t("pull.addSource")}
            labelKey="pull.addSource"
            tone="accent"
            onClick={() => setDialog("new")}
            className="shrink-0"
          />
        )}
      </div>

      {loading && <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}
      {error !== null && <p className="text-sm text-statusFail wrap-break-word">{error}</p>}

      {showEmptyState && (
        <div
          className="relative glim-notch-card glim-hue bg-carbon-surface rounded-card p-6 text-center flex flex-col items-center gap-3"
          style={hueVars(0) as CSSProperties}
        >
          <h2 className="flex items-center">
            <Badge tone="heading" size="heading" wrap hueIndex={0} insetStart={6}>
              {t("pull.emptyTitle")}
              <InfoBubble tip={t("pull.empty")} onAccent />
            </Badge>
          </h2>
          <EmptyStateIcon icon={IconReceiver} />
          <Button
            label={t("pull.addSource")}
            labelKey="pull.addSource"
            tone="accent"
            onClick={() => setDialog("new")}
          />
        </div>
      )}

      {!loading && sources.length > 0 && (
        <div className="flex flex-col gap-3 glim-content-fade">
          {sources.map((ps, i) => (
            <PullSourceCard
              key={ps.id}
              source={ps}
              t={t}
              index={i}
              onRefresh={() => void load()}
              onEdit={() => setDialog(ps)}
            />
          ))}
        </div>
      )}

      {dialog !== null && (
        <PullDialog
          initial={dialog === "new" ? null : dialog}
          t={t}
          onClose={() => setDialog(null)}
          onSaved={() => {
            setDialog(null);
            void load();
          }}
        />
      )}
    </div>
  );
}
