// The first step of the emergency window: the person shows the key of the
// install that is gone, with its recovery kit, through the group or typed.
// The server tries it on every place and keeps it in memory. Only taking the
// key over stores it.
import { useEffect, useRef, useState, type Dispatch, type ReactNode, type SetStateAction } from "react";
import { createPortal } from "react-dom";
import {
  adoptEmergencyKey,
  finishEmergency,
  getEmergency,
  getGroup,
  readEmergencyKey,
  readEmergencyKit,
  storeEmergencySignIns,
  type EmergencyPlace,
  type EmergencyRefusal,
  type EmergencyState,
  type OkEnvelope,
  type Settings,
} from "../../lib/api";
import { PLACE_LABEL, minutesLeft, placeAdvice, placeName, refusalText, sessionKeyInUse, takenOver } from "../../lib/emergency";
import { useT, type TranslationKey } from "../../lib/i18n";
import type { EmergencyWay } from "../../lib/restorePlan";
import { useConfirm } from "../../lib/useConfirm";
import { useReveal } from "../../lib/useReveal";
import { CloudCard } from "../../pages/settings/CloudCard";
import { RcloneCard } from "../../pages/settings/RcloneCard";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { FolderBrowser } from "../FolderBrowser";
import { IconBack, IconKey, IconUpload } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { IconCheck, IconFleet, IconRestore } from "../navGlyphs";
import { RevealInput } from "../RevealInput";
import { Rows } from "../storage/rows";
import { EMERGENCY_FIELD, Note, SECTION_OFFSITE, SECTION_PATHS, Section } from "./EmergencyParts";
import { EmergencyGroup, askedDeposit } from "./EmergencyGroup";
import { EmergencyScene } from "./EmergencyScene";
import { KitIdentityField, KitPicker, takeIdentity } from "./KitPicker";

type Answer = Omit<OkEnvelope, "code"> & { code?: EmergencyRefusal; emergency?: EmergencyState };

const NO_CREDS = { s3KeyId: "", s3Secret: "", s3Region: "", restUser: "", restPassword: "" };

// The key leaves the server's memory on the minute, so the line that counts
// them is read again a few times in each.
const CLOCK_MS = 15_000;

function WayCard({
  on,
  glyph,
  title,
  sub,
  note,
  wide = false,
  onPick,
}: {
  on: boolean;
  glyph: ReactNode;
  title: string;
  sub: string;
  /** Something of this way that waits for the person. */
  note?: string;
  /** Takes the whole row, as the way of last resort does. */
  wide?: boolean;
  onPick: () => void;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={on}
      onClick={onPick}
      className={`flex items-start gap-3 rounded-card bg-carbon-surface2 p-4 text-start ring-2 ring-inset transition-colors hover:bg-carbon-surface3 ${
        on ? "ring-accent" : "ring-transparent"
      } ${wide ? "sm:col-span-2" : ""}`}
    >
      <span className="flex h-11 w-11 flex-none items-center justify-center rounded-control bg-accent text-accentContrast [&_svg]:h-5 [&_svg]:w-5">
        {glyph}
      </span>
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="text-sm font-semibold text-carbon-text wrap-anywhere">{title}</span>
        <span className="text-xs leading-relaxed text-carbon-textMuted wrap-anywhere">{sub}</span>
        {note && <span className="pt-1 text-xs font-semibold text-accentText wrap-anywhere">{note}</span>}
      </span>
    </button>
  );
}

