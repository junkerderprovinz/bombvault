import { useEffect, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";

import {
  createZFSReplicaServer,
  getZFSReplicaKey,
  patchZFSReplicaServer,
  testZFSReplicaConnection,
  testZFSReplicaServer,
} from "../../../lib/api";
import type { ZFSCodedEnvelope, ZFSReplicaPool, ZFSReplicaServer } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { tLtr } from "../../../lib/ltrFragments";
import { useToast } from "../../../lib/toast";
import { useTestVerdict } from "../../../lib/useTestVerdict";
import { zfsCodeSentence } from "../../../lib/zfsCodes";
import { Badge } from "../../Badge";
import { Button } from "../../Button";
import { CopyBlock } from "../../CopyBlock";
import { InfoBubble } from "../../InfoBubble";
import { NumberField } from "../../NumberField";
import { Selector } from "../../Selector";
import { TestButton, VerdictLine } from "../../TestButton";
import { IconZFS } from "../../navGlyphs";
import { allowLine, defaultRoot, examplePath, poolLabel } from "./replicaModel";
import { useGroup } from "./replicaStore";

const inputCls = "w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus";

function Field({ label, hint, aside, children }: { label: string; hint?: string; aside?: ReactNode; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
        {label}
        {hint && <InfoBubble tip={hint} />}
      </span>
      {children}
      {aside && <span className="text-caption text-carbon-textMuted">{aside}</span>}
    </div>
  );
}

/** AddReplicaServerDialog connects BombVault to a ZFS server in two steps:
 *  first the address with the key to put there and a test, then the pool and
 *  root the replicas go under. With `server` it changes a stored one. */
export function AddReplicaServerDialog({
  server,
  onClose,
  onSaved,
}: {
  server?: ZFSReplicaServer;
  onClose: () => void;
  onSaved: (id: string) => void;
}) {
  const { t } = useT();
  const { push } = useToast();
  const group = useGroup();
  const [step, setStep] = useState(server ? 2 : 1);
  const [host, setHost] = useState(server?.host ?? "");
  const [user, setUser] = useState(server?.user ?? "root");
  const [port, setPort] = useState(server?.port ?? 22);
  const [pools, setPools] = useState<ZFSReplicaPool[]>([]);
  const [pool, setPool] = useState(server?.pool ?? "");
  const [root, setRoot] = useState(server?.root ?? "");
  const [name, setName] = useState(server?.name ?? "");
  const [publicKey, setPublicKey] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const check = useTestVerdict([host, user, port], t("common.networkError"));

  useEffect(() => {
    getZFSReplicaKey()
      .then((r) => setPublicKey(r.publicKey))
      .catch(() => undefined);
  }, []);

  // A stored server was tested when it was added; its pools come from a fresh
  // look so the free space is current.
  const serverId = server?.id;
  useEffect(() => {
    if (!serverId) return;
    testZFSReplicaServer(serverId)
      .then((r) => {
        if (r.ok) setPools(r.pools);
      })
      .catch(() => undefined);
  }, [serverId]);

  const unchanged = server !== undefined && host === server.host && user === server.user && port === server.port;
  const tested = check.verdict?.ok === true || unchanged;
  const login = user.trim() || "root";
  const ownName = group?.name || "<server>";

  function runTest() {
    if (!host.trim()) {
      setError(t("zfs.replica.add.needHost"));
      return;
    }
    setError(null);
    void check.run(async () => {
      const r = await testZFSReplicaConnection(host.trim(), login, port);
      if (!r.ok) return { ok: false, reason: zfsCodeSentence(t, r.code) };
      setPools(r.pools);
      if (!name.trim()) setName(host.trim());
      return { ok: true };
    });
  }

  function goTo(next: number) {
    if (next === 2 && !tested) {
      setError(t("zfs.replica.add.testFirst"));
      return;
    }
    setError(null);
    setStep(next);
  }

  function pickPool(next: string) {
    setPool(next);
    setRoot(defaultRoot(next));
  }

  async function save() {
    if (!pool) return setError(t("zfs.replica.add.needPool"));
    if (!root.trim()) return setError(t("zfs.replica.add.needRoot"));
    setError(null);
    setSaving(true);
    const body = { name: name.trim() || host.trim(), host: host.trim(), user: login, port, pool, root: root.trim() };
    try {
      let id = server?.id ?? "";
      let res: ZFSCodedEnvelope;
      if (server) {
        res = await patchZFSReplicaServer(server.id, body);
      } else {
        const made = await createZFSReplicaServer(body);
        res = made;
        id = made.id ?? "";
      }
      if (!res.ok) {
        setError(res.code ? zfsCodeSentence(t, res.code) : (res.error ?? t("settings.error")));
        return;
      }
      push(server ? t("folders.saved") : t("zfs.replica.add.added").replace("{name}", () => body.name), "success");
      onSaved(id);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("settings.error"));
    } finally {
      setSaving(false);
    }
  }

  const heading = server ? t("zfs.replica.server.edit") : t("zfs.replica.addServer");
  const poolItems = (pools.length > 0 ? pools : pool ? [{ name: pool, sizeBytes: 0, freeBytes: 0 }] : []).map((p) => ({
    id: p.name,
    label: poolLabel(t, p),
    icon: <IconZFS />,
  }));

  return createPortal(
    <div
      className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4"
      onClick={saving ? undefined : onClose}
    >
      <div className="relative w-full max-w-lg">
        <h2 className="flex items-center px-5">
          <Badge tone="heading" size="heading" wrap>
            {heading}
            <InfoBubble tip={t("zfs.replica.hint")} onAccent />
          </Badge>
        </h2>
        <div
          role="dialog"
          aria-modal="true"
          aria-label={heading}
          onClick={(e) => e.stopPropagation()}
          className="flex max-h-[90vh] w-full flex-col gap-4 overflow-y-auto rounded-card bg-carbon-surface p-5 shadow-2xl"
        >
          <Selector
            label={heading}
            items={[
              { id: "1", label: t("zfs.replica.add.stepConnect") },
              { id: "2", label: t("zfs.replica.add.stepPool"), disabled: !tested },
            ]}
            active={String(step)}
            onChange={(id) => goTo(Number(id))}
          />

          {step === 1 ? (
            <>
              <Field label={t("zfs.replica.server.address")} hint={t("zfs.replica.add.addressHint")}>
                <input
                  type="text"
                  value={host}
                  onChange={(e) => setHost(e.target.value)}
                  aria-label={t("zfs.replica.server.address")}
                  placeholder="192.168.1.30"
                  spellCheck={false}
                  autoComplete="off"
                  dir="ltr"
                  className={inputCls}
                />
              </Field>
              <Field label={t("zfs.replica.add.user")} hint={t("zfs.replica.add.userHint")}>
                <input
                  type="text"
                  value={user}
                  onChange={(e) => setUser(e.target.value)}
                  aria-label={t("zfs.replica.add.user")}
                  spellCheck={false}
                  autoComplete="off"
                  dir="ltr"
                  className={inputCls}
                />
              </Field>
              <Field label={t("zfs.replica.add.port")}>
                <NumberField
                  min={1}
                  max={65535}
                  value={port}
                  aria-label={t("zfs.replica.add.port")}
                  onChange={(e) => setPort(parseInt(e.target.value, 10) || 22)}
                  className={inputCls}
                  wrapperClassName="max-w-40"
                />
              </Field>
              <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
                <span className="text-xs font-semibold text-carbon-textSub">{t("zfs.replica.add.authorize")}</span>
                <p className="text-xs text-carbon-textSub">{tLtr(t, "zfs.replica.add.authorizeHint")}</p>
                {publicKey && <CopyBlock text={publicKey} />}
                {login !== "root" && (
                  <>
                    <p className="text-xs text-carbon-textSub">
                      {t("zfs.replica.add.allowHint").replace("{user}", () => login)}
                    </p>
                    <CopyBlock text={allowLine(login)} />
                  </>
                )}
              </div>
              <VerdictLine verdict={check.verdict} />
              <div className="flex justify-end">
                <TestButton
                  label={t("zfs.replica.server.test")}
                  labelKey="zfs.replica.server.test"
                  test={check}
                  onClick={runTest}
                />
              </div>
            </>
          ) : (
            <>
              <Field label={t("zfs.replica.add.stepPool")} hint={t("zfs.replica.add.poolHint")}>
                <Selector
                  label={t("zfs.replica.add.stepPool")}
                  items={poolItems}
                  active={pool || null}
                  onChange={pickPool}
                  activation="manual"
                />
              </Field>
              <Field
                label={t("zfs.replica.server.root")}
                hint={t("zfs.replica.add.rootHint")}
                aside={
                  root.trim() && (
                    <bdi dir="ltr">
                      {t("zfs.replica.server.example").replace("{path}", () => examplePath(root.trim(), ownName))}
                    </bdi>
                  )
                }
              >
                <input
                  type="text"
                  value={root}
                  onChange={(e) => setRoot(e.target.value)}
                  aria-label={t("zfs.replica.server.root")}
                  spellCheck={false}
                  autoComplete="off"
                  dir="ltr"
                  className={inputCls}
                />
              </Field>
              <Field label={t("zfs.replica.add.name")}>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  aria-label={t("zfs.replica.add.name")}
                  placeholder="Backup-NAS"
                  spellCheck={false}
                  autoComplete="off"
                  className={inputCls}
                />
              </Field>
            </>
          )}

          {error && <p className="text-xs text-statusFail">{error}</p>}

          <div className="flex flex-wrap items-center justify-end gap-2 pt-1">
            <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={onClose} />
            {step === 1 ? (
              <Button
                label={t("zfs.replica.add.next")}
                labelKey="zfs.replica.add.next"
                tone="accent"
                onClick={() => goTo(2)}
              />
            ) : (
              <>
                {!server && (
                  <Button label={t("common.back")} labelKey="common.back" tone="neutral" onClick={() => goTo(1)} />
                )}
                <Button
                  label={server ? t("settings.save") : t("zfs.replica.add.submit")}
                  labelKey={server ? "settings.save" : "zfs.replica.add.submit"}
                  tone="accent"
                  onClick={() => void save()}
                  busy={saving}
                  disabled={saving}
                />
              </>
            )}
          </div>
        </div>
      </div>
    </div>,
    document.body,
  );
}
