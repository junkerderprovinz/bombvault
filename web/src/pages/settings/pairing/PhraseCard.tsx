// PhraseCard is where an instance joins a group. Outside one it offers two
// tiles: generate a phrase, or enter one that already exists, each opening a
// window. Inside one it holds the way back to the words and out of the group,
// and when nobody has come after a minute it offers the two ways on, in the
// same two windows. Who is in the group stands in the card below it.
import { useEffect, useState, type ReactNode } from "react";
import { Card, useOpenPasswordField } from "../shared";
import { Button } from "../../../components/Button";
import { InfoBubble } from "../../../components/InfoBubble";
import { RevealInput } from "../../../components/RevealInput";
import { IconEye, IconInfo, IconRefresh, IconSearch, IconSignOut } from "../../../components/glyphs";
import { IconCheck, IconClose, IconCopy, IconPencil } from "../../../components/navGlyphs";
import {
  createPhrase,
  joinGroup,
  leaveGroup,
  probeAddress,
  showPhrase,
  type GroupState,
  type PhraseRefusal,
} from "../../../lib/api";
import { copyText } from "../../../lib/clipboard";
import type { useT } from "../../../lib/i18n";
import { useReveal } from "../../../lib/useReveal";
import { useToast } from "../../../lib/toast";
import { InstanceWindow } from "../../instances/InstanceWindow";
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
          <Button label={t("pairing.checkAgain")} labelKey="pairing.checkAgain" glyph={<IconRefresh />} tone="neutral" onClick={onRefresh} />
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

/** CantFindIt is the fallback for when the LAN sweep finds nobody: a person
 *  types the other instance's address by hand, for a subnet or a port the
 *  sweep does not try on its own. */
function CantFindIt({ onProbe, busy, t, shake }: { onProbe: (url: string) => void; busy: boolean; t: T; shake: number }) {
  const [open, setOpen] = useState(false);
  const [value, setValue] = useState("");
  return (
    <div>
      <Button label={t("pairing.cantFindTitle")} labelKey="pairing.cantFindTitle" tone="neutral" glyph={<IconSearch />} onClick={() => setOpen(true)} />
      {open && (
        <InstanceWindow
          title={t("pairing.cantFindTitle")}
          hint={t("pairing.cantFindBody")}
          onClose={() => setOpen(false)}
          footer={
            <>
              <Button label={t("common.cancel")} labelKey="common.cancel" glyph={<IconClose />} tone="neutral" onClick={() => setOpen(false)} />
              <Button
                key={`cant-find-${shake}`}
                label={t("pairing.cantFindSearch")}
                labelKey="pairing.cantFindSearch"
                tone="accent"
                onClick={() => onProbe(value)}
                disabled={busy || value.trim() === ""}
                busy={busy}
              />
            </>
          }
        >
          <div className="flex flex-col gap-1.5">
            <label htmlFor="pairing-cant-find" className="text-xs text-carbon-textSub">
              {t("pairing.cantFindLabel")}
            </label>
            <input
              id="pairing-cant-find"
              type="url"
              value={value}
              onChange={(e) => setValue(e.target.value)}
              spellCheck={false}
              autoComplete="off"
              placeholder="https://192.168.2.20:3443"
              dir="ltr"
              className="rounded-control bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus"
            />
          </div>
        </InstanceWindow>
      )}
    </div>
  );
}

