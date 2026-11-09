import { useEffect, useRef, useState, type ReactNode } from "react";
import { Button } from "../../components/Button";
import { DropdownListbox } from "../../components/DropdownListbox";
import { Exceptions } from "../../components/Exceptions";
import { InfoBubble } from "../../components/InfoBubble";
import { NumberField } from "../../components/NumberField";
import { RevealInput } from "../../components/RevealInput";
import { SelectField } from "../../components/SelectField";
import { Selector } from "../../components/Selector";
import { TestButton, VerdictLine } from "../../components/TestButton";
import { SettingRow } from "../../components/storage/rows";
import { KIND_LABEL as SECTION_LABEL, KindGlyph } from "../../components/backups/kinds";
import { IconAdd, IconTrash } from "../../components/navGlyphs";
import {
  REDACTED,
  getNotify,
  setNotify,
  testNotify,
  type NotifyChannel,
  type NotifyConfig,
  type NotifyKind,
} from "../../lib/api";
import type { EntryKind } from "../../lib/backupEntry";
import { useT } from "../../lib/i18n";
import { tLtr } from "../../lib/ltrFragments";
import { useToast } from "../../lib/toast";
import { useReveal } from "../../lib/useReveal";
import { useTestVerdict } from "../../lib/useTestVerdict";
import {
  KINDS,
  KIND_HINT,
  KIND_LABEL,
  PLACEHOLDERS,
  hasSecret,
  newChannel,
  outgoing,
  placeProblems,
  problemKey,
  resultKey,
  saved,
  waitsForSecret,
  type ChannelProblem,
} from "./notifyChannels";
import { PageSentence } from "./SidebarTile";
import { Card, ToggleRow, type SaveState } from "./shared";

type T = ReturnType<typeof useT>["t"];

// The key of each section in a Healthchecks channel's map of its own checks.
const HC_KEY: Record<EntryKind, string> = {
  container: "container",
  vm: "VM",
  flash: "flash",
  files: "files",
  zfs: "zfs",
  config: "config",
};

const PRIORITIES = Array.from({ length: 11 }, (_, i) => ({ value: String(i), label: String(i) }));

const EMPTY: NotifyConfig = { channels: [], scheduledSummary: false, notifyOnUpdate: false };

const inputCls = "rounded-control bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus-well";
const selectCls = "rounded-control bg-carbon-surface3 text-carbon-text text-sm px-2.5 py-1.5 glim-field-focus-well";
// A channel's own fields stand under its switch, set in by a step.
const fieldCls = "flex flex-wrap items-center justify-between gap-x-4 gap-y-1 ps-4 text-sm text-carbon-text";
const fieldInputCls = "w-full sm:w-md";

/** How a channel edit saves: after a pause for typing, at once for a choice. */
type Edit = (update: (c: NotifyChannel) => NotifyChannel, field: string, wait: boolean) => void;

