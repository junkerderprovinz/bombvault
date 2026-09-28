// PhraseCard is where the twelve words appear and are typed: start a group,
// join one, show the words again, see who is in the group, and leave it.
import { useState } from "react";
import { Card } from "../settings/shared";
import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { RevealInput } from "../../components/RevealInput";
import {
  createPhrase,
  joinGroup,
  leaveGroup,
  showPhrase,
  type GroupMember,
  type GroupState,
  type PhraseRefusal,
} from "../../lib/api";
import { copyText } from "../../lib/clipboard";
import type { useT } from "../../lib/i18n";
import { useReveal } from "../../lib/useReveal";
import { useToast } from "../../lib/toast";

type T = ReturnType<typeof useT>["t"];

const inputCls = "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus";

/** refusalText says what is wrong with a typed phrase in the reader's
 *  language, naming the word at fault. */
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

function WordGrid({ phrase, t }: { phrase: string; t: T }) {
  const { push } = useToast();
  const words = phrase.split(/\s+/);
  async function copy() {
    if (await copyText(phrase)) push(t("common.copied"), "success");
    else push(t("vm.ssh.copyFailed"), "fail");
  }
  return (
    <div className="flex flex-col gap-2">
      <ol className="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4" aria-label={t("pairing.wordsLabel")}>
        {words.map((w, i) => (
          <li key={i} className="flex items-baseline gap-2 rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm">
            <span className="w-5 shrink-0 text-end text-xs tabular-nums text-carbon-textMuted">{i + 1}</span>
            <span dir="ltr" className="font-mono text-carbon-text">
              {w}
            </span>
          </li>
        ))}
      </ol>
      <div className="flex flex-wrap items-center gap-2">
        <Button label={t("common.copy")} labelKey="common.copy" tone="neutral" onClick={() => void copy()} />
        <p className="text-caption text-carbon-textMuted">{t("pairing.wordsNote")}</p>
      </div>
    </div>
  );
}

function MemberRow({ m, t }: { m: GroupMember; t: T }) {
  return (
    <li className="flex flex-wrap items-center gap-2 rounded-control bg-carbon-surface2 px-3 py-2">
      <span className="text-sm font-medium text-carbon-text min-w-0 wrap-break-word">{m.name || m.id}</span>
      {m.version && <span className="text-xs text-carbon-textMuted">{m.version}</span>}
      <span className="ms-auto">
        <Badge tone={m.direct ? "ok" : "neutral"}>{m.direct ? t("pairing.direct") : t("pairing.viaRelay")}</Badge>
      </span>
    </li>
  );
}

