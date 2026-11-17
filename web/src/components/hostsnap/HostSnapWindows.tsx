// The windows of the Snapshots page: taking a snapshot, renaming one, and
// freeing one that ZFS calls busy.
import { useEffect, useMemo, useState } from "react";
import {
  createHostSnapshot,
  freeHostSnapshot,
  renameHostSnapshot,
  whereHostSnapshot,
  type HostSnapMount,
  type HostSnapSource,
  type HostSnapshot,
} from "../../lib/api";
import { codeKey, defaultName, KIND_LABEL, sourceKey, sourceLabel, validName } from "../../lib/hostSnapshots";
import { useT } from "../../lib/i18n";
import { useOwnLogin } from "../../lib/useOwnLogin";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Button } from "../Button";
import { Checkbox } from "../Checkbox";
import { Note } from "../entry/Note";
import { IconAdd, IconCheck, IconPencil, IconTrash } from "../Sidebar";
import { SelectField } from "../SelectField";
import { Rows, SettingRow } from "../storage/rows";

const FIELD = "w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus";

/** HostSnapDetail is what the server said, folded away under the sentence. */
export function HostSnapDetail({ detail }: { detail?: string }) {
  const { t } = useT();
  if (!detail) return null;
  return (
    <details className="text-xs text-carbon-textMuted">
      <summary className="cursor-pointer">{t("hostSnap.detail")}</summary>
      <pre dir="ltr" className="mt-1 whitespace-pre-wrap wrap-anywhere font-mono">{detail}</pre>
    </details>
  );
}

/** CreateWindow takes one snapshot of a source the server offers. */
export function CreateWindow({
  sources,
  preset,
  onDone,
  onClose,
}: {
  sources: HostSnapSource[];
  preset?: string;
  onDone: (name: string) => void;
  onClose: () => void;
}) {
  const { t } = useT();
  const ownLogin = useOwnLogin();
  const options = useMemo(
    () =>
      sources
        .filter((s) => s.canCreate)
        .map((s) => ({
          value: sourceKey(s.kind, s.fs, s.source),
          label: `${t(KIND_LABEL[s.kind])}: ${sourceLabel(s.kind, s.fs, s.source, t)}`,
          src: s,
        })),
    [sources, t]
  );
  const [picked, setPicked] = useState(preset ?? options[0]?.value ?? "");
  const [name, setName] = useState(() => defaultName(new Date()));
  const [recursive, setRecursive] = useState(false);
  const [busy, setBusy] = useState(false);
  const [refusal, setRefusal] = useState<{ text: string; detail?: string } | null>(null);
  const src = options.find((o) => o.value === picked)?.src;
  const ok = !!src && validName(name);

  async function take() {
    if (!src || !ok) return;
    setBusy(true);
    setRefusal(null);
    try {
      const res = await createHostSnapshot({ kind: src.kind, fs: src.fs, source: src.source, name, recursive: recursive && src.kind === "zfs" });
      if (res.ok) {
        onDone(name);
        return;
      }
      if (!ownLogin.handled(res)) setRefusal({ text: t(codeKey(res.code)), detail: res.error });
    } catch (err) {
      setRefusal({ text: t("hostSnap.code.failed"), detail: err instanceof Error ? err.message : undefined });
    } finally {
      setBusy(false);
    }
  }

  return (
    <InstanceWindow
      title={t("hostSnap.create.title")}
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} />
          <Button
            label={t("hostSnap.create.title")}
            labelKey="hostSnap.create.title"
            glyph={<IconAdd />}
            tone="accent"
            disabled={!ok || busy}
            busy={busy}
            onClick={() => void take()}
          />
        </>
      }
    >
      <Rows>
        <SettingRow label={t("hostSnap.create.source")}>
          <SelectField value={picked} onChange={setPicked} options={options} label={t("hostSnap.create.source")} wrap />
        </SettingRow>
        <SettingRow label={t("hostSnap.create.name")} htmlFor="host-snap-name">
          <input
            id="host-snap-name"
            type="text"
            dir="ltr"
            value={name}
            spellCheck={false}
            onChange={(e) => setName(e.target.value.trim())}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !busy) void take();
            }}
            className={FIELD}
          />
        </SettingRow>
      </Rows>
      {!validName(name) && <Note tone="warn">{t("hostSnap.nameHint")}</Note>}
      {src?.kind === "zfs" && (
        <Checkbox checked={recursive} onChange={setRecursive} label={t("hostSnap.create.recursive")} />
      )}
      {src?.kind === "btrfs" && <Note>{t("hostSnap.create.btrfsHint")}</Note>}
      {src?.kind === "vm" && <Note>{t("hostSnap.create.vmHint")}</Note>}
      {refusal && (
        <div role="alert" className="flex flex-col gap-1">
          <Note tone="fail">{refusal.text}</Note>
          <HostSnapDetail detail={refusal.detail} />
        </div>
      )}
      {ownLogin.dialog}
    </InstanceWindow>
  );
}

