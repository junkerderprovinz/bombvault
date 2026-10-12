import { useEffect, useRef, useState } from "react";
import { deleteBackupsZFSDataset, listContainers, patchZFSDataset, probeZFSDataset } from "../../lib/api";
import type { AnomalySeriesInfo, Container, ZFSDatasetPatch, ZFSDatasetView } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { deleteSentence } from "../../lib/secretsRefusal";
import { useConfirm } from "../../lib/useConfirm";
import { useDebouncedSave } from "../../lib/useDebouncedSave";
import { useToast } from "../../lib/toast";
import { useOwnLogin } from "../../lib/useOwnLogin";
import { tLtr } from "../../lib/ltrFragments";
import { zfsCodeSentence } from "../../lib/zfsCodes";
import { SubCard } from "../../pages/settings/shared";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { ZFSConnectionWindow } from "../backups/ZFSConnectionWindow";
import { DropdownListbox } from "../DropdownListbox";
import { Note } from "../entry/Note";
import { InfoBubble } from "../InfoBubble";
import { IconEye } from "../glyphs";
import { IconAdd, IconContainers } from "../navGlyphs";
import { SelectField } from "../SelectField";
import { SettingRow } from "../storage/rows";
import { ZFSMemberList } from "./ZFSMemberList";
import { failText } from "./failText";
import { ZFSExcludesEditor } from "./ZFSExcludesEditor";

type T = ReturnType<typeof useT>["t"];

const FIELD = "w-80 max-w-full rounded-control bg-carbon-surface3 px-3 py-1.5 font-mono text-xs text-carbon-text text-start glim-field-focus";

