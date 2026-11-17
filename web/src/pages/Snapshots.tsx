// Snapshots is the page of the snapshots the server keeps itself: ZFS, btrfs
// and VM snapshots, one card per dataset, subvolume or VM. They are not
// backups; the page says so, and every action on it changes the server.
import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { Checkbox } from "../components/Checkbox";
import { Note } from "../components/entry/Note";
import { BusyWindow, CreateWindow, HostSnapDetail, RenameWindow } from "../components/hostsnap/HostSnapWindows";
import { PageAction, PageActions } from "../components/PageActions";
import { PageTitle } from "../components/PageTitle";
import { IconAdd, IconPencil, IconTrash } from "../components/Sidebar";
import { IconForward, IconRefresh, IconShieldOff, IconShieldOn } from "../components/glyphs";
import { Rows } from "../components/storage/rows";
import {
  deleteHostSnapshots,
  estimateHostSnapshots,
  holdHostSnapshot,
  listHostSnapshots,
  type HostSnapPlan,
  type HostSnapshot,
  type HostSnapshotList,
} from "../lib/api";
import { humanBytes } from "../lib/forecast";
import {
  codeKey,
  freedEstimate,
  groupSnapshots,
  KIND_LABEL,
  kindStates,
  ownerKey,
  planOf,
  refKey,
  sleepingPools,
  sourceKey,
  sourceLabel,
  stillAsleep,
  wokenPools,
} from "../lib/hostSnapshots";
import { useT } from "../lib/i18n";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { relativeTime } from "../lib/reltime";
import { useToast } from "../lib/toast";
import { useConfirm } from "../lib/useConfirm";
import { useOwnLogin } from "../lib/useOwnLogin";
import { Card, hueCounter } from "./settings/shared";

type T = ReturnType<typeof useT>["t"];

type Open =
  | { kind: "create"; preset?: string }
  | { kind: "rename"; snapshot: HostSnapshot }
  | { kind: "busy"; snapshot: HostSnapshot }
  | null;

function names(pools: { name: string }[]): string {
  return pools.map((p) => p.name).join(", ");
}

function facts(s: HostSnapshot, t: T): string {
  const parts = [s.createdAt > 0 ? relativeTime(t, s.createdAt) : ""];
  parts.push(s.usedBytes >= 0 ? t("hostSnap.row.used").replace("{size}", humanBytes(s.usedBytes)) : t("hostSnap.row.usedUnknown"));
  if (s.newSinceBytes >= 0) parts.push(t("hostSnap.row.newSince").replace("{size}", humanBytes(s.newSinceBytes)));
  return parts.filter(Boolean).join(", ");
}

function SnapshotRow({
  s,
  plans,
  selected,
  onSelect,
  onRename,
  onHold,
  onDelete,
}: {
  s: HostSnapshot;
  plans: HostSnapPlan[];
  selected: boolean;
  onSelect: (on: boolean) => void;
  onRename: () => void;
  onHold: (on: boolean) => void;
  onDelete: () => void;
}) {
  const { t } = useT();
  const owner = ownerKey(s);
  const ownHold = s.holds.includes("bombvault-keep");
  const otherHolds = s.holds.filter((h) => h && h !== "bombvault-keep");
  const plan = planOf(s, plans);
  const ownerTip =
    s.managedBy === "unraid"
      ? t("hostSnap.owner.unraidHint")
      : s.owner === "backup" || s.owner === "replica"
        ? t("hostSnap.owner.managedHint")
        : plan
          ? t("hostSnap.owner.planHint").replace("{cadence}", () => plan.cadence).replace("{keep}", String(plan.keep))
          : undefined;
  return (
    <li className="flex flex-wrap items-center gap-x-2 gap-y-1 py-2 text-xs first:pt-0 last:pb-0">
      {s.caps.delete ? (
        <Checkbox checked={selected} onChange={onSelect} label={t("hostSnap.select").replace("{name}", () => s.label)} className="shrink-0 [&>span:last-child]:sr-only" />
      ) : (
        <span className="w-4 shrink-0" />
      )}
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="flex flex-wrap items-center gap-1.5">
          <span dir="ltr" className="font-mono text-carbon-text wrap-anywhere">
            {s.label}
          </span>
          {owner && (
            <Badge tone="neutral" size="small" tip={ownerTip}>
              {t(owner)}
            </Badge>
          )}
          {ownHold && (
            <Badge tone="active" size="small" tip={t("hostSnap.holdHint")}>
              {t("hostSnap.held")}
            </Badge>
          )}
          {otherHolds.length > 0 && (
            <Badge tone="neutral" size="small">
              {t("hostSnap.heldBy").replace("{tags}", () => otherHolds.join(", "))}
            </Badge>
          )}
        </span>
        <span className="text-carbon-textMuted">{facts(s, t)}</span>
        {s.clones.length > 0 && (
          <span className="text-carbon-textMuted wrap-anywhere">{t("hostSnap.clones").replace("{names}", () => s.clones.join(", "))}</span>
        )}
      </div>
      <span className="ms-auto flex shrink-0 items-center gap-1">
        {s.caps.rename && (
          <Button label={t("hostSnap.rename")} labelKey="hostSnap.rename" glyph={<IconPencil />} variant="icon" glyphOnly onClick={onRename} />
        )}
        {s.caps.hold && (
          <Button
            label={ownHold ? t("hostSnap.release") : t("hostSnap.hold")}
            labelKey={ownHold ? "hostSnap.release" : "hostSnap.hold"}
            glyph={ownHold ? <IconShieldOff /> : <IconShieldOn />}
            variant="icon"
            glyphOnly
            onClick={() => onHold(!ownHold)}
          />
        )}
        {s.caps.delete && (
          <Button label={t("common.delete")} labelKey="common.delete" glyph={<IconTrash />} variant="icon" glyphOnly onClick={onDelete} />
        )}
      </span>
    </li>
  );
}