/** RenameWindow gives one snapshot a new name. */
export function RenameWindow({
  snapshot,
  onDone,
  onClose,
}: {
  snapshot: HostSnapshot;
  onDone: () => void;
  onClose: () => void;
}) {
  const { t } = useT();
  const ownLogin = useOwnLogin();
  const [name, setName] = useState(snapshot.label);
  const [busy, setBusy] = useState(false);
  const [refusal, setRefusal] = useState<{ text: string; detail?: string } | null>(null);
  const changed = name !== snapshot.label;

  async function save() {
    if (!changed) {
      onClose();
      return;
    }
    setBusy(true);
    setRefusal(null);
    try {
      const res = await renameHostSnapshot(snapshot, name);
      if (res.ok) {
        onDone();
        return;
      }
      if (!ownLogin.handled(res)) setRefusal({ text: t(codeKey(res.code)), detail: res.error });
    } catch (err) {
      setRefusal({ text: t("hostSnap.code.failed"), detail: err instanceof Error ? err.message : undefined });
    } finally {
      setBusy(false);
    }
  }

  return (
    <InstanceWindow
      title={t("hostSnap.rename.title")}
      facts={<span dir="ltr">{snapshot.label}</span>}
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} />
          <Button
            label={t("hostSnap.rename")}
            labelKey="hostSnap.rename"
            glyph={<IconPencil />}
            tone="accent"
            disabled={busy || !validName(name)}
            busy={busy}
            onClick={() => void save()}
          />
        </>
      }
    >
      <input
        type="text"
        dir="ltr"
        value={name}
        autoFocus
        spellCheck={false}
        aria-label={t("hostSnap.create.name")}
        onChange={(e) => setName(e.target.value.trim())}
        onKeyDown={(e) => {
          if (e.key === "Enter" && !busy) void save();
        }}
        className={FIELD}
      />
      {!validName(name) && <Note tone="warn">{t("hostSnap.nameHint")}</Note>}
      {snapshot.owner === "plan" && <Note>{t("hostSnap.rename.planHint")}</Note>}
      {refusal && (
        <div role="alert" className="flex flex-col gap-1">
          <Note tone="fail">{refusal.text}</Note>
          <HostSnapDetail detail={refusal.detail} />
        </div>
      )}
      {ownLogin.dialog}
    </InstanceWindow>
  );
}

function mountLine(m: HostSnapMount, t: ReturnType<typeof useT>["t"]): string {
  if (m.host) return t("hostSnap.busy.mountHost").replace("{path}", () => m.path);
  if (m.container)
    return t("hostSnap.busy.mountContainer")
      .replace("{name}", () => m.container ?? "")
      .replace("{path}", () => m.path);
  return t("hostSnap.busy.mountProcess")
    .replace("{pid}", String(m.pid))
    .replace("{path}", () => m.path);
}

/** BusyWindow says where a busy ZFS snapshot is mounted and frees it there. */
export function BusyWindow({
  snapshot,
  onDone,
  onClose,
}: {
  snapshot: HostSnapshot;
  onDone: () => void;
  onClose: () => void;
}) {
  const { t } = useT();
  const ownLogin = useOwnLogin();
  const [mounts, setMounts] = useState<HostSnapMount[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [refusal, setRefusal] = useState<{ text: string; detail?: string } | null>(null);
  const full = `${snapshot.source}@${snapshot.name}`;

  useEffect(() => {
    let live = true;
    whereHostSnapshot(snapshot)
      .then((res) => {
        if (!live) return;
        if (res.ok) setMounts(res.mounts ?? []);
        else setRefusal({ text: t(codeKey(res.code)), detail: res.error });
      })
      .catch(() => live && setRefusal({ text: t("hostSnap.code.failed") }));
    return () => {
      live = false;
    };
  }, [snapshot, t]);

  async function free() {
    setBusy(true);
    setRefusal(null);
    try {
      const res = await freeHostSnapshot(snapshot);
      if (res.ok) {
        onDone();
        return;
      }
      if (ownLogin.handled(res)) return;
      if (res.mounts) setMounts(res.mounts);
      setRefusal({ text: t(codeKey(res.code)), detail: res.error });
    } catch (err) {
      setRefusal({ text: t("hostSnap.code.failed"), detail: err instanceof Error ? err.message : undefined });
    } finally {
      setBusy(false);
    }
  }

  return (
    <InstanceWindow
      title={t("hostSnap.busy.title")}
      hint={t("hostSnap.busy.freeHint")}
      facts={<span dir="ltr">{full}</span>}
      onClose={onClose}
      footer={
        <>
          <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} />
          <Button
            label={t("hostSnap.busy.free")}
            labelKey="hostSnap.busy.free"
            glyph={<IconTrash />}
            tone="accent"
            disabled={busy || mounts === null}
            busy={busy}
            onClick={() => void free()}
          />
        </>
      }
    >
      <p className="text-sm text-carbon-textSub">{t("hostSnap.busy.text").replace("{name}", () => full)}</p>
      {mounts !== null && mounts.length === 0 && <Note>{t("hostSnap.busy.none")}</Note>}
      {mounts !== null && mounts.length > 0 && (
        <ul className="flex flex-col gap-1.5 text-xs">
          {mounts.map((m) => (
            <li key={`${m.pid}:${m.path}`} className="flex flex-col gap-0.5">
              <span className="flex items-center gap-1.5 text-carbon-text">
                {m.code === "ok" && <IconCheck />}
                <span className="wrap-anywhere">{mountLine(m, t)}</span>
              </span>
              {m.code && m.code !== "ok" && (
                <span className="text-statusFail">
                  {t("hostSnap.busy.mountFailed").replace("{path}", () => m.path).replace("{reason}", () => m.detail ?? "")}
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
      {refusal && (
        <div role="alert" className="flex flex-col gap-1">
          <Note tone="fail">{refusal.text}</Note>
          <HostSnapDetail detail={refusal.detail} />
        </div>
      )}
      {ownLogin.dialog}
    </InstanceWindow>
  );
}