// NotifyCard configures the notifications: what BombVault sends, and the
// channels it sends through. The settings are stored encrypted, each field
// saves itself, and a channel's Test sends to it as the form stands, saved or
// not.
export function NotifyCard({
  t,
  platformKind,
  platformSet = false,
  hueIndex,
  channelsHueIndex,
  sections,
  children,
  tail,
}: {
  t: T;
  // The detected or overridden platform kind ("unraid" | "generic" |
  // "truenas"). The banner of an Unraid channel reads it.
  platformKind: string;
  // Whether the PLATFORM override chose platformKind. Then there was no
  // detection to correct and the banner leaves out the mount advice.
  platformSet?: boolean;
  hueIndex?: number;
  channelsHueIndex?: number;
  /** The sections that are switched on, which can each get a check of their own. */
  sections: EntryKind[];
  /** Rows of the first card that the page keeps itself, after the update row. */
  children?: ReactNode;
  /** Rows of the first card that the page keeps itself, at its end. */
  tail?: ReactNode;
}) {
  const { push } = useToast();
  const [cfg, setCfg] = useState<NotifyConfig>(EMPTY);
  // The form as typed, read by a save that fires after a pause, which the
  // state of the render that scheduled it would not show.
  const cfgRef = useRef<NotifyConfig>(EMPTY);
  // The settings as the server last took them. A failed save puts its field
  // back to this, so the form never shows an edit that did not persist.
  const goodRef = useRef<NotifyConfig>(EMPTY);
  const [, setState] = useState<SaveState>("idle");
  const [problems, setProblems] = useState<ChannelProblem[]>([]);
  // A counter per field. Each control uses its counter as its key, so bumping
  // it remounts the control and replays the .glim-shake animation.
  const [shake, setShake] = useState<Partial<Record<string, number>>>({});
  const bumpShake = (key: string) => setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  const timers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  function debounced(key: string, run: () => void) {
    clearTimeout(timers.current[key]);
    timers.current[key] = setTimeout(run, 800);
  }

  // Saving replaces every channel, so a save on top of a failed load would
  // wipe the stored ones. Nothing saves until the load has succeeded.
  const [loaded, setLoaded] = useState(false);
  const [loadErr, setLoadErr] = useState(false);
  useEffect(() => {
    getNotify()
      .then((r) => {
        if (!r.ok || !r.notify) {
          setLoadErr(true);
          return;
        }
        const got = { ...EMPTY, ...r.notify, channels: r.notify.channels ?? [] };
        cfgRef.current = got;
        setCfg(got);
        goodRef.current = got;
        setLoaded(true);
        setLoadErr(false);
      })
      .catch(() => setLoadErr(true));
  }, []);

  function apply(next: NotifyConfig) {
    cfgRef.current = next;
    setCfg(next);
  }

  /** persist sends the form. "problem" means the server named what is wrong,
   *  which stays on screen next to the value the user typed. */
  async function persist(): Promise<"ok" | "problem" | "fail"> {
    if (!loaded) {
      push(t("settings.notLoadedNoSave"), "fail");
      return "fail";
    }
    const sent = outgoing(cfgRef.current, goodRef.current);
    if (JSON.stringify(sent) === JSON.stringify(goodRef.current)) return "ok";
    setState("saving");
    try {
      const r = await setNotify(sent);
      setState("idle");
      if (r.ok) {
        goodRef.current = sent;
        setProblems([]);
        push(t("settings.saved"), "success");
        return "ok";
      }
      if (r.problems?.length) {
        const placed = placeProblems(sent, r.problems);
        setProblems(placed);
        for (const p of placed) bumpShake(`${p.channel}:${p.field}`);
        return "problem";
      }
      push(r.error ?? t("settings.error"), "fail");
      return "fail";
    } catch (err) {
      setState("idle");
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      return "fail";
    }
  }

  function save(field: string, revert: () => void, wait: boolean) {
    const run = () =>
      void persist().then((outcome) => {
        if (outcome !== "fail") return;
        revert();
        bumpShake(field);
      });
    if (wait) debounced(field, run);
    else run();
  }

  function setSwitch(key: "scheduledSummary" | "notifyOnUpdate", v: boolean) {
    apply({ ...cfgRef.current, [key]: v });
    save(key, () => apply({ ...cfgRef.current, [key]: goodRef.current[key] }), false);
  }

  function editChannel(id: string): Edit {
    return (update, field, wait) => {
      apply({ ...cfgRef.current, channels: cfgRef.current.channels.map((c) => (c.id === id ? update(c) : c)) });
      save(`${id}:${field}`, () => {
        const before = goodRef.current.channels.find((c) => c.id === id);
        const channels = cfgRef.current.channels.flatMap((c) => (c.id !== id ? [c] : before ? [before] : []));
        apply({ ...cfgRef.current, channels });
      }, wait);
    };
  }

  function addChannel(kind: NotifyKind) {
    const added = newChannel(kind, cfgRef.current.channels);
    apply({ ...cfgRef.current, channels: [...cfgRef.current.channels, added] });
    save("add", () => apply({ ...cfgRef.current, channels: cfgRef.current.channels.filter((c) => c.id !== added.id) }), false);
  }

  function removeChannel(id: string) {
    const at = cfgRef.current.channels.findIndex((c) => c.id === id);
    const gone = cfgRef.current.channels[at];
    apply({ ...cfgRef.current, channels: cfgRef.current.channels.filter((c) => c.id !== id) });
    save(`${id}:remove`, () => {
      const channels = [...cfgRef.current.channels];
      channels.splice(at, 0, gone);
      apply({ ...cfgRef.current, channels });
    }, false);
  }

  const [adding, setAdding] = useState(false);
  const addRef = useRef<HTMLButtonElement>(null);

  return (
    <>
      <Card title={t("notify.title")} hint={t("notify.hint")} hueIndex={hueIndex}>
        {/* The card refuses to save until the load succeeds, so a failed load
            has to be on screen. */}
        {loadErr && <span className="text-xs text-statusFail">{t("settings.notLoadedNoSave")}</span>}
        <ToggleRow
          label={t("notify.scheduledSummary")}
          hint={t("notify.scheduledSummaryHint")}
          checked={cfg.scheduledSummary}
          onChange={(v) => setSwitch("scheduledSummary", v)}
          hueIndex={0}
          shakeNonce={shake.scheduledSummary}
        />
        <ToggleRow
          label={t("notify.notifyOnUpdate")}
          hint={t("notify.notifyOnUpdateHint")}
          checked={cfg.notifyOnUpdate}
          onChange={(v) => setSwitch("notifyOnUpdate", v)}
          hueIndex={1}
          shakeNonce={shake.notifyOnUpdate}
        />
        {children}
        {tail}
        <PageSentence text="notify.anomaliesWhere" page="activityLog.typeVerify" to="/settings/integrity#anomalies" />
      </Card>

      <Card
        title={t("notify.channelsTitle")}
        hint={t("notify.channelsHint")}
        hueIndex={channelsHueIndex}
        foot={
          <Button
            ref={addRef}
            label={t("notify.add")}
            labelKey="notify.add"
            glyph={<IconAdd />}
            ariaExpanded={adding}
            onClick={() => setAdding((v) => !v)}
          />
        }
      >
        {cfg.channels.length === 0 ? (
          <p className="text-sm text-carbon-textMuted">{t("notify.empty")}</p>
        ) : (
          <div className="flex flex-col divide-y divide-carbon-border/60 [&>*]:py-3 [&>*:first-child]:pt-0 [&>*:last-child]:pb-0">
            {cfg.channels.map((c, i) => (
              <ChannelBlock
                key={c.id}
                t={t}
                channel={c}
                good={goodRef.current}
                hueIndex={i}
                edit={editChannel(c.id)}
                onRemove={() => removeChannel(c.id)}
                problems={problems.filter((p) => p.channel === c.id)}
                shake={shake}
                sections={sections}
                platformKind={platformKind}
                platformSet={platformSet}
              />
            ))}
          </div>
        )}
        <DropdownListbox open={adding} onClose={() => setAdding(false)} triggerRef={addRef} label={t("notify.add")}>
          {KINDS.map((kind) => (
            <button
              key={kind}
              type="button"
              role="option"
              aria-selected={false}
              onClick={() => {
                setAdding(false);
                addChannel(kind);
              }}
              className="flex w-full items-center px-3 py-2 text-start text-sm text-carbon-textSub transition-colors hover:bg-carbon-hover hover:text-carbon-text"
            >
              <span className="truncate">{t(KIND_LABEL[kind])}</span>
            </button>
          ))}
        </DropdownListbox>
      </Card>
    </>
  );
}

