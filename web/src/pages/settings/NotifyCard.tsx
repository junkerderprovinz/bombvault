// NotifyCard, lifted out of Settings.tsx ([337]).
//
// That file had grown to 9,841 lines against a 3,242-line runner-up, which
// is not a style problem: every reported issue in Settings began with
// finding the card it belonged to. The components are unchanged - this is a
// move, not a rewrite.

// NotifyCard configures backup notifications (webhook / Matrix / Healthchecks).
// Stored encrypted at rest; the form pre-fills from the saved config and Test
// sends to the CURRENT form values (no save needed).
// NotifyCard's own hint→bubble pass (GlimStone form-engine Phase 2, Task 4):
// this Card, plus the Weekly-digest and Overdue-watchdog Cards further down
// (the whole "notifications" tab — the only complete, self-contained tab
// migrated by that task; every OTHER Settings tab's permanent hint <p>s were
// left untouched, deliberately, same scope discipline as Phase 1 Task 9's
// toast adoption — Task 4 documented its own remainder rather than force a
// same-sitting judgment call on ~80 more sites it hadn't yet triaged), moved
// these 7 disposable-after-first-read hints into
// InfoBubble: three card-level intros — NotifyCard's own, the Weekly-digest
// Card's, and the Overdue-watchdog Card's (all three now Card's own `hint`
// prop) — plus four inline ones: the "scheduled summary" and "notify on
// update" checkbox captions, the Apprise section intro, and the
// per-domain-Healthchecks section intro.
//
// Two hints in THIS card were ORIGINALLY left as permanent text, not
// bubbled, reasoned as reference a user consults again later rather than a
// one-time "what does this do" explainer (the spec's own test):
// notify.unraidHint names the EXACT error string ("libvirt not reachable")
// to ignore when VMs aren't backed up; notify.healthchecksLifecycle
// documents a non-obvious cross-setting interaction (Healthchecks pings
// regardless of the "notify on" policy above it). BOTH are now InfoBubbles —
// jdp's live-review ask, "Info-Texte immer in eine Infobubble," is explicit
// and unconditional, and supersedes both per-instance carve-outs the same
// way: notify.unraidHint moved in an earlier round (see its own ToggleRow
// call site's comment below for that override's full reasoning);
// notify.healthchecksLifecycle moved in the round that also added the
// per-channel enable toggles (see the Healthchecks label's own call site
// below) — each override is a one-line revert back to permanent text if the
// debugging-findability concern turns out to matter once live, not a
// redesign.
//
// GlimStone follow-up pass (v8.0.0): closed out the rest of the file's
// remainder Task 4 flagged above — every other tab's Card-level and
// field-level permanent hint <p>s are now bubbled too, on the exact same
// mechanism (Card's `hint` prop; FolderBrowser gained the identical optional
// `hint` prop for its two Settings.tsx call sites that had one). A small
// family of sites earned the SAME "reference, not a one-time explainer"
// carve-out THIS card's own two originally did (both later overridden — see
// the paragraph above): RcloneCard's pathHint and CloudCard's
// own hint (both name exact Backup Path URL-prefix syntax used on a
// different tab), settings.metricsHint (names the exact /metrics path +
// Authorization header — see its own call site's comment). A fourth,
// jobs.flashNotImplemented, was in this list until the caveat it carried
// stopped being true and the whole string was deleted. One
// site — settings.offsiteHint — was a genuine toss-up between "syntax
// reference" and "already covered by the field's own placeholder + caption"
// and was left as-is with its own comment rather than force that call here.
import { Button } from "../../components/Button";
import { Card, ToggleRow, type SaveState } from "./shared";
import { InfoBubble } from "../../components/InfoBubble";
import { RevealInput } from "../../components/RevealInput";
import { Selector } from "../../components/Selector";
import { getNotify, setNotify, testNotify, type NotifyConfig } from "../../lib/api";
import { tLtr } from "../../lib/ltrFragments";
import { useAdvanced } from "../../lib/advanced";
import { useEffect, useRef, useState } from "react";
import { useReveal } from "../../lib/useReveal";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";

// emptyNotify is the default notification config shown before the saved one loads.
const emptyNotify: NotifyConfig = {
  on: "never",
  webhookEnabled: false,
  webhookUrl: "",
  webhookFormat: "generic",
  matrixEnabled: false,
  matrixHomeserver: "",
  matrixToken: "",
  matrixRoom: "",
  healthchecksUrl: "",
  unraid: false,
  smtpEnabled: false,
  smtpHost: "",
  smtpPort: 587,
  smtpUsername: "",
  smtpPassword: "",
  smtpFrom: "",
  smtpTo: "",
  smtpTls: "starttls",
  appriseEnabled: false,
  appriseUrl: "",
  appriseTags: "",
  scheduledSummary: false,
  notifyOnUpdate: false,
};