function PlaceRow({ place }: { place: EmergencyPlace }) {
  const { t } = useT();
  const advice = placeAdvice(t, place);
  const name = placeName(t, place);
  return (
    <div data-testid="emergency-place" className="flex flex-col gap-1 py-3 first:pt-0 last:pb-0">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <div className="flex min-w-0 flex-1 basis-48 flex-col">
          <span className="text-sm font-semibold text-carbon-text wrap-anywhere">{name}</span>
          {name !== place.location && (
            <span dir="ltr" className="font-mono text-start text-xs text-carbon-textMuted wrap-anywhere">
              {place.location}
            </span>
          )}
        </div>
        {place.state === "readable" ? (
          <span className="flex items-center gap-1.5 text-sm text-carbon-text">
            <span aria-hidden="true" className="h-2 w-2 rounded-full bg-statusOkSolid" />
            {t(PLACE_LABEL.readable)}
          </span>
        ) : (
          <Badge tone={place.state === "wrong-key" ? "fail" : "warn"}>{t(PLACE_LABEL[place.state])}</Badge>
        )}
      </div>
      {advice && <p className="text-xs leading-relaxed text-carbon-textMuted wrap-break-word">{advice}</p>}
    </div>
  );
}

/** How long the key stays in the server's memory, counted down by the minute. */
function TimeLeft({ expiresAt, onEnd }: { expiresAt: number; onEnd: () => void }) {
  const { t } = useT();
  const [left, setLeft] = useState(() => minutesLeft(expiresAt, Date.now()));
  const ended = useRef(onEnd);
  ended.current = onEnd;
  useEffect(() => {
    const read = () => {
      const now = minutesLeft(expiresAt, Date.now());
      setLeft(now);
      if (now === 0) ended.current();
    };
    read();
    const timer = window.setInterval(read, CLOCK_MS);
    return () => window.clearInterval(timer);
  }, [expiresAt]);
  return <>{t("emergency.timeLeft", left).replace("{n}", String(left))}</>;
}

