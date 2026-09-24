import { useCallback, useEffect, useState } from "react";
import { createPortal } from "react-dom";

import { createZFSDatasets, probeZFSDataset, zfsHostDatasets } from "../../lib/api";
import type { ZFSCheck, ZFSCreateResult, ZFSHostDataset, ZFSHostResult } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { useConfirm } from "../../lib/useConfirm";
import { useToast } from "../../lib/toast";
import { zfsCodeSentence } from "../../lib/zfsCodes";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { InfoBubble } from "../InfoBubble";
import { RepoPicker } from "../RepoPicker";
import { isBelow, ZFSDatasetTree } from "./ZFSDatasetTree";
import { ZFSMemberList } from "./ZFSMemberList";

/** What one probe found about one freshly added item. */
interface ProbeResult {
  dataset: string;
  check?: ZFSCheck;
}

/** The refusal's own text, without the code the server put in front of it. */
function refusalNames(code: string, detail: string): string[] {
  const body = detail.startsWith(code + ": ") ? detail.slice(code.length + 2) : detail;
  return body === "" ? [] : [body];
}

function directChildren(entries: ZFSHostDataset[], root: string): string[] {
  return entries.filter((e) => isBelow(e.dataset, root) && !e.dataset.slice(root.length + 1).includes("/"))
    .map((e) => e.dataset);
}

/** ZFSAddDialog turns the host's pools into items. It is the only place that
 *  probes snapshot access on add, so nobody leaves it with an item nobody has
 *  tried to read. */
