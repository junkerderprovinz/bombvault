// PhraseCard is the one place of the phrase. Outside a group it offers two
// tiles: generate a phrase, or enter one that already exists, each opening a
// window. On a phrase it is the fortress's card, which shows and changes the
// phrase and leaves, with this instance's name above, and while nobody has
// come it says what to do next. The phrase is the group's and the fortress's
// at once, so other apps join with the same words.
import { useEffect, useState, type ReactNode } from "react";
import { Card } from "../shared";
import { Button } from "../../../components/Button";
import { InfoBubble } from "../../../components/InfoBubble";
import { IconInfo, IconRetry } from "../../../components/glyphs";
import { IconCheck, IconClose, IconCopy } from "../../../components/navGlyphs";
import { createPhrase, joinGroup, leaveGroup, type FortressState, type GroupState, type PhraseRefusal } from "../../../lib/api";
import { copyText } from "../../../lib/clipboard";
import type { useT } from "../../../lib/i18n";
import { useToast } from "../../../lib/toast";
import { InstanceWindow } from "../../instances/InstanceWindow";
import { FortressBlock, RemovedHere, type Others } from "./FortressSection";
import { PairingWindow } from "./PairingWindow";
import { usePhraseEntry } from "./PhraseInput";
import { WordSlots } from "./WordSlots";
import { PhraseGlyph } from "./pairingArt";
import type { PairStage } from "./pairStage";

