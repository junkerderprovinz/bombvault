import { useEffect, useId, useLayoutEffect, useRef, useState, type CSSProperties } from "react";
import { colorFor, glyphFor, glyphLabelKey } from "../components/ActivityLog";
import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { IconCancel } from "../components/glyphs";
import { InfoBubble } from "../components/InfoBubble";
import { BottomSheet } from "../components/mobile/BottomSheet";
import { MobileSectionLabel } from "../components/mobile/MobileSectionLabel";
import { IconAdd, IconPencil } from "../components/navGlyphs";
import { buildLogLines, domainLabel, type LogLine, type ResolveName } from "../lib/activityLog";
import type { Run, ScheduleNext } from "../lib/api";
import { hueVars } from "../lib/appearance";
import { useT, type TranslationKey } from "../lib/i18n";
import { parseProgressFrame, progressEntry, type ProgressMap } from "../lib/progress";
import { formatClockTime } from "../lib/reltime";
import { useConfirm } from "../lib/useConfirm";
import {
  displayAddress,
  normalizeAddress,
  origin,
  type Bridge,
  type FoundServer,
  type LauncherState,
  type PairError,
  type Problem,
  type Server,
} from "./bridge";

type T = ReturnType<typeof useT>["t"];

/** How a server answered the last poll. */
type Reach = "connected" | "signIn" | "certificate" | "offline";

interface ServerLine extends LogLine {
  server: string;
}

const POLL_MS = 10000;
/** The launcher shows the tail of the log; the whole one is on each server. */
const LOG_LINES = 8;

/** Launcher lists the servers the app knows, with what is running on them
 *  above the list. A tap opens one; the pencil edits it; the plus adds one. */
export function Launcher({ bridge }: { bridge: Bridge }) {
  const { t } = useT();
  const [state, setState] = useState<LauncherState | null>(null);
  const [editing, setEditing] = useState<Server | "new" | null>(null);
  const [reach, setReach] = useState<Record<string, Reach>>({});
  const [lines, setLines] = useState<ServerLine[]>([]);

  useEffect(() => {
    const off = bridge.listen(setState);
    bridge.send({ op: "state" });
    return off;
  }, [bridge]);

  // A joined group closes the sheet it was joined from: what comes next is the
  // list filling up.
  const paired = state?.group.paired ?? false;
  const wasPaired = useRef(paired);
  useEffect(() => {
    if (paired && !wasPaired.current) setEditing(null);
    wasPaired.current = paired;
  }, [paired]);

  // Every state the app pushes is a new array; the poll restarts only when
  // the servers themselves change.
  const watched = JSON.stringify(state?.servers ?? []);
  useEffect(() => {
    const list = JSON.parse(watched) as Server[];
    if (list.length === 0) return;
    let live = true;
    const resolveName: ResolveName = (key, params, count) => {
      let s = t(key as TranslationKey, count);
      if (params) for (const [name, value] of Object.entries(params)) s = s.split(`{${name}}`).join(value);
      return s;
    };
    async function poll() {
      const now = Date.now();
      const results = await Promise.all(list.map((s) => readServer(bridge, s, resolveName, now)));
      if (!live) return;
      setReach(Object.fromEntries(results.map((r, i) => [list[i].id, r.reach])));
      setLines(
        results
          .flatMap((r) => r.lines)
          .sort((a, b) => a.atMs - b.atMs)
          .slice(-LOG_LINES)
      );
    }
    void poll();
    const timer = setInterval(() => void poll(), POLL_MS);
    return () => {
      live = false;
      clearInterval(timer);
    };
  }, [bridge, watched, t]);

  if (state === null) return null;

  const problem = state.problem;
  const certProblem = problem && problem.kind !== "unreachable" ? problem : undefined;
  const saved = new Set(state.servers.map((s) => origin(s.url)));
  const found = state.found.filter((f) => !saved.has(origin(f.url)));

  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-xl flex-col gap-6 px-4 py-5">
      <header className="flex items-center gap-3">
        <img src="/logo.svg" alt="" aria-hidden="true" draggable={false} className="block h-10 w-10 dark:hidden" />
        <img src="/logo-light.svg" alt="" aria-hidden="true" draggable={false} className="hidden h-10 w-10 dark:block" />
        <h1 className="min-w-0 flex-1 truncate text-xl font-bold text-carbon-text">BombVault</h1>
        <Badge as="button" shape="square" size="icon" tone="active" tip={t("launcher.add")} onClick={() => setEditing("new")}>
          <IconAdd />
        </Badge>
      </header>

      {state.servers.length === 0 ? (
        <div className="mt-6 flex flex-col items-center gap-4 rounded-card bg-carbon-surface px-6 py-8 text-center">
          <img src="/logo.svg" alt="" aria-hidden="true" draggable={false} className="block h-12 w-12 opacity-50 dark:hidden" />
          <img src="/logo-light.svg" alt="" aria-hidden="true" draggable={false} className="hidden h-12 w-12 opacity-50 dark:block" />
          <p className="text-sm text-carbon-textMuted">{t("launcher.empty")}</p>
          <Button label={t("launcher.add")} labelKey="launcher.add" tone="accent" onClick={() => setEditing("new")} />
        </div>
      ) : (
        <>
          <ActivityCard t={t} lines={lines} />
          <div className="flex flex-col gap-2">
            {state.servers.map((s, i) => (
              <ServerCard
                key={s.id}
                t={t}
                server={s}
                index={i + 1}
                reach={reach[s.id]}
                trouble={problem?.id === s.id && problem.kind === "unreachable" ? (problem.detail ?? "") : undefined}
                onOpen={() => bridge.send({ op: "open", id: s.id })}
                onEdit={() => setEditing(s)}
              />
            ))}
          </div>
        </>
      )}

      <ServerSheet
        t={t}
        editing={editing}
        found={found}
        group={state.group}
        pairError={state.pairError}
        onScan={() => bridge.send({ op: "scan" })}
        onJoin={(code) => bridge.send({ op: "join", code })}
        onLeave={() => bridge.send({ op: "leave" })}
        onClose={() => setEditing(null)}
        onSave={(server, open) => {
          bridge.send({ op: "save", server, open });
          setEditing(null);
        }}
        onRemove={(id) => {
          bridge.send({ op: "remove", id });
          setEditing(null);
        }}
      />

      <TrustSheet
        t={t}
        problem={certProblem}
        server={state.servers.find((s) => s.id === certProblem?.id)}
        onTrust={(p) => bridge.send({ op: "trust", id: p.id, fingerprint: p.fingerprint ?? "" })}
        onCancel={() => bridge.send({ op: "dismiss" })}
      />
    </main>
  );
}

