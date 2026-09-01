// RcloneCard, lifted out of Settings.tsx ([337]).
//
// A MOVE, not a rewrite: the component is byte-identical to what stood
// in Settings.tsx, and it was already module-level and prop-driven, so
// nothing crosses a new seam. See that file's own note for why the cut
// stops here rather than continuing into SettingsPage itself.
import type { SaveState } from "./shared";
import { Button } from "../../components/Button";
import { Card } from "../settings/shared";
import { getRclone, setRclone } from "../../lib/api";
import { useEffect, useState } from "react";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";

export function RcloneCard({
  t,
  hueIndex,
  nested,
}: {
  t: ReturnType<typeof useT>["t"];
  hueIndex?: number;
  /** Passed straight through to Card — see its own `nested` doc. Set by
   *  Recovery's step 3, which renders this Card inside its own step card. */
  nested?: boolean;
}) {
  const { push } = useToast();
  const [remotes, setRemotes] = useState<string[]>([]);
  const [conf, setConf] = useState("");
  const [state, setState] = useState<SaveState>("idle");
  // GlimStone standing rule (jdp, live review, emphatic — "Wenn etwas
  // fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!"), found by
  // this same pass's own proactive sweep (not on the original finding list):
  // this is one of the FOUR documented genuine hold-outs on manual Save (this
  // Card's own file-header comment), the identical shape as CloudCredSetsCard's
  // save() — a fail toast already fired here, but nothing ever bumped a shake
  // nonce for the Save button. Same fix, same mechanism.
  const [shake, setShake] = useState(0);

  function refresh() {
    getRclone()
      .then((r) => {
        if (r.ok) setRemotes(r.remotes ?? []);
      })
      .catch(() => undefined);
  }
  useEffect(() => {
    refresh();
  }, []);

  // GlimStone follow-up pass (v8.0.0): the "saved"/"error" 3000ms inline flash
  // is now a toast, same shape as the shared save() helper further down.
  async function handleSave() {
    setState("saving");
    try {
      const r = await setRclone(conf);
      if (r.ok) {
        setState("idle");
        setConf("");
        refresh();
        push(t("settings.saved"), "success");
      } else {
        setState("idle");
        push(r.error ?? t("settings.error"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      setState("idle");
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      setShake((n) => n + 1);
    }
  }

  return (
    <Card title={t("rclone.title")} hint={t("rclone.hint")} hueIndex={hueIndex} nested={nested}>
      <div className="text-sm text-carbon-text">
        {t("rclone.configured")}:{" "}
        <span dir="ltr" className="font-mono text-start">{remotes.length > 0 ? remotes.join(", ") : "—"}</span>
      </div>
      <textarea
        value={conf}
        onChange={(e) => setConf(e.target.value)}
        spellCheck={false}
        rows={6}
        placeholder={"[b2]\ntype = b2\naccount = ...\nkey = ..."}
        dir="ltr"
        className="rounded-control bg-carbon-surface2 text-carbon-text text-xs font-mono px-3 py-2 bv-field-focus text-start"
      />
      {/* GlimStone follow-up pass (Phase 2 Task 4's remainder): stays permanent
          text, NOT bubbled — it names the exact "rclone:<remote>:<bucket>/path"
          Backup Path syntax, which is the ONLY place that convention is
          documented (PathModeSwitch's own remote-mode placeholder shows only
          s3:/rest: examples, never rclone:). Someone back on the Storage tab
          filling in a Backup Path for a domain they just wired up here needs
          this findable without already knowing to hover an icon on a
          different tab — the same "exact syntax to copy correctly" carve-out
          the task spec calls out by name. */}
      <p className="text-xs text-carbon-textMuted">{t("rclone.pathHint")}</p>
      <div className="flex items-center gap-3 pt-1">
        <Button
          key={shake || 0}
          label={t("rclone.save")}
          labelKey="rclone.save"
          tone="accent"
          onClick={() => void handleSave()}
          disabled={state === "saving" || conf.trim() === ""}
          busy={state === "saving"}
          title={state === "saving" ? t("auth.saving") : undefined}
          className={shake ? "glim-shake" : ""}
        />
      </div>
    </Card>
  );
}
