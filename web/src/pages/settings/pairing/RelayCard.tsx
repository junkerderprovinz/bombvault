// RelayCard picks how this instance reaches members on other networks: the
// project relay, an own relay, a paired BombVault that serves as one, or
// none. Each route says in one sentence what it is and what it needs, beside
// a picture of a message on its way. Every control saves on its own, as the
// rest of the settings do.
import { useEffect, useRef, useState, type ReactNode } from "react";
import { Card, ToggleRow } from "../shared";
import { Badge } from "../../../components/Badge";
import { Button } from "../../../components/Button";
import { InfoBubble } from "../../../components/InfoBubble";
import { Selector } from "../../../components/Selector";
import { IconEye, IconRefresh } from "../../../components/glyphs";
import { IconClose } from "../../../components/navGlyphs";
import { setDirectAddress, setRelay, type GroupState, type RelayMode } from "../../../lib/api";
import type { TranslationKey, useT } from "../../../lib/i18n";
import { useToast } from "../../../lib/toast";
import { tLtr } from "../../../lib/ltrFragments";
import { PairingWindow } from "./PairingWindow";
import { FactGlyph, RouteDiagram, RouteGlyph } from "./pairingArt";
import { emphasize } from "./PairingSteps";
import type { PairStage } from "./pairStage";
import { relayForm, type RelayForm } from "./relayForm";

type T = ReturnType<typeof useT>["t"];

const FORMS: RelayForm[] = ["project", "own", "instance", "off"];

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

/** SelfAddressField is the addresses this instance is known under: the ones
 *  it found by itself, which nobody can edit, and below them the one members
 *  are told, prefilled from the browser, corrected by hand when it is wrong
 *  and saved on its own. */