// The paths of the fields each kind draws. A problem on any other path is
// said once under the channel's switch.
const FIELDS: Record<NotifyKind, string[]> = {
  webhook: ["webhook.url", "webhook.method", "webhook.headers", "webhook.body"],
  discord: ["discord.url"],
  slack: ["slack.url"],
  gotify: ["gotify.url", "gotify.token", "gotify.priorityFailure", "gotify.prioritySuccess"],
  ntfy: ["ntfy.url", "ntfy.token"],
  matrix: ["matrix.homeserver", "matrix.token", "matrix.room"],
  smtp: ["smtp.host", "smtp.port", "smtp.tls", "smtp.username", "smtp.password", "smtp.from", "smtp.to"],
  apprise: ["apprise.url", "apprise.tags", "apprise.urls"],
  healthchecks: ["healthchecks.url", "healthchecks.byKey"],
  unraid: [],
};

function under(field: string, path: string): boolean {
  return field === path || field.startsWith(path + ".");
}

function ChannelBlock({
  t,
  channel: c,
  good,
  hueIndex,
  edit,
  onRemove,
  problems,
  shake,
  sections,
  platformKind,
  platformSet,
}: {
  t: T;
  channel: NotifyChannel;
  good: NotifyConfig;
  hueIndex: number;
  edit: Edit;
  onRemove: () => void;
  problems: ChannelProblem[];
  shake: Partial<Record<string, number>>;
  sections: EntryKind[];
  platformKind: string;
  platformSet: boolean;
}) {
  const title = c.name?.trim() || t(KIND_LABEL[c.kind]);
  const hint = KIND_HINT[c.kind];
  const shakeOf = (path: string) => shake[`${c.id}:${path}`];
  const shook = (path: string) => (shakeOf(path) ? "glim-shake" : "");
  const test = useTestVerdict(c, t("settings.error"));
  // A section added as an exception keeps its row while its URL is still blank.
  const [addedHc, setAddedHc] = useState<string[]>([]);

  function handleTest() {
    void test.run(async () => {
      const r = await testNotify(c);
      if (r.ok) return { ok: true };
      const key = resultKey(r.code);
      return { ok: false, reason: key ? t(key) : t("settings.error") };
    });
  }

  function problemLines(path: string): ReactNode {
    return problems
      .filter((p) => under(p.field, path))
      .map((p) => (
        <p key={`${p.field}:${p.code}`} className="ps-4 text-xs text-statusFail">
          {t(problemKey(p.code))}
        </p>
      ));
  }
  const loose = problems.filter((p) => !FIELDS[c.kind].some((path) => under(p.field, path)));

  // text is a typed field of the kind's block.
  function text<K extends keyof NotifyChannel>(block: K, key: string, path = `${String(block)}.${key}`) {
    return {
      value: String((c[block] as unknown as Record<string, unknown> | undefined)?.[key] ?? ""),
      onChange: (e: { target: { value: string } }) =>
        edit((ch) => ({ ...ch, [block]: { ...(ch[block] as object), [key]: e.target.value } }), path, true),
    };
  }

  function field(label: string, path: string, control: ReactNode, note?: ReactNode): ReactNode {
    return (
      <>
        <label className={fieldCls}>
          {label}
          {control}
        </label>
        {note}
        {problemLines(path)}
      </>
    );
  }

  function urlInput(block: keyof NotifyChannel, key: string, placeholder: string) {
    const path = `${String(block)}.${key}`;
    return (
      <input
        key={shakeOf(path)}
        {...text(block, key)}
        spellCheck={false}
        placeholder={placeholder}
        dir="ltr"
        className={`${inputCls} ${fieldInputCls} text-start ${shook(path)}`}
      />
    );
  }

  // secret is a field the server never hands back. Emptied, it keeps the
  // stored one, and a moved address waits for it to be typed again.
  function secret(block: "gotify" | "ntfy" | "matrix" | "smtp", key: string, label: string) {
    const path = `${block}.${key}`;
    const value = String((c[block] as unknown as Record<string, unknown> | undefined)?.[key] ?? "");
    const kept = hasSecret(c, good);
    const waiting = waitsForSecret(c, good);
    const note =
      value === REDACTED ? (
        <p className={`ps-4 text-xs ${waiting ? "text-statusWarn" : "text-carbon-textMuted"}`}>
          {t(waiting ? "notify.secretAgain" : "notify.secretKept")}
        </p>
      ) : null;
    return field(
      label,
      path,
      <SecretInput
        key={shakeOf(path)}
        value={value === REDACTED ? "" : value}
        onChange={(v) =>
          edit((ch) => ({ ...ch, [block]: { ...(ch[block] as object), [key]: v === "" && kept ? REDACTED : v } }), path, true)
        }
        className={shook(path)}
      />,
      note
    );
  }

  let fields: ReactNode = null;
  switch (c.kind) {
    case "discord":
    case "slack":
      fields = field(t("notify.webhook"), `${c.kind}.url`, urlInput(c.kind, "url", c.kind === "discord" ? "https://discord.com/api/webhooks/..." : "https://hooks.slack.com/services/..."));
      break;
    case "ntfy":
      fields = (
        <>
          {field(t("notify.ntfyUrl"), "ntfy.url", urlInput("ntfy", "url", "https://ntfy.sh/my-backups"))}
          {secret("ntfy", "token", t("notify.ntfyToken"))}
        </>
      );
      break;
    case "gotify":
      fields = (
        <div data-new="notify-gotify" className="flex flex-col gap-2">
          {field(t("notify.gotifyUrl"), "gotify.url", urlInput("gotify", "url", "https://gotify.example.com"))}
          {secret("gotify", "token", t("notify.gotifyToken"))}
          {(["priorityFailure", "prioritySuccess"] as const).map((key) => {
            const label = t(key === "priorityFailure" ? "notify.gotifyPriorityFailure" : "notify.gotifyPrioritySuccess");
            const path = `gotify.${key}`;
            return (
              <div key={key} className="contents">
                {field(
                  label,
                  path,
                  <SelectField
                    key={shakeOf(path)}
                    value={String(c.gotify?.[key] ?? (key === "priorityFailure" ? 8 : 5))}
                    onChange={(v) => edit((ch) => ({ ...ch, gotify: { ...ch.gotify!, [key]: Number(v) } }), path, false)}
                    label={label}
                    options={PRIORITIES}
                    className={`${selectCls} ${fieldInputCls} ${shook(path)}`}
                  />
                )}
              </div>
            );
          })}
        </div>
      );
      break;
    case "matrix":
      fields = (
        <>
          {field(t("notify.matrixHomeserver"), "matrix.homeserver", urlInput("matrix", "homeserver", "https://matrix.org"))}
          {secret("matrix", "token", t("notify.matrixToken"))}
          {field(t("notify.matrixRoom"), "matrix.room", urlInput("matrix", "room", "!abcdef:matrix.org"))}
        </>
      );
      break;
    case "smtp":
      fields = (
        <>
          {field(t("notify.smtpHost"), "smtp.host", urlInput("smtp", "host", "smtp.example.com"))}
          {field(
            t("notify.smtpPort"),
            "smtp.port",
            // The steppers read min/max to know when to grey out.
            <NumberField
              key={shakeOf("smtp.port")}
              value={c.smtp?.port || ""}
              onChange={(e) => edit((ch) => ({ ...ch, smtp: { ...ch.smtp!, port: Number(e.target.value) || 0 } }), "smtp.port", true)}
              spellCheck={false}
              min={1}
              max={65535}
              placeholder="587"
              className={`${inputCls} ${fieldInputCls} ${shook("smtp.port")}`}
            />
          )}
          {field(
            t("notify.smtpTls"),
            "smtp.tls",
            <SelectField
              key={shakeOf("smtp.tls")}
              value={c.smtp?.tls || "starttls"}
              onChange={(v) => edit((ch) => ({ ...ch, smtp: { ...ch.smtp!, tls: v } }), "smtp.tls", false)}
              label={t("notify.smtpTls")}
              options={[
                { value: "starttls", label: "STARTTLS" },
                { value: "tls", label: "TLS (implicit)" },
                { value: "none", label: "None" },
              ]}
              className={`${selectCls} ${fieldInputCls} ${shook("smtp.tls")}`}
            />
          )}
          {field(t("notify.smtpUser"), "smtp.username", urlInput("smtp", "username", ""))}
          {secret("smtp", "password", t("notify.smtpPass"))}
          {field(t("notify.smtpFrom"), "smtp.from", urlInput("smtp", "from", "bombvault@example.com"))}
          {field(t("notify.smtpTo"), "smtp.to", urlInput("smtp", "to", "admin@example.com"))}
        </>
      );
      break;
    case "apprise":
      fields = (
        <>
          {field(t("notify.appriseUrl"), "apprise.url", urlInput("apprise", "url", "http://apprise:8000/notify/bombvault"))}
          {field(
            t("notify.appriseTags"),
            "apprise.tags",
            <input
              key={shakeOf("apprise.tags")}
              {...text("apprise", "tags")}
              spellCheck={false}
              placeholder="backups,homelab"
              className={`${inputCls} ${fieldInputCls} ${shook("apprise.tags")}`}
            />
          )}
          <label className={fieldCls}>
            <span className="flex items-center gap-1.5">
              {t("notify.appriseUrls")}
              <InfoBubble tip={tLtr(t, "notify.appriseUrlsHint")} />
            </span>
            <textarea
              key={shakeOf("apprise.urls")}
              value={(c.apprise?.urls ?? []).join("\n")}
              onChange={(e) =>
                edit((ch) => ({ ...ch, apprise: { ...ch.apprise!, urls: e.target.value.split("\n") } }), "apprise.urls", true)
              }
              rows={2}
              spellCheck={false}
              placeholder="tgram://bottoken/ChatID"
              dir="ltr"
              className={`${inputCls} ${fieldInputCls} text-start ${shook("apprise.urls")}`}
            />
          </label>
          {problemLines("apprise.urls")}
        </>
      );
      break;
    case "webhook":
      fields = (
        <>
          {field(t("notify.webhook"), "webhook.url", urlInput("webhook", "url", "https://hooks.example.com/in"))}
          {field(
            t("notify.webhookMethod"),
            "webhook.method",
            <SelectField
              key={shakeOf("webhook.method")}
              value={(c.webhook?.method || "POST").toUpperCase()}
              onChange={(v) => edit((ch) => ({ ...ch, webhook: { ...ch.webhook!, method: v } }), "webhook.method", false)}
              label={t("notify.webhookMethod")}
              options={["GET", "POST", "PUT"].map((m) => ({ value: m, label: m }))}
              className={`${selectCls} ${fieldInputCls} ${shook("webhook.method")}`}
            />
          )}
          <HeadersEditor
            t={t}
            headers={c.webhook?.headers ?? {}}
            stored={Object.keys(saved(c, good)?.webhook?.headers ?? {})}
            onChange={(headers, wait) => edit((ch) => ({ ...ch, webhook: { ...ch.webhook!, headers } }), "webhook.headers", wait)}
            className={shook("webhook.headers")}
          />
          {Object.values(c.webhook?.headers ?? {}).includes(REDACTED) && (
            <p className={`ps-4 text-xs ${waitsForSecret(c, good) ? "text-statusWarn" : "text-carbon-textMuted"}`}>
              {t(waitsForSecret(c, good) ? "notify.secretAgain" : "notify.secretKept")}
            </p>
          )}
          {problemLines("webhook.headers")}
          <label className={fieldCls}>
            <span className="flex items-center gap-1.5">
              {t("notify.webhookBody")}
              <InfoBubble tip={tLtr(t, "notify.webhookBodyHint").replace("{list}", PLACEHOLDERS.map((p) => `%%${p}%%`).join(", "))} />
            </span>
            <textarea
              key={shakeOf("webhook.body")}
              {...text("webhook", "body")}
              rows={3}
              spellCheck={false}
              dir="ltr"
              className={`${inputCls} ${fieldInputCls} text-start ${shook("webhook.body")}`}
            />
          </label>
          {problemLines("webhook.body")}
        </>
      );
      break;
    case "healthchecks": {
      const byKey = c.healthchecks?.byKey ?? {};
      const own = (kind: EntryKind) => (byKey[HC_KEY[kind]] ?? "") !== "" || addedHc.includes(HC_KEY[kind]);
      const asSection = (kind: EntryKind) => ({ id: kind, label: t(SECTION_LABEL[kind]), glyph: <KindGlyph kind={kind} /> });
      const taken = sections.filter(own);
      const setHc = (key: string, v: string | null, wait: boolean) =>
        edit(
          (ch) => {
            const next = { ...ch.healthchecks?.byKey };
            if (v === null) delete next[key];
            else next[key] = v;
            return { ...ch, healthchecks: { ...ch.healthchecks, byKey: next } };
          },
          `healthchecks.byKey.${key}`,
          wait
        );
      fields = (
        <>
          <label className={fieldCls}>
            <span className="flex items-center gap-1.5">
              {t("notify.healthchecks")}
              <InfoBubble tip={t("notify.healthchecksLifecycle")} />
            </span>
            <input
              key={shakeOf("healthchecks.url")}
              {...text("healthchecks", "url")}
              spellCheck={false}
              placeholder="https://hc-ping.com/your-uuid"
              dir="ltr"
              className={`${inputCls} ${fieldInputCls} text-start ${shook("healthchecks.url")}`}
            />
          </label>
          {problemLines("healthchecks.url")}
          <div className="ps-4">
            <Exceptions
              label={t("notify.healthchecksTitle")}
              taken={taken.map(asSection)}
              free={sections.filter((kind) => !own(kind)).map(asSection)}
              rule={t(taken.length > 0 ? "notify.hcRuleOthers" : "notify.hcRuleAll")}
              control={(kind) => {
                const key = HC_KEY[kind as EntryKind];
                const path = `healthchecks.byKey.${key}`;
                return (
                  <input
                    key={shakeOf(path)}
                    value={byKey[key] ?? ""}
                    onChange={(e) => setHc(key, e.target.value, true)}
                    aria-label={`${t("notify.healthchecks")}: ${t(SECTION_LABEL[kind as EntryKind])}`}
                    spellCheck={false}
                    placeholder="https://hc-ping.com/your-uuid"
                    dir="ltr"
                    className={`${inputCls} ${fieldInputCls} text-start ${shook(path)}`}
                  />
                );
              }}
              onAdd={(kind) => setAddedHc((a) => [...a, HC_KEY[kind as EntryKind]])}
              onRemove={(kind) => {
                const key = HC_KEY[kind as EntryKind];
                setAddedHc((a) => a.filter((k) => k !== key));
                if (key in byKey) setHc(key, null, false);
              }}
            />
          </div>
          {problemLines("healthchecks.byKey")}
        </>
      );
      break;
    }
    case "unraid":
      // The switch is on but platform detection did not find Unraid, so the
      // backend sends nothing. Usually an Unraid host whose container lacks
      // the template's /boot -> /host/boot mount. TrueNAS is only ever set
      // explicitly, and neither it nor a platform set with PLATFORM has a
      // mount to suggest.
      fields =
        c.enabled && platformKind !== "unraid" ? (
          <div className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
            {platformKind === "truenas"
              ? t("notify.unraidOffOnTrueNAS")
              : platformSet
                ? t("notify.unraidOffOnPlatformSetting").replace("{platform}", platformKind)
                : tLtr(t, "notify.unraidPlatformMismatch").replace("{platform}", platformKind)}
          </div>
        ) : null;
      break;
  }

  return (
    <div data-channel={c.id} className="flex flex-col gap-2">
      <ToggleRow
        label={title}
        hint={hint ? tLtr(t, hint) : undefined}
        checked={c.enabled}
        onChange={(v) => edit((ch) => ({ ...ch, enabled: v }), "enabled", false)}
        hueIndex={hueIndex}
        shakeNonce={shakeOf("enabled") ?? shakeOf("")}
      />
      {loose.map((p) => (
        <p key={`${p.field}:${p.code}`} className="text-xs text-statusFail">
          {t(problemKey(p.code))}
        </p>
      ))}
      {c.kind !== "healthchecks" && (
        <div className="ps-4">
          <SettingRow label={t("notify.on")}>
            <Selector
              key={shakeOf("on")}
              items={[
                { id: "never", label: t("notify.onNever") },
                { id: "failure", label: t("notify.onFailure") },
                { id: "always", label: t("notify.onAlways") },
              ]}
              label={`${t("notify.on")}: ${title}`}
              select="one"
              active={c.on || "failure"}
              onChange={(id) => edit((ch) => ({ ...ch, on: id as NotifyChannel["on"] }), "on", false)}
              inline
              className={shakeOf("on") ? "glim-shake" : undefined}
            />
          </SettingRow>
        </div>
      )}
      {field(
        t("notify.name"),
        "name",
        <input
          key={shakeOf("name")}
          value={c.name ?? ""}
          onChange={(e) => edit((ch) => ({ ...ch, name: e.target.value }), "name", true)}
          placeholder={t(KIND_LABEL[c.kind])}
          className={`${inputCls} ${fieldInputCls} font-sans ${shook("name")}`}
        />
      )}
      {fields}
      <VerdictLine verdict={test.verdict} className="ps-4" />
      <div className="flex flex-wrap items-center justify-end gap-2">
        <TestButton label={t("notify.test")} labelKey="notify.test" words="delivery" test={test} onClick={handleTest} />
        <Button label={t("notify.remove")} labelKey="notify.remove" glyph={<IconTrash />} variant="icon" glyphOnly onClick={onRemove} />
      </div>
    </div>
  );
}

