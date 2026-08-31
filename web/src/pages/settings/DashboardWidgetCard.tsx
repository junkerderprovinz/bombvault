// DashboardWidgetCard, lifted out of Settings.tsx ([337]).
//
// A move, not a rewrite: the component and its comments are unchanged.

import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { IconTipButton } from "../../components/IconTipButton";
import { InfoBubble } from "../../components/InfoBubble";
import { RevealInput } from "../../components/RevealInput";
import { IconCopy, IconTrash } from "../../components/navGlyphs";
import { disableWidgetToken, generateWidgetToken, getDashboardPlugin, installDashboardPlugin, removeDashboardPlugin } from "../../lib/api";
import { hueVars, rainbowAt } from "../../lib/appearance";
import { copyText } from "../../lib/clipboard";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { useReveal } from "../../lib/useReveal";
import { Card } from "./shared";
import { type CSSProperties, useEffect, useState } from "react";

const DASH_PLUGIN_PLG_URL =
  "https://raw.githubusercontent.com/junkerderprovinz/bombvault-widget/main/plugin/bombvaultwidget.plg";

const DASH_PLUGIN_REPO_URL = "https://github.com/junkerderprovinz/bombvault-widget";

type DashPluginStatus =
  | { kind: "loading" }
  | { kind: "noSsh" }
  | { kind: "absent" }
  | { kind: "installed"; version: string }
  // `output` moved to UnraidTileSection's own local `runErr` (see its doc
  // comment): this "error" kind now means only "the status CHECK itself
  // failed" (refresh()'s own catch), which never carried command output —
  // only a failed install/remove action ever did, and that no longer flows
  // through `status` at all.
  | { kind: "error"; message: string };