export function Snapshots() {
  const { t } = useT();
  const navigate = useNavigate();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const ownLogin = useOwnLogin();
  const [list, setList] = useState<HostSnapshotList | null>(null);
  const [failed, setFailed] = useState(false);
  const [wake, setWake] = useState(false);
  const [loading, setLoading] = useState(false);
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [open, setOpen] = useState<Open>(null);

  const load = useCallback((readSleeping: boolean) => {
    setLoading(true);
    listHostSnapshots(readSleeping)
      .then((res) => {
        setFailed(!res.ok);
        if (res.ok) setList(res);
      })
      .catch(() => setFailed(true))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => load(wake), [load, wake]);

  const snapshots = useMemo(() => list?.snapshots ?? [], [list]);
  const groups = useMemo(() => groupSnapshots(snapshots), [snapshots]);
  const plans = list?.plans ?? [];
  const pools = list?.pools ?? [];
  const states = kindStates(list?.kinds);
  const sources = list?.sources ?? [];
  const selected = snapshots.filter((s) => picked.has(refKey(s)));
  const nextHue = hueCounter();

  function refresh() {
    setPicked(new Set());
    load(wake);
  }

  async function remove(targets: HostSnapshot[]) {
    if (targets.length === 0) return;
    let zfsBytes = 0;
    if (targets.some((s) => s.kind === "zfs")) {
      try {
        const est = await estimateHostSnapshots(targets);
        zfsBytes = est.ok ? (est.zfsBytes ?? 0) : 0;
      } catch {
        zfsBytes = 0;
      }
    }
    const freed = freedEstimate(targets, zfsBytes);
    const freedLine =
      freed.bytes === 0 && !freed.complete
        ? t("hostSnap.freed.unknown")
        : t(freed.complete ? "hostSnap.freed.exact" : "hostSnap.freed.floor").replace("{size}", humanBytes(freed.bytes));
    const question = `${t("hostSnap.deleteQuestion", targets.length)} ${freedLine}`;
    if (!(await confirm(question, { confirmKey: "common.delete", confirmTone: "neutral", cancelTone: "accent" }))) return;
    try {
      const res = await deleteHostSnapshots(targets);
      if (!res.ok) {
        if (!ownLogin.handled(res)) push(t(codeKey(res.code)), "fail");
        return;
      }
      const results = res.results ?? [];
      const done = results.filter((r) => r.code === "ok").length;
      const left = results.filter((r) => r.code !== "ok");
      if (done > 0) push(t("hostSnap.deleteDone", done), "success");
      const busy = left.find((r) => r.code === "busy");
      if (busy && left.length === 1) {
        const snap = targets.find((s) => refKey(s) === refKey(busy.ref));
        if (snap) setOpen({ kind: "busy", snapshot: snap });
      } else if (left.length > 0) {
        push(t("hostSnap.deleteLeft", left.length).replace("{reason}", () => t(codeKey(left[0].code))), "fail");
      }
    } catch {
      push(t("hostSnap.code.failed"), "fail");
    }
    refresh();
  }

  async function hold(s: HostSnapshot, on: boolean) {
    try {
      const res = await holdHostSnapshot(s, on);
      if (!res.ok && !ownLogin.handled(res)) push(t(codeKey(res.code)), "fail");
    } catch {
      push(t("hostSnap.code.failed"), "fail");
    }
    refresh();
  }

  const asleep = sleepingPools(pools);
  const woke = wokenPools(pools);
  const stayed = stillAsleep(pools);

  return (
    <div className={PAGE_SHELL_RESPONSIVE}>
      <PageTitle>{t("hostSnap.title")}</PageTitle>
      <p className="text-sm text-carbon-textSub">{t("hostSnap.intro")}</p>

      {failed && <Note tone="fail">{t("hostSnap.code.failed")}</Note>}
      {states.noHost && (
        <Note
          tone="warn"
          action={
            <Button
              label={t("hostSnap.noHostLink")}
              labelKey="hostSnap.noHostLink"
              glyph={<IconForward />}
              tone="subtle"
              onClick={() => navigate("/settings/integrations")}
            />
          }
        >
          {t("hostSnap.noHost")}
        </Note>
      )}
      {states.failed.map((f) => (
        <div key={f.kind} className="flex flex-col gap-1">
          <Note tone="warn">
            {t("hostSnap.failed").replace("{kind}", () => t(KIND_LABEL[f.kind])).replace("{reason}", () => t(codeKey(f.code)))}
          </Note>
          <HostSnapDetail detail={f.detail} />
        </div>
      ))}

      {(asleep.length > 0 || woke.length > 0 || stayed.length > 0) && (
        <Card title={t("hostSnap.asleep.title")} hueIndex={nextHue()}>
          {asleep.length > 0 && (
            <div className="flex flex-wrap items-center gap-2">
              <p className="min-w-0 flex-1 text-sm text-carbon-textSub">{t("hostSnap.asleep.text").replace("{pools}", () => names(asleep))}</p>
              <Button
                label={t("hostSnap.asleep.read")}
                labelKey="hostSnap.asleep.read"
                glyph={<IconRefresh />}
                tone="subtle"
                busy={loading && wake}
                onClick={() => setWake(true)}
              />
            </div>
          )}
          {woke.length > 0 && <Note tone="warn">{t("hostSnap.asleep.woke").replace("{pools}", () => names(woke))}</Note>}
          {stayed.length > 0 && <Note>{t("hostSnap.asleep.stayed").replace("{pools}", () => names(stayed))}</Note>}
        </Card>
      )}

      {list && groups.length === 0 && !states.noHost && <Note>{t("hostSnap.empty")}</Note>}

      {groups.map((g) => {
        const title = `${t(KIND_LABEL[g.kind])}: ${sourceLabel(g.kind, g.fs, g.source, t)}`;
        const canTake = sources.some((s) => sourceKey(s.kind, s.fs, s.source) === g.key && s.canCreate);
        return (
          <Card key={g.key} title={title} hueIndex={nextHue()}>
            <Rows as="ul" label={title}>
              {g.snapshots.map((s) => (
                <SnapshotRow
                  key={refKey(s)}
                  s={s}
                  plans={plans}
                  selected={picked.has(refKey(s))}
                  onSelect={(on) =>
                    setPicked((prev) => {
                      const next = new Set(prev);
                      if (on) next.add(refKey(s));
                      else next.delete(refKey(s));
                      return next;
                    })
                  }
                  onRename={() => setOpen({ kind: "rename", snapshot: s })}
                  onHold={(on) => void hold(s, on)}
                  onDelete={() => void remove([s])}
                />
              ))}
            </Rows>
            {canTake && (
              <div className="flex justify-end">
                <Button
                  label={t("hostSnap.create.title")}
                  labelKey="hostSnap.create.title"
                  glyph={<IconAdd />}
                  tone="subtle"
                  onClick={() => setOpen({ kind: "create", preset: g.key })}
                />
              </div>
            )}
          </Card>
        );
      })}

      {(list?.hiddenSystem ?? 0) > 0 && (
        <p className="text-xs text-carbon-textMuted">{t("hostSnap.hiddenSystem", list?.hiddenSystem ?? 0)}</p>
      )}

      {list && (
        <PageActions>
          {selected.length > 0 && (
            <PageAction label={t("hostSnap.deleteSelected", selected.length)} glyph={<IconTrash />} onClick={() => void remove(selected)} />
          )}
          <PageAction label={t("hostSnap.refresh")} glyph={<IconRefresh />} busy={loading} onClick={refresh} />
          {sources.some((s) => s.canCreate) && (
            <PageAction primary label={t("hostSnap.create.title")} glyph={<IconAdd />} onClick={() => setOpen({ kind: "create" })} />
          )}
        </PageActions>
      )}

      {open?.kind === "create" && (
        <CreateWindow
          sources={sources}
          preset={open.preset}
          onClose={() => setOpen(null)}
          onDone={(name) => {
            setOpen(null);
            push(t("hostSnap.create.done").replace("{name}", () => name), "success");
            refresh();
          }}
        />
      )}
      {open?.kind === "rename" && (
        <RenameWindow
          snapshot={open.snapshot}
          onClose={() => setOpen(null)}
          onDone={() => {
            setOpen(null);
            refresh();
          }}
        />
      )}
      {open?.kind === "busy" && (
        <BusyWindow
          snapshot={open.snapshot}
          onClose={() => setOpen(null)}
          onDone={() => {
            push(t("hostSnap.busy.freed").replace("{name}", () => open.snapshot.label), "success");
            setOpen(null);
            refresh();
          }}
        />
      )}
      {confirmDialog}
      {ownLogin.dialog}
    </div>
  );
}
