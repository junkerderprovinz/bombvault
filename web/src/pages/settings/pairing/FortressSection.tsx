// The fortress on the Pairing page, in GlimStone's parts: FortressBlock holds
// the one phrase at the top of the page, with its change and the way out,
// and OtherApps shows the other apps of the fortress in the cards of the
// Instances tab, with their permissions.
import { useMemo, useState, type ReactNode } from "react";
import {
  appCardView,
  fill,
  fortressText,
  memberName,
  othersLine,
  type ChangeAnswer,
  type FortressAnswer,
  type FortressApp,
  type FortressInstance,
  type FortressPermission,
  type FortressText,
  type PhraseAnswer,
} from "@glimstone/core";
import { FieldRow } from "@glimstone/react/FieldRow";
import { Fortress, FortressPairing } from "@glimstone/react/Fortress";
import { PasswordField } from "@glimstone/react/PasswordField";
import { Window } from "@glimstone/react/Window";
import { Badge } from "../../../components/Badge";
import { Button } from "../../../components/Button";
import { Toggle } from "../../../components/Toggle";
import { IconEye } from "../../../components/glyphs";
import { IconTabApps, IconTrash } from "../../../components/navGlyphs";
import {
  changeFortressPhrase,
  leaveFortress,
  removeFortressMember,
  setFortressPermission,
  showFortressPhrase,
  showPhrase,
  type FortressState,
  type GroupState,
} from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { useToast } from "../../../lib/toast";
import { Facts, GlyphMark, InstanceCard, LogoMark } from "../../instances/InstanceCard";
import { fortressPermissions, permissionOf } from "./fortressModel";

const APP = { id: "bombvault", name: "BombVault" };
// The words BombVault's own buttons use for the same acts, so that they wear
// the same glyphs: a new phrase is a key rotated.
const LABEL_KEYS = {
  join: "pairing.join",
  leave: "pairing.leave",
  showPhrase: "pairing.show",
  changePhrase: "api.rotate",
  remove: "fortress.remove",
  later: "fortress.dismiss",
};

/** GiveBack is the setting that lets the apps in the fortress take their own
 *  backups back at all. */
export interface GiveBack {
  on: boolean;
  onChange: (on: boolean) => void;
}

/** Others is the switch that shows the other apps of the fortress on this
 *  device, which stands at the top of the fortress's card. */
export interface Others {
  apps: FortressApp[];
  on: boolean;
  onChange: (on: boolean) => void;
}

/** useFortressText is GlimStone's words for the fortress with BombVault's own
 *  where this page says more: the phrase also pairs phones, whose app needs
 *  a relay, and members away follow a new phrase for 30 days. */
function useFortressText(relayOff: boolean): FortressText {
  const { t, lang } = useT();
  return useMemo(() => {
    const own = fortressText(lang);
    return {
      ...own,
      pairing: t("fortress.title"),
      changePhraseAbout: t("fortress.changePhraseAbout"),
      changeBody: t("fortress.changeBody"),
      changeAway: t("fortress.changeAway"),
      changedLeft: t("fortress.changedLeft"),
      scanHint: t(relayOff ? "pairing.phoneNeedsRelay" : "pairing.qrCaption"),
    };
  }, [lang, t, relayOff]);
}

/** FortressBlock is the one place of the phrase once this instance is on
 *  one: rows of the page's own first, then the phrase with its change, and
 *  leaving, which takes the instance out of its group and the fortress
 *  together. */
