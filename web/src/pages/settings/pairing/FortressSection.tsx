// FortressSection is BombVault's place in the fortress, on the Pairing page:
// GlimStone's fortress with the twelve words to join, the phrase behind the
// password and the other apps with their permissions, then the name of this
// server and the card where the phrase changes and BombVault leaves.
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { fill, fortressText, type ChangeAnswer, type FortressAnswer, type FortressText, type PhraseAnswer } from "@glimstone/core";
import { Fortress, FortressPairing } from "@glimstone/react/Fortress";
import { PasswordField } from "@glimstone/react/PasswordField";
import { Window } from "@glimstone/react/Window";
import {
  changeFortressPhrase,
  getFortress,
  joinFortress,
  leaveFortress,
  setFortressDevice,
  setFortressPermission,
  showFortressPhrase,
  type FortressState,
} from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { relativeTime } from "../../../lib/reltime";
import { useToast } from "../../../lib/toast";
import { Rows, SettingRow } from "../../../components/storage/rows";
import { Card, ToggleRow } from "../shared";
import { fortressApps, fortressPermissions, permissionOf } from "./fortressModel";
import { refusalText } from "./PhraseCard";

// Members come and go with the relay and the network, which nothing pushes
// here, so the page asks again as the group's cards do.
const REFRESH_MS = 10_000;
// The name saves itself after a pause in typing, as the instance name does.
const SAVE_AFTER_MS = 800;

const APP = { id: "bombvault", name: "BombVault" };
// The words BombVault's own buttons use for the same acts, so that they wear
// the same glyphs: a new phrase is a key rotated.
const LABEL_KEYS = { join: "pairing.join", leave: "pairing.leave", showPhrase: "pairing.show", changePhrase: "api.rotate" };

/** GiveBack is the switch that lets the apps in the fortress take their own
 *  backups back, with what saving it needs. */
export interface GiveBack {
  on: boolean;
  onChange: (on: boolean) => void;
  busy?: boolean;
  shakeNonce?: number;
  pulseNonce?: number;
}

export function FortressSection({ nextHue, giveBack }: { nextHue: () => number; giveBack?: GiveBack }) {
  const { t, lang } = useT();
  const { push } = useToast();
  const [view, setView] = useState<FortressState | null>(null);
  const [error, setError] = useState<string | null>(null);
  // GlimStone words the fortress for an Instances tab. On this page the
  // group's card has an Enter phrase too, so the fortress's says what it
  // joins, and the hints do not send anybody to Instances. GlimStone's change
  // texts tell members out of reach that they need the new phrase, but the
  // kit lets them follow on the old one for 30 days.
  const text = useMemo(() => {
    const own = fortressText(lang);
    return {
      ...own,
      enterPhrase: own.joinTitle,
      outHint: t("fortress.outHint"),
      phraseWhere: t("fortress.phraseWhere"),
      changePhraseAbout: t("fortress.changePhraseAbout"),
      changeBody: t("fortress.changeBody"),
      changeAway: t("fortress.changeAway"),
      changedLeft: t("fortress.changedLeft"),
    };
  }, [lang, t]);
  // The step that waits for the password before a new phrase is made.
  const [asking, setAsking] = useState<((answer: ChangeAnswer) => void) | null>(null);

  const load = useCallback(() => {
    getFortress()
      .then((v) => {
        if (v.ok) {
          setView(v);
          setError(null);
        } else {
          setError(v.error ?? text.failed);
        }
      })
      .catch((err) => setError(err instanceof Error ? err.message : text.failed));
  }, [text]);

  useEffect(() => {
    load();
    const id = setInterval(load, REFRESH_MS);
    return () => clearInterval(id);
  }, [load]);

  if (!view) return error ? <p className="text-sm text-statusFail wrap-break-word">{error}</p> : null;

  const apps = fortressApps(view.members, (unix) => relativeTime(t, unix));
  const hueStart = nextHue();
  if (view.joined) for (let i = 0; i <= apps.length; i++) nextHue();

  const onJoin = async (words: string[]): Promise<FortressAnswer> => {
    const r = await joinFortress(words.join(" "));
    if (r.ok) {
      setView(r);
      return { ok: true };
    }
    return { error: r.code === "inFortress" ? t("fortress.inFortress") : refusalText(t, r) };
  };
  const onShowPhrase = async (password?: string): Promise<PhraseAnswer> => {
    const r = await showFortressPhrase(password ?? "");
    if (r.ok && r.phrase) return { words: r.phrase.split(" ") };
    return { error: r.code === "passwordWrong" ? text.passwordWrong : (r.error ?? text.failed) };
  };
  // The answer shows the new words, so they come only for the password, as
  // showing the phrase does. GlimStone's change window waits meanwhile.
  const onChangePhrase = (): Promise<ChangeAnswer> =>
    view.passwordSet
      ? new Promise((resolve) => setAsking(() => resolve))
      : changePhrase(text, "").then((answer) => {
          if (answer.words) load();
          return answer;
        });
  const onLeave = async (): Promise<FortressAnswer> => {
    const r = await leaveFortress();
    if (!r.ok) return { error: r.error ?? text.failed };
    setView(r);
    return { ok: true };
  };
  const onPermission = async (id: string, on: boolean): Promise<FortressAnswer> => {
    const r = await setFortressPermission(permissionOf(id, on));
    if (!r.ok) return { error: r.error ?? text.failed };
    load();
    return { ok: true };
  };

  return (
    <div id="fortress" className="flex scroll-mt-6 flex-col gap-10">
      <Fortress
        text={text}
        lang={lang}
        app={APP}
        joined={view.joined}
        apps={apps}
        permissions={fortressPermissions(view.permissions, view.members, t)}
        onPermission={onPermission}
        notify={(message, severity) => push(message, severity)}
        onJoin={onJoin}
        onRemove={() => Promise.resolve({ error: text.failed })}
        onShowPhrase={view.phrase ? onShowPhrase : undefined}
        phrasePassword={view.passwordSet}
        pairedWith={view.pairedWith}
        labelKeys={LABEL_KEYS}
        hueStart={hueStart}
      />
      {view.joined && (
        <>
          <DeviceCard view={view} onView={setView} giveBack={giveBack} hueIndex={nextHue()} />
          <FortressPairing
            text={{ ...text, pairing: t("fortress.title") }}
            lang={lang}
            app={APP}
            onLeave={onLeave}
            onChangePhrase={view.phrase ? onChangePhrase : undefined}
            pairedWith={view.pairedWith}
            labelKeys={LABEL_KEYS}
            hueIndex={nextHue()}
          />
          {asking && (
            <ChangePassword
              text={text}
              onAnswer={(answer) => {
                setAsking(null);
                if (answer.words) load();
                asking(answer);
              }}
            />
          )}
        </>
      )}
    </div>
  );
}