/** JoinWindow takes the words of another instance's group. Joining from a
 *  group of one replaces it in one step, which is right while nobody else is
 *  in it. */
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
  stage,
  onCreatedHere,
  onGroup,
  onRefresh,
  t,
  hueIndex,
}: {
  group: GroupState;
  stage: PairStage;
  /** Tells the page whether the group this instance is in was made here. */
  onCreatedHere: (created: boolean) => void;
  onGroup: (g: GroupState) => void;
  /** Asks the server for the group again, for "Check again". */
  onRefresh: () => void;
  t: T;
  hueIndex?: number;
}) {
  const { push } = useToast();
  const openPasswordField = useOpenPasswordField();
  const [phrase, setPhrase] = useState<string | null>(null);
  // The window over the card: the words to read out, the field for the words
  // of a first instance, or the field for another group's words.
  const [shown, setShown] = useState<"words" | "enter" | "join" | null>(null);
  const [password, setPassword] = useState("");
  const [confirmLeave, setConfirmLeave] = useState(false);
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);
  const [copies, setCopies] = useState(0);
  const reveal = useReveal();
  const shakeCls = shake ? "glim-shake" : "";

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
    setConfirmLeave(false);
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
      push(t("pairing.joined"), "success");
      return null;
    } catch (err) {
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

  /** probe tries one address a person typed under "Can't find it?" for a
   *  member, the same way the LAN sweep tries an address on its own. */
  const probe = (url: string) =>
    run(async () => {
      const res = await probeAddress(url);
      if (res.ok) {
        onGroup(res);
        push(t("pairing.cantFindFound"), "success");
      } else {
        refuse(res.error ?? t("pairing.cantFindError"));
      }
    });

  async function copy() {
    if (!phrase) return;
    if (await copyText(phrase)) setCopies((n) => n + 1);
    else push(t("vm.ssh.copyFailed"), "fail");
  }

  // Pairing works without a login password, but then whoever opens this page
  // holds the words and, through the group, every member's restic password.
  // The warning stays until a password is set.
  const noPasswordNote = !group.passwordSet && (
    <section className="flex flex-wrap items-center gap-x-4 gap-y-3 rounded-control bg-statusWarnBgSoft p-4" data-testid="no-password">
      <span className="shrink-0 text-statusWarn">
        <PhraseGlyph kind="warn" size={22} />
      </span>
      <div className="min-w-0 flex-[1_1_18rem]">
        <p className="text-title font-semibold text-carbon-text">{t("pairing.noPasswordTitle")}</p>
        <p className="mt-0.5 text-sm leading-relaxed text-carbon-text">{t("pairing.noPasswordHint")}</p>
      </div>
      <Button
        label={t("auth.setPassword")}
        labelKey="auth.setPassword"
        tone="accent"
        onClick={openPasswordField}
      />
    </section>
  );

  const passwordPrompt = (
    <div className="flex flex-col gap-1.5">
      <label htmlFor="pairing-password" className="flex items-center gap-1.5 text-xs text-carbon-textSub">
        {t("pairing.passwordLabel")}
      </label>
      <RevealInput
        {...reveal}
        id="pairing-password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && password !== "") void show();
        }}
        autoComplete="current-password"
        wrapperClassName="w-full"
        className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
      />
    </div>
  );

  const openWords = () => {
    setShown("words");
    // Without a password there is nothing to enter before the words show.
    if (!phrase && !group.passwordSet) void show();
  };

  const closeShown = () => {
    setShown(null);
    setPassword("");
  };

  const leaveButton = confirmLeave ? (
    <Button
      key={`leave-${shake}`}
      label={t("pairing.confirmLeave")}
      labelKey="pairing.confirmLeave"
      glyph={<IconSignOut />}
      tone="neutral"
      onClick={() => void leave(() => {})}
      disabled={busy}
      busy={busy}
      className={shakeCls}
    />
  ) : (
    <Button label={t("pairing.leave")} labelKey="pairing.leave" glyph={<IconSignOut />} tone="neutral" onClick={() => setConfirmLeave(true)} />
  );

  const enterTip = t("pairing.enterTip").replace(
    "{path}",
    [t("settings.title"), t("pairing.title"), t("pairing.show")].join(", "),
  );

  const foot = (
    <div className="flex flex-wrap items-center justify-end gap-2">
      {stage === "paired" && (
        <Button
          label={t("pairing.notFirstButton")}
          labelKey="pairing.notFirstButton"
          glyph={<IconPencil />}
          tone="neutral"
          onClick={() => setShown("enter")}
          disabled={busy}
        />
      )}
      <Button
        label={t("pairing.show")}
        labelKey="pairing.show"
        glyph={<IconEye />}
        tone="neutral"
        onClick={openWords}
        disabled={busy}
      />
      {leaveButton}
    </div>
  );

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
            {phrase ? (
              copyButton
            ) : (
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
            )}
          </>
        }
      >
        {phrase ? (
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
        ) : (
          passwordPrompt
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

  let body: ReactNode;
  if (stage === "unpaired") {
    body = (
      <>
        {noPasswordNote}
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
          <Choice
            glyph={<PhraseGlyph kind="enter" size={22} />}
            title={t("pairing.enter")}
            sub={t("pairing.enterSub")}
            onClick={() => setShown("enter")}
          />
        </div>
      </>
    );
  } else if (stage === "alone") {
    body = (
      <>
        <p className="text-sm text-carbon-textSub">{t("pairing.aloneLead")}</p>
        {noPasswordNote}
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <Choice
            glyph={<IconEye />}
            title={t("pairing.notYetTitle")}
            sub={t("pairing.notYetBody")}
            onClick={openWords}
          />
          <Choice
            glyph={<PhraseGlyph kind="enter" size={22} />}
            title={t("pairing.twoTitle")}
            sub={t("pairing.twoBody")}
            onClick={() => setShown("join")}
          />
        </div>
        {noRelay && (
          <p className="text-sm text-carbon-textSub">
            <strong className="font-semibold text-carbon-text">{t("pairing.otherNetTitle")}</strong> {t("pairing.otherNetBody")}
          </p>
        )}
        {noRelay && <CantFindIt onProbe={probe} busy={busy} t={t} shake={shake} />}
        {relayHint}
      </>
    );
  } else {
    body = (
      <>
        {noPasswordNote}
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
        {stage !== "new" && (
          <p className="text-sm text-carbon-textSub">{t(stage === "paired" ? "pairing.pairedHint" : "pairing.searchingHint")}</p>
        )}
        {foot}
      </>
    );
  }

  return (
    <Card
      title={t("pairing.phraseTitle")}
      hint={`${t("pairing.phraseTip")} ${t("pairing.lead")} ${t("pairing.keyNote")}`}
      hueIndex={hueIndex}
    >
      <div className="flex flex-col gap-4.5" data-stage={stage}>
        {body}
      </div>
      {windows}
    </Card>
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