function SecretInput({ value, onChange, className }: { value: string; onChange: (v: string) => void; className: string }) {
  const reveal = useReveal();
  return (
    <RevealInput
      {...reveal}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      spellCheck={false}
      autoComplete="new-password"
      wrapperClassName={fieldInputCls}
      className={`${inputCls} w-full ${className}`}
    />
  );
}

interface HeaderRow {
  name: string;
  value: string;
}

const toRows = (headers: Record<string, string>): HeaderRow[] => Object.entries(headers).map(([name, value]) => ({ name, value }));
const toMap = (rows: HeaderRow[]): Record<string, string> =>
  Object.fromEntries(rows.filter((r) => r.name.trim() !== "").map((r) => [r.name.trim(), r.value]));

// HeadersEditor edits a webhook's headers as rows. A row without a name stays
// on screen only, since a header needs one. Values are secrets: stored ones
// come back masked and stay as they were while their field is empty.
function HeadersEditor({
  t,
  headers,
  stored,
  onChange,
  className,
}: {
  t: T;
  headers: Record<string, string>;
  /** The names whose value the server holds. */
  stored: string[];
  onChange: (headers: Record<string, string>, wait: boolean) => void;
  className: string;
}) {
  const [rows, setRows] = useState<HeaderRow[]>(() => toRows(headers));
  // A save that failed puts the headers back from outside; the rows follow.
  const outside = JSON.stringify(headers);
  const [seen, setSeen] = useState(outside);
  if (seen !== outside) {
    setSeen(outside);
    if (JSON.stringify(toMap(rows)) !== outside) setRows(toRows(headers));
  }

  function change(next: HeaderRow[], wait: boolean) {
    setRows(next);
    onChange(toMap(next), wait);
  }

  return (
    <div className="flex flex-col gap-2 ps-4">
      <span className="text-sm text-carbon-text">{t("notify.webhookHeaders")}</span>
      {rows.map((row, i) => (
        <div key={i} className={`flex flex-wrap items-center gap-2 ${className}`}>
          <input
            value={row.name}
            onChange={(e) => change(rows.map((r, j) => (j === i ? { ...r, name: e.target.value } : r)), true)}
            aria-label={t("notify.webhookHeaderName")}
            placeholder="Authorization"
            spellCheck={false}
            dir="ltr"
            className={`${inputCls} w-full sm:w-44 text-start`}
          />
          <SecretField
            value={row.value === REDACTED ? "" : row.value}
            label={t("notify.webhookHeaderValue")}
            onChange={(v) =>
              change(rows.map((r, j) => (j === i ? { ...r, value: v === "" && stored.includes(r.name) ? REDACTED : v } : r)), true)
            }
          />
          <Button
            label={t("notify.webhookHeaderRemove")}
            labelKey="notify.webhookHeaderRemove"
            glyph={<IconTrash />}
            variant="icon"
            glyphOnly
            onClick={() => change(rows.filter((_, j) => j !== i), false)}
          />
        </div>
      ))}
      <div>
        <Button
          label={t("notify.webhookHeaderAdd")}
          labelKey="notify.webhookHeaderAdd"
          glyph={<IconAdd />}
          onClick={() => setRows([...rows, { name: "", value: "" }])}
        />
      </div>
    </div>
  );
}

function SecretField({ value, label, onChange }: { value: string; label: string; onChange: (v: string) => void }) {
  const reveal = useReveal();
  return (
    <RevealInput
      {...reveal}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      aria-label={label}
      spellCheck={false}
      autoComplete="new-password"
      wrapperClassName="min-w-0 flex-1"
      className={inputCls}
    />
  );
}
