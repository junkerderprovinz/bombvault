// PhraseCard is where an instance joins a group. Outside one it offers two
// tiles: generate a phrase, or enter one that already exists. Inside one it
// shows who else is there, and when nobody has come after a minute it says
// the likely reasons with the steps that fix them.
import { useEffect, useState, type ReactNode } from "react";
import { Card } from "../settings/shared";
import { Badge, type BadgeTone } from "../../components/Badge";
import { Button } from "../../components/Button";
import { InfoBubble } from "../../components/InfoBubble";
import { RevealInput } from "../../components/RevealInput";
import { IconEye, IconRefresh, IconSignOut } from "../../components/glyphs";
import {
  ApiError,
  createPhrase,
  joinGroup,
  leaveGroup,
  setRelay,
  showPhrase,
  type GroupMember,
  type GroupState,
  type PhraseRefusal,
} from "../../lib/api";
import { copyText } from "../../lib/clipboard";
import type { TranslationKey, useT } from "../../lib/i18n";
import { useReveal } from "../../lib/useReveal";
import { useToast } from "../../lib/toast";
import { PhraseInput } from "./PhraseInput";
import { PhraseGlyph, RouteGlyph } from "./pairingArt";

type T = ReturnType<typeof useT>["t"];

/** How long a new member may take to show up before the card suggests why
 *  nobody has. Discovery and the relay both find a member within seconds. */
export const ALONE_AFTER_S = 60;

/** Where an instance stands in its group. "gone" is a group whose members
 *  were there once and are unreachable now. */
export type PairStage = "unpaired" | "new" | "searching" | "alone" | "gone" | "paired";

/** pairStage reads the stage from the group, the seconds since this instance
 *  entered it and whether this page created it. Only a member found now
 *  counts as paired. */
export function pairStage(g: GroupState, joinedAgo: number, createdHere: boolean): PairStage {
  if (!g.active) return "unpaired";
  if (g.members.length > 0) return "paired";
  if (g.memberSeen) return "gone";
  if (joinedAgo >= ALONE_AFTER_S) return "alone";
  return createdHere ? "new" : "searching";
}

const STAGE_BADGE: Record<Exclude<PairStage, "unpaired">, { tone: BadgeTone; key: TranslationKey }> = {
  new: { tone: "active", key: "pairing.stateNew" },
  searching: { tone: "neutral", key: "pairing.stateSearching" },
  gone: { tone: "neutral", key: "pairing.stateSearching" },
  alone: { tone: "warn", key: "pairing.stateAlone" },
  paired: { tone: "ok", key: "pairing.paired" },
};

/** refusalText says what is wrong with a phrase the server turned down, in
 *  the reader's language. */
export function refusalText(t: T, r: PhraseRefusal): string {
  switch (r.reason) {
    case "word_count":
      return t("pairing.errWordCount").replace("{count}", String(r.count ?? 0));
    case "unknown_word":
      return t("pairing.errUnknownWord")
        .replace("{position}", String(r.position ?? 0))
        .replace("{word}", r.word ?? "");
    case "checksum":
      return t("pairing.errChecksum");
    default:
      return r.error ?? t("pairing.joinError");
  }
}

function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

/** useJoinedAgo counts on from the seconds the server last reported, so the
 *  search timer runs and the minute passes between two reloads of the group. */
function useJoinedAgo(group: GroupState): number {
  const [base, setBase] = useState({ ago: group.joinedAgo, at: Date.now() });
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const at = Date.now();
    setBase({ ago: group.joinedAgo, at });
    setNow(at);
  }, [group]);
  const waiting = group.active && group.members.length === 0 && !group.memberSeen;
  useEffect(() => {
    if (!waiting) return;
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, [waiting]);
  return base.ago + Math.max(0, Math.floor((now - base.at) / 1000));
}

function clock(seconds: number): string {
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}

function SectionTitle({ children }: { children: ReactNode }) {
  return <p className="mb-2 flex items-center gap-1.5 text-[12.5px] text-carbon-textSub">{children}</p>;
}