export function UnraidTileSection({
  t,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  /** GlimStone follow-up round (jdp, live review, five-escalations-deep
   *  standing rule): this section's own Install/Remove/Retry/copy-URL
   *  buttons had no tie to the enclosing DashboardWidgetCard's hue at all —
   *  threaded straight through from that Card's own single `nextHue()` call
   *  (see its own call site below), the same "one Card, several hue-aware
   *  children share the SAME position" shape CadenceBuilder/TimePicker pairs
   *  elsewhere in this file already use, not a second independent
   *  `nextHue()` call that would land a different position for one visually-
   *  grouped Card. */
  hueIndex?: number;
}) {
  const { push } = useToast();
  const hueOn = hueIndex !== undefined;
  // `status` stays exactly as it was — GlimStone follow-up pass (v8.0.0)
  // audit note: this is a PERSISTENT "is the tile currently installed" fact
  // (plus, on failure, a possibly multi-line command `output` block), not a
  // one-shot completion notice — a poor fit for a 4s, w-80 toast, so it stays
  // as inline status rather than being replaced by one. Toast-on-save sweep
  // (jdp, live review, "a toast every time something auto-saves"): install
  // already pushed a success toast alongside this persistent status, but
  // remove's success and BOTH ops' failures didn't — an asymmetry with no
  // real justification (a failed install/remove is exactly the "auto-save
  // error" case that toast-on-save exists to always surface, quiet mode or
  // not), now fixed to match. The toast is the ephemeral "it worked/it
  // didn't" ping; `status`'s inline banner still separately carries the full
  // multi-line command `output` a toast has no room for.
  const [status, setStatus] = useState<DashPluginStatus>({ kind: "loading" });
  const [busy, setBusy] = useState<"idle" | "install" | "remove">("idle");
  // GlimStone standing rule (jdp, live review, emphatic — "Wenn etwas
  // fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!"): run()
  // below already pushed a fail toast (per this file's own comment above),
  // but never bumped a shake nonce. Naively bumping one alongside the
  // EXISTING `setStatus({kind:"error", ...})` call would be a no-op live —
  // that call switches the whole section to the dedicated "error" branch
  // (a plain Retry button, not Install/Remove) in the SAME React commit as
  // the shake, so the Install/Remove button the shake targets would never
  // actually paint with the class on it (caught live: a code-read alone
  // missed this, a real Playwright render against the deployed container
  // did not). `runErr` decouples "the install/remove ACTION just failed"
  // (we already know the install/absent state — that didn't change, the
  // button should stay put, shake, and show a companion error line) from
  // `status`'s own "error" kind, which now means only "the status CHECK
  // itself failed" (refresh()'s own catch, below — we genuinely don't know
  // if it's installed, hence the full-section takeover + Retry). Same
  // "keep the message off the toast's own persistent multi-line output"
  // shape this component already established for `status.output`.
  const [runErr, setRunErr] = useState<{ message: string; output?: string } | null>(null);
  const [shake, setShake] = useState<Record<string, number>>({});
  function bumpShake(key: string) {
    setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }

  function refresh() {
    getDashboardPlugin()
      .then((r) => {
        if (!r.ok) {
          setStatus({ kind: "error", message: r.error ?? t("settings.error") });
        } else if (!r.sshConfigured) {
          setStatus({ kind: "noSsh" });
        } else if (r.installed) {
          setStatus({ kind: "installed", version: r.version ?? "" });
        } else {
          setStatus({ kind: "absent" });
        }
      })
      .catch((err) => {
        setStatus({
          kind: "error",
          message: err instanceof Error ? err.message : t("settings.error"),
        });
      });
  }

  useEffect(refresh, []); // eslint-disable-line react-hooks/exhaustive-deps -- status check on card mount only

  async function run(op: "install" | "remove") {
    setBusy(op);
    setRunErr(null);
    try {
      const r = await (op === "install" ? installDashboardPlugin() : removeDashboardPlugin());
      if (r.ok) {
        push(op === "install" ? t("settings.dashTileInstallOk") : t("settings.dashTileRemoveOk"), "success");
        refresh();
      } else {
        const message = r.error ?? t("settings.error");
        setRunErr({ message, output: r.output });
        push(message, "fail");
        bumpShake(op);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : t("settings.error");
      setRunErr({ message });
      push(message, "fail");
      bumpShake(op);
    } finally {
      setBusy("idle");
    }
  }

  async function handleCopyUrl() {
    if (await copyText(DASH_PLUGIN_PLG_URL)) {
      push(t("vm.ssh.copied"), "success");
    } else {
      push(t("vm.ssh.copyFailed"), "fail");
    }
  }

  return (
    <div className="flex flex-col gap-3 border-t border-carbon-border pt-4">
      <h3 className="flex items-center gap-1.5 text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
        {t("settings.dashTile")}
        <InfoBubble tip={t("settings.dashTileHint")} />
      </h3>

      {status.kind === "loading" && (
        <span className="text-xs text-carbon-textMuted">{t("settings.dashTileChecking")}</span>
      )}

      {status.kind === "noSsh" && (
        <div className="flex flex-col gap-2">
          <p className="text-xs text-carbon-textSub">{t("settings.dashTileNoSsh")}</p>
          <div className="flex items-start gap-2">
            <code className="flex-1 break-all rounded-control bg-carbon-surface2 p-2 text-xs text-carbon-text">
              {DASH_PLUGIN_PLG_URL}
            </code>
            <Button
              label={t("vm.ssh.copy")}
              labelKey="vm.ssh.copy"
              tone="accent"
              onClick={() => void handleCopyUrl()}
              className={`shrink-0 rounded-control bg-accent px-3 py-2 text-xs font-medium text-accentContrast${hueOn ? " glim-hue" : ""}`}
            />
          </div>
          <p className="text-xs text-carbon-textMuted">{t("settings.dashTileCa")}</p>
        </div>
      )}

      {status.kind === "absent" && (
        <div className="flex flex-col gap-2">
          <span className="text-sm text-carbon-text">{t("settings.dashTileNotInstalled")}</span>
          {/* Transparency BEFORE the call: what Install does, and where the code lives. */}
          <p className="text-xs text-carbon-textMuted">{t("settings.dashTileConfirm")}</p>
          {/* Task 5 (rule 13): was a plain underline-on-hover text link. Task 7:
              same reasoning as vm.ssh.guide's badge above — plain doc-link,
              tone="neutral" not the old fifth hue. */}
          <Badge
            as="a"
            href={DASH_PLUGIN_REPO_URL}
            target="_blank"
            rel="noopener noreferrer"
            tone="neutral"
            size="small"
            className="self-start"
          >
            {t("settings.dashTileRepo")} →
          </Badge>
          <Button
            key={shake.install || 0}
            label={t("settings.dashTileInstall")}
            labelKey="settings.dashTileInstall"
            tone="accent"
            onClick={() => void run("install")}
            disabled={busy !== "idle"}
            hueIndex={hueIndex}
            busy={busy === "install"}
            title={busy === "install" ? t("settings.dashTileInstalling") : undefined}
            className={`self-start${shake.install ? " glim-shake" : ""}`}
          />
          {/* A failed install stays right here (the button never disappears —
              see this component's own `runErr` doc comment above for why) —
              the toast already carries the one-line message; only the
              possibly multi-line command output, when present, needs a
              spot that outlives the toast's few seconds. */}
          {runErr?.output && (
            <pre className="overflow-x-auto rounded-control bg-carbon-background p-2 text-caption leading-snug text-carbon-text whitespace-pre-wrap">
              {runErr.output}
            </pre>
          )}
        </div>
      )}

      {status.kind === "installed" && (
        <div className="flex flex-col gap-2">
          <span className="text-sm text-statusOk">
            ✓{" "}
            {status.version
              ? t("settings.dashTileInstalled").replace("{version}", status.version)
              : t("settings.dashTileInstalledNoV")}
          </span>
          <p className="text-xs text-carbon-textMuted">{t("settings.dashTileInstalledHint")}</p>
          {/* NO bespoke red ink (whole-app sweep): this was
              `text-statusFail` on a neutral fill — the same "a destructive
              control paints itself red" treatment removed from every other
              such control in this pass. Neutral secondary ink now; the label
              ("Kachel entfernen") and the in-flight label are what name the
              action. `glim-shake`/`glim-hue`/hueStyle all survive unchanged —
              behaviour and colour-engine membership, not a bespoke colour. */}
          <Button
            key={shake.remove || 0}
            label={t("settings.dashTileRemove")}
            labelKey="settings.dashTileRemove"
            // Accent ([291]). `danger` exists in the tone table and is unused
            // anywhere in the app, so red here would be this button inventing a
            // convention rather than following one; the confirmation dialog is
            // what guards the removal.
            tone="accent"
            glyph={<IconTrash />}
            onClick={() => void run("remove")}
            disabled={busy !== "idle"}
            hueIndex={hueIndex}
            busy={busy === "remove"}
            title={busy === "remove" ? t("settings.dashTileRemoving") : undefined}
            className={`self-start${shake.remove ? " glim-shake" : ""}`}
          />
          {/* Same "stays put, shows the multi-line output only" treatment as
              the install button above. */}
          {runErr?.output && (
            <pre className="overflow-x-auto rounded-control bg-carbon-background p-2 text-caption leading-snug text-carbon-text whitespace-pre-wrap">
              {runErr.output}
            </pre>
          )}
        </div>
      )}

      {status.kind === "error" && (
        <div className="flex flex-col gap-2">
          <span className="text-xs text-statusFail wrap-break-word">✗ {status.message}</span>
          <Button
            label={t("whatsnew.retry")}
            labelKey="whatsnew.retry"
            tone="neutral"
            onClick={() => {
              setStatus({ kind: "loading" });
              refresh();
            }}
            className={`self-start rounded-control px-3 py-2 text-xs text-carbon-text${hueOn ? " glim-hue" : ""}`}
          />
        </div>
      )}
    </div>
  );
}




export function DashboardWidgetCard({
  t,
  tokenSet,
  onTokenSet,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  tokenSet: boolean;
  onTokenSet: (set: boolean) => void;
  hueIndex?: number;
}) {
  const { push } = useToast();
  const [token, setToken] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  // GlimStone standing rule (jdp, live review, emphatic — "Wenn etwas
  // fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!"): the
  // toast below already fired on every failure, but nothing ever bumped a
  // shake nonce, so the Generate/Regenerate/Disable buttons never played
  // `.glim-shake` — same per-key nonce shape as IntegrityCard's own `shake`
  // state (see its `bumpShake` doc comment). Generate and Disable render as
  // mutually-exclusive buttons (tokenSet ? ... : ...), so one shared key per
  // action is enough — whichever of Generate/Regenerate is on screen when
  // `generate` fails is the one that shakes.
  const [shake, setShake] = useState<Record<string, number>>({});
  function bumpShake(key: string) {
    setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }
  const reveal = useReveal();

  const widgetUrl = token ? `${window.location.origin}/widget?token=${token}` : null;

  // GlimStone follow-up pass (v8.0.0): same migration as FleetSettingsCard's
  // own generate/disable/copy handlers further down — see that card's comment.
  async function handleGenerate() {
    setBusy(true);
    try {
      const r = await generateWidgetToken();
      if (r.ok && r.token) {
        setToken(r.token);
        onTokenSet(true);
      } else {
        push(r.error ?? t("settings.error"), "fail");
        bumpShake("generate");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      bumpShake("generate");
    } finally {
      setBusy(false);
    }
  }

  async function handleDisable() {
    setBusy(true);
    try {
      const r = await disableWidgetToken();
      if (r.ok) {
        setToken(null);
        onTokenSet(false);
      } else {
        push(r.error ?? t("settings.error"), "fail");
        bumpShake("disable");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      bumpShake("disable");
    } finally {
      setBusy(false);
    }
  }

  async function handleCopy() {
    if (!widgetUrl) return;
    if (await copyText(widgetUrl)) {
      push(t("vm.ssh.copied"), "success");
    } else {
      push(t("vm.ssh.copyFailed"), "fail");
    }
  }

  // Button-size/colour-engine sweep (jdp, live review — see VMSSHCard's own
  // identical comment for the full reasoning): Generate/Regenerate/Disable
  // were already at this page's dominant 32px control height but had no tie
  // to this Card's own hueIndex. Computed once, reused below AND threaded
  // into UnraidTileSection (its own children live inside THIS SAME Card, so
  // they share this Card's one rainbow position, not a second independent
  // `nextHue()` call).
  const hueOn = hueIndex !== undefined;
  const hueStyle = hueOn ? (hueVars(rainbowAt(hueIndex)) as CSSProperties) : undefined;

  return (
    <Card title={t("settings.widget")} hint={t("settings.widgetHint")} hueIndex={hueIndex}>

      <ul className="list-disc ps-5 text-xs text-carbon-textSub flex flex-col gap-1">
        <li>{t("settings.widgetHow")}</li>
        <li>{t("settings.widgetAccess")}</li>
        <li>{t("settings.widgetEnglish")}</li>
      </ul>

      {tokenSet ? (
        <div className="flex flex-col gap-1.5">
          <span className="text-xs text-carbon-textSub">{t("settings.widgetToken")}</span>
          {/* Show-once secret: value is only the freshly generated token; a
              stored-but-unknown one renders the cloud.secretSet placeholder.
              The verify/regenerate/disable actions sit on their OWN line
              below the field (design-language.md's reveal-eye rule), not
              beside it in the same row. */}
          <RevealInput
            {...reveal}
            readOnly
            value={token ?? ""}
            placeholder={token ? "" : t("cloud.secretSet")}
            wrapperClassName="w-full"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 bv-field-focus"
          />
          <div className="flex items-center gap-2">
            <Button
              label={t("settings.widgetRegenerate")}
              labelKey="settings.widgetRegenerate"
              tone="neutral"
              onClick={() => void handleGenerate()}
              disabled={busy}
              className={`shrink-0 rounded-control px-3 py-2 text-xs text-carbon-text disabled:opacity-50${
                shake.generate ? " glim-shake" : ""
              }${hueOn ? " glim-hue" : ""}`}
            />
            <Button
              label={t("settings.widgetDisable")}
              labelKey="settings.widgetDisable"
              tone="neutral"
              onClick={() => void handleDisable()}
              disabled={busy}
              className={`shrink-0 rounded-control px-3 py-2 text-xs text-carbon-text disabled:opacity-50${
                shake.disable ? " glim-shake" : ""
              }${hueOn ? " glim-hue" : ""}`}
            />
          </div>
        </div>
      ) : (
        <Button
          label={t("settings.widgetGenerate")}
          labelKey="settings.widgetGenerate"
          tone="accent"
          onClick={() => void handleGenerate()}
          disabled={busy}
          className={`self-start rounded-control bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
            shake.generate ? " glim-shake" : ""
          }${hueOn ? " glim-hue" : ""}`}
        />
      )}

      {tokenSet && !token && (
        <p className="text-xs text-carbon-textMuted">{t("settings.widgetUrlOnce")}</p>
      )}
      {widgetUrl && (
        <>
          <div className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("settings.widgetUrl")}</span>
            <div className="flex items-start gap-2">
              <code className="flex-1 break-all rounded-control bg-carbon-surface2 p-2 text-xs text-carbon-text">
                {widgetUrl}
              </code>
              {/* GlimStone follow-up round (jdp, live review: "Der Widget-
                  URL-kopieren-Button soll ein quadratischer Badge mit Glyph
                  sein"): was a short-text `bg-accent` button — reuses
                  IconCopy (Sidebar.tsx) verbatim, the same glyph VMSSHCard's
                  own two copy badges in this file already use for the
                  identical "copy this value" role. Standing rule (icon
                  badges get engine+tooltip automatically): `.glim-hue` +
                  this Card's own hueStyle wires real colour-engine
                  integration, `tip` carries the exact text the button used
                  to show (`vm.ssh.copy`, unchanged key — the same generic
                  "Kopieren" action every other copy control in this file
                  already uses, not a new one-off string). h-8 w-8 (32px) —
                  the app's ONE square-icon-badge size, identical to Badge's
                  own `size="icon"` stage and to every other square icon
                  control in the app. (This literal predates that stage and
                  happens to already be the right number; it is kept as a
                  literal only because this call site renders IconTipButton
                  directly rather than through Badge. Do not re-derive it from
                  this row's own `<code>` sibling — per-neighbour sizing is the
                  rejected split, see Badge.tsx's "ONE SIZE FOR SQUARE ICON
                  BADGES" block.) bg-accent/text-accentContrast
                  preserved from the button it replaces (this control read as
                  a primary action, unlike VMSSHCard's neutral grey copy
                  badges) — `.glim-hue` recolours that fill to this Card's
                  own rainbow position exactly like every other bg-accent
                  control in this file. */}
              <IconTipButton
                onClick={() => void handleCopy()}
                tip={t("vm.ssh.copy")}
                className={`shrink-0 inline-flex items-center justify-center rounded-control bg-accent h-8 w-8 text-accentContrast hover:opacity-90 transition-opacity${hueOn ? " glim-hue" : ""}`}
                style={hueStyle}
              >
                <IconCopy />
              </IconTipButton>
            </div>
          </div>
          <div className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("settings.widgetPreview")}</span>
            <iframe
              src={widgetUrl}
              title={t("settings.widgetPreview")}
              className="w-full max-w-[560px] h-[300px] rounded-card bg-carbon-surface2"
            />
          </div>
        </>
      )}

      {/* Companion Unraid dashboard-tile plugin (one-click install over SSH).
          hueIndex threaded straight through — its own buttons live inside
          THIS SAME Card, so they share this Card's one rainbow position, not
          a second independent nextHue() call (see its own prop doc). */}
      <UnraidTileSection t={t} hueIndex={hueIndex} />
    </Card>
  );
}