export function ZFSAddDialog({ onClose, onAdded }: { onClose: () => void; onAdded: () => void }) {
  const { t } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [host, setHost] = useState<ZFSHostResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState("");
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [excluded, setExcluded] = useState<ReadonlySet<string>>(new Set());
  const [repo, setRepo] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [results, setResults] = useState<ZFSCreateResult[] | null>(null);
  const [probing, setProbing] = useState("");
  const [probed, setProbed] = useState<ProbeResult[]>([]);

  const load = useCallback(() => {
    setLoading(true);
    zfsHostDatasets()
      .then(setHost)
      .catch(() => setHost(null))
      .finally(() => setLoading(false));
  }, []);

  useEffect(load, [load]);

  const entries = host?.datasets ?? [];

  async function handleSelectRoot(dataset: string, on: boolean) {
    if (!on) {
      setSelected(new Set([...selected].filter((d) => d !== dataset)));
      setExcluded(new Set([...excluded].filter((d) => !isBelow(d, dataset))));
      return;
    }
    if (!dataset.includes("/")) {
      const question = t("zfs.add.poolRootConfirm")
        .replace("{dataset}", dataset)
        .replace("{names}", directChildren(entries, dataset).join(", "));
      if (!(await confirm(question, { confirmKey: "zfs.add.asItem" }))) return;
    }
    setSelected(new Set([...selected, dataset]));
    // A VM's disks are backed up on the VMs page and system data is not worth
    // a copy, so they start out of the item rather than surprising the reader
    // with their size on the first run.
    const off = entries
      .filter((e) => isBelow(e.dataset, dataset) && (e.vmDisk || e.system))
      .map((e) => e.dataset);
    setExcluded(new Set([...excluded, ...off]));
  }

  function handleIncludeChild(dataset: string, include: boolean) {
    const next = new Set(excluded);
    if (include) next.delete(dataset);
    else next.add(dataset);
    setExcluded(next);
  }

  async function handleSubmit() {
    setSubmitting(true);
    try {
      const items = [...selected].sort().map((dataset) => ({
        dataset,
        excludedChildren: [...excluded].filter((d) => isBelow(d, dataset)).sort(),
        repo,
      }));
      const res = await createZFSDatasets(items);
      if (!res.ok) {
        push(res.error ?? t("settings.error"), "fail");
        return;
      }
      const created = res.results ?? [];
      setResults(created);
      // One probe at a time: each one takes a real snapshot on the host, and
      // two of them on the same pool would fight over the same names.
      for (const item of created.filter((r) => r.id !== "")) {
        setProbing(item.dataset);
        const probe = await probeZFSDataset(item.id);
        setProbed((prev) => [...prev, { dataset: item.dataset, check: probe.check }]);
      }
      setProbing("");
    } finally {
      setSubmitting(false);
    }
  }

  function handleDone() {
    onAdded();
    onClose();
  }

  const heading = t("zfs.add.title");

  return createPortal(
    <>
      {/* The confirm dialog is a sibling of the backdrop, not a child: a click
          inside it would otherwise reach the backdrop's close handler. */}
      <div
        className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4"
        onClick={results === null && !submitting ? onClose : undefined}
      >
      <div className="relative w-full max-w-3xl">
        <h2 className="flex items-center px-5">
          <Badge tone="heading" size="heading" wrap>
            {heading}
            <InfoBubble tip={t("zfs.add.hint")} onAccent />
          </Badge>
        </h2>
        <div
          role="dialog"
          aria-modal="true"
          aria-label={heading}
          onClick={(e) => e.stopPropagation()}
          className="flex max-h-[90vh] w-full flex-col gap-4 overflow-y-auto rounded-card bg-carbon-surface p-5 shadow-2xl"
        >
          {results !== null ? (
            <div className="flex flex-col gap-2">
              {results.map((res) => (
                <p key={res.dataset} className={`text-sm ${res.id ? "text-carbon-text" : "text-statusFail"}`}>
                  {res.id
                    ? t("zfs.add.result.ok").replace("{dataset}", res.dataset)
                    : t("zfs.add.result.failed")
                        .replace("{dataset}", res.dataset)
                        .replace(
                          "{reason}",
                          zfsCodeSentence(t, res.code, {
                            names: refusalNames(res.code, res.detail),
                            max: host?.maxNameLength,
                          }),
                        )}
                </p>
              ))}
              {probing !== "" && (
                <p className="text-sm text-carbon-textMuted">
                  {t("zfs.add.testing").replace("{dataset}", probing)}
                </p>
              )}
              {probed.map(
                (item) =>
                  item.check && (
                    <div key={item.dataset} className="flex flex-col gap-1">
                      <p className="text-sm text-carbon-textSub">
                        {zfsCodeSentence(t, item.check.code, {
                          hostMountpoint: item.check.hostMountpoint,
                          max: item.check.max,
                          names: item.check.names,
                        })}
                      </p>
                      <ZFSMemberList members={item.check.members} root={item.check.dataset} t={t} />
                    </div>
                  ),
              )}
              <div className="flex items-center justify-end pt-1">
                <Button
                  label={t("zfs.add.done")}
                  labelKey="zfs.add.done"
                  tone="accent"
                  onClick={handleDone}
                  disabled={probing !== ""}
                />
              </div>
            </div>
          ) : (
            <>
              {loading && <p className="text-sm text-carbon-textMuted">{t("zfs.add.loading")}</p>}

              {!loading && (host === null || !host.available) && (
                <div className="flex flex-col items-start gap-2">
                  <p className="text-sm text-statusFail">{t("zfs.add.unavailable")}</p>
                  {host && (
                    <p className="text-sm text-carbon-textSub">
                      {zfsCodeSentence(t, host.code, { target: host.target })}
                    </p>
                  )}
                  <Button label={t("zfs.add.retry")} labelKey="zfs.add.retry" tone="neutral" onClick={load} />
                </div>
              )}

              {!loading && host !== null && host.available && (
                <>
                  <div className="flex items-center gap-3 flex-wrap">
                    <input
                      type="text"
                      value={filter}
                      onChange={(e) => setFilter(e.target.value)}
                      aria-label={t("zfs.add.filter")}
                      placeholder={t("zfs.add.filter")}
                      spellCheck={false}
                      autoComplete="off"
                      className="w-64 max-w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus"
                    />
                    <span className="text-xs text-carbon-textMuted">
                      {t("zfs.add.selected").replace("{n}", String(selected.size))}
                    </span>
                  </div>

                  <ZFSDatasetTree
                    entries={entries}
                    filter={filter}
                    selected={selected}
                    excluded={excluded}
                    onSelectRoot={(dataset, on) => void handleSelectRoot(dataset, on)}
                    onIncludeChild={handleIncludeChild}
                    maxNameLength={host.maxNameLength}
                    t={t}
                  />

                  {host.unusedZvols > 0 && (
                    <p className="flex items-center gap-1.5 text-xs text-carbon-textMuted">
                      {t("zfs.add.unusedZvols").replace("{n}", String(host.unusedZvols))}
                      <InfoBubble tip={t("zfs.unusedZvolsHint")} />
                    </p>
                  )}
                  {host.truncated && (
                    <p className="text-xs text-carbon-textMuted">
                      {t("zfs.add.truncated").replace("{n}", String(entries.length))}
                    </p>
                  )}

                  <RepoPicker
                    value={repo}
                    onChange={setRepo}
                    labelKey="zfs.repo"
                    hintKey="zfs.repoHint"
                    defaultLabelKey="zfs.repoPlaceholder"
                    lockedKey="zfs.repoLocked"
                  />

                  <div className="flex items-center justify-end gap-2 pt-1">
                    <Button
                      label={t("files.cancel")}
                      labelKey="files.cancel"
                      tone="neutral"
                      onClick={onClose}
                      disabled={submitting}
                    />
                    <Button
                      label={t("zfs.add.submit").replace("{n}", String(selected.size))}
                      labelKey="zfs.add.submit"
                      tone="accent"
                      onClick={() => void handleSubmit()}
                      disabled={submitting || selected.size === 0}
                      busy={submitting}
                    />
                  </div>
                </>
              )}
            </>
          )}
        </div>
      </div>
      </div>
      {confirmDialog}
    </>,
    document.body,
  );
}
