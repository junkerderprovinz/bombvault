// Restores single entries out of the repository of another BombVault
// instance, opened read-only with that instance's key. Nothing is stored: the
// session lives in the server's memory for half an hour and is closed when
// the window goes, so the foreign key does not linger.
import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import {
  foreignClose,
  foreignContainerWarnings,
  foreignOpen,
  foreignRestore,
  listContainers,
  listForeignFiles,
  listVMs,
  type FileEntry,
  type ForeignBindWarning,
  type ForeignInventory,
  type ForeignItem,
  type Snapshot,
} from "../../lib/api";
import type { EntryKind } from "../../lib/backupEntry";
import { fireAndWaitRun } from "../../lib/backupWatch";
import { useT, type TranslationKey } from "../../lib/i18n";
import { FOREIGN_APPDATA_DEST_HINT_LTR_FRAGMENTS, withLtrFragments, withLtrIsolates } from "../../lib/ltrFragments";
import { anyActive, useProgress } from "../../lib/progress";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useReveal } from "../../lib/useReveal";
import { checkRestoreOnce, restoreBlockReason } from "../../lib/useRestoreCheck";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { KindGlyph } from "../backups/kinds";
import { Button } from "../Button";
import { FolderBrowser } from "../FolderBrowser";
import { IconBack, IconLink } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { IconCheck, IconClose, IconGear, IconRestore } from "../navGlyphs";
import { RevealInput } from "../RevealInput";
import { Selector } from "../Selector";
import { SelectField } from "../SelectField";
import { SnapshotFileTree } from "../SnapshotFileTree";
import { Toggle } from "../Toggle";
import { RestoreCheckPanel } from "./RestoreCheckPanel";
import { StepBar } from "./StepBar";

type ForeignDomain = "containers" | "vms" | "files" | "zfs";

const KIND: Record<ForeignDomain, EntryKind> = { containers: "container", vms: "vm", files: "files", zfs: "zfs" };
const GROUP_LABEL: Record<ForeignDomain, TranslationKey> = {
  containers: "nav.containers",
  vms: "nav.vms",
  files: "nav.files",
  zfs: "nav.zfs",
};

const FIELD = "rounded-control bg-carbon-surface2 px-3 py-2 font-mono text-sm text-carbon-text glim-field-focus";
const SELECT = "rounded-control bg-carbon-surface3 px-2 py-1.5 text-xs text-carbon-text glim-field-focus";

/** Whether a restore failed because the session lapsed, which a reconnect fixes. */
function isSessionGone(err: string | undefined): boolean {
  return !!err && /session/i.test(err) && /(expired|unknown)/i.test(err);
}

/** The dataset a foreign ZFS snapshot holds, from its zfs: tag. */
function zfsSnapshotDataset(s: Snapshot): string {
  return s.tags.find((tag) => tag.startsWith("zfs:"))?.slice(4) ?? "";
}

/** The host snapshot a ZFS backup read, which every dataset of one run
 *  shares. A snapshot without one stands for itself. */
function zfsRunStamp(s: Snapshot): string {
  return /\/\.zfs\/snapshot\/([^/]+)$/.exec(s.paths[0] ?? "")?.[1] ?? s.id;
}

/** Whether two datasets are one, or one lies below the other. A restore is
 *  recorded on the item that covers the dataset, which may be its parent. */
function zfsSameTree(a: string, b: string): boolean {
  return a === b || a.startsWith(b + "/") || b.startsWith(a + "/");
}

/**
 * ForeignItemRow restores one foreign entry from a chosen snapshot. Folder sets
 * and dataset trees go into a folder, because a foreign repository carries no
 * trusted place on this host. A VM gets a folder for its disks as well: the
 * paths of the other server can point at this host's RAM disk (#122). Its
 * disks land in <destination>/<vm-name>/ and it stays stopped, so it can be
 * checked before its first start.
 */