export function FortressBlock({
  view,
  group,
  onChanged,
  hueIndex,
  others,
  children,
}: {
  view: FortressState;
  group: GroupState;
  /** Reads the group and the fortress again after a new phrase or a leave. */
  onChanged: () => void;
  hueIndex: number;
  others?: Others;
  children?: ReactNode;
}) {
  const { t, lang } = useT();
  const text = useFortressText(group.relay.mode === "off");
  const [onChangePhrase, passwordWindow] = usePhraseChange(view, text, onChanged);
  // An own relay rides along on a second line of the code, since the app has
  // no other way to learn its address.
  const phraseQr = group.relay.mode === "own" ? (words: string[]) => `${words.join(" ")}\n${group.relay.url}` : undefined;

  const onShowPhrase = async (password?: string): Promise<PhraseAnswer> => {
    const r = await (view.phrase ? showFortressPhrase(password ?? "") : showPhrase(password ?? ""));
    if (r.ok && r.phrase) return { words: r.phrase.split(" ") };
    return { error: r.code === "passwordWrong" ? text.passwordWrong : (r.error ?? text.failed) };
  };
  const onLeave = async (): Promise<FortressAnswer> => {
    const r = await leaveFortress();
    if (!r.ok) return { error: r.error ?? text.failed };
    onChanged();
    return { ok: true };
  };

  return (
    <div id="fortress" data-new="fortress-join" className="scroll-mt-6">
      <FortressPairing
        text={text}
        lang={lang}
        app={APP}
        onShowPhrase={view.phrase || (group.active && !view.joined) ? onShowPhrase : undefined}
        phrasePassword={view.passwordSet}
        phraseQr={phraseQr}
        onChangePhrase={view.phrase ? onChangePhrase : undefined}
        onLeave={onLeave}
        pairedWith={view.pairedWith}
        labelKeys={LABEL_KEYS}
        hueIndex={hueIndex}
      >
        {others && (
          <FieldRow label={text.showOthers} hint={`${text.about} ${othersLine(text, lang, others.apps)}`}>
            <Toggle label={text.showOthers} hideLabel checked={others.on} onChange={others.onChange} />
          </FieldRow>
        )}
        {children}
        {view.joined && !view.phrase && <p className="text-sm text-carbon-textSub">{t("fortress.noPhraseKept")}</p>}
      </FortressPairing>
      {passwordWindow}
    </div>
  );
}

/** usePhraseChange is the change of the phrase, which the phrase card offers
 *  and the fortress again right after a removal. The answer shows the new
 *  words, so they come only for the password, as showing the phrase does,
 *  and GlimStone's change window waits for it meanwhile. */
function usePhraseChange(view: FortressState, text: FortressText, onChanged: () => void): [() => Promise<ChangeAnswer>, ReactNode] {
  const [asking, setAsking] = useState<((answer: ChangeAnswer) => void) | null>(null);
  const onChangePhrase = (): Promise<ChangeAnswer> =>
    view.passwordSet
      ? new Promise((resolve) => setAsking(() => resolve))
      : changePhrase(text, "").then((answer) => {
          if (answer.words) onChanged();
          return answer;
        });
  const passwordWindow = asking && (
    <ChangePassword
      text={text}
      onAnswer={(answer) => {
        setAsking(null);
        if (answer.words) onChanged();
        asking(answer);
      }}
    />
  );
  return [onChangePhrase, passwordWindow];
}

/** RemovedHere says another app took this instance out of the fortress,
 *  from then until it joins again. */
