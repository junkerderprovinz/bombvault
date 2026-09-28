// RelayCard picks how this instance reaches members on other networks: the
// project relay, an own relay, or none. Each route gets a picture, one
// sentence, what it needs and what the relay sees. Every control saves on its
// own, as the rest of the settings do.
import { useEffect, useRef, useState } from "react";
import { Card, ToggleRow } from "../settings/shared";
import { Badge } from "../../components/Badge";
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

const COPY: Record<RelayMode, { name: TranslationKey; sentence: TranslationKey; need: TranslationKey; sees: TranslationKey; alt: TranslationKey }> = {
  project: {
    name: "relay.project",
    sentence: "relay.projectSentence",
    need: "relay.projectNeed",
    sees: "relay.projectSees",
    alt: "relay.projectAlt",
  },
  own: {
    name: "relay.own",
    sentence: "relay.ownSentence",
    need: "relay.ownNeed",
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
  const connectState =
    !group.active || mode === "off" ? null : relay.connected ? (
      <Badge tone="ok">{t("relay.connected")}</Badge>
    ) : (
      <Badge tone="warn">{t("relay.notConnected")}</Badge>
    );

  return (
    <Card title={t("relay.title")} hint={t("relay.hint")} hueIndex={hueIndex}>
      <Selector
        items={MODES.map((m) => ({ id: m, label: t(COPY[m].name), icon: <RouteGlyph kind={m} /> }))}
        label={t("relay.title")}
        select="one"
        active={mode}
        onChange={(id) => pick(id as RelayMode)}
        size="lg"
        equalWidth
      />

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
        <div className="flex flex-col gap-3.5">
          <p className="text-[15px] text-carbon-text">
            {mode === "project" ? t(copy.sentence).replace("{host}", hostOf(relay.projectUrl)) : t(copy.sentence)}
          </p>
          <div className="flex flex-col gap-2">
            <Fact glyph="need" label={t("relay.needLabel")} text={t(copy.need)} />
            <Fact glyph="sees" label={t("relay.seesLabel")} text={t(copy.sees)} />
          </div>
          {connectState && <div>{connectState}</div>}
        </div>
      </div>

      {mode === "own" && (
        <div className="flex flex-col gap-3.5" data-testid="own-relay">
          <span className="text-xs font-semibold uppercase tracking-widest text-carbon-textMuted">{t("relay.sourcesTitle")}</span>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            <Source
              icon={<RouteGlyph kind="own" />}
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

      <div className="flex flex-col gap-3 border-t border-carbon-border pt-4">
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
        <p className="flex items-start gap-2.5 text-sm text-carbon-textSub">
          <span className="text-accentText">
            <FactGlyph kind="lock" />
          </span>
          <span>
            {t("relay.e2e")} <InfoBubble tip={t("relay.e2eTip")} />
          </span>
        </p>
        <p className="flex items-start gap-2.5 text-sm text-carbon-textSub">
          <span className="text-accentText">
            <FactGlyph kind="box" />
          </span>
          <span>{t("relay.backupsNever")}</span>
        </p>
      </div>
    </Card>
  );
}
