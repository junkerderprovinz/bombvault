import { useState } from "react";
import { setContainerHooks } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { useDebouncedSave } from "../../lib/useDebouncedSave";

type T = ReturnType<typeof useT>["t"];

// HooksEditor edits the per-container pre/post-backup commands. The caller's
// Selector strip decides whether the pane is open. Both fields save through
// useDebouncedSave, 800ms after the last keystroke, as one
// setContainerHooks(pre, post) call. A failed save does not revert the field:
// a shell command is free text the user may still be typing, so the toast
// reports the failure and the value stays for the next edit. One-click toggles
// such as FoldersEditor's mount switches do revert on failure.
export function HooksEditor({
  name,
  initialPre,
  initialPost,
  open,
  t,
}: {
  name: string;
  initialPre: string;
  initialPost: string;
  open: boolean;
  t: T;
}) {
  const [pre, setPre] = useState(initialPre);
  const [post, setPost] = useState(initialPost);
  const { push } = useToast();
  const { debouncedSave } = useDebouncedSave();

  async function saveHooks(nextPre: string, nextPost: string) {
    try {
      const r = await setContainerHooks(name, nextPre, nextPost);
      if (r.ok) push(t("settings.saved"), "success");
      else push(r.error ?? t("settings.error"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
  }

  const inputCls =
    "rounded-control bg-carbon-surface2 text-carbon-text text-xs font-mono px-2 py-1 glim-field-focus";

  if (!open) return null;

  return (
    <div className="mt-2 rounded-card bg-carbon-background p-3 flex flex-col gap-2">
      <p className="text-xs text-carbon-textMuted">{t("hooks.hint")}</p>
      <label className="flex flex-col gap-1">
        <span className="text-xs text-carbon-textSub">{t("hooks.pre")}</span>
        <input value={pre} onChange={(e) => {
          const nextPre = e.target.value;
          setPre(nextPre);
          debouncedSave(() => void saveHooks(nextPre, post));
        }} spellCheck={false}
          placeholder="redis-cli SAVE" className={inputCls} />
      </label>
      <label className="flex flex-col gap-1">
        <span className="text-xs text-carbon-textSub">{t("hooks.post")}</span>
        <input value={post} onChange={(e) => {
          const nextPost = e.target.value;
          setPost(nextPost);
          debouncedSave(() => void saveHooks(pre, nextPost));
        }} spellCheck={false}
          placeholder="curl -fsS https://hooks.example/done" className={inputCls} />
      </label>
    </div>
  );
}