type T = ReturnType<typeof useT>["t"];

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
export function useJoinedAgo(group: GroupState): number {
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

/** RelayDownLine says in one line that the relay cannot be reached, and opens
 *  what to check on this instance. */
function RelayDownLine({ group, onRefresh, t }: { group: GroupState; onRefresh: () => void; t: T }) {
  const [open, setOpen] = useState(false);
  const own = group.relay.mode === "own";
  const [before, after] = t("pairing.relayCheckProject").split("{host}");
  return (
    <section className="flex flex-col gap-2 rounded-control bg-statusWarnBgSoft px-3 py-2.5 text-sm text-carbon-textSub">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-2">
        <span className="text-statusWarn">
          <PhraseGlyph kind="warn" size={18} />
        </span>
        <span className="font-semibold text-carbon-text">{t("relay.notConnected")}</span>
        <InfoBubble tip={t("pairing.relayCheckTip")} />
        <span className="ms-auto flex flex-wrap gap-2">
          <Button
            label={t("pairing.relayCheckOpen")}
            labelKey="pairing.relayCheckOpen"
            tone="neutral"
            onClick={() => setOpen(true)}
            glyph={<IconInfo />}
          />
          <Button label={t("pairing.checkAgain")} labelKey="pairing.checkAgain" glyph={<IconRetry />} tone="neutral" onClick={onRefresh} />
        </span>
      </div>
      {open && (
        <InstanceWindow
          title={t("pairing.relayCheckOpen")}
          onClose={() => setOpen(false)}
          footer={<Button label={t("common.done")} labelKey="common.done" tone="accent" onClick={() => setOpen(false)} />}
        >
          <p className="text-sm text-carbon-textSub">{t("pairing.relayCheckLead")}</p>
          <ul className="flex list-disc flex-col gap-1 ps-5 text-sm text-carbon-textSub">
            <li>
              {own ? (
                t("pairing.relayCheckOwn")
              ) : (
                <>
                  {before}
                  <span dir="ltr" className="font-mono text-subline text-carbon-text">
                    {hostOf(group.relay.projectUrl)}
                  </span>
                  {after}
                </>
              )}
            </li>
            <li>{t("pairing.relayCheckFilter")}</li>
          </ul>
        </InstanceWindow>
      )}
    </section>
  );
}

/** JoinWindow takes the words of another instance's group. The server leaves
 *  the group this instance is in first, so its members hear of it. */
function JoinWindow({
  id,
  title,
  hint,
  label,
  tip,
  busy,
  hueIndex,
  onPair,
  onClose,
  t,
}: {
  id: string;
  title: string;
  hint: string;
  label: string;
  tip: string;
  busy: boolean;
  hueIndex?: number;
  onPair: (phrase: string) => Promise<string | null>;
  onClose: () => void;
  t: T;
}) {
  const { field, paste, pair } = usePhraseEntry({ id, label, tip, bare: true, busy, onPair, t });
  return (
    <PairingWindow
      title={title}
      hint={hint}
      hueIndex={hueIndex}
      onClose={onClose}
      footer={
        <>
          <CloseButton onClose={onClose} t={t} />
          {paste}
          {pair}
        </>
      }
    >
      {field}
    </PairingWindow>
  );
}

function CloseButton({ onClose, t }: { onClose: () => void; t: T }) {
  return <Button label={t("common.close")} labelKey="common.close" glyph={<IconClose />} tone="neutral" onClick={onClose} />;
}

export function PhraseCard({
  group,
  view,
  stage,
  nameRow,
  onCreatedHere,
  onGroup,
  onRefresh,
  t,
  hueIndex,
  others,
}: {
  group: GroupState;
  view: FortressState;
  /** Where this instance stands. With other apps in the fortress it counts as paired. */
  stage: PairStage;
  /** The row that names this instance, at the top of the card on a phrase. */
  nameRow: ReactNode;
  /** Tells the page whether the group this instance is in was made here. */
  onCreatedHere: (created: boolean) => void;
  onGroup: (g: GroupState) => void;
  /** Asks the server for the group and the fortress again. */
  onRefresh: () => void;
  t: T;
  hueIndex: number;
  others?: Others;
}) {
  const { push } = useToast();
  // The words of a phrase made here, which show once without the password.
  const [phrase, setPhrase] = useState<string | null>(null);
  // The window over the card: the words just made, the field for the words
  // of a first instance, or the field for another group's words.
  const [shown, setShown] = useState<"words" | "enter" | "join" | null>(null);
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);
  const [copies, setCopies] = useState(0);

  async function run(action: () => Promise<void>) {
    setBusy(true);
    try {
      await action();
    } catch (err) {
      push(err instanceof Error ? err.message : t("pairing.actionError"), "fail");
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
    onCreatedHere(false);
    setCopies(0);
  }

  const create = () =>
    run(async () => {
      const res = await createPhrase();
      if (res.ok && res.phrase) {
        setPhrase(res.phrase);
        onCreatedHere(true);
        if (res.group) onGroup(res.group);
        setShown("words");
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
      setShown(null);
      onGroup(res);
      onRefresh();
      push(t("pairing.joined"), "success");
      return null;
    } catch (err) {
      return err instanceof Error ? err.message : t("pairing.actionError");
    } finally {
      setBusy(false);
    }
  }

  /** leave takes this instance out of its group, then runs then to set up
   *  what the card shows next. */
  const leave = (then: () => void) =>
    run(async () => {
      const res = await leaveGroup();
      if (res.ok) {
        forget();
        then();
        onGroup(res);
        onRefresh();
      } else {
        refuse(res.error);
      }
    });

  async function copy() {
    if (!phrase) return;
    if (await copyText(phrase)) setCopies((n) => n + 1);
    else push(t("vm.ssh.copyFailed"), "fail");
  }

  const enterTip = t("pairing.enterTip").replace("{path}", [t("settings.title"), t("pairing.title"), t("pairing.show")].join(", "));
  const closeShown = () => setShown(null);

  const relayHint = (stage === "alone" || stage === "gone") && group.relay.mode !== "off" && !group.relay.connected && (
    <RelayDownLine group={group} onRefresh={onRefresh} t={t} />
  );

  const noRelay = group.relay.mode === "off";

  const copyButton = (
    <Button
      label={copies > 0 ? t("common.copied") : t("common.copy")}
      labelKey={copies > 0 ? "common.copied" : "common.copy"}
      glyph={copies > 0 ? <IconCheck /> : <IconCopy />}
      tone="neutral"
      onClick={() => void copy()}
    />
  );

  const windows =
    shown === "words" ? (
      <PairingWindow
        title={t("pairing.wordsLabel")}
        hint={t("pairing.wordsTip")}
        hueIndex={hueIndex}
        onClose={closeShown}
        footer={
          <>
            <CloseButton onClose={closeShown} t={t} />
            {copyButton}
          </>
        }
      >
        {phrase && (
          <div className="flex flex-col gap-2">
            <WordSlots
              words={phrase.split(/\s+/)}
              label={t("pairing.wordsLabel")}
              // An own relay rides along on a second line, since the app has
              // no other way to learn its address.
              qr={group.relay.mode === "own" ? `${phrase}\n${group.relay.url}` : phrase}
            />
            {noRelay ? (
              <p className="rounded-card bg-statusWarnBgSoft px-3 py-2.5 text-sm leading-relaxed text-carbon-text">{t("pairing.phoneNeedsRelay")}</p>
            ) : (
              <p className="text-center text-xs text-carbon-textMuted sm:text-end">{t("pairing.qrCaption")}</p>
            )}
          </div>
        )}
      </PairingWindow>
    ) : shown === "enter" ? (
      <JoinWindow
        id="pairing-phrase"
        title={t("pairing.enter")}
        hint={t("pairing.enterSub")}
        label={t("pairing.enterLabel")}
        tip={enterTip}
        busy={busy}
        hueIndex={hueIndex}
        onPair={join}
        onClose={closeShown}
        t={t}
      />
    ) : shown === "join" ? (
      <JoinWindow
        id="pairing-phrase-other"
        title={t("pairing.enterIts")}
        hint={t("pairing.twoBody")}
        label={t("pairing.enterLabelOther")}
        tip={enterTip}
        busy={busy}
        hueIndex={hueIndex}
        onPair={join}
        onClose={closeShown}
        t={t}
      />
    ) : null;

  if (stage === "unpaired" && !view.joined) {
    return (
      <Card title={t("pairing.phraseTitle")} hint={`${t("pairing.phraseTip")} ${t("pairing.lead")} ${t("pairing.keyNote")}`} hueIndex={hueIndex}>
        <div className="flex flex-col gap-4.5" data-stage={stage} data-new="pairing-card">
          <RemovedHere view={view} />
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            <Choice
              key={`create-${shake}`}
              glyph={<PhraseGlyph kind="create" size={22} />}
              title={t("pairing.create")}
              sub={t("pairing.createSub")}
              disabled={busy}
              shaking={shake > 0}
              onClick={() => void create()}
            />
            <Choice glyph={<PhraseGlyph kind="enter" size={22} />} title={t("pairing.enter")} sub={t("pairing.enterSub")} onClick={() => setShown("enter")} />
          </div>
        </div>
        {windows}
      </Card>
    );
  }

  return (
    <>
      <FortressBlock view={view} group={group} onChanged={onRefresh} hueIndex={hueIndex} others={others}>
        {nameRow}
        <div className="flex flex-col gap-4.5" data-stage={stage} data-new="pairing-card">
          <RemovedHere view={view} />
          {stage === "alone" && (
            <>
              <p className="text-sm text-carbon-textSub">
                {t("pairing.aloneLead")} {t("pairing.notYetBody")}
              </p>
              <Choice
                glyph={<PhraseGlyph kind="enter" size={22} />}
                title={t("pairing.twoTitle")}
                sub={t("pairing.twoBody")}
                onClick={() => setShown("join")}
              />
              {noRelay && (
                <p className="text-sm text-carbon-textSub">
                  <strong className="font-semibold text-carbon-text">{t("pairing.otherNetTitle")}</strong> {t("pairing.otherNetBody")}
                </p>
              )}
            </>
          )}
          {relayHint}
          {stage === "new" && (
            <>
              <div className="flex flex-wrap items-center justify-end gap-2 text-subline text-carbon-textMuted">
                {t("pairing.notFirst")}
                <Button
                  label={t("pairing.notFirstButton")}
                  labelKey="pairing.notFirstButton"
                  glyph={<PhraseGlyph kind="enter" />}
                  tone="neutral"
                  onClick={() => void leave(() => setShown("enter"))}
                  disabled={busy}
                />
              </div>
              <NextStep t={t} />
            </>
          )}
          {stage === "searching" && <p className="text-sm text-carbon-textSub">{t("pairing.searchingHint")}</p>}
        </div>
      </FortressBlock>
      {windows}
    </>
  );
}

/** Choice is one of two ways forward, as a tile large enough to say what
 *  happens after it. */
function Choice({
  glyph,
  title,
  sub,
  disabled = false,
  shaking = false,
  onClick,
}: {
  glyph: ReactNode;
  title: string;
  sub: string;
  disabled?: boolean;
  shaking?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      data-choice
      disabled={disabled}
      onClick={onClick}
      className={`group grid grid-cols-[44px_minmax(0,1fr)] items-center gap-3.5 rounded-control bg-carbon-surface2 p-4 text-start transition-colors enabled:hover:bg-carbon-surface3 disabled:cursor-not-allowed disabled:opacity-45 glim-field-focus ${
        shaking ? "glim-shake" : ""
      }`}
    >
      <span className="grid h-11 w-11 place-items-center rounded-control bg-accent text-accentContrast [&>svg]:h-5.5 [&>svg]:w-5.5">
        {glyph}
      </span>
      <span>
        <span className="block text-title font-semibold leading-snug text-carbon-text">{title}</span>
        <span className="mt-0.5 block text-subline leading-snug text-carbon-textSub">{sub}</span>
      </span>
    </button>
  );
}

/** NextStep points at the button to press on the other instance, along the
 *  path to it. */
function NextStep({ t }: { t: T }) {
  const path = [t("settings.title"), t("pairing.title"), t("pairing.enter")];
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
