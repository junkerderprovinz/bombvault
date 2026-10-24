import { useEffect, useState } from "react";
import { patchZFSDataset, previewZFSExcludes } from "../../lib/api";
import type { ZFSDatasetView, ZFSExcludePreviewRow } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { useDebouncedSave } from "../../lib/useDebouncedSave";
import { useToast } from "../../lib/toast";
import { zfsCodeSentence } from "../../lib/zfsCodes";

type T = ReturnType<typeof useT>["t"];

export function ZFSExcludesEditor({ item, t, onSaved }: { item: ZFSDatasetView; t: T; onSaved: () => void }) {
  const { push } = useToast();
  const [text, setText] = useState(item.excludes.join("\n"));
  const [rows, setRows] = useState<ZFSExcludePreviewRow[]>([]);
  const [refusal, setRefusal] = useState("");
  const { debouncedSave } = useDebouncedSave();

  const lines = text.split("\n").map((l) => l.trim()).filter(Boolean);
  const linesKey = lines.join("\n");

  useEffect(() => {
    if (linesKey === "") {
      setRows([]);
      return;
    }
    let alive = true;
    const id = setTimeout(() => {
      previewZFSExcludes(item.id, linesKey.split("\n"))
        .then((res) => {
          if (alive) setRows(res.rows ?? []);
        })
        .catch(() => undefined);
    }, 400);
    return () => {
      alive = false;
      clearTimeout(id);
    };
  }, [item.id, linesKey]);

  function handleChange(next: string) {
    setText(next);
    const list = next.split("\n").map((l) => l.trim()).filter(Boolean);
    debouncedSave(() => {
      void patchZFSDataset(item.id, { excludes: list }).then((res) => {
        if (res.ok) {
          setRefusal("");
          onSaved();
        } else if (res.code) {
          setRefusal(zfsCodeSentence(t, res.code, { names: list }));
        } else {
          push(res.error ?? t("excludes.error"), "fail");
        }
      });
    });
  }

  return (
    <div className="flex flex-col gap-1">
      <textarea
        dir="ltr"
        rows={4}
        value={text}
        onChange={(e) => handleChange(e.target.value)}
        aria-label={t("zfs.excludes")}
        className="rounded-control bg-carbon-surface2 p-2 font-mono text-xs text-carbon-text text-start glim-field-focus"
      />
      {refusal && <p className="text-xs text-statusFail">{refusal}</p>}
      {rows.map((row) =>
        row.everything ? (
          <p key={row.pattern} className="text-caption text-statusFail">
            <span dir="ltr" className="font-mono text-start">{row.pattern}</span>
            {": ⚠ "}
            {t("excludes.excludesEverything")}
          </p>
        ) : (
          <p key={row.pattern} className="text-caption text-carbon-textMuted">
            <span dir="ltr" className="font-mono text-start">{row.pattern}</span>
            {": "}
            {t("zfs.excludeMatches", row.matches)}
            {row.sample.length > 0 && ` (${row.sample.join(", ")})`}
          </p>
        ),
      )}
    </div>
  );
}