function WordGrid({ phrase, t }: { phrase: string; t: T }) {
  return (
    <div>
      <SectionTitle>
        {t("pairing.wordsLabel")} <InfoBubble tip={t("pairing.wordsTip")} />
      </SectionTitle>
      <ol className="grid max-w-205 grid-cols-2 gap-2 md:grid-cols-4" aria-label={t("pairing.wordsLabel")}>
        {phrase.split(/\s+/).map((w, i) => (
          <li key={i} className="flex min-w-0 items-baseline gap-2 rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm">
            <span className="w-4.5 shrink-0 text-end text-xs tabular-nums text-carbon-textMuted">{i + 1}</span>
            <span dir="ltr" className="truncate font-mono text-carbon-text">
              {w}
            </span>
          </li>
        ))}
      </ol>
    </div>
  );
}

function MemberRow({ m, t }: { m: GroupMember; t: T }) {
  return (
    <li className="flex flex-wrap items-center gap-2 rounded-control bg-carbon-surface2 px-3 py-2">
      <span className="text-sm font-semibold text-carbon-text min-w-0 wrap-break-word">{m.name || m.id}</span>
      {m.version && <span className="text-xs text-carbon-textMuted">{m.version}</span>}
      <span className="ms-auto">
        <Badge tone={m.direct ? "ok" : "neutral"}>{m.direct ? t("pairing.direct") : t("pairing.viaRelay")}</Badge>
      </span>
    </li>
  );
}

function WaitRow({ text, seconds }: { text: string; seconds?: number }) {
  return (
    <div className="flex items-center gap-3 rounded-control bg-carbon-surface2 px-3 py-2.5 text-carbon-textSub">
      <span className="flex shrink-0 gap-1" aria-hidden="true">
        {[0, 1, 2].map((i) => (
          <span key={i} className="glim-wait-dot h-1.5 w-1.5 rounded-full bg-accentText" style={{ animationDelay: `${i * 200}ms` }} />
        ))}
      </span>
      <span className="text-sm">{text}</span>
      {seconds !== undefined && <span className="ms-auto text-xs tabular-nums text-carbon-textMuted">{clock(seconds)}</span>}
    </div>
  );
}

/** RelayLine says in one line how this instance reaches members elsewhere. */
function RelayLine({ group, t }: { group: GroupState; t: T }) {
  const { mode, connected } = group.relay;
  let dot = "bg-statusOkSolid";
  let text = t(mode === "own" ? "pairing.relayOwn" : "pairing.relayProject");
  let tip: string | null = null;
  if (mode === "off") {
    dot = "bg-carbon-textMuted";
    text = t("pairing.relayOff");
    tip = t("pairing.relayOffTip");
  } else if (!connected) {
    dot = "bg-statusWarnSolid";
    text = t(mode === "own" ? "pairing.relayOwnDown" : "pairing.relayProjectDown");
    tip = t("pairing.relayDownTip");
  }
  return (
    <span className="inline-flex min-w-0 items-center gap-2 text-[13px] text-carbon-textSub" data-testid="relay-line">
      <span className="shrink-0 text-carbon-textMuted [&>svg]:h-4.5 [&>svg]:w-4.5">
        <RouteGlyph kind={mode} />
      </span>
      <span className={`h-2 w-2 shrink-0 rounded-full ${dot}`} aria-hidden="true" />
      <span>{text}</span>
      {tip && <InfoBubble tip={tip} />}
    </span>
  );
}

/** Hint is the warning panel for something the reader has to act on. */
function Hint({ title, tip, children }: { title: string; tip?: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-3 rounded-control bg-statusWarnBgSoft p-4 text-sm text-carbon-textSub">
      <h3 className="flex items-center gap-2 text-[15px] font-semibold text-carbon-text">
        <span className="text-statusWarn">
          <PhraseGlyph kind="warn" size={18} />
        </span>
        {title}
        {tip && <InfoBubble tip={tip} />}
      </h3>
      {children}
    </section>
  );
}

function Rule() {
  return <hr className="w-full border-0 border-t border-carbon-border" />;
}

function Lead({ title, children }: { title: string; children: ReactNode }) {
  return (
    <p className="min-w-0">
      <strong className="font-semibold text-carbon-text">{title}</strong> {children}
    </p>
  );
}

/** StepNumber is the round mark of one fix step: its number, a check once
 *  done, or muted while it has to wait for the step before. */