export function PhraseCard({
  group,
  onGroup,
  t,
  hueIndex,
}: {
  group: GroupState;
  onGroup: (g: GroupState) => void;
  t: T;
  hueIndex?: number;
}) {
  const { push } = useToast();
  const [phrase, setPhrase] = useState<string | null>(null);
  const [joining, setJoining] = useState(false);
  const [typed, setTyped] = useState("");
  const [refusal, setRefusal] = useState<string | null>(null);
  const [password, setPassword] = useState("");
  const [askPassword, setAskPassword] = useState(false);
  const [confirmLeave, setConfirmLeave] = useState(false);
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);
  const reveal = useReveal();

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

  const create = () =>
    run(async () => {
      const res = await createPhrase();
      if (res.ok && res.phrase) {
        setPhrase(res.phrase);
        if (res.group) onGroup(res.group);
      } else {
        push(res.error ?? t("pairing.actionError"), "fail");
        setShake((n) => n + 1);
      }
    });

  const join = () =>
    run(async () => {
      const res = await joinGroup(typed);
      if (res.ok) {
        setTyped("");
        setJoining(false);
        setRefusal(null);
        onGroup(res);
        push(t("pairing.joined"), "success");
      } else {
        setRefusal(refusalText(t, res));
        setShake((n) => n + 1);
      }
    });

  const show = () =>
    run(async () => {
      const res = await showPhrase(password);
      if (res.ok && res.phrase) {
        setPhrase(res.phrase);
        setAskPassword(false);
        setPassword("");
      } else {
        push(res.code === "passwordWrong" ? t("pairing.passwordWrong") : (res.error ?? t("pairing.actionError")), "fail");
        setShake((n) => n + 1);
      }
    });

  const leave = () =>
    run(async () => {
      const res = await leaveGroup();
      if (res.ok) {
        setPhrase(null);
        setConfirmLeave(false);
        onGroup(res);
      } else {
        push(res.error ?? t("pairing.actionError"), "fail");
        setShake((n) => n + 1);
      }
    });

  const shakeCls = shake ? "glim-shake" : "";

  return (
    <Card title={t("pairing.phraseTitle")} hint={t("pairing.phraseHint")} hueIndex={hueIndex}>
      {!group.active ? (
        <>
          <p className="text-sm text-carbon-textSub">{t("pairing.notPaired")}</p>
          {!joining && (
            <div className="flex flex-wrap items-center gap-2">
              <Button
                key={`create-${shake}`}
                label={t("pairing.create")}
                labelKey="pairing.create"
                tone="accent"
                onClick={() => void create()}
                disabled={busy}
                busy={busy}
                className={shakeCls}
              />
              <Button label={t("pairing.enter")} labelKey="pairing.enter" tone="neutral" onClick={() => setJoining(true)} />
            </div>
          )}
          {joining && (
            <div className="flex flex-col gap-2">
              <label htmlFor="pairing-phrase" className="text-xs text-carbon-textSub">
                {t("pairing.enterLabel")}
              </label>
              <textarea
                id="pairing-phrase"
                value={typed}
                onChange={(e) => {
                  setTyped(e.target.value);
                  setRefusal(null);
                }}
                rows={3}
                spellCheck={false}
                autoComplete="off"
                autoCapitalize="none"
                dir="ltr"
                autoFocus
                placeholder={t("pairing.enterPlaceholder")}
                className={`${inputCls} font-mono resize-y`}
              />
              {refusal && (
                <p role="alert" className="text-caption text-statusFail">
                  {refusal}
                </p>
              )}
              <div className="flex flex-wrap items-center justify-end gap-2">
                <Button
                  label={t("common.cancel")}
                  labelKey="common.cancel"
                  tone="neutral"
                  onClick={() => {
                    setJoining(false);
                    setRefusal(null);
                  }}
                  disabled={busy}
                />
                <Button
                  key={`join-${shake}`}
                  label={t("pairing.join")}
                  labelKey="pairing.join"
                  tone="accent"
                  onClick={() => void join()}
                  disabled={busy || typed.trim() === ""}
                  busy={busy}
                  className={shakeCls}
                />
              </div>
            </div>
          )}
        </>
      ) : (
        <>
          <div className="flex flex-wrap items-center gap-2">
            <Badge tone="ok">{t("pairing.paired")}</Badge>
            <span className="text-xs text-carbon-textMuted">
              {t("pairing.thisInstance").replace("{name}", group.name)}
            </span>
          </div>

          {phrase ? (
            <WordGrid phrase={phrase} t={t} />
          ) : askPassword ? (
            <div className="flex flex-col gap-1.5">
              <label className="text-xs text-carbon-textSub">{t("pairing.passwordLabel")}</label>
              <RevealInput
                {...reveal}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
                wrapperClassName="w-full"
                className={inputCls}
              />
              <div className="flex flex-wrap items-center justify-end gap-2">
                <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={() => setAskPassword(false)} />
                <Button
                  key={`show-${shake}`}
                  label={t("pairing.show")}
                  labelKey="pairing.show"
                  tone="accent"
                  onClick={() => void show()}
                  disabled={busy || password === ""}
                  busy={busy}
                  className={shakeCls}
                />
              </div>
            </div>
          ) : null}

          <div className="flex flex-col gap-2">
            <span className="text-xs text-carbon-textSub">{t("pairing.membersTitle")}</span>
            {group.members.length === 0 ? (
              <p className="text-sm text-carbon-textMuted">{t("pairing.noMembers")}</p>
            ) : (
              <ul className="flex flex-col gap-2">
                {group.members.map((m) => (
                  <MemberRow key={m.id} m={m} t={t} />
                ))}
              </ul>
            )}
          </div>

          <div className="flex flex-wrap items-center justify-end gap-2">
            {phrase ? (
              <Button label={t("pairing.hide")} labelKey="pairing.hide" tone="neutral" onClick={() => setPhrase(null)} />
            ) : (
              !askPassword && (
                <Button
                  key={`reveal-${shake}`}
                  label={t("pairing.show")}
                  labelKey="pairing.show"
                  tone="neutral"
                  onClick={() => (group.passwordSet ? setAskPassword(true) : void show())}
                  disabled={busy}
                  className={shakeCls}
                />
              )
            )}
            {confirmLeave ? (
              <Button
                key={`leave-${shake}`}
                label={t("pairing.confirmLeave")}
                labelKey="pairing.confirmLeave"
                tone="neutral"
                onClick={() => void leave()}
                disabled={busy}
                busy={busy}
                className={shakeCls}
              />
            ) : (
              <Button label={t("pairing.leave")} labelKey="pairing.leave" tone="neutral" onClick={() => setConfirmLeave(true)} />
            )}
          </div>
        </>
      )}
    </Card>
  );
}