function ForeignItemRow({
  domain,
  item,
  session,
  hostMountRoot,
  existsLocally,
  collisionKnown,
  blocked,
  open,
  hidden,
  footer,
  onOpen,
  onBusyChange,
  onSessionGone,
}: {
  domain: ForeignDomain;
  item: ForeignItem;
  session: string;
  hostMountRoot: string;
  /** A restore asks before it overwrites: a real or an unverifiable collision. */
  existsLocally: boolean;
  /** An entry of this name is known to exist here. False when the entries of
   *  this host could not be read, so the question says "could not verify". */
  collisionKnown: boolean;
  blocked: boolean;
  /** The row shows its options and puts its restore into the window's foot. */
  open: boolean;
  /** Another row is open. This one keeps its choices and steps aside. */
  hidden: boolean;
  footer: HTMLElement | null;
  onOpen: () => void;
  onBusyChange: (busy: boolean) => void;
  onSessionGone: () => void;
}) {
  const { t } = useT();
  const [snapshot, setSnapshot] = useState("latest");
  // A VM starts at the folder the server falls back to anyway.
  const [target, setTarget] = useState(domain === "vms" ? "user/domains" : "");
  const needsTarget = domain !== "containers";
  const [busy, setBusy] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();

  // Folder sets: the whole set or a picked part of it, such as one stack out
  // of a set that holds all of appdata.
  const [subset, setSubset] = useState(false);
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [filesLoading, setFilesLoading] = useState(false);
  const [filesError, setFilesError] = useState<string | null>(null);
  const [filter, setFilter] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());

  // ZFS: the whole tree of one run, or one dataset of it, whole or in part.
  const [zfsDataset, setZFSDataset] = useState("");
  const [zfsPickFiles, setZFSPickFiles] = useState(false);
  const picking = (domain === "files" && subset) || (domain === "zfs" && zfsDataset !== "" && zfsPickFiles);
  const restoreItem = domain === "zfs" && zfsDataset !== "" ? zfsDataset : item.name;

  // Containers: appdata is remapped onto this host. `overwrite` allows writing
  // into a folder that already holds data, which may be another container's.
  const [overwrite, setOverwrite] = useState(false);
  const [warnings, setWarnings] = useState<ForeignBindWarning[]>([]);
  // The container's network when this host lacks it, and the one picked in
  // its place. Unraid's br0 networks are the usual case on a plain Docker host.
  const [missingNetwork, setMissingNetwork] = useState("");
  const [networks, setNetworks] = useState<string[]>([]);
  const [network, setNetwork] = useState("");
  const chosenNetwork = missingNetwork !== "" ? network : undefined;

  // The parent hands down a new function with every progress tick. As a
  // dependency it would wipe the selection mid-pick, so the effects read it
  // through a ref.
  const sessionGone = useRef(onSessionGone);
  sessionGone.current = onSessionGone;

  // restic lists oldest first. A whole tree is restored per run, so each run
  // is offered once, and a single dataset offers its own snapshots.
  const newestFirst = [...item.snapshots].reverse();
  const zfsDatasets = domain === "zfs" ? [...new Set(item.snapshots.map(zfsSnapshotDataset).filter(Boolean))].sort() : [];
  const snaps =
    domain !== "zfs"
      ? newestFirst
      : zfsDataset !== ""
        ? newestFirst.filter((s) => zfsSnapshotDataset(s) === zfsDataset)
        : newestFirst.filter((s, i) => newestFirst.findIndex((o) => zfsRunStamp(o) === zfsRunStamp(s)) === i);

  function chooseZFSDataset(next: string) {
    setZFSDataset(next);
    setSnapshot("latest");
    if (next === "") setZFSPickFiles(false);
  }

  useEffect(() => {
    if (domain !== "containers") return;
    let cancelled = false;
    foreignContainerWarnings(session, item.name)
      .then((res) => {
        if (cancelled) return;
        if (res.ok) {
          const nets = res.networks ?? [];
          setWarnings(res.warnings ?? []);
          setMissingNetwork(res.missingNetwork ?? "");
          setNetworks(nets);
          setNetwork(nets.includes("bridge") ? "bridge" : (nets[0] ?? ""));
        } else if (isSessionGone(res.error)) sessionGone.current();
      })
      // The appdata remap and the guard on the destination protect the restore either way.
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [domain, session, item.name]);

  useEffect(() => {
    if (!picking) return;
    let cancelled = false;
    setFilesLoading(true);
    setFilesError(null);
    // What was ticked belonged to the tree of another snapshot.
    setSelected(new Set());
    listForeignFiles(session, restoreItem, snapshot, domain === "zfs" ? "zfs" : "files")
      .then((res) => {
        if (cancelled) return;
        setFiles(res.ok ? (res.files ?? []) : []);
        if (!res.ok) {
          setFilesError(res.error ?? t("files.loadFailed"));
          if (isSessionGone(res.error)) sessionGone.current();
        }
      })
      .catch((err) => {
        if (!cancelled) setFilesError(err instanceof Error ? err.message : String(err));
      })
      .finally(() => {
        if (!cancelled) setFilesLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [picking, session, restoreItem, snapshot, domain, t]);

  function toggleSelected(p: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(p)) next.delete(p);
      else next.add(p);
      return next;
    });
  }

  const incomplete = (needsTarget && target.trim() === "") || (picking && selected.size === 0);

  async function handleRestore() {
    if (busy || blocked || incomplete) return;
    const choice = {
      session,
      domain,
      paths: picking ? [...selected] : undefined,
      overwrite: domain === "containers" ? overwrite : undefined,
      wholeTree: domain === "zfs" ? zfsDataset === "" : undefined,
      network: chosenNetwork,
    };
    setBusy(true);
    const check = await checkRestoreOnce({
      ...choice,
      kind: "foreign",
      name: restoreItem,
      snapshotId: snapshot,
      targetPath: target.trim(),
    });
    setBusy(false);
    const refusal = restoreBlockReason(check, t);
    const collision = existsLocally
      ? t(collisionKnown ? "recovery.foreignExistsConfirm" : "recovery.foreignUnverifiedConfirm").replace("{name}", item.name)
      : undefined;
    const question = collision ?? refusal;
    if (question !== undefined) {
      const extra = <RestoreCheckPanel check={check} t={t} />;
      if (!(await confirm(question, { extra, confirmBlocked: refusal }))) return;
    }
    setBusy(true);
    onBusyChange(true);
    try {
      const res = await fireAndWaitRun({
        kind: "restore",
        matchRun: (r) => r.domain === KIND[domain] && (domain === "zfs" ? zfsSameTree(r.target, restoreItem) : r.target === item.name),
        start: () =>
          foreignRestore({
            ...choice,
            item: restoreItem,
            snapshot,
            confirm: true,
            // An empty target leaves the default to the server.
            target: target.trim() || undefined,
          }),
        t,
      });
      if (res.ok) push(t("common.done"), "success");
      else {
        push(res.error ?? t("settings.error"), "fail");
        if (isSessionGone(res.error)) onSessionGone();
      }
    } catch (err) {
      push(err instanceof Error ? err.message : String(err), "fail");
    } finally {
      setBusy(false);
      onBusyChange(false);
    }
  }

  const tree = picking && (
    <SnapshotFileTree
      files={files}
      loading={filesLoading}
      error={filesError}
      filter={filter}
      onFilterChange={setFilter}
      selected={selected}
      onToggle={toggleSelected}
      t={t}
    />
  );
  const restoreButton = (tone: "accent" | "neutral") => (
    <Button
      label={t("recovery.foreignRestore")}
      labelKey="recovery.foreignRestore"
      glyph={<IconRestore />}
      tone={tone}
      onClick={() => void handleRestore()}
      disabled={busy || blocked || incomplete}
      busy={busy}
    />
  );

  return (
    <div data-testid="foreign-item" hidden={hidden} className="flex flex-col gap-3 py-3 first:pt-0 last:pb-0">
      <div className="flex flex-wrap items-center gap-3">
        <span className="flex h-9 w-9 flex-none items-center justify-center rounded-control bg-carbon-surface3 text-accentText [&_svg]:h-[18px] [&_svg]:w-[18px]">
          <KindGlyph kind={KIND[domain]} />
        </span>
        <span className="min-w-0 flex-1 truncate text-sm font-semibold text-carbon-text">{item.name}</span>
        <SelectField
          value={snapshot}
          onChange={setSnapshot}
          label={t("recovery.foreignLatest")}
          disabled={busy}
          options={[
            { value: "latest", label: t("recovery.foreignLatest") },
            ...snaps.map((s) => ({ value: s.id, label: `${new Date(s.time).toLocaleString()}, ${s.id.slice(0, 8)}` })),
          ]}
          className={SELECT}
        />
        {!open && (
          <>
            <Button
              label={t("oldFolder.options")}
              labelKey={null}
              glyph={<IconGear />}
              variant="icon"
              tone="subtle"
              title={t("oldFolder.options")}
              onClick={onOpen}
            />
            {restoreButton("neutral")}
          </>
        )}
      </div>

      {open && (
        <div className="flex flex-col gap-3">
          {domain === "files" && (
            <>
              <div className="flex flex-wrap items-center gap-2">
                <Selector
                  items={[
                    { id: "whole", label: t("recovery.foreignWholeSet") },
                    { id: "subset", label: t("recovery.foreignPickSubfolder") },
                  ]}
                  label={item.name}
                  select="one"
                  active={subset ? "subset" : "whole"}
                  onChange={(id) => setSubset(id === "subset")}
                  disabled={busy}
                  inline
                />
                <InfoBubble tip={t("recovery.foreignSubfolderHint")} />
              </div>
              {tree}
              <FolderBrowser label={t("recovery.foreignTargetFolder")} value={target} hostMountRoot={hostMountRoot} onChange={setTarget} />
            </>
          )}
          {domain === "vms" && (
            <FolderBrowser
              label={t("recovery.foreignVMDest")}
              value={target}
              hostMountRoot={hostMountRoot}
              onChange={setTarget}
              placeholder="user/domains"
              hint={t("recovery.foreignVMDestHint")}
            />
          )}
          {domain === "zfs" && (
            <>
              <div className="flex flex-wrap items-center gap-3 text-xs">
                <span className="text-carbon-textSub">{t("zfs.restore.dataset")}</span>
                <SelectField
                  value={zfsDataset}
                  onChange={chooseZFSDataset}
                  label={t("zfs.restore.dataset")}
                  disabled={busy}
                  options={[{ value: "", label: t("zfs.restore.wholeTree") }, ...zfsDatasets.map((d) => ({ value: d, label: d }))]}
                  className={SELECT}
                />
                {zfsDataset !== "" && (
                  <Toggle checked={zfsPickFiles} onChange={setZFSPickFiles} disabled={busy} label={t("zfs.restore.selectFiles")} />
                )}
              </div>
              {tree}
              <FolderBrowser label={t("recovery.foreignTargetFolder")} value={target} hostMountRoot={hostMountRoot} onChange={setTarget} />
            </>
          )}
          {domain === "containers" && (
            <>
              {/* A tip is a plain string that also names the bubble, so its
                  path is isolated by characters and not by an element. */}
              <FolderBrowser
                label={t("recovery.foreignAppdataDest")}
                value={target}
                hostMountRoot={hostMountRoot}
                onChange={setTarget}
                placeholder="user/appdata"
                hint={withLtrIsolates(t("recovery.foreignAppdataDestHint"), FOREIGN_APPDATA_DEST_HINT_LTR_FRAGMENTS)}
              />
              <Toggle checked={overwrite} onChange={setOverwrite} disabled={busy} label={t("recovery.foreignOverwrite")} />
              {missingNetwork !== "" && (
                <div className="flex flex-wrap items-center gap-3 text-xs">
                  <span className="text-carbon-textSub">
                    {withLtrFragments(t("recovery.foreignNetworkMissing").replace("{name}", missingNetwork), [missingNetwork])}
                  </span>
                  <SelectField
                    value={network}
                    onChange={setNetwork}
                    label={t("recovery.foreignNetwork")}
                    disabled={busy || networks.length === 0}
                    options={networks.map((n) => ({ value: n, label: n }))}
                    className={SELECT}
                  />
                  <InfoBubble tip={t("recovery.foreignNetworkHint")} />
                </div>
              )}
              {warnings.length > 0 && (
                <div className="rounded-card bg-statusWarnBg px-3 py-2 text-xs">
                  <p className="text-statusWarn">{t("recovery.foreignBindWarning")}</p>
                  <ul className="mt-1 flex flex-col gap-0.5 text-carbon-text">
                    {/* The separator is one neither path can hold. Written as
                        an escape, because a literal NUL byte makes git treat
                        the file as binary. */}
                    {warnings.map((wn) => (
                      <li key={wn.host + "\u0000" + wn.container} className="font-mono text-start wrap-break-word" dir="ltr">
                        {wn.host} → {wn.container}
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </>
          )}
        </div>
      )}
      {open && footer && createPortal(restoreButton("accent"), footer)}
      {confirmDialog}
    </div>
  );
}

export function OldFolderWindow({ hostMountRoot, onClose }: { hostMountRoot: string; onClose: () => void }) {
  const { t } = useT();
  const { push } = useToast();
  const [remote, setRemote] = useState(false);
  const [folder, setFolder] = useState("");
  const [url, setUrl] = useState("");
  const [key, setKey] = useState("");
  // The credentials of the foreign repository, held for one session. This
  // instance's own are never offered: lending them to an address somebody
  // typed would hand them to whoever runs it.
  const [creds, setCreds] = useState({ s3KeyId: "", s3Secret: "", s3Region: "", restUser: "", restPassword: "" });
  const revealKey = useReveal();
  const revealSecret = useReveal();
  const revealPassword = useReveal();

  const [connecting, setConnecting] = useState(false);
  const [connectError, setConnectError] = useState<string | null>(null);
  const [session, setSession] = useState<string | null>(null);
  const [inventory, setInventory] = useState<ForeignInventory | null>(null);
  const [expired, setExpired] = useState(false);
  // "container:x" and "vm:y" of this host, read when the session opens, so
  // each row knows whether its restore overwrites something.
  const [localNames, setLocalNames] = useState<Set<string>>(new Set());
  // False when this host's entries could not be read. Every container and VM
  // then asks before it restores.
  const [localKnown, setLocalKnown] = useState(true);
  const [busyRows, setBusyRows] = useState(0);
  const [openItem, setOpenItem] = useState<string | null>(null);
  const [footer, setFooter] = useState<HTMLElement | null>(null);
  const otherActive = anyActive(useProgress()).active;

  // An effect on the session would close each id when the next one opens.
  const sessionRef = useRef<string | null>(null);
  sessionRef.current = session;
  useEffect(
    () => () => {
      if (sessionRef.current) foreignClose(sessionRef.current).catch(() => undefined);
    },
    []
  );

  const location = (remote ? url : folder).trim();
  // Which credential a remote repository needs depends on its backend, so any
  // one unlocks Connect and the server says what is missing.
  const hasCreds = [creds.s3KeyId, creds.s3Secret, creds.restUser, creds.restPassword].some((v) => v.trim() !== "");
  const lacks = location === "" ? t("oldFolder.needsAddress") : key.trim() === "" ? t("oldFolder.needsKey") : undefined;

  const connect = useCallback(async () => {
    setConnecting(true);
    setConnectError(null);
    setExpired(false);
    if (sessionRef.current) {
      foreignClose(sessionRef.current).catch(() => undefined);
      setSession(null);
      setInventory(null);
    }
    try {
      // A mounted folder needs no credentials, so none travel for it.
      const res = await foreignOpen(
        location,
        key.trim(),
        remote ? { ...creds, s3KeyId: creds.s3KeyId.trim(), s3Region: creds.s3Region.trim(), restUser: creds.restUser.trim(), s3StorageClass: "" } : undefined
      );
      if (!res.ok || !res.session) {
        setConnectError(res.error ?? t("settings.error"));
        return;
      }
      const names = new Set<string>();
      let known = true;
      try {
        const [cs, vs] = await Promise.all([listContainers(), listVMs()]);
        // Both answer with ok false while Docker or libvirt is away for a moment.
        if (!cs.ok || !vs.ok) known = false;
        else {
          for (const c of cs.containers ?? []) names.add(`container:${c.name}`);
          // Foreign names come from restic tags and are libvirt names.
          for (const v of vs.vms ?? []) names.add(`vm:${v.libvirtName}`);
        }
      } catch {
        known = false;
      }
      setLocalNames(names);
      setLocalKnown(known);
      setSession(res.session);
      setInventory(res.inventory ?? { containers: [], vms: [], fileSets: [], dbDumps: [], zfs: [] });
      setOpenItem(null);
    } catch (err) {
      setConnectError(err instanceof Error ? err.message : String(err));
    } finally {
      setConnecting(false);
    }
  }, [location, key, remote, creds, t]);

  function disconnect() {
    if (sessionRef.current) foreignClose(sessionRef.current).catch(() => undefined);
    setSession(null);
    setInventory(null);
    setExpired(false);
    push(t("oldFolder.disconnected"), "success");
  }

  const onBusyChange = useCallback((busy: boolean) => {
    setBusyRows((n) => (busy ? n + 1 : Math.max(0, n - 1)));
  }, []);

  const groups: { domain: ForeignDomain; items: ForeignItem[] }[] = inventory
    ? [
        { domain: "containers" as const, items: inventory.containers },
        { domain: "vms" as const, items: inventory.vms },
        { domain: "files" as const, items: inventory.fileSets },
        { domain: "zfs" as const, items: inventory.zfs },
      ].filter((g) => g.items.length > 0)
    : [];
  // A repository that holds nothing but dumps is not empty: it holds the
  // only copy of those databases.
  const dumps = inventory?.dbDumps.length ?? 0;
  const empty = groups.length === 0 && dumps === 0;
  const browsing = session !== null && inventory !== null;

  const credField = (name: keyof typeof creds, label: TranslationKey, hint?: TranslationKey) => (
    <label className="flex flex-col gap-1 text-xs text-carbon-textSub">
      <span className="flex items-center gap-1">
        {t(label)}
        {hint && <InfoBubble tip={t(hint)} />}
      </span>
      <input
        value={creds[name]}
        spellCheck={false}
        autoComplete="off"
        onChange={(e) => setCreds((prev) => ({ ...prev, [name]: e.target.value }))}
        className={FIELD}
      />
    </label>
  );
  const secretField = (name: "s3Secret" | "restPassword", label: TranslationKey, reveal: ReturnType<typeof useReveal>) => (
    <label className="flex flex-col gap-1 text-xs text-carbon-textSub">
      {t(label)}
      <RevealInput
        {...reveal}
        value={creds[name]}
        spellCheck={false}
        autoComplete="off"
        onChange={(e) => setCreds((prev) => ({ ...prev, [name]: e.target.value }))}
        wrapperClassName="w-full"
        className={FIELD}
      />
    </label>
  );

  let foot: ReactNode;
  if (!browsing) {
    foot = (
      <>
        <Button label={t("common.cancel")} labelKey="common.cancel" tone="subtle" onClick={onClose} />
        <Button
          label={t("recovery.foreignConnect")}
          labelKey="recovery.foreignConnect"
          glyph={<IconLink />}
          tone="accent"
          onClick={() => void connect()}
          disabled={connecting || lacks !== undefined || (remote && !hasCreds)}
          busy={connecting}
          hint={lacks}
        />
      </>
    );
  } else if (openItem !== null) {
    foot = (
      <>
        <Button label={t("common.back")} labelKey="common.back" glyph={<IconBack />} tone="subtle" onClick={() => setOpenItem(null)} />
        <span ref={setFooter} className="flex" />
      </>
    );
  } else {
    foot = <Button label={t("common.done")} labelKey="common.done" glyph={<IconCheck />} tone="accent" onClick={onClose} />;
  }

  return (
    <InstanceWindow title={t("restorePage.oldFolder")} hint={t("oldFolder.intro")} onClose={onClose} footer={foot}>
      <StepBar
        steps={[t("recovery.foreignConnect"), t("oldFolder.browse")]}
        current={browsing ? 2 : 1}
        reach={browsing ? 2 : 1}
        label={t("restorePage.oldFolder")}
        onGo={(n) => {
          if (n === 1 && browsing) disconnect();
        }}
      />

      {!browsing && (
        <>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <span className="flex items-center gap-1.5 text-sm text-carbon-text">
              {t("recovery.foreignLocation")}
              <InfoBubble tip={t("recovery.foreignLocationHint")} />
            </span>
            <Selector
              items={[
                { id: "folder", label: t("oldFolder.folder") },
                { id: "remote", label: t("oldFolder.remote") },
              ]}
              label={t("recovery.foreignLocation")}
              select="one"
              active={remote ? "remote" : "folder"}
              onChange={(id) => setRemote(id === "remote")}
              inline
            />
          </div>
          {remote ? (
            <>
              <input
                aria-label={t("oldFolder.remote")}
                value={url}
                spellCheck={false}
                autoComplete="off"
                onChange={(e) => setUrl(e.target.value)}
                placeholder={t("oldFolder.remotePlaceholder")}
                dir="ltr"
                className={`${FIELD} text-start`}
              />
              <p className="rounded-card bg-carbon-surface2 px-3 py-2.5 text-xs leading-relaxed text-carbon-textSub">
                {t("recovery.foreignCredsIntro")}
              </p>
              <div className="grid gap-3 sm:grid-cols-2">
                {credField("s3KeyId", "recovery.foreignS3KeyId")}
                {secretField("s3Secret", "recovery.foreignS3Secret", revealSecret)}
                {credField("s3Region", "recovery.foreignS3Region")}
                {credField("restUser", "recovery.foreignRestUser", "recovery.foreignRestHint")}
                {secretField("restPassword", "recovery.foreignRestPassword", revealPassword)}
              </div>
            </>
          ) : (
            <FolderBrowser
              label={t("oldFolder.folder")}
              value={folder}
              hostMountRoot={hostMountRoot}
              onChange={setFolder}
              hint={t("oldFolder.folderHint")}
            />
          )}
          <label className="flex flex-col gap-1 text-xs text-carbon-textSub">
            <span className="flex items-center gap-1">
              {t("recovery.foreignKey")}
              <InfoBubble tip={t("recovery.foreignKeyHint")} />
            </span>
            <RevealInput
              {...revealKey}
              value={key}
              spellCheck={false}
              autoComplete="off"
              onChange={(e) => setKey(e.target.value)}
              wrapperClassName="w-full"
              className={FIELD}
            />
          </label>
          {connectError && (
            <p className="rounded-card bg-statusFailBgSoft px-3 py-2.5 text-xs leading-relaxed text-statusFail wrap-break-word">
              {connectError}
            </p>
          )}
        </>
      )}

      {browsing && (
        <>
          {expired ? (
            <div className="flex flex-wrap items-center gap-3 rounded-card bg-statusWarnBg px-3 py-2.5 text-xs leading-relaxed text-statusWarn">
              <span className="flex-1">{t("recovery.foreignExpired")}</span>
              <Button label={t("recovery.foreignReconnect")} labelKey="recovery.foreignReconnect" onClick={() => void connect()} busy={connecting} />
            </div>
          ) : (
            openItem === null && (
              <div className="flex flex-wrap items-center gap-3 rounded-card bg-statusOkBg px-3 py-2.5 text-sm text-statusOk">
                <span className="flex-1">{t("recovery.foreignConnected")}</span>
                <Button label={t("recovery.foreignClose")} labelKey="recovery.foreignClose" glyph={<IconClose />} tone="subtle" onClick={disconnect} />
              </div>
            )
          )}
          {empty && <p className="text-sm text-statusWarn">{t("recovery.foreignEmpty")}</p>}
          {groups.map((g) => {
            const shown = openItem === null || g.items.some((item) => openItem === `${g.domain}:${item.name}`);
            return (
              <section key={g.domain} hidden={!shown} className="flex flex-col gap-3 rounded-card bg-carbon-surface2 p-4">
                <h3 className="text-xs font-semibold uppercase tracking-widest text-carbon-textSub">{t(GROUP_LABEL[g.domain])}</h3>
                <div className="flex flex-col divide-y divide-carbon-border/60">
                  {g.items.map((item) => {
                    const id = `${g.domain}:${item.name}`;
                    // Folder sets and dataset trees go into a chosen folder
                    // and overwrite nothing.
                    const collides = g.domain === "containers" || g.domain === "vms";
                    const here = localNames.has((g.domain === "containers" ? "container:" : "vm:") + item.name);
                    return (
                      <ForeignItemRow
                        key={id}
                        domain={g.domain}
                        item={item}
                        session={session}
                        hostMountRoot={hostMountRoot}
                        existsLocally={collides && (!localKnown || here)}
                        collisionKnown={collides && localKnown && here}
                        blocked={otherActive || busyRows > 0}
                        open={openItem === id}
                        hidden={openItem !== null && openItem !== id}
                        footer={footer}
                        onOpen={() => setOpenItem(id)}
                        onBusyChange={onBusyChange}
                        onSessionGone={() => setExpired(true)}
                      />
                    );
                  })}
                </div>
              </section>
            );
          })}
          {/* A dump is named and not offered: it comes back with the restic
              commands the recovery kit carries. */}
          {dumps > 0 && openItem === null && (
            <p className="text-xs leading-relaxed text-carbon-textMuted">{t("recovery.foreignDbDumps", dumps)}</p>
          )}
        </>
      )}
    </InstanceWindow>
  );
}
