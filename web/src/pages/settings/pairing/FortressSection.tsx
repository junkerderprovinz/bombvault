// FortressSection is BombVault's place in the fortress, on the Pairing page:
// GlimStone's fortress with the twelve words to join, the phrase behind the
// password and the other apps with their permissions, then the name of this
// server and the card where the phrase changes and BombVault leaves.
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { fill, fortressText, type ChangeAnswer, type FortressAnswer, type PhraseAnswer } from "@glimstone/core";
import { Fortress, FortressPairing } from "@glimstone/react/Fortress";
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
import { Card } from "../shared";
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

export function FortressSection({ nextHue }: { nextHue: () => number }) {
  const { t, lang } = useT();
  const { push } = useToast();
  const [view, setView] = useState<FortressState | null>(null);
  const [error, setError] = useState<string | null>(null);
  // The group's own card on this page has an Enter phrase as well, so the
  // fortress's says what it joins.
  const text = useMemo(() => {
    const own = fortressText(lang);
    return { ...own, enterPhrase: own.joinTitle };
  }, [lang]);

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
  const onChangePhrase = async (): Promise<ChangeAnswer> => {
    const r = await changeFortressPhrase();
    if (!r.ok || !r.phrase) return { error: r.error ?? text.failed };
    load();
    return { words: r.phrase.split(" "), left: r.left ?? [] };
  };
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
        labelKeys={LABEL_KEYS}
        hueStart={hueStart}
      />
      {view.joined && (
        <>
          <DeviceCard view={view} onView={setView} hueIndex={nextHue()} />
          <FortressPairing
            text={{ ...text, pairing: t("fortress.title") }}
            lang={lang}
            app={APP}
            onLeave={onLeave}
            onChangePhrase={view.phrase ? onChangePhrase : undefined}
            labelKeys={LABEL_KEYS}
            hueIndex={nextHue()}
          />
        </>
      )}
    </div>
  );
}

/** DeviceCard holds the name the other members know this server by. */
function DeviceCard({ view, onView, hueIndex }: { view: FortressState; onView: (v: FortressState) => void; hueIndex: number }) {
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
      {!view.phrase && <p className="text-sm text-carbon-textSub">{t("fortress.noPhraseKept")}</p>}
    </Card>
  );
}