export function EmergencyAccess({
  emergency,
  onEmergency,
  settings,
  setSettings,
  hostMountRoot,
  footer,
  start = null,
  onOldFolder,
  onPairing,
}: {
  /** What GET /api/emergency answered last, null before the first answer. */
  emergency: EmergencyState | null;
  onEmergency: (state: EmergencyState) => void;
  /** The settings the window holds. The typed way names its places in them. */
  settings: Settings | null;
  setSettings: Dispatch<SetStateAction<Settings | null>>;
  hostMountRoot: string;
  /** The place in the window's foot for taking the key over, which is the
   *  way forward while a key is shown and undecided. */
  footer: HTMLElement | null;
  /** The way a link opened the window on. */
  start?: EmergencyWay | null;
  /** Opens the window that reads another install's backups without taking
   *  its key over. */
  onOldFolder?: () => void;
  /** Leads to the page where this install joins a group. */
  onPairing?: () => void;
}) {
  const { t } = useT();
  const { confirm, confirmDialog } = useConfirm();
  // A request for a deposit that waits in the group is where the person
  // left off, so the window opens on that way.
  const [waitingAt] = useState(() => askedDeposit()?.memberName ?? null);
  const [way, setWay] = useState<EmergencyWay | null>(start ?? (waitingAt !== null ? "group" : null));
  const [inGroup, setInGroup] = useState<boolean | null>(null);
  useEffect(() => {
    getGroup()
      .then((group) => setInGroup(group.ok && group.active))
      .catch(() => setInGroup(false));
  }, []);
  const [busy, setBusy] = useState(false);
  const [refusal, setRefusal] = useState<{ code?: string; said?: string } | null>(null);

  const [kitName, setKitName] = useState<string | null>(null);
  // A sealed kit waits here for its identity. A plain one is sent and dropped.
  const [sealed, setSealed] = useState<string | null>(null);
  const identity = useRef<HTMLInputElement>(null);

  const [key, setKey] = useState("");
  const [creds, setCreds] = useState(NO_CREDS);
  const revealKey = useReveal();
  const revealSecret = useReveal();
  const revealPassword = useReveal();

  const [signInsSaved, setSignInsSaved] = useState(false);

  const session = emergency?.session;
  const taken = takenOver(emergency);
  const inUse = sessionKeyInUse(emergency);

  function forgetSecrets() {
    setKey("");
    setCreds((prev) => ({ ...prev, s3Secret: "", restPassword: "" }));
    setSealed(null);
    setKitName(null);
  }

  async function ended() {
    forgetSecrets();
    setRefusal({ code: "emergency-ended" });
    const now = await getEmergency().catch(() => null);
    if (now?.ok && now.emergency) onEmergency(now.emergency);
  }

  async function ask(call: () => Promise<Answer>): Promise<Answer | null> {
    setBusy(true);
    try {
      const res = await call();
      if (res.emergency) onEmergency(res.emergency);
      if (res.ok) setRefusal(null);
      else if (res.code === "emergency-ended") await ended();
      else setRefusal({ code: res.code, said: res.error });
      return res;
    } catch (err) {
      setRefusal({ said: err instanceof Error ? err.message : String(err) });
      return null;
    } finally {
      setBusy(false);
    }
  }

  async function readKit(text: string, name: string) {
    setKitName(name);
    setSealed(null);
    const res = await ask(() => readEmergencyKit(text));
    if (res?.code === "kit-sealed") {
      setSealed(text);
      setRefusal(null);
    }
  }

  async function openSealed() {
    if (sealed === null) return;
    const secret = takeIdentity(identity);
    const res = await ask(() => readEmergencyKit(sealed, secret));
    if (res?.ok) setSealed(null);
  }

  async function readKey() {
    if (!settings) return;
    const places = [...SECTION_PATHS, ...SECTION_OFFSITE].map(([name]) => settings[name].trim()).filter((place) => place !== "");
    const typed = Object.values(creds).some((value) => value.trim() !== "");
    await ask(() =>
      readEmergencyKey(
        key.trim(),
        places,
        typed ? { ...creds, s3KeyId: creds.s3KeyId.trim(), s3Region: creds.s3Region.trim(), restUser: creds.restUser.trim(), s3StorageClass: "" } : undefined
      )
    );
  }

  async function adopt() {
    const yes = await confirm(t("emergency.adoptAsk"), { confirmLabel: t("emergency.adopt"), confirmLabelKey: "snapshots.restore" });
    if (yes) await ask(adoptEmergencyKey);
  }

  async function saveSignIns() {
    const res = await ask(storeEmergencySignIns);
    if (res?.ok) setSignInsSaved(true);
  }

  // Nothing was stored, so the key only has to leave the server's memory.
  async function anotherWay() {
    await ask(finishEmergency);
    forgetSecrets();
    setWay(null);
  }

  const refused = refusal && <Note tone="fail">{refusalText(t, refusal.code, refusal.said)}</Note>;

  if (session) {
    const readable = session.places.filter((place) => place.state === "readable").length;
    const needsSignIn = session.places.some((place) => place.state === "sign-in");
    const count = t(session.source === "kit" ? "emergency.readKit" : session.source === "group" ? "emergency.readGroup" : "emergency.readKey")
      .replace("{n}", String(readable))
      .replace("{total}", String(session.places.length));
    return (
      <>
        {taken ? (
          <Note tone="ok">
            {t("emergency.adopted")}{" "}
            {t(emergency?.undoable ? "emergency.undoable" : "emergency.code.adopt-in-use")}
          </Note>
        ) : (
          <Note tone={readable > 0 ? "ok" : "warn"}>{count}</Note>
        )}
        <p className="text-xs leading-relaxed text-carbon-textMuted">
          {!inUse && <>{t("emergency.notSaved")} </>}
          <TimeLeft expiresAt={session.expiresAt} onEnd={() => void ended()} />
        </p>
        {inUse && !taken && <Note tone="neutral">{t("emergency.code.adopt-same-key")}</Note>}

        <Section title={t("storage.title")}>
          {session.places.length === 0 ? (
            <p className="text-sm text-carbon-textMuted">{t("emergency.noPlaces")}</p>
          ) : (
            <Rows>
              {session.places.map((place) => (
                <PlaceRow key={`${place.kind}:${place.domain ?? ""}:${place.name ?? ""}:${place.location}`} place={place} />
              ))}
            </Rows>
          )}
          {readable > 0 && (
            <p className="text-xs text-carbon-textMuted">{t(session.encrypted ? "recovery.encEncrypted" : "recovery.encPlain")}</p>
          )}
        </Section>

        {!inUse && session.refusal === "adopt-own-backups" && (
          <Note tone="warn">
            <span className="flex flex-wrap items-center gap-3">
              <span className="min-w-0 flex-1 basis-56">{t("emergency.code.adopt-own-backups")}</span>
              {onOldFolder && (
                <Button label={t("restorePage.oldFolder")} labelKey={null} glyph={<IconRestore />} onClick={onOldFolder} />
              )}
            </span>
          </Note>
        )}
        {!inUse && session.refusal === "adopt-nothing-readable" && <Note tone="warn">{t("emergency.code.adopt-nothing-readable")}</Note>}
        {refused}

        {taken && (session.cloudSignIn || session.rcloneConfig) && (
          <Section title={t("emergency.signIns")}>
            <p className="text-sm text-carbon-text">{t("emergency.signInsText")}</p>
            <div className="flex flex-wrap items-center gap-3">
              <Button
                label={t("emergency.signInsSave")}
                labelKey={null}
                glyph={<IconCheck />}
                onClick={() => void saveSignIns()}
                disabled={busy || signInsSaved}
                busy={busy}
              />
              {signInsSaved && <span className="text-sm text-statusOk">{t("emergency.signInsSaved")}</span>}
            </div>
          </Section>
        )}
        {inUse && needsSignIn && (
          <Section title={t("emergency.signInByHand")} hint={`${t("recovery.cloudCredsHint")} ${t("recovery.credsSaveHint")}`}>
            <CloudCard t={t} hueIndex={5} nested />
            <RcloneCard t={t} hueIndex={6} nested />
          </Section>
        )}

        {!taken && (
          <div>
            <Button
              label={t("emergency.another")}
              labelKey={null}
              glyph={<IconBack />}
              tone="subtle"
              onClick={() => void anotherWay()}
              disabled={busy}
            />
          </div>
        )}
        {!inUse &&
          footer &&
          createPortal(
            <Button
              label={t("emergency.adopt")}
              labelKey={null}
              glyph={<IconKey />}
              tone="accent"
              onClick={() => void adopt()}
              disabled={busy || session.refusal !== undefined}
              busy={busy}
            />,
            footer
          )}
        {confirmDialog}
      </>
    );
  }

  const credField = (name: "s3KeyId" | "s3Region" | "restUser", label: TranslationKey, hint?: TranslationKey) => (
    <label className="flex flex-col gap-1 text-xs text-carbon-textSub">
      <span className="flex items-center gap-1">
        {t(label)}
        {hint && <InfoBubble tip={t(hint)} />}
      </span>
      <input
        value={creds[name]}
        spellCheck={false}
        autoComplete="off"
        onChange={(e) => setCreds((prev) => ({ ...prev, [name]: e.target.value }))}
        className={EMERGENCY_FIELD}
      />
    </label>
  );
  const secretField = (name: "s3Secret" | "restPassword", label: TranslationKey, reveal: ReturnType<typeof useReveal>) => (
    <label className="flex flex-col gap-1 text-xs text-carbon-textSub">
      {t(label)}
      <RevealInput
        {...reveal}
        value={creds[name]}
        spellCheck={false}
        autoComplete="off"
        onChange={(e) => setCreds((prev) => ({ ...prev, [name]: e.target.value }))}
        wrapperClassName="w-full"
        className={EMERGENCY_FIELD}
      />
    </label>
  );

  return (
    <>
      <EmergencyScene />
      {emergency?.backupKey === "locked" && <Note tone="fail">{t("emergency.code.backup-key-locked")}</Note>}
      <p className="text-sm text-carbon-text">{t("emergency.waysIntro")}</p>
      <div role="radiogroup" aria-label={t("emergency.access")} className="grid gap-3 sm:grid-cols-2">
        <WayCard
          on={way === "kit"}
          glyph={<IconUpload />}
          title={t("emergency.way.kit")}
          sub={t("emergency.way.kitSub")}
          onPick={() => setWay("kit")}
        />
        <WayCard
          on={way === "group"}
          glyph={<IconFleet />}
          title={t("emergency.way.group")}
          sub={t("emergency.way.groupSub")}
          note={waitingAt !== null ? t("emergency.group.pending").replace("{keeper}", () => waitingAt) : undefined}
          onPick={() => setWay("group")}
        />
        <WayCard
          wide
          on={way === "key"}
          glyph={<IconKey />}
          title={t("emergency.way.key")}
          sub={t("emergency.way.keySub")}
          onPick={() => setWay("key")}
        />
      </div>
      {refused}

      {way === "kit" && (
        <>
          <KitPicker name={kitName} busy={busy && sealed === null} onPick={(text, name) => void readKit(text, name)} />
          {sealed !== null && (
            <Section title={t("emergency.way.kit")}>
              <p className="text-sm text-carbon-text">{t("emergency.code.kit-sealed")}</p>
              <KitIdentityField inputRef={identity} onEnter={() => void openSealed()} />
              <div className="flex justify-end">
                <Button
                  label={t("kit.open")}
                  labelKey={null}
                  glyph={<IconKey />}
                  tone="accent"
                  onClick={() => void openSealed()}
                  disabled={busy}
                  busy={busy}
                />
              </div>
            </Section>
          )}
        </>
      )}

      {way === "group" && (
        <EmergencyGroup
          active={inGroup}
          onEmergency={(state) => {
            setRefusal(null);
            onEmergency(state);
          }}
          onPairing={onPairing}
        />
      )}

      {way === "key" &&
        (settings ? (
          <Section title={t("emergency.way.key")} hint={t("emergency.attachHint")}>
            <label className="flex flex-col gap-1 text-xs text-carbon-textSub">
              <span className="flex items-center gap-1">
                {t("emergency.key")}
                <InfoBubble tip={t("emergency.keyHint")} />
              </span>
              <RevealInput
                {...revealKey}
                value={key}
                spellCheck={false}
                autoComplete="off"
                onChange={(e) => setKey(e.target.value)}
                wrapperClassName="w-full"
                className={EMERGENCY_FIELD}
              />
            </label>
            {SECTION_PATHS.map(([name, label], i) => (
              <FolderBrowser
                key={name}
                label={t(label)}
                value={settings[name]}
                hostMountRoot={hostMountRoot}
                hueIndex={i}
                onChange={(v) => setSettings((prev) => (prev ? { ...prev, [name]: v } : prev))}
              />
            ))}
            <h4 className="pt-1 text-xs font-semibold text-carbon-textSub">{t("emergency.offsite")}</h4>
            {SECTION_OFFSITE.map(([name, label]) => (
              <label key={name} className="flex flex-col gap-1 text-xs text-carbon-textSub">
                {t(label)}
                <input
                  value={settings[name]}
                  spellCheck={false}
                  onChange={(e) => setSettings((prev) => (prev ? { ...prev, [name]: e.target.value } : prev))}
                  placeholder="rest:http://host:8000/repo"
                  dir="ltr"
                  className={EMERGENCY_FIELD}
                />
              </label>
            ))}
            <h4 className="flex items-center gap-1.5 pt-1 text-xs font-semibold text-carbon-textSub">
              {t("recovery.cloudCreds")}
              <InfoBubble tip={t("recovery.cloudCredsHint")} />
            </h4>
            <div className="grid gap-3 sm:grid-cols-2">
              {credField("s3KeyId", "recovery.foreignS3KeyId")}
              {secretField("s3Secret", "recovery.foreignS3Secret", revealSecret)}
              {credField("s3Region", "recovery.foreignS3Region")}
              {credField("restUser", "recovery.foreignRestUser", "recovery.foreignRestHint")}
              {secretField("restPassword", "recovery.foreignRestPassword", revealPassword)}
            </div>
            <div className="flex justify-end">
              <Button
                label={t("emergency.connect")}
                labelKey={null}
                glyph={<IconCheck />}
                tone="accent"
                onClick={() => void readKey()}
                disabled={busy || key.trim() === ""}
                busy={busy}
              />
            </div>
          </Section>
        ) : (
          <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
        ))}
    </>
  );
}