/** changePhrase asks the server for a new phrase and words its answer for GlimStone's flow. */
async function changePhrase(text: FortressText, password: string): Promise<ChangeAnswer & { wrong?: boolean }> {
  const r = await changeFortressPhrase(password);
  if (r.code === "passwordWrong") return { wrong: true, error: text.passwordWrong };
  if (!r.ok || !r.phrase) return { error: r.error ?? text.failed };
  return { words: r.phrase.split(" "), left: r.left ?? [] };
}

/** ChangePassword asks for the login password in front of a phrase change.
 *  A wrong one keeps it open, and closing it keeps the phrase. */
function ChangePassword({ text, onAnswer }: { text: FortressText; onAnswer: (answer: ChangeAnswer) => void }) {
  const { t } = useT();
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const send = async () => {
    if (busy || password === "") return;
    setBusy(true);
    const answer = await changePhrase(text, password).catch(() => ({ error: text.failed, wrong: false }));
    setBusy(false);
    if (answer.wrong) {
      setError(answer.error ?? text.passwordWrong);
      return;
    }
    onAnswer(answer);
  };

  return (
    <Window
      title={text.phraseTitle}
      text={{ cancel: text.cancel, close: text.close }}
      onClose={() => onAnswer({ error: t("fortress.changeKept") })}
      actions={[
        {
          label: text.changeConfirm,
          labelKey: "api.rotate",
          primary: true,
          disabled: busy || password === "",
          attrs: { "data-fortress": "change-unlock" },
          onClick: () => {
            void send();
            return false;
          },
        },
      ]}
    >
      <form
        className="glim-fortress-text"
        onSubmit={(e) => {
          e.preventDefault();
          void send();
        }}
      >
        <p>{fill(text.phrasePassword, { app: APP.name })}</p>
        <PasswordField
          aria-label={fill(text.password, { app: APP.name })}
          placeholder={fill(text.password, { app: APP.name })}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          text={{ show: text.showPassword, hide: text.hidePassword }}
          autoFocus
        />
        {error && (
          <p className="glim-fortress-error" role="alert">
            {error}
          </p>
        )}
      </form>
    </Window>
  );
}

/** DeviceCard holds the name the other members know this server by, and
 *  whether the apps among them may take their own backups back. */
function DeviceCard({
  view,
  onView,
  giveBack,
  hueIndex,
}: {
  view: FortressState;
  onView: (v: FortressState) => void;
  giveBack?: GiveBack;
  hueIndex: number;
}) {
  const { t } = useT();
  const { push } = useToast();
  // What is being typed, null while the field shows the saved name.
  const [draft, setDraft] = useState<string | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => () => clearTimeout(timer.current ?? undefined), []);

  const save = (name: string) => {
    setFortressDevice(name)
      .then((r) => {
        if (r.ok) {
          onView(r);
          setDraft((d) => (d === name ? null : d));
          push(t("settings.saved"), "success");
        } else {
          push(r.code === "deviceName" ? t("fortress.deviceTooLong") : (r.error ?? t("settings.error")), "fail");
        }
      })
      .catch((err) => push(err instanceof Error ? err.message : t("settings.error"), "fail"));
  };

  return (
    <Card title={t("fortress.deviceTitle")} hueIndex={hueIndex}>
      <Rows>
        <SettingRow
          label={t("fortress.deviceName")}
          hint={fill(t("fortress.deviceHint"), { name: view.defaultDevice })}
          htmlFor="fortress-device"
        >
          <input
            id="fortress-device"
            type="text"
            value={draft ?? view.device}
            maxLength={64}
            onChange={(e) => {
              const v = e.target.value;
              setDraft(v);
              clearTimeout(timer.current ?? undefined);
              timer.current = setTimeout(() => save(v), SAVE_AFTER_MS);
            }}
            spellCheck={false}
            autoComplete="off"
            placeholder={view.defaultDevice}
            className="w-[18rem] max-w-full rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
          />
        </SettingRow>
      </Rows>
      {giveBack && (
        <ToggleRow
          label={t("appImport.toggle")}
          hint={t("appImport.toggleHint")}
          checked={giveBack.on}
          onChange={giveBack.onChange}
          disabled={giveBack.busy}
          shakeNonce={giveBack.shakeNonce}
          pulseNonce={giveBack.pulseNonce}
        />
      )}
      {!view.phrase && <p className="text-sm text-carbon-textSub">{t("fortress.noPhraseKept")}</p>}
    </Card>
  );
}