/** ZFSItemSettings is what can be set about one ZFS entry on its Settings tab. */
export function ZFSItemSettings({
  item,
  t,
  onChanged,
  series,
}: {
  item: ZFSDatasetView;
  t: T;
  onChanged: () => void;
  series: ReadonlyMap<string, AnomalySeriesInfo>;
}) {
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const ownLogin = useOwnLogin();
  const [excluded, setExcluded] = useState(new Set(item.excludedChildren));
  const [busy, setBusy] = useState(false);
  const [containers, setContainers] = useState<Container[]>([]);
  const [stopped, setStopped] = useState(item.stopContainers);
  const [probing, setProbing] = useState(false);
  const [pre, setPre] = useState(item.preSnapshot);
  const [post, setPost] = useState(item.postSnapshot);
  const [picking, setPicking] = useState(false);
  const [connection, setConnection] = useState(false);
  const addRef = useRef<HTMLButtonElement>(null);
  const { debouncedSave } = useDebouncedSave();

  useEffect(() => {
    listContainers()
      .then((res) => setContainers(res.ok ? (res.containers ?? []) : []))
      .catch(() => undefined);
  }, []);

  async function save(patch: ZFSDatasetPatch): Promise<boolean> {
    setBusy(true);
    try {
      const res = await patchZFSDataset(item.id, patch);
      if (res.ok) {
        onChanged();
        return true;
      }
      push(res.code ? zfsCodeSentence(t, res.code) : (res.error ?? t("settings.error")), "fail");
      return false;
    } catch (err) {
      push(failText(t, err), "fail");
      return false;
    } finally {
      setBusy(false);
    }
  }

  async function toggleChild(dataset: string, include: boolean) {
    const next = new Set(excluded);
    if (include) next.delete(dataset);
    else next.add(dataset);
    setExcluded(next);
    if (!(await save({ excludedChildren: [...next] }))) setExcluded(excluded);
  }

  async function dropStoredExclusion(dataset: string) {
    const next = new Set(excluded);
    next.delete(dataset);
    setExcluded(next);
    await save({ excludedChildren: [...next] });
  }

  async function setStopList(list: string[]) {
    setStopped(list);
    if (!(await save({ stopContainers: list }))) setStopped(stopped);
  }

  function saveHooks(nextPre: string, nextPost: string) {
    debouncedSave(() => {
      void save({ hookContainer: item.hookContainer, preSnapshot: nextPre, postSnapshot: nextPost });
    });
  }

  async function handleProbe() {
    setProbing(true);
    try {
      const res = await probeZFSDataset(item.id);
      if (!res.ok) push(res.error ?? t("settings.error"), "fail");
      onChanged();
    } catch (err) {
      push(failText(t, err), "fail");
    } finally {
      setProbing(false);
    }
  }

  async function handleDeleteBackups() {
    if (!(await confirm(t("zfs.deleteBackupsConfirm"), { confirmKey: "snapshots.deleteAll" }))) return;
    try {
      const res = await deleteBackupsZFSDataset(item.id);
      if (res.ok) onChanged();
      else if (!ownLogin.handled(res)) push(deleteSentence(t, res.code) ?? res.error ?? t("common.deleteBackupsFailed"), "fail");
    } catch (err) {
      push(failText(t, err), "fail");
    }
  }

  const known = new Set(item.members.map((m) => m.dataset));
  const gone = [...excluded].filter((d) => !known.has(d));
  const candidates = containers.filter((c) => !c.self && !stopped.includes(c.name));
  const overlap = stopped.filter((n) => containers.some((c) => c.name === n && c.includeInSchedule));
  const included = item.members.filter((m) => !excluded.has(m.dataset)).length;

  return (
    <div className="flex flex-col gap-4">
      <section className="flex flex-col gap-2">
        <h3 className="flex flex-wrap items-center gap-x-1.5 text-sm text-carbon-text">
          {t("zfs.children")}
          <InfoBubble tip={t("zfs.childrenHint")} />
          <span className="text-xs text-carbon-textMuted">{t("zfs.membersSummary", included)}</span>
        </h3>
        <div role="group" aria-label={t("zfs.children")}>
          <ZFSMemberList
            members={item.members}
            root={item.dataset}
            t={t}
            excluded={excluded}
            busy={busy}
            onToggle={(dataset, include) => void toggleChild(dataset, include)}
            series={series}
            targetId={item.id}
          />
        </div>
        {gone.map((dataset) => (
          <div key={dataset} className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 border-t border-carbon-border/60 pt-2 ps-3.5">
            <span className="flex min-w-0 flex-col">
              <span dir="ltr" className="font-mono text-xs text-carbon-text text-start wrap-anywhere">{dataset}</span>
              <span className="text-xs text-carbon-textMuted">{t("zfs.excludedGone")}</span>
            </span>
            <Button label={t("zfs.removeMissing")} labelKey="zfs.removeMissing" onClick={() => void dropStoredExclusion(dataset)} />
          </div>
        ))}
      </section>

      <div className="flex flex-col divide-y divide-carbon-border/60 border-t border-carbon-border/60 pt-3">
        <SettingRow
          label={t("zfs.stopContainers")}
          hint={t("zfs.stopContainersHint")}
          note={stopped.length === 0 ? t("zfs.stopContainersNone") : undefined}
        >
          {stopped.map((name) => (
            <Badge key={name} tone="neutral" wrap>
              {name}
              <Button
                label={`${t("offsite.targets.remove")}: ${name}`}
                labelKey="offsite.targets.remove"
                tone="subtle"
                variant="chip"
                onClick={() => void setStopList(stopped.filter((n) => n !== name))}
              />
            </Badge>
          ))}
          <Button
            ref={addRef}
            label={t("zfs.stopContainersPlaceholder")}
            labelKey="zfs.stopContainersPlaceholder"
            glyph={<IconAdd />}
            disabled={busy || candidates.length === 0}
            ariaExpanded={picking}
            onClick={() => setPicking((v) => !v)}
          />
          <DropdownListbox open={picking} onClose={() => setPicking(false)} triggerRef={addRef} label={t("zfs.stopContainersPlaceholder")}>
            {candidates.map((c) => (
              <button
                key={c.name}
                type="button"
                role="option"
                aria-selected={false}
                onClick={() => {
                  setPicking(false);
                  void setStopList([...stopped, c.name]);
                }}
                className="flex w-full items-center gap-2.5 px-3 py-2 text-start text-sm text-carbon-textSub transition-colors hover:bg-carbon-hover hover:text-carbon-text"
              >
                <span className="inline-flex shrink-0 text-accentText">
                  <IconContainers />
                </span>
                <span className="truncate">{c.name}</span>
              </button>
            ))}
          </DropdownListbox>
        </SettingRow>
      </div>
      {overlap.length > 0 && <Note tone="warn">{t("zfs.overlapContainers").replace("{names}", overlap.join(", "))}</Note>}

      <SubCard title={t("zfs.commands")}>
        <div className="flex flex-col divide-y divide-carbon-border/60">
          <SettingRow label={t("zfs.hookContainer")} hint={t("zfs.hooksHint")}>
            <SelectField
              value={item.hookContainer}
              label={t("zfs.hookContainer")}
              onChange={(name) => void save({ hookContainer: name, preSnapshot: pre, postSnapshot: post })}
              disabled={busy}
              className="w-80 max-w-full rounded-control bg-carbon-surface3 px-3 py-1.5 text-sm text-carbon-text glim-field-focus"
              options={[
                { value: "", label: t("zfs.hookContainerNone") },
                ...containers.filter((c) => !c.self).map((c) => ({ value: c.name, label: c.name })),
              ]}
            />
          </SettingRow>
          <SettingRow label={t("zfs.preSnapshot")}>
            <input
              dir="ltr"
              value={pre}
              aria-label={t("zfs.preSnapshot")}
              onChange={(e) => {
                setPre(e.target.value);
                saveHooks(e.target.value, post);
              }}
              disabled={item.hookContainer === ""}
              className={`${FIELD} disabled:opacity-50`}
            />
          </SettingRow>
          <SettingRow label={t("zfs.postSnapshot")}>
            <input
              dir="ltr"
              value={post}
              aria-label={t("zfs.postSnapshot")}
              onChange={(e) => {
                setPost(e.target.value);
                saveHooks(pre, e.target.value);
              }}
              disabled={item.hookContainer === ""}
              className={`${FIELD} disabled:opacity-50`}
            />
          </SettingRow>
        </div>
      </SubCard>

      <SubCard title={t("zfs.excludes")} hint={tLtr(t, "zfs.excludesHint")}>
        <ZFSExcludesEditor item={item} t={t} onSaved={onChanged} />
      </SubCard>

      <SettingRow label={t("zfs.connection.title")} hint={t("zfs.connection.hint")}>
        <Button
          label={t("zfs.connection.view")}
          labelKey="zfs.connection.view"
          glyph={<IconEye />}
          onClick={() => setConnection(true)}
        />
      </SettingRow>

      <div className="flex flex-wrap items-center gap-2">
        <Button
          label={t("zfs.probe")}
          labelKey="zfs.probe"
          onClick={() => void handleProbe()}
          disabled={probing}
          busy={probing}
          title={probing ? t("zfs.probing") : undefined}
        />
        <Button
          label={t("snapshots.deleteAll")}
          labelKey="snapshots.deleteAll"
          tone="subtle"
          onClick={() => void handleDeleteBackups()}
          className="ms-auto"
        />
      </div>
      {connection && <ZFSConnectionWindow onClose={() => setConnection(false)} />}
      {confirmDialog}
      {ownLogin.dialog}
    </div>
  );
}
