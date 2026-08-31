// VMSSHCard, lifted out of Settings.tsx ([337]).
//
// A move, not a rewrite: the component and its comments are unchanged.

// ---------------------------------------------------------------------------
// Settings page
// ---------------------------------------------------------------------------

// VMSSHCard shows BombVault's SSH public key (to authorize on the Unraid host)
// and a connection test. Self-contained: fetches its own data so the large
// SettingsPage doesn't need extra state.
import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { IconCopy } from "../../components/Sidebar";
import { getVMSSH, testVMSSH } from "../../lib/api";
import { copyText } from "../../lib/clipboard";
import { useT } from "../../lib/i18n";
import { tLtr } from "../../lib/ltrFragments";
import { useToast } from "../../lib/toast";
import { Card } from "./shared";
import { useEffect, useState } from "react";

export function VMSSHCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const { push } = useToast();
  const [host, setHost] = useState("");
  const [pub, setPub] = useState("");
  const [testState, setTestState] = useState<"idle" | "testing" | "ok" | "fail">("idle");
  // GlimStone standing rule (jdp, live review, emphatic — "Wenn etwas
  // fehlschlägt soll der Toggle/Button kurz zittern. Systemweit!!"): a failed
  // action shows its message via a TOAST, never as permanent page text, and
  // the button that triggered it replays `.glim-shake` once — the exact
  // pattern this component's own handleCopy/handleCopyCmd a few lines below
  // already use, and IntegrityCard's own run()/runTamperFor() established
  // for the file (see its `bumpShake` doc comment). Replaces the old
  // permanent `testMsg` paragraph next to the Test button with `shake` (a
  // bumped nonce — see ToggleRow's own shakeNonce doc comment for why).
  const [shake, setShake] = useState(0);

  // Ready-to-paste command that authorizes this key on the Unraid host, both for
  // the live session and persistently (Unraid restores root.pubkeys on boot).
  const authorizeCmd = pub
    ? `mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
echo '${pub}' | tee -a /root/.ssh/authorized_keys /boot/config/ssh/root.pubkeys >/dev/null
chmod 600 /root/.ssh/authorized_keys`
    : "";

  useEffect(() => {
    getVMSSH()
      .then((r) => {
        if (r.ok) {
          setHost(r.host ?? "");
          setPub(r.publicKey ?? "");
        }
      })
      .catch(() => undefined);
  }, []);

  async function handleTest() {
    setTestState("testing");
    try {
      const r = await testVMSSH();
      if (r.ok) {
        setTestState("ok");
      } else {
        setTestState("fail");
        push(r.error ?? t("vm.ssh.testFail"), "fail");
        setShake((n) => n + 1);
      }
    } catch {
      setTestState("fail");
      push(t("vm.ssh.testFail"), "fail");
      setShake((n) => n + 1);
    }
  }

  // copyText falls back to execCommand in non-secure contexts (#112). The
  // "Copied" flash used to be a local 2000ms button-label swap
  // (GlimStone form-engine Task 9's copy-feedback candidate); it's now a
  // routine (quiet-mode-suppressible) toast instead — see lib/toast.tsx.
  async function handleCopy() {
    if (await copyText(pub)) {
      push(t("vm.ssh.copied"), "success");
    } else {
      // "failures always surface" (design-language.md) — copyText() only
      // returns false when BOTH the Clipboard API and the execCommand
      // fallback failed, so this is a real, user-actionable failure, not
      // routine noise a quiet-mode user would want suppressed.
      push(t("vm.ssh.copyFailed"), "fail");
    }
  }

  async function handleCopyCmd() {
    if (await copyText(authorizeCmd)) {
      push(t("vm.ssh.copied"), "success");
    } else {
      push(t("vm.ssh.copyFailed"), "fail");
    }
  }

  // GlimStone follow-up round (jdp, live review, five-escalations-deep
  // standing rule — "IMMER alles in die Farb- und Formengine integrieren"):
  // this Card's own two copy badges and its Test button were plain
  // `bg-carbon-surface2/3` controls with zero tie to this Card's own
  // `hueIndex` — flat regardless of rainbow mode, exactly the gap rule 1
  // exists to close. Same mechanism ToggleRow/Selector/TimePicker/Badge
  // already use: `.glim-hue` + `hueVars(rainbowAt(hueIndex))` inline,
  // computed once here and reused by every control below rather than three
  // separate near-identical blocks. On a neutral `bg-carbon-surface*`
  // control this doesn't repaint the fill (design-language.md's own
  // "icons/neutral surfaces carry no colour of their own" rule, restated at
  // [data-rainbow] .glim-hue-icon's own header in index.css) — it wires the
  // one thing that DOES apply to a neutral control, the same `--focus-ring`
  // redefinition every other `.glim-hue` element already gets, a real,
  // live-verifiable per-item colour a keyboard user actually sees on
  // Tab/click.
  return (
    <Card title={t("vm.ssh.title")} hint={t("vm.ssh.desc")} hueIndex={hueIndex}>
      <div className="flex flex-col gap-3">
        <div className="text-sm text-carbon-text">
          {t("vm.ssh.host")}: <span dir="ltr" className="font-mono text-start">{host || "—"}</span>
        </div>
        <div className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textMuted">{tLtr(t, "vm.ssh.publicKey")}</span>
          <div className="flex items-start gap-2">
            <code className="flex-1 break-all rounded-control bg-carbon-surface2 p-2 text-xs text-carbon-text">
              {pub || "—"}
            </code>
            {/* GlimStone follow-up round (jdp, live review: "beide
                Kopieren-Buttons sollen ein quadratischer Badge mit Glyph
                sein"): was a short-text button (`px-3 py-2 text-xs`) — the
                text is gone, an icon-only trigger needs the real
                `.glim-bubble` tooltip IconTipButton gives, not a bare
                `title=`. h-8 w-8 (32px), NOT guessed: verified live via
                getComputedStyle against this exact row's own `<code>`
                sibling (`p-2 text-xs`, renders 32px tall) — the same
                measured-not-reused discipline, and the identical 32px
                result, FolderBrowser.tsx's own "Durchsuchen" icon button
                already established for its own `px-3 py-1.5` field
                neighbour (see that file's own comment); `rounded-control`
                for the same reason given there, this button sits flush
                beside a `rounded-control` sibling.
                  COLOUR-ENGINE ROUND: both copy badges were still flat
                `bg-carbon-surface3` grey with only `.glim-hue`'s focus-ring
                redefinition tying them to this Card's position — the fill
                itself never moved, which is precisely the gap jdp's standing
                rule exists to close, and the reason the delete badge's own
                grey special-casing was removed a round earlier. Both are real
                `Badge`s now (`as="button" tone="active" shape="square"
                size="icon"`), the same construction ReplicateNowButton/
                TestConnectionButton in this file already use — an icon-only
                `tone="active"` Badge resolves to the full solid
                `bg-accent`/`text-accentContrast` fill, not the pale wash jdp
                rejected as "halb abgedunkelt". `hueIndex` is passed
                explicitly here (rather than left to inherit from the Card's
                own `.glim-hue` subtree, which would compute the identical
                colour) purely because this component already HAS the value in
                scope, matching every other hue-aware control in it. The
                `size="icon"` is the app's one square-
                icon-badge size and is the same 32px these buttons already
                measured to, so the footprint is unchanged. */}
            <Button
              label={t("vm.ssh.copy")}
              labelKey="vm.ssh.copy"
              glyph={<IconCopy />}
              tone="accent"
              onClick={() => void handleCopy()}
              disabled={!pub}
              hueIndex={hueIndex}
              className={"shrink-0"}
            />
          </div>
        </div>

        {/* One-time setup instructions */}
        <div className="rounded-card bg-carbon-surface2 p-3 flex flex-col gap-2">
          <span className="text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
            {t("vm.ssh.setupTitle")}
          </span>
          <ol className="list-decimal ps-5 text-xs text-carbon-textSub flex flex-col gap-1">
            <li>{t("vm.ssh.step1")}</li>
            <li>{t("vm.ssh.step2")}</li>
            <li>{t("vm.ssh.step3")}</li>
          </ol>
          <div className="flex items-start gap-2">
            <pre className="flex-1 overflow-x-auto rounded-control bg-carbon-background p-2 text-caption leading-snug text-carbon-text whitespace-pre">{authorizeCmd || "—"}</pre>
            {/* Same conversion, same measured 32px square, and the same
                colour-engine round — see the pub-key copy button above for the
                full reasoning on both. This `<pre>` sibling wraps to several
                lines (unlike the `<code>` above), but the row stays
                `items-start` so the badge never stretches to match it — it
                keeps its own fixed 32px regardless, exactly as it already did
                as a plain button before either round. */}
            <Button
              label={t("vm.ssh.copyCmd")}
              labelKey="vm.ssh.copyCmd"
              glyph={<IconCopy />}
              tone="accent"
              onClick={() => void handleCopyCmd()}
              disabled={!pub}
              hueIndex={hueIndex}
              className={"shrink-0"}
            />
          </div>
          {/* Task 5 (rule 13): was a plain underline-on-hover text link. Task 7:
              tone was "info" (the old fifth hue) only because it was the
              nearest tone available at the time — a plain doc-link badge
              isn't activity or a state, it's the same kind of element as
              Recovery.tsx's own tone="neutral" reload-link badge. */}
          <Badge
            as="a"
            href="https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md"
            target="_blank"
            rel="noreferrer"
            tone="neutral"
            size="small"
            className="self-start"
          >
            {t("vm.ssh.guide")} →
          </Badge>
        </div>

        <div className="flex items-center gap-3">
          {/* Button-size sweep (jdp, live review: "Die vielen Buttons sind
              unterschiedlich groß"): was `px-3 py-2` — measured live at 36px,
              taller than every sibling secondary button on this tab (Export/
              Choose file/Cancel/Logout, all `px-4 py-1.5` = 32px, the
              dominant control height this whole page already standardizes
              on — see SettingsPortabilityCard's own Export button for the
              same 32px recipe). Now matches that convention exactly, plus
              `.glim-hue` from its own `hueIndex`. */}
          <Button
            key={shake || 0}
            label={t("vm.ssh.test")}
            labelKey="vm.ssh.test"
            // Accent, not neutral ([292]): it is the one thing this card asks
            // you to do, and the card is useless until it has been done once.
            tone="accent"
            onClick={handleTest}
            disabled={testState === "testing"}
            busy={testState === "testing"}
            title={testState === "testing" ? t("vm.ssh.testing") : undefined}
            className={shake ? "glim-shake" : ""}
            hueIndex={hueIndex}
          />
          {testState === "ok" && (
            <span className="text-sm text-statusOk">{t("vm.ssh.testOk")}</span>
          )}
          {/* Minimal fixed glyph, matching IntegrityCard's own "ok"/"fail"
              indicator weight — the actual error text now lives in the toast
              the failure just pushed above, not a permanent inline sentence. */}
          {testState === "fail" && (
            <span className="text-sm text-statusFail">{t("vm.ssh.testFail")}</span>
          )}
        </div>
      </div>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// SettingsPortabilityCard — export this instance's configuration to a JSON file,
// or import a previously exported file. Self-contained: it moves only settings +
// off-site destinations (and, opt-in, the decrypted credentials). Backups,
// snapshots and history are never touched. Import always previews first and asks
// for confirmation before it replaces anything.
// ---------------------------------------------------------------------------

// The machine ids the import summary returns for populated setting areas, mapped
// to their translation keys so the preview lists them human-readably. An unknown
// (future) id falls back to its raw value.