/** readServer asks one server what runs there and turns the answer into the
 *  lines its own dashboard shows. */
async function readServer(
  bridge: Bridge,
  server: Server,
  resolveName: ResolveName,
  now: number
): Promise<{ reach: Reach; lines: ServerLine[] }> {
  const answer = await bridge.activity(server.id);
  if (answer.status === 401 || answer.status === 403) return { reach: "signIn", lines: [] };
  if (answer.status === -1) return { reach: "certificate", lines: [] };
  if (answer.status !== 200) return { reach: "offline", lines: [] };
  try {
    const a = JSON.parse(answer.body) as { runs?: Run[]; progress?: unknown[]; next?: ScheduleNext[] };
    const lines = buildLogLines(a.runs ?? [], inFlight(a.progress ?? [], now), a.next ?? [], resolveName, now).map((l) => ({
      ...l,
      id: `${server.id}:${l.id}`,
      server: server.name,
    }));
    return { reach: "connected", lines };
  } catch {
    // Something answered that is not BombVault, or a proxy's error page.
    return { reach: "offline", lines: [] };
  }
}

/** inFlight turns the progress events in flight into the map the log reads. */
function inFlight(events: unknown[], now: number): ProgressMap {
  const map: ProgressMap = {};
  for (const ev of events) {
    const frame = parseProgressFrame(JSON.stringify(ev));
    if (frame) map[frame.key] = progressEntry(frame, now);
  }
  return map;
}

