// RelayCard picks how this instance reaches members on other networks. It
// says first what a relay is for and offers three routes: the project relay,
// an own relay with the steps to set one up, and the home network only. An
// address that belongs to a paired BombVault still shows that route. Each
// route says in one sentence what it is and what it needs, beside a picture
// of a message on its way. Every control saves on its own, as the rest of the
// settings do.
import { useEffect, useRef, useState, type ReactNode } from "react";
import { Card, ToggleRow } from "../shared";
import { Badge } from "../../../components/Badge";
import { Button } from "../../../components/Button";
import { InfoBubble } from "../../../components/InfoBubble";
import { Selector } from "../../../components/Selector";
import { SettingRow } from "../../../components/storage/rows";
import { IconRetry } from "../../../components/glyphs";
import { setRelay, type GroupState, type RelayMode } from "../../../lib/api";
import type { TranslationKey, useT } from "../../../lib/i18n";
import { useToast } from "../../../lib/toast";
import { tLtr } from "../../../lib/ltrFragments";
import { FactGlyph, RouteDiagram, RouteGlyph } from "./pairingArt";
import { emphasize } from "./PairingSteps";
import type { PairStage } from "./pairStage";
import { relayForm, type RelayForm } from "./relayForm";

type T = ReturnType<typeof useT>["t"];

const FORMS: RelayForm[] = ["project", "own", "off"];
const OWN_STEPS: TranslationKey[] = ["relay.ownStep1", "relay.ownStep2", "relay.ownStep3"];

// An own relay without TLS still works on a LAN, but its relay key then
// travels in the clear, so the card says so.
const PLAINTEXT_RELAY = /^(ws|http):\/\//i;

const COPY: Record<
  RelayForm,
  { name: TranslationKey; glyph: "project" | "own" | "server" | "off"; sentence: TranslationKey; need: TranslationKey; sees: TranslationKey }
> = {
  project: { name: "relay.project", glyph: "project", sentence: "relay.projectSentence", need: "relay.projectNeed", sees: "relay.projectSees" },
  own: { name: "relay.own", glyph: "own", sentence: "relay.ownRuns", need: "relay.ownNeed", sees: "relay.ownSees" },
  instance: { name: "relay.instanceName", glyph: "server", sentence: "relay.instanceSub", need: "relay.instanceNeed", sees: "relay.ownSees" },
  off: { name: "relay.off", glyph: "off", sentence: "relay.offSentence", need: "relay.offNeed", sees: "relay.offSees" },
};

function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

function Fact({ glyph, text }: { glyph: ReactNode; text: ReactNode }) {
  return (
    <p className="flex items-start gap-2 text-sm text-carbon-textSub">
      <span className="text-carbon-textMuted">{glyph}</span>
      <span className="min-w-0">{text}</span>
    </p>
  );
}

const strong = (text: string) => <strong className="font-semibold text-carbon-text">{text} </strong>;