export function NotifyCard({
  t,
  platformKind,
  hueIndex,
  channelsHueIndex,
  healthchecksHueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  // The detected/overridden platform.Kind ("unraid" | "generic" | "truenas"),
  // sourced from GET /api/settings' sibling "platform" field. Drives the
  // mismatch banner below the Unraid toggle (code-review fix: a c.Unraid=true
  // + Kind()!=KindUnraid mismatch used to silently disable the feature with
  // no UI trace — see unraidGate's doc comment in internal/api/service.go for
  // why the backend gate itself stays hard rather than trusting the toggle).
  platformKind: string;
  // This function now returns THREE Cards (jdp, live-review: "Können wir die
  // Einstellungen für die Benachrichtigungen und die Kanäle für
  // Benachrichtigungen (Matrix, Apprise, Email) in zwei eigene Cards
  // auftrennen?", then a FOLLOW-UP round splitting Healthchecks out again:
  // "Sollen wir Healthchecks.io-Ping-URL und Prüfungen pro Domäne (erweitert)
  // nicht in eine eigene Card machen?") — the settings Card (trigger
  // condition, the three toggles, the platform-mismatch banner, Test), the
  // channels Card (webhook/Apprise/Matrix/Email), and the Healthchecks Card
  // (global ping URL + per-domain overrides). One shared cfg/persistNotify/
  // debounce underneath (all three Cards edit the same NotifyConfig object),
  // so this stays ONE component/one set of hooks — only the JSX splits,
  // hence three distinct hueIndex props rather than three components each
  // taking their own.
  hueIndex?: number;
  // The channels Card only renders while `advanced` is true, so THIS value
  // must come from a `nextHue()` call made INSIDE the same `tab ===
  // "notifications" && advanced &&` gate the call site guards the Card
  // itself with — never an unconditional call whose result just happens not
  // to get used. See this file's own "hueIndex={nextHue()} inside it fires
  // every render regardless" comment (the SYSTEM tab's Spike Card, and the
  // three Storage-tab Cards it references) for the exact silent-hue-shift
  // bug this guards against: an eager `nextHue()` at the call site burns a
  // slot even with Advanced off, shifting every later tab heading by one.
  channelsHueIndex?: number;
  // The Healthchecks Card (card-split follow-up) is gated identically to the
  // channels Card above (`advanced` only) — same "must come from a nextHue()
  // call made INSIDE the advanced gate" rule, same reasoning, its own
  // independent slot.
  healthchecksHueIndex?: number;
}) {
  const { push } = useToast();
  // Simple mode still gets notify-on-failure via Unraid; the extra channels
  // (webhook/Matrix/Healthchecks/SMTP) are power-user features, so gate those.
  const { advanced } = useAdvanced();
  const [cfg, setCfg] = useState<NotifyConfig>(emptyNotify);
  // Only the setter is needed post-conversion (see persistNotify below) — no
  // Speichern button reads this back anymore, same "only the setters are
  // needed" shape as Settings.tsx's own setDomSaveState.
  const [, setState] = useState<SaveState>("idle");
  // The SMTP password / Matrix token are never sent to the browser; track whether
  // one is stored so the field shows "configured" and a blank submit keeps it.
  const [secretSet, setSecretSet] = useState({ smtp: false, matrix: false });
  const revealMatrixToken = useReveal();
  const revealSmtpPassword = useReveal();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide: "Wenn
  // etwas fehlschlägt soll der Toggle/Button kurz zittern"), closing a gap
  // that survived even this Card's own full-page Speichern-Button sweep
  // (see that comment below): persistNotify already pushed a "fail" toast
  // on a rejected save but never reverted the optimistically-updated field
  // back to what the server actually still holds, nor shook it — an
  // un-reverted field showing a value that never actually saved is worse
  // than a silent failure (it reads as "saved", not "failed"). lastGoodRef
  // tracks the last CONFIRMED-persisted snapshot (the initial getNotify
  // load below, then re-stamped to the exact object `setNotify` just
  // accepted on every successful persistNotify — see that function's own
  // comment for why the WHOLE merged object, not just the touched patch
  // key, is the correct "last good" snapshot here). set()/setImmediate()
  // below both revert their own key to `lastGoodRef.current[k]` and bump
  // `fieldShake[key]` on failure — ONE shared mechanism so every one of
  // this Card's ~25 auto-save fields (8 toggles, the "on" Selector, 2
  // selects, ~15 text/number fields including the 5 per-domain Healthchecks
  // overrides) gets revert+shake for free, the same "fix the shared helper
  // once, every caller benefits" shape autoSaveField/autoSaveToggle already
  // established in SettingsPage's own save() further down this file.
  const lastGoodRef = useRef<NotifyConfig>(emptyNotify);
  const [fieldShake, setFieldShake] = useState<Partial<Record<string, number>>>({});
  function bumpShake(key: string) {
    setFieldShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }
  // Full-page Speichern-Button sweep (jdp, live review, emphatic: "Die
  // Speicher-Buttons sollen in allen Tabs weg. Überall soll es automatisch
  // speichern."): every field below already round-trips a real persisted
  // value (getNotify below), the two secrets (matrixToken/smtpPassword)
  // included via the SAME "blank = keep the stored one" contract every other
  // secret field on this page already uses safely — no draft/cancel shape to
  // protect here, unlike RcloneCard right above. Local debounce mirrors
  // CloudCard's own mechanism (this Card is fully self-contained, no access
  // to SettingsPage's shared debouncedSave).
  const debounceTimers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  function debounced(key: string, run: () => void) {
    const existing = debounceTimers.current[key];
    if (existing) clearTimeout(existing);
    debounceTimers.current[key] = setTimeout(run, 800);
  }

  // loaded gates persistNotify below, exactly like OffsiteWizard's cloudLoaded
  // and for the same reason: setNotify is a FULL REPLACE. Before this gate, a
  // failed or non-ok GET left cfg at emptyNotify with nothing shown, and the
  // very next click posted all ~20 fields as blank. The server then sees an
  // unconfigured, "never" notify config and clears the whole encrypted blob,
  // taking the SMTP password, the Matrix token and healthchecksByDomain with
  // it, while the toast said "saved". No undo, and nothing on screen had ever
  // said the load failed.
  const [loaded, setLoaded] = useState(false);
  const [loadErr, setLoadErr] = useState(false);
  useEffect(() => {
    getNotify()
      .then((r) => {
        if (r.ok && r.notify) {
          const merged = { ...emptyNotify, ...r.notify };
          setCfg(merged);
          // The freshly-loaded server value IS the last-known-good snapshot —
          // a revert before any edit has ever been attempted rolls back to
          // exactly this, same as a fresh page load never shaking anything.
          lastGoodRef.current = merged;
          setLoaded(true);
          setLoadErr(false);
        } else {
          setLoadErr(true);
        }
        setSecretSet({ smtp: !!r.smtpPasswordSet, matrix: !!r.matrixTokenSet });
      })
      .catch(() => setLoadErr(true));
  }, []);

  // persistNotify merges patch onto the CURRENT cfg snapshot and POSTs the
  // whole object — setNotify has no partial-patch form, same "merge onto the
  // freshest local state" shape as CloudCard's own persistPatch (this Card
  // has no OTHER card's concurrent edits to protect against, unlike
  // SettingsPage's own baseline-merging save()). Returns whether the save
  // actually succeeded so set()/setImmediate() below can revert+shake their
  // OWN field on failure — merges the WHOLE accepted `merged` object into
  // lastGoodRef on success, not just the patch's own key: setNotify always
  // POSTs a full replace, so a successful call really did persist every
  // field `merged` carried at that moment (including another field's own
  // not-yet-separately-saved edit, if one was mid-debounce when THIS call
  // fired) — the same "the whole object is now the server's truth" fact
  // toggleDomainEnabled/autoSaveField don't need to reason about, since
  // SettingsPage's own PATCH-shaped save() never sends untouched sibling
  // fields in the first place.
  async function persistNotify(patch: Partial<NotifyConfig>): Promise<boolean> {
    // Never write a full replace built on a state that was never read.
    if (!loaded) {
      push(t("settings.notLoadedNoSave"), "fail");
      return false;
    }
    setState("saving");
    const merged = { ...cfg, ...patch };
    try {
      const r = await setNotify(merged);
      if (r.ok) {
        setState("idle");
        push(t("settings.saved"), "success");
        lastGoodRef.current = merged;
        return true;
      } else {
        setState("idle");
        push(r.error ?? t("settings.error"), "fail");
        return false;
      }
    } catch (err) {
      setState("idle");
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      return false;
    }
  }

  // set — continuously-typed text/number fields: optimistic update + debounce,
  // same shape as every other free-text field on this page. Reverts THIS
  // field to its last-known-good value and bumps its own shake nonce on a
  // failed persist (see lastGoodRef's own doc comment above) — the value the
  // user sees snaps back to what the server actually still holds instead of
  // silently keeping an unsaved edit that only a toast (now gone the moment
  // it auto-dismisses) ever said didn't take.
  function set<K extends keyof NotifyConfig>(k: K, v: NotifyConfig[K]) {
    setCfg((c) => ({ ...c, [k]: v }));
    debounced(String(k), () => {
      void persistNotify({ [k]: v } as Partial<NotifyConfig>).then((ok) => {
        if (!ok) {
          setCfg((c) => ({ ...c, [k]: lastGoodRef.current[k] }));
          bumpShake(String(k));
        }
      });
    });
  }

  // setImmediate — discrete clicks (checkboxes/selects): no debounce, same
  // "single discrete choice, not continuous typing" reasoning
  // autoSaveScheduleField's own header comment gives elsewhere on this page.
  // Same revert+shake on failure as set() above — every toggle/select in
  // this Card routes through here, so this one fix covers all of them.
  function setImmediate<K extends keyof NotifyConfig>(k: K, v: NotifyConfig[K]) {
    setCfg((c) => ({ ...c, [k]: v }));
    void persistNotify({ [k]: v } as Partial<NotifyConfig>).then((ok) => {
      if (!ok) {
        setCfg((c) => ({ ...c, [k]: lastGoodRef.current[k] }));
        bumpShake(String(k));
      }
    });
  }

  async function handleTest() {
    try {
      const r = await testNotify(cfg);
      if (r.ok) {
        push(t("notify.tested"), "success");
      } else {
        push(r.error ?? t("settings.error"), "fail");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
  }

  const inputCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 bv-field-focus-well";
  const selectCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm px-2.5 py-1.5 bv-field-focus-well";
  // selectCardCls (the Card-level sibling of selectCls above, for a select
  // sitting directly on the Card rather than a surface2 panel) was REMOVED
  // here — its one call site, the "on" select, became a Selector (task 1 of
  // this round; see that call site's own comment). No other field in this
  // Card sits directly on the Card surface rather than inside one of the
  // rounded-card bg-carbon-surface2 panels below, so nothing else needs it.
  const labelCls = "flex flex-col gap-1 text-xs text-carbon-textSub";

  return (
    <>
    <Card title={t("notify.title")} hint={t("notify.hint")} hueIndex={hueIndex}>
      {/* A failed read used to be invisible. It has to be on screen, because
          the card refuses to save until it succeeds. */}
      {loadErr && <span className="text-xs text-statusFail">{t("settings.notLoadedNoSave")}</span>}
      {/* "on" select → Selector (jdp, live-review: "Benachrichtigen: nie, nur
          bei Fehler, bei Fehler und Erfolg soll bitte ein horizontaler
          Selektor sein"). `variant="well"` with no `equalWidth` — the SMALL
          scale of the app's one grooved selector (round 7 escalation, jdp,
          live-review: "Du hast keinen richtigen horizontalen Selektor
          gemacht!" — compared live, side by side, against the page's big
          pickers, the earlier plain `variant="chip"` default read as three
          loose separate buttons, not one real Selector control; round 8 then
          folded that round's separate "track" variant back into "well", so
          the two scales cannot drift apart again — see Selector.tsx's own
          file header item 6). Same `size="md"` scale as before — this stays
          a small 3-item control inside a settings form, literally the SAME
          variant+scale as the Integrity Card's own drill-kind Selector
          (search "drill.kindLabel" in this file, converted alongside this
          one in the same round). No `equalWidth`, so it does not take the
          Theme/Shape/Motion pickers' standardized MIN_PINNED_WIDTH box — see
          Selector.tsx's own file header for why that pinning is scoped to
          those larger pickers only.
            A plain `<span>` caption OUTSIDE the Selector, NOT the `<label>`
          this field used to be (Selector.tsx's own header is explicit: a
          `<label>` wrapping a multi-segment control hands its click to the
          first segment and announces that segment's name as the label's own
          name to a screen reader — the same trap the drill-kind Selector
          below in this file already avoids).
            Follow-up (jdp, live-review: "Benachrichtigen bei nie, Fehler,
          Erfolg und Fehler soll ein horizontaler Selektor sein (in gleicher
          Zeile wie 'Benachrichtigung')"): the Selector itself was already
          horizontal (the `select="one"` chip row above), but the wrapping
          div still used `labelCls`'s `flex flex-col gap-1` — that stacks the
          caption ABOVE the control, not beside it, which is a different ask
          than jdp meant. Swapped the wrapper to the exact
          `flex items-center gap-2 flex-wrap` shape the drill-kind Selector
          above already uses for the identical label-then-Selector-in-one-row
          layout (same file, search "drill.kindLabel") — label and Selector
          now sit on one line, wrapping only if the viewport is too narrow to
          fit both. Not `labelCls` anymore, so `text-carbon-textSub` is
          spelled out explicitly on the span to keep this field's caption the
          same color it always had (the drill-kind precedent uses
          `text-carbon-textMuted` instead, a difference already present
          between those two Cards before this change — not something to
          unify here as a drive-by). */}
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs text-carbon-textSub">{t("notify.on")}</span>
        {/* key + conditional .glim-shake (revert+shake sweep, this Card's own
            `set`/`setImmediate` doc comment above): the same remount-to-
            replay mechanism every other failing control in this session
            uses, just on Selector's own `className` (which lands on its
            outer tablist wrapper, Selector.tsx's own `className` prop —
            there is no per-segment target since "on" is one control, not
            three independent ones). */}
        <Selector
          key={fieldShake.on}
          items={[
            { id: "never", label: t("notify.onNever") },
            { id: "failure", label: t("notify.onFailure") },
            { id: "always", label: t("notify.onAlways") },
          ]}
          label={t("notify.on")}
          select="one"
          // `cfg.on || "never"`, NOT bare `cfg.on` (caught live while
          // verifying the round-7 variant change above, on the running
          // test container — a stored config predating this field's
          // introduction round-trips as the empty string, not "never").
          // The backend's own Config.shouldSend (internal/notify/notify.go)
          // already documents "" as functionally identical to "never" (its
          // switch's `default:` case is literally commented "// 'never' or
          // unset") — this was purely a DISPLAY gap: the Selector had no
          // segment to light up for that legacy empty value, so all three
          // read as unselected even though the backend was correctly
          // behaving as "never" the whole time. Inside a real enclosing
          // groove, "nothing selected" reads far more like a broken control
          // than it did under the old faint plain-chip idle fill (an empty
          // groove with no accent segment anywhere in it), so this was worth
          // fixing alongside rather than shipping the more visible
          // regression. Display-only: does not touch the stored `cfg.on`
          // value or `persistNotify`'s own merge — a genuine explicit
          // "never" click still round-trips as the literal string "never",
          // this only supplies a fallback for RENDERING an already-legacy
          // empty value, never for anything this session itself writes.
          active={cfg.on || "never"}
          onChange={(id) => setImmediate("on", id)}
          variant="well"
          className={fieldShake.on ? "glim-shake" : undefined}
        />
      </div>

      {/* #56/#106 toggle sweep (jdp, live-review: "Geplante Läufe
          zusammenfassen, Bei Container-Update benachrichtigen, Unraid-
          Benachrichtigungen sollen Toggles sein"): these three used to be
          raw hand-rolled <input type="checkbox"> + <label> blocks, NOT the
          shared ToggleRow component — so they never picked up ToggleRow's
          own shake/disabled/hue plumbing the rest of this page's toggles
          get for free. Converted to ToggleRow here, matching every other
          toggle on this page. hueIndex 0/1/2: three ToggleRows rendered
          together, one per row, in the same Card — a list by construction
          regardless of whether the switches are logically independent (the
          merged Colors Card's own master/Reactive/Rotate trio and the
          Domains Card's seven rows are the identical shape; see ToggleRow's
          own `hueIndex` doc comment for the full "own local 0-based index
          per group" rule this follows). */}
      <ToggleRow
        label={t("notify.scheduledSummary")}
        hint={t("notify.scheduledSummaryHint")}
        checked={cfg.scheduledSummary}
        onChange={(v) => setImmediate("scheduledSummary", v)}
        hueIndex={0}
        shakeNonce={fieldShake.scheduledSummary}
      />
      <ToggleRow
        label={t("notify.notifyOnUpdate")}
        hint={t("notify.notifyOnUpdateHint")}
        checked={cfg.notifyOnUpdate}
        onChange={(v) => setImmediate("notifyOnUpdate", v)}
        hueIndex={1}
        shakeNonce={fieldShake.notifyOnUpdate}
      />
      {/* Unraid native notifications (delivered over the host SSH connection).
          notify.unraidHint USED to stay permanent text instead of an
          InfoBubble, reasoned as "it names the exact 'libvirt not
          reachable' error string to ignore, which needs to stay findable on
          the page for someone debugging that message, not hidden behind a
          hover target." jdp's live-review ask on this same pass was
          explicit and unconditional — "Info-Texte immer in eine
          Infobubble" — which supersedes that per-instance exception; moved
          into ToggleRow's own `hint` bubble like the two rows above. If
          that debugging-findability concern turns out to matter once this
          is live, it's a one-line revert back to permanent text, not a
          redesign. */}
      <ToggleRow
        label={t("notify.unraid")}
        hint={t("notify.unraidHint")}
        checked={cfg.unraid}
        onChange={(v) => setImmediate("unraid", v)}
        hueIndex={2}
        shakeNonce={fieldShake.unraid}
      />

      {/* Platform-mismatch banner (code-review fix): the toggle above is ON,
          but BombVault's platform detection did not resolve to Unraid, so the
          backend gate (unraidGate, internal/api/service.go) is silently
          keeping every Unraid-only push disabled. Most often a genuinely
          Unraid host whose container is missing the /boot -> /host/boot bind
          mount the template wires up — surfaced here so a user relying only
          on the toggle (never clicking "Send test" below) still finds out. */}
      {cfg.unraid && platformKind !== "unraid" && (
        <div className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
          {tLtr(t, "notify.unraidPlatformMismatch").replace("{platform}", platformKind)}
        </div>
      )}

      {/* Full-page Speichern-Button sweep: the "Speichern" button that used to
          sit beside Test is gone — every field on both this Card and the
          channels Card below now auto-saves itself (debounced text/number,
          immediate checkboxes/selects/toggles). Test keeps working exactly
          as before: it always sends the CURRENT form values (from the SAME
          shared `cfg`, whichever Card each field actually lives in), whether
          or not this Card's own 800ms debounce has already flushed them.
          Stays on THIS Card, not the channels one below, so testing (and
          Unraid notifications generally) keeps working with Advanced off —
          see this function's own header comment on `channelsHueIndex` and
          the "Simple mode still gets notify-on-failure via Unraid" comment
          on `advanced` above for why the channels Card is allowed to
          disappear but this button never is. */}
      <div className="flex items-center gap-3 pt-1 flex-wrap">
        <Button
          label={t("notify.test")}
          labelKey="notify.test"
          // Accent ([289]). Sending a test is the action that tells you the
          // channel above is actually wired up, so it carries the card.
          tone="accent"
          onClick={() => void handleTest()}
        />
      </div>
    </Card>

    {advanced && (
      <Card title={t("notify.channelsTitle")} hint={t("notify.channelsHint")} hueIndex={channelsHueIndex}>
      {/* Webhook (generic JSON / Discord / Slack / Gotify / ntfy). Gets a real
          enable/disable toggle (jdp, live-review: "Matrix, Apprise, Webhook-
          URL sollen alle ein Toggle bekommen wie E-Mail") — same shape as
          Email/SMTP's own smtpEnabled further below: a persisted
          `webhookEnabled` boolean that ALSO gates the backend send
          (internal/notify.Config's WebhookEnabled + its own webhookReady(),
          mirroring smtpReady()'s enabled-AND-fields-set gate exactly).
          `hueIndex={0}` (GlimStone follow-up round, jdp's live review of the
          just-hued Webhook/Matrix/Apprise toggles, explicitly OVERRIDING
          this section's prior "sole toggle in its own single-purpose
          subsection" reasoning: "Ebenso sind die Toggles bei Apprise etc.
          nicht im Regenbogenmodus") - treat Webhook/Apprise/Matrix/SMTP as
          one related GROUP (own local 0-based index per group, ToggleRow's
          own hueIndex doc), the same shape as the Rainbow Card's own three
          toggles (master/Reactive/Rotate - individually hueIndex'd despite
          sitting in visually separate blocks too). Fields hide while off,
          matching SMTP's own established pattern.
            notify.webhookChannel ("Webhook") is a NEW key distinct from
          notify.webhook ("Webhook URL", kept unchanged below on the URL
          field itself) — Matrix and Apprise already had their own
          channel-identity label (notify.matrix/notify.apprise) separate from
          their field labels; Webhook never did before this toggle needed
          one. */}
      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <ToggleRow
          label={t("notify.webhookChannel")}
          checked={cfg.webhookEnabled}
          onChange={(v) => setImmediate("webhookEnabled", v)}
          hueIndex={0}
          shakeNonce={fieldShake.webhookEnabled}
        />
        {cfg.webhookEnabled && (
          <>
            <label className={labelCls}>
              {t("notify.webhook")}
              <input key={fieldShake.webhookUrl} value={cfg.webhookUrl} onChange={(e) => set("webhookUrl", e.target.value)} spellCheck={false}
                placeholder="https://discord.com/api/webhooks/..." dir="ltr"
                className={`${inputCls} text-start${fieldShake.webhookUrl ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.webhookFormat")}
              <select key={fieldShake.webhookFormat} value={cfg.webhookFormat} onChange={(e) => setImmediate("webhookFormat", e.target.value)}
                className={`${selectCls}${fieldShake.webhookFormat ? " glim-shake" : ""}`}>
                <option value="generic">Generic JSON</option>
                <option value="discord">Discord</option>
                <option value="slack">Slack</option>
                <option value="gotify">Gotify</option>
                <option value="ntfy">ntfy</option>
              </select>
            </label>
          </>
        )}
      </div>

      {/* Apprise API: posts to a user-run apprise-api server, unlocking Apprise's
          100+ services without bundling Python. Auto-saves + shares the card's
          Test button like the other channels (full-page Speichern-Button
          sweep — see this Card's own header comment).
            Same real enable/disable toggle as Webhook above (`appriseEnabled`
          — internal/notify.Config's AppriseEnabled + appriseReady()). The
          section's own former header `<span>` + InfoBubble is GONE, folded
          into this ToggleRow's own `hint` prop instead — the exact mechanism
          notify.unraidHint's own override already uses (see this function's
          header comment) — rather than showing "Apprise" twice (once as a
          plain caption, once as the toggle's own required visible label).
          `hueIndex={1}` - see Webhook's own comment above for jdp's
          explicit override of the old "sole toggle in its own subsection"
          reasoning; same Webhook/Apprise/Matrix/SMTP group, next slot. */}
      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <ToggleRow
          label={t("notify.apprise")}
          hint={tLtr(t, "notify.appriseHint")}
          checked={cfg.appriseEnabled}
          onChange={(v) => setImmediate("appriseEnabled", v)}
          hueIndex={1}
          shakeNonce={fieldShake.appriseEnabled}
        />
        {cfg.appriseEnabled && (
          <>
            <label className={labelCls}>
              {t("notify.appriseUrl")}
              <input key={fieldShake.appriseUrl} value={cfg.appriseUrl} onChange={(e) => set("appriseUrl", e.target.value)} spellCheck={false}
                placeholder="http://apprise:8000/notify/bombvault" dir="ltr"
                className={`${inputCls} text-start${fieldShake.appriseUrl ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.appriseTags")}
              <input key={fieldShake.appriseTags} value={cfg.appriseTags} onChange={(e) => set("appriseTags", e.target.value)} spellCheck={false}
                placeholder="backups,homelab" className={`${inputCls}${fieldShake.appriseTags ? " glim-shake" : ""}`} />
            </label>
          </>
        )}
      </div>

      {/* Matrix room push. Same real enable/disable toggle as Webhook/Apprise
          above (`matrixEnabled` — internal/notify.Config's MatrixEnabled,
          folded into matrixReady()'s existing homeserver/token/room gate).
          The section's former plain `<span>` header is gone, replaced by
          this ToggleRow's own required visible label (no hint text existed
          for Matrix to fold in, unlike Apprise above). `hueIndex={2}` - see
          Webhook's own comment above for jdp's explicit override; same
          Webhook/Apprise/Matrix/SMTP group, next slot. */}
      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <ToggleRow
          label={t("notify.matrix")}
          checked={cfg.matrixEnabled}
          onChange={(v) => setImmediate("matrixEnabled", v)}
          hueIndex={2}
          shakeNonce={fieldShake.matrixEnabled}
        />
        {cfg.matrixEnabled && (
          <>
            <label className={labelCls}>
              {t("notify.matrixHomeserver")}
              <input key={fieldShake.matrixHomeserver} value={cfg.matrixHomeserver} onChange={(e) => set("matrixHomeserver", e.target.value)} spellCheck={false}
                placeholder="https://matrix.org" dir="ltr"
                className={`${inputCls} text-start${fieldShake.matrixHomeserver ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.matrixToken")}
              <RevealInput {...revealMatrixToken} key={fieldShake.matrixToken} value={cfg.matrixToken} onChange={(e) => set("matrixToken", e.target.value)} spellCheck={false}
                placeholder={secretSet.matrix ? t("cloud.secretSet") : ""} wrapperClassName="w-full"
                className={`${inputCls}${fieldShake.matrixToken ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.matrixRoom")}
              <input key={fieldShake.matrixRoom} value={cfg.matrixRoom} onChange={(e) => set("matrixRoom", e.target.value)} spellCheck={false}
                placeholder="!abcdef:matrix.org" dir="ltr"
                className={`${inputCls} text-start${fieldShake.matrixRoom ? " glim-shake" : ""}`} />
            </label>
          </>
        )}
      </div>

      {/* Email (SMTP), sent via the configured mail server. Same "same
          component/area, same raw-checkbox anti-pattern" gap as the three
          settings-Card toggles converted in an earlier round (standing rule:
          fix every existing caller of the gap being fixed, not just the
          reported instance) — this was still a hand-rolled
          <input type="checkbox">, the fourth one in this Card.
          `hueIndex={3}` (GlimStone follow-up round, jdp's explicit override of
          the "sole toggle in its own subsection" reasoning that used to sit
          here — see Webhook's own comment above for the full history):
          Webhook/Apprise/Matrix/SMTP are one related GROUP now, four
          independent channel subsections each getting their own stable
          rainbow position (0/1/2/3), not four un-hued singletons.
            MOVED directly above Healthchecks below (jdp, live-review: "E-Mail
          soll über Healthchecks-Ping-URL verschoben werden") — was the LAST
          section in this Card; the new order is Webhook/Apprise/Matrix/
          Email, with Healthchecks(-global+per-domain) now its OWN separate
          Card below (card-split follow-up) rather than a trailing section
          here. */}
      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <ToggleRow
          label={t("notify.smtp")}
          checked={cfg.smtpEnabled}
          onChange={(v) => setImmediate("smtpEnabled", v)}
          hueIndex={3}
          shakeNonce={fieldShake.smtpEnabled}
        />
        {cfg.smtpEnabled && (
          <>
            <label className={labelCls}>
              {t("notify.smtpHost")}
              <input key={fieldShake.smtpHost} value={cfg.smtpHost} onChange={(e) => set("smtpHost", e.target.value)} spellCheck={false}
                placeholder="smtp.example.com" dir="ltr" className={`${inputCls} text-start${fieldShake.smtpHost ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpPort")}
              <input key={fieldShake.smtpPort} value={cfg.smtpPort} onChange={(e) => set("smtpPort", Number(e.target.value) || 0)} spellCheck={false}
                type="number" placeholder="587" className={`${inputCls}${fieldShake.smtpPort ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpTls")}
              <select key={fieldShake.smtpTls} value={cfg.smtpTls} onChange={(e) => setImmediate("smtpTls", e.target.value)}
                className={`${selectCls}${fieldShake.smtpTls ? " glim-shake" : ""}`}>
                <option value="starttls">STARTTLS</option>
                <option value="tls">TLS (implicit)</option>
                <option value="none">None</option>
              </select>
            </label>
            <label className={labelCls}>
              {t("notify.smtpUser")}
              <input key={fieldShake.smtpUsername} value={cfg.smtpUsername} onChange={(e) => set("smtpUsername", e.target.value)} spellCheck={false}
                dir="ltr" className={`${inputCls} text-start${fieldShake.smtpUsername ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpPass")}
              <RevealInput {...revealSmtpPassword} key={fieldShake.smtpPassword} value={cfg.smtpPassword} onChange={(e) => set("smtpPassword", e.target.value)} spellCheck={false}
                placeholder={secretSet.smtp ? t("cloud.secretSet") : ""} wrapperClassName="w-full"
                className={`${inputCls}${fieldShake.smtpPassword ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpFrom")}
              <input key={fieldShake.smtpFrom} value={cfg.smtpFrom} onChange={(e) => set("smtpFrom", e.target.value)} spellCheck={false}
                placeholder="bombvault@example.com" dir="ltr" className={`${inputCls} text-start${fieldShake.smtpFrom ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpTo")}
              <input key={fieldShake.smtpTo} value={cfg.smtpTo} onChange={(e) => set("smtpTo", e.target.value)} spellCheck={false}
                placeholder="admin@example.com" dir="ltr" className={`${inputCls} text-start${fieldShake.smtpTo ? " glim-shake" : ""}`} />
            </label>
          </>
        )}
      </div>
      </Card>
    )}

    {/* Healthchecks Card (card-split follow-up, jdp: "Sollen wir
        Healthchecks.io-Ping-URL und Prüfungen pro Domäne (erweitert) nicht in
        eine eigene Card machen?") — SPLIT OUT of the channels Card above into
        its own standalone Card, own heading, own hueIndex, matching this
        file's established per-topic Card-splitting pattern (the same move
        NotifyCard's own header comment documents for the settings/channels
        split one round earlier). Same `advanced &&` gate as the channels
        Card (Healthchecks is itself an advanced-only feature, same as
        Webhook/Matrix/Apprise/SMTP above it), same shared cfg/persistNotify.
          Neither section's own JSX changed beyond the wrapping Card — global
        URL + per-domain overrides render exactly as before. */}
    {advanced && (
      <Card title={t("notify.healthchecksTitle")} hueIndex={healthchecksHueIndex}>
      {/* Healthchecks global ping URL.
            notify.healthchecksLifecycle moved from a permanent <p> into an
          InfoBubble attached to this label (jdp, live-review, explicit and
          unconditional: "der Infotext dort in eine Infobubble" — overriding
          the prior "must stay findable on-page for someone debugging an
          unexpected check status" reasoning this Card's own header comment
          used to document verbatim; see that comment for the full
          before/after). One-line revert back to a permanent `<p>` if that
          debugging-findability concern turns out to matter once live.
            InfoBubble nested INSIDE the `<label>`, wrapped in its own
          `<span className="flex items-center gap-1">` — the SAME established
          pattern this file already uses at "cloud.storageClass.label" and
          "flash.zipExport.keepN" (an InfoBubble is a real interactive
          `<button>`, so clicking it activates the bubble, not the label's
          associated `<input>` — verified live against those existing call
          sites, not a new mechanism invented for this one). */}
      <label className={labelCls}>
        <span className="flex items-center gap-1">
          {t("notify.healthchecks")}
          <InfoBubble tip={t("notify.healthchecksLifecycle")} />
        </span>
        <input key={fieldShake.healthchecksUrl} value={cfg.healthchecksUrl} onChange={(e) => set("healthchecksUrl", e.target.value)} spellCheck={false}
          placeholder="https://hc-ping.com/your-uuid" className={`${inputCls}${fieldShake.healthchecksUrl ? " glim-shake" : ""}`} />
      </label>

      {/* Per-domain Healthchecks overrides (advanced). A blank field falls back to the global URL above.
          Revert+shake sweep: this field bypasses set()/setImmediate() (it PATCHes
          one entry of a nested Record, not a top-level NotifyConfig key), so it
          gets its own bespoke copy of the SAME lastGoodRef-revert + bumpShake
          shape those two helpers share — reverting only THIS domain's own map
          entry, not the whole healthchecksByDomain object, so an unrelated
          domain's already-good override never gets clobbered by this one
          failing. Shake keyed on the SAME `hc-${key}` string already used for
          the debounce above, not a second name for the identical field. */}
      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <span className="flex items-center gap-1 text-xs font-medium text-carbon-textSub">
          {t("notify.hcPerDomain")}
          <InfoBubble tip={t("notify.hcPerDomainHint")} />
        </span>
        {(
          [
            ["container", t("nav.containers")],
            ["VM", t("nav.vms")],
            ["flash", t("nav.flash")],
            ["config", t("nav.config")],
            ["files", t("nav.files")],
          ] as const
        ).map(([key, label]) => (
          <label key={key} className={labelCls}>
            {label}
            <input
              key={fieldShake[`hc-${key}`]}
              value={cfg.healthchecksByDomain?.[key] ?? ""}
              onChange={(e) => {
                const v = e.target.value;
                const nextMap = { ...cfg.healthchecksByDomain, [key]: v };
                setCfg((c) => ({ ...c, healthchecksByDomain: nextMap }));
                debounced(`hc-${key}`, () => {
                  void persistNotify({ healthchecksByDomain: nextMap }).then((ok) => {
                    if (!ok) {
                      setCfg((c) => ({
                        ...c,
                        healthchecksByDomain: {
                          ...c.healthchecksByDomain,
                          [key]: lastGoodRef.current.healthchecksByDomain?.[key] ?? "",
                        },
                      }));
                      bumpShake(`hc-${key}`);
                    }
                  });
                });
              }}
              spellCheck={false}
              placeholder="https://hc-ping.com/your-uuid"
              dir="ltr"
              className={`${inputCls} text-start${fieldShake[`hc-${key}`] ? " glim-shake" : ""}`}
            />
          </label>
        ))}
      </div>
      </Card>
    )}
    </>
  );
}

// ReplicateNowButton triggers an on-demand off-site replication for one domain
// (restic copy local→off-site), surfacing the result inline.
// GlimStone follow-up pass (v8.0.0): both this button's "started"/"failed"
// flash AND TestConnectionButton's ok/uninit/fail verdict below moved to
// toasts — same one-shot completion-notice reasoning as the shared save()
// helper further down, just for the off-site tab's per-domain action buttons
// instead of a settings PUT. Each is a single shared component instantiated
// per domain (containers/vms/flash/files), so this migrates every one of
// those call sites at once, the same leverage as the save() helper.
//
// GlimStone follow-up round (jdp, live review of the just-hued text badges:
// "Die Buttons Verbindung testen, Jetzt replizieren, Einrichten, Ziel
// hinzufügen haben farbige Schrift. Können wir die Buttons in quadratische
// Badges mit Glyphen umwandeln?"): the visible "Jetzt replizieren…"/
// "Replizierend…" text is gone, replaced by IconSync (Sidebar.tsx) inside a
// square (`shape="square"`), icon-only (`tip` set — see Badge.tsx's own `tip`
// doc) Badge at `size="icon"` — the app's ONE square-icon-badge size (32px).
// These four off-site badges were `size="field"` (36px), pinned to the
// repo-url `<input>`'s own measured height; sizing each icon badge to its own
// nearest neighbour is exactly the role-based split jdp rejected, so the
// per-neighbour number is gone (see Badge.tsx's "ONE SIZE FOR SQUARE ICON
// BADGES" block). The old busy/idle text swap survives
// as the tooltip's own content instead of the button's visible label — same
// two i18n keys, same swap condition, just read by `tip` instead of
// `children` now. GlimStone follow-up round (jdp's next live review, on
// this same badge: "die sind falsch eingefaerbt, so halb abgedunkelt")
// replaced the `hueIndex`-driven background from a 14%-alpha wash to a full
// solid hue-derived fill — Badge.tsx's own `isIconOnly && tone==="active"`
// branch now swaps the glyph's ink to the computed-contrast
// `text-accentContrast` riding on that solid fill: still a neutral
// (hue-of-its-own-free) ink per "icons carry no colour of their own, only
// the badge does", just no longer the flat `text-carbon-textSub` token that
// was only safe against the old pale wash. See Badge.tsx's own `toneClasses`
// comment for the full before/after and the measured contrast numbers.