function ActivityCard({ t, lines }: { t: T; lines: ServerLine[] }) {
  const box = useRef<HTMLDivElement>(null);
  // The newest line is at the bottom, as on the dashboard.
  useLayoutEffect(() => {
    if (box.current) box.current.scrollTop = box.current.scrollHeight;
  }, [lines]);
  const resolveName: ResolveName = (key) => t(key as TranslationKey);
  return (
    <section className="relative glim-notch-card glim-hue" style={hueVars(0) as CSSProperties}>
      <MobileSectionLabel t={t} labelKey="activityLog.title" />
      <div className="rounded-card bg-carbon-surface p-2 pt-5">
        <div
          ref={box}
          className="flex max-h-48 flex-col gap-1.5 overflow-y-auto rounded-card bg-black/20 px-3 py-2 font-mono text-xs leading-relaxed"
        >
          {lines.length === 0 && <p className="text-carbon-textMuted">{t("launcher.activityEmpty")}</p>}
          {lines.map((l) => (
            <div key={l.id} className="flex flex-col gap-0.5">
              <div className="flex min-w-0 items-start gap-2">
                <span className="shrink-0 tabular-nums text-carbon-textMuted">{formatClockTime(l.atMs / 1000, false)}</span>
                <span className={`w-4 shrink-0 text-center ${colorFor(l.status)}`} aria-label={t(glyphLabelKey(l.status))}>
                  {glyphFor(l.status)}
                </span>
                <span className="min-w-0 truncate font-semibold text-carbon-textSub">{l.server}</span>
                {!l.idle && <span className="shrink-0 text-carbon-textMuted">{domainLabel(resolveName, l.domain)}</span>}
              </div>
              <span className={`w-full min-w-0 wrap-break-word ${l.warn ? "text-statusWarn" : colorFor(l.status)}`}>{l.text}</span>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

const REACH_BADGE: Record<Reach, { tone: "ok" | "fail" | "warn"; key: TranslationKey }> = {
  connected: { tone: "ok", key: "instances.connected" },
  signIn: { tone: "warn", key: "launcher.signIn" },
  certificate: { tone: "warn", key: "launcher.checkCertificate" },
  offline: { tone: "fail", key: "instances.notConnected" },
};

function ServerCard({
  t,
  server,
  index,
  reach,
  trouble,
  onOpen,
  onEdit,
}: {
  t: T;
  server: Server;
  /** Rainbow position; the activity card holds 0. */
  index: number;
  reach?: Reach;
  /** What went wrong the last time this server was opened. */
  trouble?: string;
  onOpen: () => void;
  onEdit: () => void;
}) {
  const badge = reach && REACH_BADGE[reach];
  return (
    <div
      style={hueVars(index) as CSSProperties}
      className="flex items-center gap-1 rounded-card bg-carbon-surface pe-2 glim-hue glim-content-fade"
    >
      <button
        type="button"
        onClick={onOpen}
        className="flex min-h-[3.75rem] min-w-0 flex-1 items-center gap-3 rounded-card p-4 text-start glim-field-focus"
      >
        <span className="flex min-w-0 flex-1 flex-col gap-1">
          <span className="truncate text-sm font-semibold text-carbon-text">{server.name}</span>
          <span dir="ltr" className="truncate text-start font-mono text-xs text-carbon-textMuted">
            {server.url ? displayAddress(server.url) : t("launcher.viaGroup")}
          </span>
          {trouble !== undefined && (
            <span dir="ltr" className="text-start font-mono text-xs text-statusFail break-all" role="alert">
              {trouble || t("launcher.unreachable")}
            </span>
          )}
        </span>
        {badge && <Badge tone={badge.tone}>{t(badge.key)}</Badge>}
        <svg aria-hidden width="10" height="10" viewBox="0 0 12 12" fill="none" className="shrink-0 text-accentText">
          <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
        </svg>
      </button>
      {/* A glyph alone: on a phone the name needs the width a label would take. */}
      <Badge as="button" shape="square" size="icon" tone="neutral" tip={t("common.edit")} onClick={onEdit}>
        <IconPencil />
      </Badge>
    </div>
  );
}

function ServerSheet({
  t,
  editing,
  found,
  group,
  pairError,
  onScan,
  onJoin,
  onLeave,
  onClose,
  onSave,
  onRemove,
}: {
  t: T;
  editing: Server | "new" | null;
  /** Servers announced on the network and not in the list yet. */
  found: FoundServer[];
  group: LauncherState["group"];
  pairError?: PairError;
  onScan: () => void;
  onJoin: (code: string) => void;
  onLeave: () => void;
  onClose: () => void;
  onSave: (server: { id?: string; name: string; url: string }, open: boolean) => void;
  onRemove: (id: string) => void;
}) {
  const nameId = useId();
  const urlId = useId();
  const [name, setName] = useState("");
  const [address, setAddress] = useState("");
  const [invalid, setInvalid] = useState(false);
  const { confirm, confirmDialog } = useConfirm();
  const existing = editing !== null && editing !== "new" ? editing : null;

  useEffect(() => {
    if (editing === null) return;
    setName(existing?.name ?? "");
    setAddress(existing ? displayAddress(existing.url) : "");
    setInvalid(false);
  }, [editing, existing]);

  function save() {
    const url = normalizeAddress(address);
    if (url === null) {
      setInvalid(true);
      return;
    }
    onSave({ id: existing?.id, name: name.trim() || new URL(url).host, url }, false);
  }

  async function remove() {
    if (!existing) return;
    const ok = await confirm(t("launcher.removeConfirm").replace("{name}", existing.name), {
      confirmKey: "launcher.remove",
    });
    if (ok) onRemove(existing.id);
  }

  const field = "w-full rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-2 glim-field-focus";

  return (
    <>
      <BottomSheet
        open={editing !== null}
        onClose={onClose}
        title={existing ? t("launcher.editTitle") : t("launcher.add")}
        footer={
          <div className="flex flex-col gap-3 py-3">
            {/* A server from the group comes back while the app is in it. */}
            {existing && !existing.member && (
              <Button
                label={t("launcher.remove")}
                labelKey="launcher.remove"
                tone="neutral"
                onClick={() => void remove()}
                className="glim-btn-key w-full"
              />
            )}
            <Button
              label={t("launcher.save")}
              labelKey="launcher.save"
              tone="accent"
              disabled={address.trim() === ""}
              onClick={save}
              className="glim-btn-key w-full"
            />
          </div>
        }
      >
        <div className="flex flex-col gap-5 px-4 py-3">
          {!existing && <PairBlock t={t} group={group} error={pairError} onScan={onScan} onJoin={onJoin} onLeave={onLeave} />}
          {!existing && (
            <div className="flex flex-col gap-1.5">
              <span className="flex items-center gap-2 text-xs font-medium text-carbon-textSub">
                {t("launcher.found")}
                <InfoBubble tip={t("launcher.foundHint")} />
              </span>
              {found.map((f) => (
                <button
                  key={f.url}
                  type="button"
                  onClick={() => onSave({ name: f.name, url: f.url }, true)}
                  className="flex min-w-0 items-center gap-3 rounded-control bg-carbon-surface2 px-3 py-2.5 text-start glim-field-focus"
                >
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="truncate text-sm font-semibold text-carbon-text">{f.name}</span>
                    <span dir="ltr" className="truncate text-start font-mono text-xs text-carbon-textMuted">
                      {`${displayAddress(f.url)} · ${f.version}`}
                    </span>
                  </span>
                  <svg aria-hidden width="10" height="10" viewBox="0 0 12 12" fill="none" className="shrink-0 text-accentText">
                    <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
                  </svg>
                </button>
              ))}
              <p className="text-xs text-carbon-textMuted">{found.length === 0 ? t("launcher.searching") : t("launcher.foundLead")}</p>
            </div>
          )}
          <form
            className="flex flex-col gap-4"
            onSubmit={(e) => {
              e.preventDefault();
              save();
            }}
          >
            <div className="flex flex-col gap-1.5">
              <label htmlFor={urlId} className="flex items-center gap-2 text-xs font-medium text-carbon-textSub">
                {t("launcher.address")}
                <InfoBubble tip={t("launcher.addressHint")} />
              </label>
              <input
                id={urlId}
                dir="ltr"
                value={address}
                onChange={(e) => {
                  setAddress(e.target.value);
                  setInvalid(false);
                }}
                inputMode="url"
                autoCapitalize="off"
                autoCorrect="off"
                spellCheck={false}
                placeholder="192.168.1.10:3443"
                className={`${field} font-mono`}
              />
              {invalid && (
                <p className="text-xs text-statusFail" role="alert">
                  {t("launcher.addressInvalid")}
                </p>
              )}
            </div>
            <div className="flex flex-col gap-1.5">
              <label htmlFor={nameId} className="text-xs font-medium text-carbon-textSub">
                {t("launcher.name")}
              </label>
              <input id={nameId} value={name} onChange={(e) => setName(e.target.value)} placeholder={t("launcher.namePlaceholder")} className={field} />
            </div>
            {/* Enter in either field saves. */}
            <button type="submit" hidden />
          </form>
        </div>
      </BottomSheet>
      {confirmDialog}
    </>
  );
}

/** The words of a refused code, in the sentences the pairing card uses. */
function pairErrorText(t: T, e: PairError): string {
  switch (e.reason) {
    case "count":
      return t("pairing.errWordCount").replace("{count}", String(e.count));
    case "word":
      return t("pairing.errUnknownWord").replace("{position}", String(e.position)).replace("{word}", e.word);
    case "checksum":
      return t("pairing.errChecksum");
  }
}

/** PairBlock joins the app to a BombVault group by its QR code or its twelve
 *  words, or, once joined, says so and offers to leave. */
function PairBlock({
  t,
  group,
  error,
  onScan,
  onJoin,
  onLeave,
}: {
  t: T;
  group: LauncherState["group"];
  error?: PairError;
  onScan: () => void;
  onJoin: (code: string) => void;
  onLeave: () => void;
}) {
  const wordsId = useId();
  const [typing, setTyping] = useState(false);
  const [words, setWords] = useState("");
  const { confirm, confirmDialog } = useConfirm();

  if (group.paired) {
    return (
      <div className="flex flex-col gap-2">
        <span className="text-xs font-medium text-carbon-textSub">{t("launcher.pairTitle")}</span>
        <div className="flex items-center gap-2 rounded-control bg-carbon-surface2 px-3 py-2.5">
          <span className="min-w-0 flex-1 truncate text-sm font-semibold text-carbon-text">{t("launcher.paired")}</span>
          <Badge tone={group.connected ? "ok" : "warn"}>
            {group.connected ? t("launcher.relayConnected") : t("launcher.relayOffline")}
          </Badge>
        </div>
        <Button
          label={t("pairing.leave")}
          labelKey="pairing.leave"
          tone="neutral"
          onClick={() =>
            void confirm(t("launcher.leaveConfirm"), { confirmKey: "pairing.leave" }).then((ok) => {
              if (ok) onLeave();
            })
          }
        />
        {confirmDialog}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-2">
      <span className="flex items-center gap-2 text-xs font-medium text-carbon-textSub">
        {t("launcher.pairTitle")}
        <InfoBubble tip={t("launcher.pairHint")} />
      </span>
      <Button label={t("launcher.scan")} labelKey="launcher.scan" tone="accent" onClick={onScan} className="glim-btn-key w-full" />
      {typing ? (
        <form
          className="flex flex-col gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            onJoin(words);
          }}
        >
          <label htmlFor={wordsId} className="sr-only">
            {t("pairing.enterLabel")}
          </label>
          <textarea
            id={wordsId}
            value={words}
            onChange={(e) => setWords(e.target.value)}
            rows={3}
            autoCapitalize="off"
            autoCorrect="off"
            spellCheck={false}
            placeholder={t("pairing.enterPlaceholder")}
            className="w-full resize-none rounded-control bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text glim-field-focus"
          />
          <Button label={t("pairing.join")} labelKey="pairing.join" tone="neutral" type="submit" disabled={words.trim() === ""} />
        </form>
      ) : (
        <Button label={t("launcher.enterWords")} labelKey="launcher.enterWords" glyph={<IconPencil />} tone="neutral" onClick={() => setTyping(true)} />
      )}
      {error && (
        <p className="text-xs text-statusFail" role="alert">
          {pairErrorText(t, error)}
        </p>
      )}
    </div>
  );
}

function TrustSheet({
  t,
  problem,
  server,
  onTrust,
  onCancel,
}: {
  t: T;
  problem?: Problem;
  server?: Server;
  onTrust: (p: Problem) => void;
  onCancel: () => void;
}) {
  const bodyId = useId();
  const changed = problem?.kind === "changed";
  const host = server ? displayAddress(server.url) : "";
  return (
    <BottomSheet
      open={problem !== undefined && server !== undefined}
      onClose={onCancel}
      headerClose={false}
      title={changed ? t("launcher.changedTitle") : t("launcher.trustTitle")}
      describedBy={bodyId}
      footer={
        <div className="flex flex-col gap-3 py-3">
          <Button
            label={t("common.cancel")}
            labelKey="common.cancel"
            glyph={<IconCancel />}
            tone={changed ? "accent" : "neutral"}
            onClick={onCancel}
            className="glim-btn-key w-full"
          />
          <Button
            label={changed ? t("launcher.trustChanged") : t("launcher.trust")}
            labelKey={changed ? "launcher.trustChanged" : "launcher.trust"}
            tone={changed ? "neutral" : "accent"}
            onClick={() => problem && onTrust(problem)}
            className="glim-btn-key w-full"
          />
        </div>
      }
    >
      <div className="flex flex-col gap-4 px-4 py-3">
        <p id={bodyId} className="text-sm leading-relaxed text-carbon-textSub wrap-break-word">
          {(changed ? t("launcher.changedBody") : t("launcher.trustBody")).replace("{host}", host)}
        </p>
        <div className="flex flex-col gap-1.5">
          <span className="text-xs font-medium text-carbon-textSub">{t("launcher.fingerprint")}</span>
          <code dir="ltr" className="rounded-control bg-carbon-surface2 px-3 py-2 font-mono text-xs leading-relaxed text-carbon-text break-all">
            {problem?.fingerprint}
          </code>
        </div>
      </div>
    </BottomSheet>
  );
}