export function RelayCard({
  group,
  stage,
  onGroup,
  onRefresh,
  t,
  hueIndex,
}: {
  group: GroupState;
  stage: PairStage;
  onGroup: (g: GroupState) => void;
  /** Asks the server for the group again, for "Check again". */
  onRefresh: () => void;
  t: T;
  hueIndex?: number;
}) {
  const { push } = useToast();
  const [mode, setMode] = useState<RelayMode>(group.relay.mode);
  const [url, setUrl] = useState(group.relay.url);
  const [urlError, setUrlError] = useState<string | null>(null);
  // Which of the two own routes was picked here. The server stores one mode
  // for both, so without a pick the address decides.
  const [picked, setPicked] = useState<"own" | "instance" | null>(null);
  const editing = useRef(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Two quick picks send two saves, and their answers can come back in either
  // order. Only the newest one speaks for the card, and while a save is out a
  // reload of the group cannot take the pick back.
  const lastSave = useRef(0);
  const saving = useRef(0);

  // The page reloads the group every few seconds; that must not undo what is
  // being typed or picked right now.
  useEffect(() => {
    if (!editing.current) setUrl(group.relay.url);
  }, [group.relay.url]);
  useEffect(() => {
    if (saving.current === 0) setMode(group.relay.mode);
  }, [group.relay.mode]);
  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current);
  }, []);

  async function save(patch: { mode?: RelayMode; url?: string; serve?: boolean }): Promise<boolean> {
    const n = ++lastSave.current;
    saving.current++;
    try {
      const res = await setRelay(patch);
      if (n !== lastSave.current) return res.ok;
      if (res.ok) {
        onGroup(res);
        return true;
      }
      if (patch.url !== undefined) setUrlError(res.error ?? t("relay.saveError"));
      else push(res.error ?? t("relay.saveError"), "fail");
    } catch (err) {
      if (n === lastSave.current) push(err instanceof Error ? err.message : t("relay.saveError"), "fail");
    } finally {
      saving.current--;
    }
    return false;
  }

  const relay = group.relay;
  const form: RelayForm = mode !== "own" ? mode : (picked ?? relayForm({ ...relay, mode, url }, group.members));

  function pick(next: RelayForm) {
    const stored: RelayMode = next === "instance" ? "own" : next;
    setPicked(next === "own" || next === "instance" ? next : null);
    setMode(stored);
    if (stored !== mode) void save({ mode: stored });
  }

  function typeUrl(v: string) {
    editing.current = true;
    setUrl(v);
    setUrlError(null);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      void save({ url: v }).then(() => {
        editing.current = false;
      });
    }, 800);
  }

  const copy = COPY[form];
  // What the relay sees and what it does not, for the bubble beside its name.
  const sees =
    form === "off"
      ? undefined
      : [t("relay.seesLabel"), t(copy.sees), t("relay.notSeesLabel"), t("relay.notSeesContent"), t("relay.notSeesBackups")].join(" ");
  const connected = group.active && form !== "off" && relay.connected;
  // The relay key comes from the twelve words, so the relay is only dialled
  // inside a group; before that there is nothing to be connected.
  const state =
    form === "off" ? (
      <Badge tone="neutral" size="large">{t("relay.off")}</Badge>
    ) : !group.active ? (
      <Badge tone="neutral" size="large">{t("relay.noGroup")}</Badge>
    ) : relay.connected ? (
      <Badge tone="ok" size="large">{t("instances.connected")}</Badge>
    ) : (
      <Badge tone="warn" size="large">{t("instances.notConnected")}</Badge>
    );

  // A route that should be up and is not offers another look right where its
  // address is, or under the picture where there is no address to type.
  const checkAgain = stage !== "unpaired" && form !== "off" && !connected && (
    <div className="flex justify-end">
      <Button label={t("pairing.checkAgain")} labelKey="pairing.checkAgain" glyph={<IconRetry />} tone="neutral" onClick={onRefresh} />
    </div>
  );

  const address = (placeholder: string) => (
    <div data-relay-address className="flex flex-col gap-3">
      <SettingRow label={t("relay.addressLabel")} hint={t("relay.addressTip")} htmlFor="relay-address">
      <input
        id="relay-address"
        type="url"
        value={url}
        onChange={(e) => typeUrl(e.target.value)}
        spellCheck={false}
        autoComplete="off"
        placeholder={placeholder}
        dir="ltr"
        className="w-72 max-w-full rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 text-start glim-field-focus"
      />
      </SettingRow>
      {urlError && <p className="text-caption text-statusFail">{urlError}</p>}
      {!urlError && PLAINTEXT_RELAY.test(url.trim()) && (
        <p className="rounded-card bg-statusWarnBgSoft px-3 py-2.5 text-sm leading-relaxed text-carbon-text">
          {t("relay.plaintextWarning")}
        </p>
      )}
      {checkAgain}
    </div>
  );


  return (
    <Card
      title={t("relay.title")}
      hint={t("relay.hint")}
      hueIndex={hueIndex}
    >
      <p data-testid="relay-steps" className="text-sm text-carbon-text">
        {t("relay.lead")}
      </p>

      <div className="flex flex-wrap items-center justify-between gap-3">
        <Selector
          // Without glyphs a phone shows the routes as a list with their whole names.
          items={FORMS.map((f) => ({ id: f, label: t(COPY[f].name) }))}
          label={t("relay.title")}
          select="one"
          activation="manual"
          active={form}
          onChange={(id) => pick(id as RelayForm)}
          size="lg"
          equalWidth
          inline
          buttonHeight
        />
        {/* Where the state does not fit beside the picker, it goes under it
            at the end of the row. */}
        <span data-testid="relay-state" className="ms-auto flex">
          {state}
        </span>
      </div>

      <div className="grid grid-cols-1 items-start gap-x-10 gap-y-5 lg:grid-cols-2" data-form={form}>
        <div className="flex min-w-0 flex-col gap-3.5">
          <Fact
            glyph={<RouteGlyph kind="relay" />}
            text={form === "project" ? t(copy.sentence).replace("{host}", hostOf(relay.projectUrl)) : t(copy.sentence)}
          />
          <Fact
            glyph={<FactGlyph kind="need" />}
            text={
              <>
                {strong(t("relay.needLabel"))}
                {t(copy.need)}
              </>
            }
          />
          {form === "off" && (
            <p data-testid="no-relay-phone" className="rounded-card bg-statusWarnBgSoft px-3 py-2.5 text-sm leading-relaxed text-carbon-text">
              {t("relay.offNote")}
            </p>
          )}
          {form === "own" && (
            <div className="flex flex-col gap-3.5" data-testid="own-relay">
              <ol className="flex flex-col gap-2.5 text-sm text-carbon-textSub">
                {OWN_STEPS.map((key, i) => (
                  <li key={key} className="flex items-start gap-2.5">
                    <span className="glim-num mt-0.5 grid h-5 w-5 shrink-0 place-items-center rounded-pill bg-accent text-dense font-semibold text-accentContrast">
                      {i + 1}
                    </span>
                    <span className="min-w-0 wrap-anywhere">{i === 0 ? emphasize(t(key), "name", t("relay.containerName")) : t(key)}</span>
                  </li>
                ))}
              </ol>
            </div>
          )}
          {form === "instance" && (
            <div className="flex flex-col gap-3.5" data-testid="instance-relay">
              <div>
                <ToggleRow
                  label={t("relay.serve")}
                  hint={tLtr(t, "relay.serveHint")}
                  checked={relay.serve}
                  onChange={(v) => void save({ serve: v })}
                />
                {relay.serve && <p className="mt-1 text-xs text-carbon-textMuted">{t("relay.serveClients", relay.serveClients)}</p>}
              </div>
              <Fact
                glyph={<FactGlyph kind="shield" />}
                text={
                  <>
                    {tLtr(t, "relay.instanceCert")} <InfoBubble tip={t("relay.instanceCertTip")} />
                  </>
                }
              />
            </div>
          )}
          <Fact
            glyph={<FactGlyph kind="lock" />}
            text={
              <>
                {t("relay.e2e")} <InfoBubble tip={t("relay.e2eTip")} />
              </>
            }
          />
        </div>
        {/* The address goes under the picture of the route it fills in. */}
        <div className="flex min-w-0 flex-col gap-5">
          <RouteDiagram form={form} t={t} tip={sees} />
          {form === "own" && address("https://relay.example.org")}
          {form === "instance" && address("https://bombvault.example.org/relay/connect")}
          {form === "project" && checkAgain}
        </div>
      </div>

    </Card>
  );
}
