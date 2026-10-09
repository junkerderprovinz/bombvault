import { useEffect, useState } from "react";

import { zfsHostDatasets } from "../../../lib/api";
import type { PullSourceKind, ZFSReplicaKeep } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { InfoBubble } from "../../InfoBubble";
import { SelectField } from "../../SelectField";
import { Selector } from "../../Selector";
import { ReplicaKeepField } from "./ReplicaKeepField";

const inputCls = "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus";

/** PullKindField asks first what a new pull source fetches. */
export function PullKindField({ kind, onChange }: { kind: PullSourceKind; onChange: (kind: PullSourceKind) => void }) {
  const { t } = useT();
  return (
    <div className="flex flex-col gap-1.5">
      <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
        {t("zfs.replica.pull.kind")}
        <InfoBubble tip={t("zfs.replica.pull.kindHint")} />
      </span>
      <Selector
        label={t("zfs.replica.pull.kind")}
        items={[
          { id: "restic", label: t("zfs.replica.pull.restic") },
          { id: "zfs", label: t("zfs.title") },
        ]}
        active={kind}
        onChange={(id) => onChange(id as PullSourceKind)}
        activation="manual"
      />
    </div>
  );
}

export interface PullZfsValue {
  datasets: string[];
  pool: string;
  root: string;
  keep: ZFSReplicaKeep;
}

/** PullZfsFields are what a ZFS pull source needs beyond its instance: the
 *  member's ZFS items, and where on this server their replica lands. */
export function PullZfsFields({
  peerName,
  value,
  onChange,
}: {
  peerName: string;
  value: PullZfsValue;
  onChange: (next: PullZfsValue) => void;
}) {
  const { t } = useT();
  const [pools, setPools] = useState<string[]>([]);
  const [text, setText] = useState(value.datasets.join("\n"));

  useEffect(() => {
    zfsHostDatasets()
      .then((res) => setPools(res.available ? res.datasets.map((d) => d.dataset).filter((d) => !d.includes("/")) : []))
      .catch(() => undefined);
  }, []);

  const offered = value.pool && !pools.includes(value.pool) ? [value.pool, ...pools] : pools;
  const example = value.root && `${value.root}/${peerName || "<peer>"}/${value.datasets[0] ?? "cache/appdata"}`;

  return (
    <>
      <div className="flex flex-col gap-1.5">
        <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
          {t("zfs.replica.pull.datasets")}
          <InfoBubble tip={t("zfs.replica.pull.datasetsHint")} />
        </span>
        <textarea
          dir="ltr"
          rows={3}
          value={text}
          aria-label={t("zfs.replica.pull.datasets")}
          placeholder="cache/appdata"
          spellCheck={false}
          onChange={(e) => {
            setText(e.target.value);
            onChange({ ...value, datasets: e.target.value.split("\n").map((l) => l.trim()).filter(Boolean) });
          }}
          className={`${inputCls} font-mono text-xs text-start`}
        />
        {value.datasets.length === 0 && (
          <span className="text-caption text-carbon-textMuted">{t("zfs.replica.pull.needDataset")}</span>
        )}
      </div>

      <div className="flex flex-col gap-1.5">
        <span className="text-xs text-carbon-textSub">{t("zfs.replica.pull.pool")}</span>
        <SelectField
          value={value.pool}
          label={t("zfs.replica.pull.pool")}
          onChange={(pool) => onChange({ ...value, pool, root: `${pool}/bombvault-replica` })}
          options={offered.map((p) => ({ value: p, label: p }))}
          className={inputCls}
        />
      </div>

      <div className="flex flex-col gap-1.5">
        <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
          {t("zfs.replica.server.root")}
          <InfoBubble tip={t("zfs.replica.pull.rootHint")} />
        </span>
        <input
          type="text"
          dir="ltr"
          value={value.root}
          aria-label={t("zfs.replica.server.root")}
          spellCheck={false}
          autoComplete="off"
          onChange={(e) => onChange({ ...value, root: e.target.value })}
          className={inputCls}
        />
        {example && (
          <span className="text-caption text-carbon-textMuted">
            <bdi dir="ltr">{t("zfs.replica.server.example").replace("{path}", () => example)}</bdi>
          </span>
        )}
      </div>

      <ReplicaKeepField
        label={t("zfs.replica.pull.keep")}
        hint={t("zfs.replica.pull.keepHint")}
        keep={value.keep}
        onChange={(keep) => onChange({ ...value, keep })}
      />

      <p className="rounded-card bg-carbon-surface2 p-3 text-xs text-carbon-textSub">
        {t("zfs.replica.pull.asks").replace("{peer}", () => peerName)}
      </p>
    </>
  );
}