export function SelfAddressField({ group, onGroup, t }: { group: GroupState; onGroup: (g: GroupState) => void; t: T }) {
  const [value, setValue] = useState(group.selfAddress);
  const [error, setError] = useState<string | null>(null);
  const editing = useRef(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const found = group.ownAddresses ?? [];

  useEffect(() => {
    if (!editing.current) setValue(group.selfAddress);
  }, [group.selfAddress]);
  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current);
  }, []);

  async function save(v: string) {
    try {
      const res = await setDirectAddress(v);
      if (res.ok) {
        onGroup(res);
        setError(null);
      } else {
        setError(res.error ?? t("relay.selfAddressError"));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("relay.selfAddressError"));
    }
  }

  function type(v: string) {
    editing.current = true;
    setValue(v);
    setError(null);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      void save(v).then(() => {
        editing.current = false;
      });
    }, 800);
  }

  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor="self-address" className="inline-flex items-center gap-1.5 text-xs text-carbon-textSub">
        {t("pairing.knownAddresses")}
        <InfoBubble tip={t("relay.selfAddressTip")} />
      </label>
      {found.length > 0 && (
        <div className="flex flex-col gap-1">
          <span className="text-caption text-carbon-textMuted">{t("pairing.foundAddresses")}</span>
          <ul data-testid="own-addresses" aria-label={t("pairing.foundAddresses")} className="flex flex-col gap-0.5">
            {found.map((address) => (
              <li key={address} className="text-sm text-carbon-text wrap-anywhere">
                <bdi dir="ltr" className="font-mono">
                  {address}
                </bdi>
              </li>
            ))}
          </ul>
        </div>
      )}
      <input
        id="self-address"
        data-testid="self-address-input"
        type="url"
        value={value}
        onChange={(e) => type(e.target.value)}
        spellCheck={false}
        autoComplete="off"
        placeholder="https://192.168.1.20:3443"
        dir="ltr"
        className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 text-start glim-field-focus"
      />
      {error && <p className="text-caption text-statusFail">{error}</p>}
    </div>
  );
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
  const [seesOpen, setSeesOpen] = useState(false);
  const editing = useRef(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // The page reloads the group every few seconds; that must not undo what is
  // being typed or picked right now.
  useEffect(() => {
    if (!editing.current) setUrl(group.relay.url);
  }, [group.relay.url]);
  useEffect(() => {
    setMode(group.relay.mode);
  }, [group.relay.mode]);
  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current);
  }, []);

  async function save(patch: { mode?: RelayMode; url?: string; serve?: boolean }): Promise<boolean> {
    try {
      const res = await setRelay(patch);
      if (res.ok) {
        onGroup(res);
        return true;
      }
      if (patch.url !== undefined) setUrlError(res.error ?? t("relay.saveError"));
      else push(res.error ?? t("relay.saveError"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("relay.saveError"), "fail");
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

  const address = (placeholder: string) => (
    <div className="flex flex-col gap-1.5">
      <label htmlFor="relay-address" className="inline-flex items-center gap-1.5 text-sm font-medium text-carbon-text">
        {t("relay.addressLabel")}
        <InfoBubble tip={t("relay.addressTip")} />
      </label>
      <input
        id="relay-address"
        type="url"
        value={url}
        onChange={(e) => typeUrl(e.target.value)}
        spellCheck={false}
        autoComplete="off"
        placeholder={placeholder}
        dir="ltr"
        className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 text-start glim-field-focus"
      />
      {urlError && <p className="text-caption text-statusFail">{urlError}</p>}
      {!urlError && PLAINTEXT_RELAY.test(url.trim()) && (
        <p className="rounded-card bg-statusWarnBgSoft px-3 py-2.5 text-sm leading-relaxed text-carbon-text">
          {t("relay.plaintextWarning")}
        </p>
      )}
    </div>
  );

  const [beforeHost, afterHost] = t("pairing.relayCheckProject").split("{host}");

  return (
    <Card title={t("relay.title")} hint={t("relay.hint")} hueIndex={hueIndex}>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-3">
        <div className="min-w-0 flex-[1_1_24rem]">
          <Selector
            items={FORMS.map((f) => ({ id: f, label: t(COPY[f].name), icon: <RouteGlyph kind={COPY[f].glyph} /> }))}
            label={t("relay.title")}
            select="one"
            active={form}
            onChange={(id) => pick(id as RelayForm)}
            size="lg"
            equalWidth
          />
        </div>
        <span className="ms-auto flex flex-wrap items-center gap-2.5">
          <Button
            label={t("relay.sees.title")}
            labelKey="relay.sees.title"
            glyph={<IconEye />}
            tone="neutral"
            onClick={() => setSeesOpen(true)}
          />
          <span data-testid="relay-state">{state}</span>
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
          {form === "own" && (
            <div className="flex flex-col gap-3.5" data-testid="own-relay">
              {address("wss://relay.example.org")}
              <Fact
                glyph={<FactGlyph kind="search" />}
                text={
                  <>
                    {emphasize(t("relay.containerFind"), "name", t("relay.containerName"))} {t("relay.containerAlso")}
                  </>
                }
              />
              <Fact glyph={<FactGlyph kind="shield" />} text={t("relay.containerCert")} />
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
              {address("wss://bombvault.example.org/relay/connect")}
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
          {stage !== "unpaired" && form !== "off" && !connected && (
            <section className="flex flex-col gap-2 rounded-control bg-carbon-surface2 p-3.5 text-sm text-carbon-textSub" data-testid="relay-check">
              <h3 className="text-xs font-semibold uppercase tracking-widest text-carbon-textMuted">{t("pairing.relayCheckOpen")}</h3>
              <p>
                {t("pairing.relayCheckLead")} <InfoBubble tip={t("pairing.relayCheckTip")} />
              </p>
              <ul className="flex list-disc flex-col gap-1 ps-5">
                <li>
                  {form === "project" ? (
                    <>
                      {beforeHost}
                      <span dir="ltr" className="font-mono text-subline text-carbon-text">
                        {hostOf(relay.projectUrl)}
                      </span>
                      {afterHost}
                    </>
                  ) : (
                    t("pairing.relayCheckOwn")
                  )}
                </li>
                <li>{t("pairing.relayCheckFilter")}</li>
              </ul>
              <div className="flex justify-end">
                <Button label={t("pairing.checkAgain")} labelKey="pairing.checkAgain" glyph={<IconRefresh />} tone="neutral" onClick={onRefresh} />
              </div>
            </section>
          )}
        </div>
        <RouteDiagram form={form} t={t} />
      </div>

      {seesOpen && (
        <PairingWindow
          title={t("relay.sees.title")}
          hueIndex={hueIndex}
          onClose={() => setSeesOpen(false)}
          footer={
            <Button label={t("common.close")} labelKey="common.close" glyph={<IconClose />} tone="accent" onClick={() => setSeesOpen(false)} />
          }
        >
          <Fact
            glyph={<FactGlyph kind="sees" />}
            text={
              <>
                {strong(t("relay.seesLabel"))}
                {t(copy.sees)}
              </>
            }
          />
          {form !== "off" && (
            <>
              <Fact glyph={<FactGlyph kind="hidden" />} text={strong(t("relay.notSeesLabel"))} />
              <ul className="flex list-disc flex-col gap-1 ps-9 text-sm text-carbon-textSub">
                <li>{t("relay.notSeesContent")}</li>
                <li>{t("relay.notSeesBackups")}</li>
              </ul>
            </>
          )}
        </PairingWindow>
      )}
    </Card>
  );
}
