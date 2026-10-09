import { useCallback, useEffect, useState } from "react";

import { decideZFSReceiveRequest, listZFSLocalPools, listZFSReceiveRequests, patchZFSReceiveRequest } from "../../../lib/api";
import type { ZFSReceiveDecision, ZFSReceiveRequest, ZFSReplicaKeep, ZFSReplicaPool } from "../../../lib/api";
import { humanBytes } from "../../../lib/forecast";
import { useT } from "../../../lib/i18n";
import { relativeTime } from "../../../lib/reltime";
import { useConfirm } from "../../../lib/useConfirm";
import { useDebouncedSave } from "../../../lib/useDebouncedSave";
import { useToast } from "../../../lib/toast";
import { zfsCodeSentence } from "../../../lib/zfsCodes";
import { Badge } from "../../Badge";
import { Button } from "../../Button";
import { InfoBubble } from "../../InfoBubble";
import { Selector } from "../../Selector";
import { IconCheck } from "../../Sidebar";
import { IconZFS } from "../../navGlyphs";
import { ReplicaKeepField } from "./ReplicaKeepField";
import { defaultRoot, examplePath, poolLabel, unixOf } from "./replicaModel";

const inputCls = "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus";

function senderOf(request: ZFSReceiveRequest): string {
  return request.peerName || request.sourceServer || request.peer;
}

/** useAnswer sends one decision or keep change and turns a refusal into a
 *  toast, so each row only says what happens on success. */
function useAnswer(onDone: () => void) {
  const { t } = useT();
  const { push } = useToast();
  return useCallback(
    async (send: () => Promise<{ ok: boolean; code?: string; error?: string }>): Promise<boolean> => {
      try {
        const res = await send();
        if (!res.ok) {
          push(res.code ? zfsCodeSentence(t, res.code) : (res.error ?? t("settings.error")), "fail");
          return false;
        }
        onDone();
        return true;
      } catch (err) {
        push(err instanceof Error ? err.message : t("settings.error"), "fail");
        return false;
      }
    },
    [onDone, push, t],
  );
}

function Members({ members }: { members: string[] }) {
  const { t } = useT();
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs text-carbon-textSub">{t("zfs.replica.members")}</span>
      <ul aria-label={t("zfs.replica.members")} className="flex flex-col gap-0.5">
        {members.map((m) => (
          <li key={m} dir="ltr" className="font-mono text-xs text-carbon-text text-start wrap-anywhere">
            {m}
          </li>
        ))}
      </ul>
    </div>
  );
}

/** AskedRequest is one request nobody here has answered yet, with the pool,
 *  root and keep rule an Allow would give it. */
