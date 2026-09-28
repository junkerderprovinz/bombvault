// RelayCard picks how this instance reaches members on other networks: the
// project relay, an own relay, or none. It leads with what a relay is for and
// whether it is connected; each route gets one sentence and what it needs.
// "How does it work?" opens three short sections: the route picture, what the
// relay sees next to what it never does, and the encryption underneath both.
// Every control saves on its own, as the rest of the settings do.
import { useEffect, useId, useRef, useState } from "react";
import { Card, ToggleRow } from "../settings/shared";
import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { IconDisclosure } from "../../components/IconDisclosure";
import { InfoBubble } from "../../components/InfoBubble";
import { Selector } from "../../components/Selector";
import { setRelay, type GroupState, type RelayMode } from "../../lib/api";
import type { TranslationKey, useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { tLtr } from "../../lib/ltrFragments";
import { FactGlyph, LegendKey, LegendMessage, RouteDiagram, RouteGlyph } from "./pairingArt";
import { emphasize } from "./PairingSteps";

type T = ReturnType<typeof useT>["t"];

const MODES: RelayMode[] = ["project", "own", "off"];

// An own relay without TLS still works on a LAN, but its relay key then
// travels in the clear, so the card says so.
const PLAINTEXT_RELAY = /^(ws|http):\/\//i;

const COPY: Record<RelayMode, { name: TranslationKey; sentence: TranslationKey; need?: TranslationKey; sees: TranslationKey; alt: TranslationKey }> = {
  project: {
    name: "relay.project",
    sentence: "relay.projectSentence",
    need: "relay.projectNeed",
    sees: "relay.projectSees",
    alt: "relay.projectAlt",
  },
  // No `need`: the two source cards right below already say what an own
  // relay takes, in more detail than a one-line hint could.
  own: {
    name: "relay.own",
    sentence: "relay.ownSentence",
    sees: "relay.ownSees",
    alt: "relay.ownAlt",
  },
  off: {
    name: "relay.off",
    sentence: "relay.offSentence",
    need: "relay.offNeed",
    sees: "relay.offSees",
    alt: "relay.offAlt",
  },
};

function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

function Fact({ glyph, label, text }: { glyph: "need" | "sees" | "search" | "shield"; label?: string; text: React.ReactNode }) {
  return (
    <p className="flex items-start gap-2 text-sm text-carbon-textSub">
      <span className="text-carbon-textMuted">
        <FactGlyph kind={glyph} />
      </span>
      <span>
        {label && <strong className="font-semibold text-carbon-text">{label} </strong>}
        {text}
      </span>
    </p>
  );
}

/** One of the two places an own relay can come from. */
function Source({
  icon,
  onAccent,
  name,
  sub,
  children,
  address,
  t,
}: {
  icon: React.ReactNode;
  onAccent?: boolean;
  name: string;
  sub: string;
  children: React.ReactNode;
  address: string;
  t: T;
}) {
  return (
    <div className="flex flex-col gap-2.5 rounded-control bg-carbon-surface2 px-4 pb-4 pt-3.5">
      <div className="flex items-center gap-3">
        <span
          className={`grid h-11 w-11 shrink-0 place-items-center rounded-control ${
            onAccent ? "bg-accent text-accentContrast" : "bg-carbon-surface3 text-carbon-text"
          }`}
        >
          {icon}
        </span>
        <span className="min-w-0">
          <span className="block text-sm font-semibold text-carbon-text">{name}</span>
          <span className="block text-xs text-carbon-textSub">{sub}</span>
        </span>
      </div>
      {children}
      <p className="mt-auto pt-0.5 text-xs text-carbon-textMuted">
        {t("relay.addressAfter")}
        <span dir="ltr" className="block font-mono text-carbon-textSub break-all">
          {address}
        </span>
      </p>
    </div>
  );
}

export function RelayCard({
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
  const [mode, setMode] = useState<RelayMode>(group.relay.mode);
  const [url, setUrl] = useState(group.relay.url);
  const [urlError, setUrlError] = useState<string | null>(null);
  const [howOpen, setHowOpen] = useState(false);
  const howId = useId();
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

  function pick(next: RelayMode) {
    setMode(next);
    void save({ mode: next });
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

  const copy = COPY[mode];
  const relay = group.relay;
  // The relay is only dialled inside a group, so outside one a missing
  // connection is the expected state rather than a fault.
  const state =
    mode === "off" ? (
      <Badge tone="neutral" size="large">{t("relay.off")}</Badge>
    ) : relay.connected ? (
      <Badge tone="ok" size="large">{t("instances.connected")}</Badge>
    ) : (
      <Badge tone={group.active ? "fail" : "neutral"} size="large">{t("instances.notConnected")}</Badge>
    );

  return (
    <Card title={t("relay.title")} hint={t("relay.hint")} hueIndex={hueIndex}>
      <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2">
        <p className="min-w-0 flex-[1_1_16rem] text-[15px] text-carbon-text">{t("relay.lead")}</p>
        <span data-testid="relay-state">{state}</span>
      </div>

      <Selector
        items={MODES.map((m) => ({ id: m, label: t(COPY[m].name), icon: <RouteGlyph kind={m} /> }))}
        label={t("relay.title")}
        select="one"
        active={mode}
        onChange={(id) => pick(id as RelayMode)}
        size="lg"
        equalWidth
      />

      <div className="flex flex-col gap-2">
        <p className="text-sm text-carbon-textSub">
          {mode === "project" ? t(copy.sentence).replace("{host}", hostOf(relay.projectUrl)) : t(copy.sentence)}
        </p>
        {copy.need && <Fact glyph="need" label={t("relay.needLabel")} text={t(copy.need)} />}
      </div>

      {mode === "own" && (
        <div className="flex flex-col gap-3.5" data-testid="own-relay">
          <span className="text-xs font-semibold uppercase tracking-widest text-carbon-textMuted">{t("relay.sourcesTitle")}</span>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            <Source
              icon={<RouteGlyph kind="relay" />}
              onAccent
              name={t("relay.containerName")}
              sub={t("relay.containerSub")}
              address="wss://relay.example.org"
              t={t}
            >
              <Fact glyph="search" text={emphasize(t("relay.containerFind"), "name", t("relay.containerName"))} />
              <Fact glyph="shield" text={t("relay.containerCert")} />
            </Source>
            <Source
              icon={<RouteGlyph kind="server" />}
              name={t("relay.instanceName")}
              sub={t("relay.instanceSub")}
              address="wss://bombvault.example.org/relay/connect"
              t={t}
            >
              <div className="rounded-control bg-carbon-surface px-3 py-2">
                <ToggleRow
                  label={t("relay.serve")}
                  hint={tLtr(t, "relay.serveHint")}
                  checked={relay.serve}
                  onChange={(v) => void save({ serve: v })}
                />
                {relay.serve && (
                  <p className="mt-1 text-xs text-carbon-textMuted">
                    {t("relay.serveClients", relay.serveClients)}
                  </p>
                )}
              </div>
              <Fact
                glyph="shield"
                text={
                  <>
                    {tLtr(t, "relay.instanceCert")} <InfoBubble tip={t("relay.instanceCertTip")} />
                  </>
                }
              />
            </Source>
          </div>
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
              placeholder="wss://relay.example.org"
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
        </div>
      )}

      <div>
        <Button
          label={t("relay.howItWorks")}
          labelKey="relay.howItWorks"
          tone="neutral"
          onClick={() => setHowOpen((v) => !v)}
          ariaExpanded={howOpen}
          ariaControls={howId}
          glyph={<IconDisclosure open={howOpen} />}
        />
      </div>

      {howOpen && (
        <div id={howId} className="flex flex-col gap-5 border-t border-carbon-border pt-4">
          <section className="flex flex-col gap-3">
            <h4 className="text-xs font-semibold uppercase tracking-widest text-carbon-textMuted">{t("relay.howDoesTitle")}</h4>
            <div className="grid grid-cols-1 items-center gap-5 md:grid-cols-2 md:gap-10">
              <RouteDiagram
                mode={mode}
                alt={t(copy.alt)}
                labels={{
                  relay: t("relay.diagramRelay"),
                  projectHost: hostOf(relay.projectUrl),
                  ownAddress: t("relay.diagramOwnAddress"),
                  yourNetwork: t("relay.diagramYourNetwork"),
                  otherNetwork: t("relay.diagramOtherNetwork"),
                  yourLan: t("relay.diagramYourLan"),
                }}
              />
              <div className="flex flex-wrap gap-x-5 gap-y-1.5 text-xs text-carbon-textSub">
                <span className="inline-flex items-center gap-2">
                  <LegendKey />
                  {t("relay.legendKey")}
                </span>
                <span className="inline-flex items-center gap-2">
                  <LegendMessage />
                  {t("relay.legendMessage")}
                </span>
              </div>
            </div>
          </section>

          {/* Sees / does not see, side by side, so the one honest limit (the
              routing metadata) sits right next to everything it does not
              cost. */}
          <section className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div className="flex flex-col gap-2 rounded-control bg-carbon-surface2 p-3.5">
              <span className="inline-flex items-center gap-2 text-sm font-semibold text-carbon-text">
                <FactGlyph kind="sees" />
                {t("relay.seesLabel")}
              </span>
              <p className="text-sm text-carbon-textSub">{t(copy.sees)}</p>
            </div>
            <div className="flex flex-col gap-2 rounded-control bg-carbon-surface2 p-3.5">
              <span className="inline-flex items-center gap-2 text-sm font-semibold text-carbon-text">
                <FactGlyph kind="hidden" />
                {t("relay.notSeesLabel")}
              </span>
              <ul className="flex list-disc flex-col gap-1 ps-4 text-sm text-carbon-textSub">
                <li>{t("relay.notSeesContent")}</li>
                <li>{t("relay.notSeesBackups")}</li>
              </ul>
            </div>
          </section>

          <section className="flex flex-col gap-2">
            <h4 className="text-xs font-semibold uppercase tracking-widest text-carbon-textMuted">{t("relay.encryptionTitle")}</h4>
            <p className="flex items-start gap-2.5 text-sm text-carbon-textSub">
              <span className="text-accentText">
                <FactGlyph kind="lock" />
              </span>
              <span>
                {t("relay.e2e")} <InfoBubble tip={t("relay.e2eTip")} />
              </span>
            </p>
          </section>
        </div>
      )}
    </Card>
  );
}