export function RemovedHere({ view }: { view: FortressState }) {
  const { lang } = useT();
  if (!view.removedHere || view.joined) return null;
  const text = fortressText(lang);
  return (
    <p data-fortress="removed-here" className="text-sm text-carbon-textSub">
      <strong className="font-semibold text-carbon-text">{text.removedHereTitle}.</strong> {fill(text.removedHereHint, { app: APP.name })}
    </p>
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

/** AppCard is another app of the fortress in the card the Instances tab draws
 *  for an instance: its logo, its state and where it runs, with Remove and
 *  Open along the foot as the Instances tab has them. Where it cannot open
 *  from here, the place of Open says where it does. */
function AppCard({
  app,
  text,
  hueIndex,
  remove,
}: {
  app: FortressApp;
  text: FortressText;
  hueIndex: number;
  /** GlimStone's window that asks before an instance is removed. */
  remove?: (id: string, name: string) => void;
}) {
  const { lang } = useT();
  const view = appCardView(app, text, lang);
  const presence = (p: string) => (p === "connected" ? text.connected : p === "checking" ? text.checking : text.away);
  const removeButton = (instance: FortressInstance) =>
    remove &&
    !instance.self && (
      <Button
        label={text.remove}
        labelKey="fortress.remove"
        glyph={<IconTrash />}
        tone="neutral"
        title={memberName(text, app, instance)}
        onClick={() => remove(instance.id, memberName(text, app, instance))}
      />
    );
  return (
    <InstanceCard
      cardKey={`app-${app.id}`}
      name={app.name}
      eyebrow={view.eyebrow || undefined}
      mark={
        app.logo ? (
          <LogoMark src={app.logo} />
        ) : (
          <GlyphMark>
            <IconTabApps />
          </GlyphMark>
        )
      }
      badges={
        <>
          {view.needs && (
            <Badge tone="warn" size="large">
              {view.needs}
            </Badge>
          )}
          {view.relay && <Badge size="large">{text.viaRelay}</Badge>}
          <Badge tone={view.presence === "connected" ? "ok" : "neutral"} size="large">
            {presence(view.presence)}
          </Badge>
        </>
      }
      facts={<Facts items={view.facts} />}
      actions={
        <>
          {view.rows.length === 0 && removeButton(view.main)}
          {view.open ? (
            <Button
              label={view.open.label}
              labelKey="instances.open"
              glyph={<IconEye />}
              tone="neutral"
              onClick={() => window.open(view.open!.href, "_blank", "noopener")}
            />
          ) : (
            <span data-app-note className="inline-flex h-(--btn-h) items-center text-xs text-carbon-textMuted">
              {view.note}
            </span>
          )}
        </>
      }
      index={hueIndex}
    >
      {view.rows.length > 0 && (
        <ul className="flex flex-col gap-2" data-app-rows>
          {view.rows.map(({ instance, sub }) => (
            <li key={instance.id} className="flex items-center gap-3 rounded-control bg-carbon-surface2 px-3 py-2">
              <span className="flex min-w-0 flex-1 flex-col">
                <span className="text-sm font-medium text-carbon-text wrap-anywhere">{instance.device}</span>
                <span className="glim-num text-xs text-carbon-textMuted">{sub}</span>
              </span>
              <Badge tone={instance.presence === "connected" ? "ok" : "neutral"}>{presence(instance.presence)}</Badge>
              {removeButton(instance)}
            </li>
          ))}
        </ul>
      )}
    </InstanceCard>
  );
}

/** OtherApps is the rest of the fortress: the switch that shows the other
 *  apps, their cards with Remove, and the permissions. Giving an app its own
 *  backup back needs the permission and the setting that lets backups go back
 *  at all, so its one switch turns both. */
export function OtherApps({
  view,
  apps,
  group,
  giveBack,
  showOthers,
  hueStart,
  onChanged,
}: {
  view: FortressState;
  apps: FortressApp[];
  group: GroupState;
  giveBack?: GiveBack;
  /** The switch for it stands in the fortress's card above. */
  showOthers: boolean;
  hueStart: number;
  onChanged: () => void;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const text = useFortressText(group.relay.mode === "off");
  const [onChangePhrase, passwordWindow] = usePhraseChange(view, text, onChanged);
  const all = fortressPermissions(view.permissions, view.members, t);
  const givesBack = (p: FortressPermission | undefined) => !!giveBack && !!p && p.from === APP.id && p.id.endsWith(" backup.import");
  const permissions = all.map((p) => (givesBack(p) ? { ...p, on: p.on && !!giveBack?.on } : p));
  const giveBackAt = all.find(givesBack)?.id;

  const onPermission = async (id: string, on: boolean): Promise<FortressAnswer> => {
    const p = all.find((q) => q.id === id);
    // A give-back switch shown off over a permission that is on lacks only the setting.
    if (!givesBack(p) || p?.on !== on) {
      const r = await setFortressPermission(permissionOf(id, on));
      if (!r.ok) return { error: r.error ?? text.failed };
    }
    if (giveBack && givesBack(p)) {
      // The setting stays on while another app may still take its backup back.
      const othersOn = all.some((q) => q.id !== id && givesBack(q) && q.on);
      if (on ? !giveBack.on : giveBack.on && !othersOn) giveBack.onChange(on);
    }
    onChanged();
    return { ok: true };
  };
  const onRemove = async (id: string): Promise<FortressAnswer> => {
    const r = await removeFortressMember(id);
    if (!r.ok) return { error: r.error ?? text.failed };
    onChanged();
    return { ok: true };
  };

  return (
    <>
      <Fortress
        text={text}
        lang={lang}
        app={APP}
        joined
        apps={apps}
        showOthers={showOthers}
        hideSwitch
        permissions={permissions}
        onPermission={onPermission}
        notify={(message, severity) => push(message, severity)}
        onJoin={() => Promise.resolve({ error: text.failed })}
        onRemove={onRemove}
        onChangePhrase={view.phrase ? onChangePhrase : undefined}
        removed={view.removed.map((r) => fill(text.memberOn, { app: r.name, device: r.device }))}
        renderApp={(app, hueIndex, remove) => <AppCard app={app} text={text} hueIndex={hueIndex} remove={remove} />}
        permissionExtra={(p: FortressPermission) =>
          p.id === giveBackAt && <p className="text-xs text-carbon-textMuted">{t("appImport.toggleHint")}</p>
        }
        labelKeys={LABEL_KEYS}
        hueStart={hueStart}
      />
      {passwordWindow}
    </>
  );
}