function AskedRequest({
  request,
  pools,
  onDecided,
}: {
  request: ZFSReceiveRequest;
  pools: ZFSReplicaPool[] | null;
  onDecided: () => void;
}) {
  const { t } = useT();
  const { push } = useToast();
  const answer = useAnswer(onDecided);
  const [pool, setPool] = useState(request.pool);
  // Null until someone types or picks a pool, so the default can follow the
  // pools once they have loaded.
  const [root, setRoot] = useState<string | null>(request.root || null);
  const [keep, setKeep] = useState<ZFSReplicaKeep>(request.proposedKeep);
  const [busy, setBusy] = useState<"" | "allow" | "refuse">("");
  const [error, setError] = useState<string | null>(null);

  const sender = senderOf(request);
  const shownPool = pool || pools?.[0]?.name || "";
  const shownRoot = root ?? defaultRoot(shownPool);
  const offered = shownPool && pools && !pools.some((p) => p.name === shownPool)
    ? [{ name: shownPool, sizeBytes: 0, freeBytes: 0 }, ...pools]
    : (pools ?? []);

  function pickPool(next: string) {
    setPool(next);
    setRoot(defaultRoot(next));
  }

  async function decide(decision: ZFSReceiveDecision, done: string) {
    setBusy(decision.decision === "allow" ? "allow" : "refuse");
    if (await answer(() => decideZFSReceiveRequest(request.id, decision))) push(done, "success");
    setBusy("");
  }

  function allow() {
    if (!shownPool) return setError(t("zfs.replica.add.needPool"));
    if (!shownRoot.trim()) return setError(t("zfs.replica.add.needRoot"));
    setError(null);
    void decide(
      { decision: "allow", pool: shownPool, root: shownRoot.trim(), keep },
      t("zfs.receive.accepted").replaceAll("{peer}", () => sender).replaceAll("{name}", () => request.item),
    );
  }

  return (
    <div className="flex flex-col gap-3 rounded-card bg-carbon-background p-3">
      <div className="flex flex-col gap-0.5">
        <span className="flex items-center gap-1.5 text-sm font-semibold text-carbon-text">
          {t("zfs.receive.asked").replaceAll("{peer}", () => sender).replaceAll("{name}", () => request.item)}
          <InfoBubble tip={t("zfs.receive.askedHint").replaceAll("{peer}", () => sender)} />
        </span>
        <span className="text-xs text-carbon-textMuted">
          {t("zfs.receive.askedAt").replace("{when}", () => relativeTime(t, unixOf(request.askedAt)))}
        </span>
      </div>

      <Members members={request.members} />

      <div className="flex flex-col gap-1.5">
        <span className="text-xs text-carbon-textSub">{t("zfs.receive.pool")}</span>
        {pools === null ? null : offered.length === 0 ? (
          <p className="text-xs text-carbon-textMuted">{t("zfs.receive.noPools")}</p>
        ) : (
          <Selector
            label={t("zfs.receive.pool")}
            items={offered.map((p) => ({ id: p.name, label: poolLabel(t, p), icon: <IconZFS /> }))}
            active={shownPool || null}
            onChange={pickPool}
            activation="manual"
          />
        )}
      </div>

      <div className="flex flex-col gap-1.5">
        <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
          {t("zfs.replica.server.root")}
          <InfoBubble tip={t("zfs.receive.rootHint")} />
        </span>
        <input
          type="text"
          value={shownRoot}
          onChange={(e) => setRoot(e.target.value)}
          aria-label={t("zfs.replica.server.root")}
          spellCheck={false}
          autoComplete="off"
          dir="ltr"
          className={inputCls}
        />
        {shownRoot.trim() && (
          <span className="text-caption text-carbon-textMuted">
            <bdi dir="ltr">
              {t("zfs.replica.server.example").replace("{path}", () =>
                examplePath(shownRoot.trim(), request.sourceServer, request.members[0] ?? request.item),
              )}
            </bdi>
          </span>
        )}
      </div>

      <ReplicaKeepField
        label={t("zfs.receive.keep")}
        hint={t("zfs.receive.keepHint")}
        keep={keep}
        onChange={setKeep}
      />

      {error && <p className="text-xs text-statusFail">{error}</p>}

      <div className="flex items-center justify-end gap-2 flex-wrap">
        <Button
          label={t("zfs.receive.decline")}
          labelKey="zfs.receive.decline"
          tone="neutral"
          onClick={() => void decide({ decision: "refuse" }, t("zfs.receive.declined").replaceAll("{peer}", () => sender))}
          disabled={busy !== ""}
          busy={busy === "refuse"}
        />
        <Button
          label={t("zfs.receive.allow")}
          labelKey="zfs.receive.allow"
          glyph={<IconCheck />}
          tone="accent"
          onClick={allow}
          disabled={busy !== ""}
          busy={busy === "allow"}
        />
      </div>
    </div>
  );
}

/** ReceiveSlot is a replica this instance takes in: where it lands, when the
 *  last transfer arrived, what stays, and the way to stop taking it. */