function StepNumber({ n, done = false, waiting = false }: { n: number; done?: boolean; waiting?: boolean }) {
  const tone = done ? "bg-statusOkSolid text-carbon-background" : waiting ? "bg-carbon-surface3 text-carbon-textSub" : "bg-accent text-accentContrast";
  return (
    <span className={`mt-1 grid h-6 w-6 place-items-center rounded-full text-xs font-bold ${tone}`}>
      {done ? <PhraseGlyph kind="check" size={14} /> : n}
    </span>
  );
}

function RelayDownHint({ group, onRefresh, t }: { group: GroupState; onRefresh: () => void; t: T }) {
  const own = group.relay.mode === "own";
  const [before, after] = t("pairing.relayCheckProject").split("{host}");
  return (
    <Hint title={t("relay.notConnected")} tip={t("pairing.relayCheckTip")}>
      <p>{t("pairing.relayCheckLead")}</p>
      <ul className="flex list-disc flex-col gap-1 ps-5">
        <li>
          {own ? (
            t("pairing.relayCheckOwn")
          ) : (
            <>
              {before}
              <span dir="ltr" className="font-mono text-[0.92em] text-carbon-text">
                {hostOf(group.relay.projectUrl)}
              </span>
              {after}
            </>
          )}
        </li>
        <li>{t("pairing.relayCheckFilter")}</li>
      </ul>
      <div className="flex justify-end">
        <Button label={t("pairing.checkAgain")} labelKey="pairing.checkAgain" glyph={<IconRefresh />} tone="neutral" onClick={onRefresh} />
      </div>
    </Hint>
  );
}

