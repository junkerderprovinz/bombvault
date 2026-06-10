import { useEffect, useState } from "react";
import { listSnapshots, restore } from "../lib/api";
import type { Snapshot } from "../lib/api";
import type { useT } from "../lib/i18n";

type T = ReturnType<typeof useT>["t"];

interface RestorePanelProps {
  name: string;
  t: T;
}

type RestoreState =
  | { phase: "idle" }
  | { phase: "pending" }
  | { phase: "success" }
  | { phase: "error"; message: string };

function SnapshotRow({
  snap,
  containerName,
  t,
}: {
  snap: Snapshot;
  containerName: string;
  t: T;
}) {
  const [confirmed, setConfirmed] = useState(false);
  const [restoreState, setRestoreState] = useState<RestoreState>({ phase: "idle" });

  async function handleRestore() {
    if (!confirmed) return;
    setRestoreState({ phase: "pending" });
    try {
      const res = await restore(containerName, snap.id, true);
      if (res.ok) {
        setRestoreState({ phase: "success" });
      } else {
        setRestoreState({ phase: "error", message: res.error ?? "Restore failed" });
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Network error";
      setRestoreState({ phase: "error", message: msg });
    }
  }

  const isPending = restoreState.phase === "pending";

  return (
    <div className="flex flex-col gap-1 py-2.5 border-b border-carbon-border last:border-0">
      <div className="flex items-center gap-3 text-sm">
        {/* Snapshot ID */}
        <span className="font-mono text-carbon-text text-xs w-20 shrink-0">
          {snap.id.slice(0, 8)}
        </span>
        {/* Time */}
        <span className="text-carbon-textMuted text-xs flex-1">
          {new Date(snap.time).toLocaleString()}
        </span>
        {/* Tags */}
        {snap.tags && snap.tags.length > 0 && (
          <span className="text-carbon-textMuted text-xs hidden sm:block">
            {snap.tags.join(", ")}
          </span>
        )}

        {/* Confirm checkbox */}
        <label className="flex items-center gap-1.5 text-xs text-carbon-textSub cursor-pointer shrink-0">
          <input
            type="checkbox"
            checked={confirmed}
            onChange={(e) => setConfirmed(e.target.checked)}
            disabled={isPending || restoreState.phase === "success"}
            className="rounded border-carbon-border bg-carbon-surface2 text-[#6fdc8c] focus:ring-[#6fdc8c] focus:ring-offset-0"
          />
          {t("restore.confirm")}
        </label>

        {/* Restore button */}
        <button
          onClick={() => void handleRestore()}
          disabled={!confirmed || isPending || restoreState.phase === "success"}
          className="inline-flex items-center gap-1.5 rounded-lg bg-[#3a1c1c] border border-[#5a2a2a] px-2.5 py-1 text-xs font-medium text-[#ff8389] hover:bg-[#4a2020] transition-colors disabled:opacity-40 disabled:cursor-not-allowed shrink-0"
        >
          {isPending ? (
            <>
              <span className="h-2.5 w-2.5 rounded-full border-2 border-[#ff8389] border-t-transparent animate-spin inline-block" />
              Restoring…
            </>
          ) : (
            t("snapshots.restore")
          )}
        </button>
      </div>

      {/* Inline result */}
      {restoreState.phase === "success" && (
        <p className="text-xs text-[#6fdc8c] pl-24">
          Restore complete — container is being recreated.
        </p>
      )}
      {restoreState.phase === "error" && (
        <p className="text-xs text-[#ff8389] pl-24 break-words">
          {restoreState.message}
        </p>
      )}
    </div>
  );
}

export function RestorePanel({ name, t }: RestorePanelProps) {
  const [open, setOpen] = useState(false);
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function toggle() {
    setOpen((prev) => !prev);
  }

  useEffect(() => {
    if (!open) return;
    setLoading(true);
    setError(null);
    listSnapshots(name)
      .then((res) => {
        if (res.ok) setSnapshots(res.snapshots ?? []);
        else setError("Failed to load backups");
      })
      .catch(() => setError("Failed to load backups"))
      .finally(() => setLoading(false));
  }, [open, name]);

  return (
    <div className="mt-1">
      <button
        onClick={toggle}
        className="flex items-center gap-1.5 text-xs text-carbon-textSub hover:text-carbon-text transition-colors"
      >
        <svg
          width="12"
          height="12"
          viewBox="0 0 12 12"
          fill="none"
          className={`transition-transform ${open ? "rotate-90" : ""}`}
        >
          <path
            d="M4 2l4 4-4 4"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
        {t("snapshots.title")}
      </button>

      {open && (
        <div className="mt-2 rounded-lg border border-carbon-border bg-carbon-background px-3 py-1">
          {loading && (
            <p className="py-3 text-xs text-carbon-textMuted">Loading backups…</p>
          )}
          {error && (
            <p className="py-3 text-xs text-[#ff8389]">{error}</p>
          )}
          {!loading && !error && snapshots.length === 0 && (
            <p className="py-3 text-xs text-carbon-textMuted">{t("snapshots.none")}</p>
          )}
          {!loading && snapshots.map((snap) => (
            <SnapshotRow
              key={snap.id}
              snap={snap}
              containerName={name}
              t={t}
            />
          ))}
        </div>
      )}
    </div>
  );
}