function ReceiveSlot({ request, onChanged }: { request: ZFSReceiveRequest; onChanged: () => void }) {
  const { t } = useT();
  const { push } = useToast();
  const answer = useAnswer(onChanged);
  const { confirm, confirmDialog } = useConfirm();
  const { debouncedSave } = useDebouncedSave();
  const [keep, setKeep] = useState(request.keep);
  const [revoking, setRevoking] = useState(false);

  const sender = senderOf(request);

  function changeKeep(next: ZFSReplicaKeep) {
    setKeep(next);
    debouncedSave(() => {
      void answer(() => patchZFSReceiveRequest(request.id, next)).then((ok) => {
        if (!ok) setKeep(request.keep);
      });
    });
  }

  async function revoke() {
    const question = `${t("zfs.receive.revokeQuestion")} ${t("zfs.receive.revokeText")}`.replaceAll("{peer}", sender);
    if (!(await confirm(question, { confirmKey: "zfs.receive.revokeConfirm" }))) return;
    setRevoking(true);
    if (await answer(() => decideZFSReceiveRequest(request.id, { decision: "revoke" }))) {
      push(t("zfs.receive.revoked"), "success");
    }
    setRevoking(false);
  }

  const received = request.lastReceived
    ? t("zfs.receive.lastReceived")
        .replace("{when}", () => relativeTime(t, unixOf(request.lastReceived)))
        .replace("{size}", () => humanBytes(request.bytes))
    : t("zfs.receive.nothingYet");

  return (
    <li className="flex flex-col gap-3 rounded-card bg-carbon-background p-3">
      <div className="flex items-start gap-3 flex-wrap">
        <div className="flex-1 min-w-0 flex flex-col gap-0.5">
          <span className="flex items-center gap-1.5 text-sm text-carbon-text">
            {t("zfs.receive.allowed").replaceAll("{peer}", () => sender).replaceAll("{name}", () => request.item)}
            <InfoBubble tip={t("zfs.receive.allowedHint").replaceAll("{peer}", () => sender)} />
          </span>
          <bdi dir="ltr" className="font-mono text-caption text-carbon-textMuted text-start wrap-anywhere">
            {`${request.root}/${request.sourceServer}`}
          </bdi>
          <span className="text-xs text-carbon-textMuted">
            <span className="glim-num">{received}</span>
            {" · "}
            {t("zfs.receive.allowedAt").replace("{when}", () => relativeTime(t, unixOf(request.decidedAt)))}
          </span>
        </div>
        <Button
          label={t("zfs.receive.revoke")}
          labelKey="zfs.receive.revoke"
          tone="neutral"
          onClick={() => void revoke()}
          disabled={revoking}
          busy={revoking}
        />
      </div>
      <ReplicaKeepField label={t("zfs.receive.keep")} hint={t("zfs.receive.keepHint")} keep={keep} onChange={changeKeep} />
      {confirmDialog}
    </li>
  );
}

/** ZFSReceiveCard is where this instance answers paired instances that want
 *  to send a ZFS replica into one of its pools, and keeps the ones it allowed.
 *  It stays away until the first request arrives. */
export function ZFSReceiveCard() {
  const { t } = useT();
  const [requests, setRequests] = useState<ZFSReceiveRequest[]>([]);
  const [pools, setPools] = useState<ZFSReplicaPool[] | null>(null);

  const reload = useCallback(() => {
    listZFSReceiveRequests()
      .then(setRequests)
      .catch(() => undefined);
  }, []);
  useEffect(reload, [reload]);

  const asked = requests.filter((r) => r.state === "asked");
  const allowed = requests.filter((r) => r.state === "allowed");
  const needsPools = asked.length > 0;

  useEffect(() => {
    if (!needsPools) return;
    listZFSLocalPools()
      .then(setPools)
      .catch(() => setPools([]));
  }, [needsPools]);

  if (asked.length === 0 && allowed.length === 0) return null;

  return (
    <div className="relative glim-notch-card flex flex-col gap-3 bg-carbon-surface rounded-card p-5">
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap>
          {t("zfs.receive.title")}
          <InfoBubble tip={t("zfs.receive.hint")} onAccent />
        </Badge>
      </h2>
      {asked.map((r) => (
        <AskedRequest key={r.id} request={r} pools={pools} onDecided={reload} />
      ))}
      {allowed.length > 0 && (
        <ul aria-label={t("zfs.receive.title")} className="flex flex-col gap-3">
          {allowed.map((r) => (
            <ReceiveSlot key={r.id} request={r} onChanged={reload} />
          ))}
        </ul>
      )}
    </div>
  );
}