export function PhraseCard({
  group,
  onGroup,
  onRefresh,
  t,
  hueIndex,
}: {
  group: GroupState;
  onGroup: (g: GroupState) => void;
  /** Asks the server for the group again, for "Check again". */
  onRefresh: () => void;
  t: T;
  hueIndex?: number;
}) {
  const { push } = useToast();
  const [phrase, setPhrase] = useState<string | null>(null);
  const [createdHere, setCreatedHere] = useState(false);
  const [entering, setEntering] = useState(false);
  // Set once "Leave this group" in the hint is pressed, so the hint stays up
  // with its second step instead of turning back into the question.
  const [fixing, setFixing] = useState(false);
  const [password, setPassword] = useState("");
  const [askPassword, setAskPassword] = useState(false);
  const [confirmLeave, setConfirmLeave] = useState(false);
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);
  const reveal = useReveal();
  const joinedAgo = useJoinedAgo(group);

  const stage = pairStage(group, joinedAgo, createdHere);
  const locked = !group.passwordSet;
  const shakeCls = shake ? "glim-shake" : "";

  async function run(action: () => Promise<void>) {
    setBusy(true);
    try {
      await action();
    } catch (err) {
      // Every route that makes, shows or takes the phrase answers 403 while no
      // login password is set.
      if (err instanceof ApiError && err.status === 403) push(t("pairing.needsPassword"), "fail");
      else push(err instanceof Error ? err.message : t("pairing.actionError"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  function refuse(error?: string) {
    push(error ?? t("pairing.actionError"), "fail");
    setShake((n) => n + 1);
  }

  function forget() {
    setPhrase(null);
    setCreatedHere(false);
    setAskPassword(false);
    setConfirmLeave(false);
  }

  const create = () =>
    run(async () => {
      const res = await createPhrase();
      if (res.ok && res.phrase) {
        setPhrase(res.phrase);
        setCreatedHere(true);
        if (res.group) onGroup(res.group);
      } else {
        refuse(res.error);
      }
    });

  async function join(words: string): Promise<string | null> {
    setBusy(true);
    try {
      const res = await joinGroup(words);
      if (!res.ok) return refusalText(t, res);
      forget();
      setEntering(false);
      setFixing(false);
      onGroup(res);
      push(t("pairing.joined"), "success");
      return null;
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) return t("pairing.needsPassword");
      return err instanceof Error ? err.message : t("pairing.actionError");
    } finally {
      setBusy(false);
    }
  }

  const show = () =>
    run(async () => {
      const res = await showPhrase(password);
      if (res.ok && res.phrase) {
        setPhrase(res.phrase);
        setAskPassword(false);
        setPassword("");
      } else {
        refuse(res.code === "passwordWrong" ? t("pairing.passwordWrong") : res.error);
      }
    });

  /** leave takes this instance out of its group, then runs then to set up
   *  what the card shows next. */
  const leave = (then: () => void) =>
    run(async () => {
      const res = await leaveGroup();
      if (res.ok) {
        forget();
        then();
        onGroup(res);
      } else {
        refuse(res.error);
      }
    });

  const lockedNote = locked && (
    <p className="rounded-card bg-statusWarnBgSoft px-3 py-2.5 text-sm leading-relaxed text-carbon-text">{t("pairing.needsPassword")}</p>
  );

  const passwordPrompt = (
    <div className="flex flex-col gap-1.5">
      <label className="text-xs text-carbon-textSub">{t("pairing.passwordLabel")}</label>
      <RevealInput
        {...reveal}
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        autoComplete="current-password"
        wrapperClassName="w-full"
        className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
      />
      <div className="flex flex-wrap items-center justify-end gap-2">
        <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={() => setAskPassword(false)} />
        <Button
          key={`show-${shake}`}
          label={t("pairing.show")}
          labelKey="pairing.show"
          glyph={<IconEye />}
          tone="accent"
          onClick={() => void show()}
          disabled={busy || password === ""}
          busy={busy}
          className={shakeCls}
        />
      </div>
    </div>
  );

  const revealed = phrase ? <WordGrid phrase={phrase} t={t} /> : askPassword ? passwordPrompt : null;

  const showToggle = phrase ? (
    <Button label={t("pairing.hide")} labelKey="pairing.hide" glyph={<IconEye />} tone="neutral" onClick={() => setPhrase(null)} />
  ) : (
    <Button
      key={`reveal-${shake}`}
      label={t("pairing.show")}
      labelKey="pairing.show"
      glyph={<IconEye />}
      tone="neutral"
      onClick={() => setAskPassword(true)}
      disabled={busy || locked || askPassword}
      className={shakeCls}
    />
  );

  const leaveButton = confirmLeave ? (
    <Button
      key={`leave-${shake}`}
      label={t("pairing.confirmLeave")}
      labelKey="pairing.confirmLeave"
      glyph={<IconSignOut />}
      tone="neutral"
      onClick={() => void leave(() => setEntering(false))}
      disabled={busy}
      busy={busy}
      className={shakeCls}
    />
  ) : (
    <Button label={t("pairing.leave")} labelKey="pairing.leave" glyph={<IconSignOut />} tone="neutral" onClick={() => setConfirmLeave(true)} />
  );

  const stateRow = (tone: BadgeTone, text: string, withName = true) => (
    <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1.5">
      <Badge tone={tone}>{text}</Badge>
      {withName && <span className="text-[12.5px] text-carbon-textMuted">{t("pairing.thisInstance").replace("{name}", group.name)}</span>}
    </div>
  );

  const enterTip = t("pairing.enterTip").replace("{path}", [t("instances.title"), t("pairing.title"), t("pairing.show")].join(", "));

  const foot = (buttons: boolean) => (
    <div className="flex flex-wrap items-center gap-3 border-t border-carbon-border pt-4">
      <RelayLine group={group} t={t} />
      {buttons && (
        <div className="flex w-full flex-wrap items-center justify-end gap-2 sm:ms-auto sm:w-auto">
          {showToggle}
          {leaveButton}
        </div>
      )}
    </div>
  );

  // Without a relay, instances in Docker's bridge network never hear each
  // other's multicast, so a missing relay is the likelier cause than a second
  // group and comes first.
  const noRelay = group.relay.mode === "off" && (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
      <div className="min-w-0 flex-[1_1_16rem]">
        <Lead title={t("pairing.otherNetTitle")}>
          {t("pairing.otherNetBody")} <InfoBubble tip={t("pairing.relayCheckTip")} />
        </Lead>
      </div>
      <Button
        label={t("relay.project")}
        labelKey="relay.project"
        glyph={<RouteGlyph kind="project" />}
        tone="neutral"
        onClick={() => void run(async () => onGroup(await setRelay({ mode: "project" })))}
        disabled={busy}
        busy={busy}
      />
    </div>
  );

  function aloneHint(left: boolean) {
    return (
      <Hint title={t("pairing.aloneTitle")}>
        {!left && <p>{t("pairing.aloneLead")}</p>}
        {!left && <Rule />}
        {!left && noRelay}
        {!left && noRelay && <Rule />}
        <Lead title={t("pairing.twoTitle")}>{t("pairing.twoBody")}</Lead>
        <div className="grid grid-cols-[24px_minmax(0,1fr)] items-start gap-x-3 gap-y-2.5">
          <StepNumber n={1} done={left} />
          <div className="flex min-h-8 flex-wrap items-center gap-x-3 gap-y-2">
            <span className="font-medium text-carbon-text">{t("pairing.fixLeave")}</span>
            {left ? (
              <span className="ms-auto text-[13px] text-statusOk">{t("pairing.fixLeft")}</span>
            ) : (
              <span className="sm:ms-auto">
                <Button
                  key={`fix-leave-${shake}`}
                  label={t("pairing.leave")}
                  labelKey="pairing.leave"
                  glyph={<IconSignOut />}
                  tone="neutral"
                  onClick={() => void leave(() => setFixing(true))}
                  disabled={busy}
                  busy={busy}
                  className={shakeCls}
                />
              </span>
            )}
          </div>
          <StepNumber n={2} waiting={!left} />
          <div className="flex min-h-8 items-center">
            <span className="font-medium text-carbon-text">
              {t("pairing.fixEnter")} <InfoBubble tip={enterTip} />
            </span>
          </div>
          <div className="col-start-2">
            <PhraseInput
              id="pairing-phrase-other"
              label={t("pairing.enterLabelOther")}
              tip={enterTip}
              bare
              disabled={!left || locked}
              busy={busy}
              onPair={join}
              t={t}
            />
          </div>
        </div>
        {!left && (
          <>
            <Rule />
            <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
              <div className="min-w-0 flex-[1_1_16rem]">
                <Lead title={t("pairing.notYetTitle")}>{t("pairing.notYetBody")}</Lead>
              </div>
              {showToggle}
            </div>
            {revealed}
          </>
        )}
        {left && noRelay && (
          <>
            <Rule />
            {noRelay}
          </>
        )}
      </Hint>
    );
  }

  const relayHint = (stage === "alone" || stage === "gone") && group.relay.mode !== "off" && !group.relay.connected && (
    <RelayDownHint group={group} onRefresh={onRefresh} t={t} />
  );

  let body: ReactNode;
  if (stage === "unpaired" && fixing) {
    body = (
      <>
        {stateRow("neutral", t("pairing.stateNotPaired"), false)}
        {lockedNote}
        {aloneHint(true)}
      </>
    );
  } else if (stage === "unpaired") {
    body = (
      <>
        {lockedNote}
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <Choice
            key={`create-${shake}`}
            glyph="create"
            title={t("pairing.create")}
            sub={t("pairing.createSub")}
            pressed={false}
            dim={entering}
            disabled={busy || locked}
            shaking={shake > 0}
            onClick={() => void create()}
          />
          <Choice
            glyph="enter"
            title={t("pairing.enter")}
            sub={t("pairing.enterSub")}
            pressed={entering}
            dim={false}
            disabled={locked}
            onClick={() => setEntering(true)}
          />
        </div>
        {entering && (
          <PhraseInput
            id="pairing-phrase"
            label={t("pairing.enterLabel")}
            tip={enterTip}
            disabled={locked}
            busy={busy}
            onPair={join}
            onCancel={() => setEntering(false)}
            t={t}
          />
        )}
      </>
    );
  } else if (stage === "alone") {
    body = (
      <>
        {stateRow(STAGE_BADGE.alone.tone, t(STAGE_BADGE.alone.key))}
        {lockedNote}
        {relayHint}
        {aloneHint(false)}
        {foot(false)}
      </>
    );
  } else {
    const badge = STAGE_BADGE[stage];
    body = (
      <>
        {stateRow(badge.tone, t(badge.key))}
        {lockedNote}
        {relayHint}
        {revealed}
        {stage === "new" && (
          <>
            <div className="flex flex-wrap items-center gap-2">
              {phrase && <CopyButton phrase={phrase} t={t} />}
              <span className="ms-auto inline-flex flex-wrap items-center gap-2 text-[13px] text-carbon-textMuted">
                {t("pairing.notFirst")}
                <Button
                  label={t("pairing.notFirstButton")}
                  labelKey="pairing.notFirstButton"
                  glyph={<PhraseGlyph kind="enter" />}
                  tone="neutral"
                  onClick={() => void leave(() => setEntering(true))}
                  disabled={busy}
                />
              </span>
            </div>
            <NextStep t={t} />
          </>
        )}
        <div>
          <SectionTitle>{t("pairing.membersTitle")}</SectionTitle>
          {stage === "paired" ? (
            <ul className="flex flex-col gap-2">
              {group.members.map((m) => (
                <MemberRow key={m.id} m={m} t={t} />
              ))}
            </ul>
          ) : stage === "new" ? (
            <WaitRow text={t("pairing.waitNext")} />
          ) : (
            <WaitRow text={t("pairing.searching")} seconds={stage === "searching" ? joinedAgo : undefined} />
          )}
        </div>
        {foot(true)}
      </>
    );
  }

  return (
    <Card title={t("pairing.phraseTitle")} hint={t("pairing.phraseHint")} hueIndex={hueIndex}>
      <div className="flex flex-col gap-4.5" data-stage={fixing && stage === "unpaired" ? "fixing" : stage}>
        {body}
      </div>
    </Card>
  );
}

/** Choice is one answer to the card's question, as a tile large enough to
 *  say what happens after it. */
function Choice({
  glyph,
  title,
  sub,
  pressed,
  dim,
  disabled,
  shaking = false,
  onClick,
}: {
  glyph: "create" | "enter";
  title: string;
  sub: string;
  pressed: boolean;
  dim: boolean;
  disabled: boolean;
  shaking?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-pressed={pressed}
      disabled={disabled}
      onClick={onClick}
      className={`group grid grid-cols-[44px_minmax(0,1fr)] items-center gap-3.5 rounded-control bg-carbon-surface2 p-4 text-start transition-colors enabled:hover:bg-carbon-surface3 disabled:cursor-not-allowed disabled:opacity-45 glim-field-focus ${
        pressed ? "ring-2 ring-inset ring-accent" : ""
      } ${dim ? "opacity-60 hover:opacity-100" : ""} ${shaking ? "glim-shake" : ""}`}
    >
      <span
        className={`grid h-11 w-11 place-items-center rounded-[10px] ${
          pressed ? "bg-accent text-accentContrast" : "bg-carbon-surface3 text-carbon-text group-enabled:group-hover:bg-carbon-hoverRaised"
        }`}
      >
        <PhraseGlyph kind={glyph} size={22} />
      </span>
      <span>
        <span className="block text-[15px] font-semibold leading-snug text-carbon-text">{title}</span>
        <span className="mt-0.5 block text-[13px] leading-snug text-carbon-textSub">{sub}</span>
      </span>
    </button>
  );
}

function CopyButton({ phrase, t }: { phrase: string; t: T }) {
  const { push } = useToast();
  async function copy() {
    if (await copyText(phrase)) push(t("common.copied"), "success");
    else push(t("vm.ssh.copyFailed"), "fail");
  }
  return <Button label={t("common.copy")} labelKey="common.copy" tone="neutral" onClick={() => void copy()} />;
}

/** NextStep points at the answer to give on the other instance, along the
 *  path to it. */
function NextStep({ t }: { t: T }) {
  const path = [t("instances.title"), t("pairing.title"), t("pairing.enter")];
  return (
    <div className="grid grid-cols-[28px_minmax(0,1fr)] items-start gap-x-3 gap-y-1 rounded-control bg-accentSoft px-4 py-3.5">
      <span className="row-span-2 grid h-7 w-7 place-items-center rounded-full bg-accent text-accentContrast">
        <PhraseGlyph kind="arrow" />
      </span>
      <strong className="text-sm font-semibold text-carbon-text">{t("pairing.nextTitle")}</strong>
      <p className="text-sm text-carbon-textSub">
        <span className="font-semibold text-carbon-text">
          {path.map((step, i) => (
            <span key={i}>
              {i > 0 && (
                <span className="mx-1 inline-block align-[-1px] text-carbon-textMuted">
                  <PhraseGlyph kind="chevron" size={12} />
                </span>
              )}
              {step}
            </span>
          ))}
        </span>{" "}
        {t("pairing.nextTail")}
      </p>
    </div>
  );
}
