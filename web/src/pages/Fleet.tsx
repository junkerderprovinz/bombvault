// Fleet shows every instance of the pairing group, this one included, as a
// card with its connection state and cached protection scorecard, the same
// red/amber/green aggregate as the local dashboard. Everything goes over the
// group: the scorecard, a request to check one domain now, and Mesh off-site,
// where this instance offers its off-site storage to a member (connection
// details, never backup data) and reviews offers members sent here. Accepting
// one creates an ordinary credential set and off-site target, since BombVault
// never hosts storage itself.
//
// While the page is open it asks the server every few seconds who is
// connected, which costs nothing on the network. A member's scorecard is
// fetched over the group only once it is older than SCORECARD_STALE_S, since
// that call may cross a relay; the daily sweep keeps it fresh otherwise.

import { Fragment, useCallback, useEffect, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { useNavigate } from "react-router-dom";
import {
  getGroup,
  getHealth,
  getStatus,
  listFleetPeers,
  updateFleetPeer,
  deleteFleetPeer,
  pollFleetPeer,
  checkFleetPeer,
  listMeshOffers,
  acceptMeshOffer,
  declineMeshOffer,
  proposeMeshOffer,
} from "../lib/api";
import { IconDisclosure } from "../components/IconDisclosure";
import { PageTitle } from "../components/PageTitle";
import type { FleetPeer, DomainStatus, MeshOffer, DeploySnippetData, OffsiteDomain } from "../lib/api";
import { credSetsChanged } from "../lib/useCloudCredSets";
import { offsiteTargetsChanged } from "../lib/useOffsiteTargets";
import { placementErrorText } from "../lib/placementCodes";
import { placementChanged } from "../lib/placementEvents";
import { useNewTargetQuestion } from "../components/placement/NewTargetQuestion";
import { useT, type TranslationKey } from "../lib/i18n";
import { PAGE_SHELL_RESPONSIVE, PAGE_SHELL_TABBED_RESPONSIVE } from "../lib/pageShell";
import { SelectField } from "../components/SelectField";
import { relativeTime } from "../lib/reltime";
import { IconLink } from "../components/glyphs";
import { Badge } from "../components/Badge";
import { InfoBubble } from "../components/InfoBubble";
import { useToast } from "../lib/toast";
import { hueVars } from "../lib/appearance";
import { Button } from "../components/Button";
import { CopyBlock } from "../components/CopyBlock";

import { ToggleRow } from "./settings/shared";
type T = ReturnType<typeof useT>["t"];

const MESH_DOMAINS = ["containers", "vms", "flash", "config", "files", "zfs"] as const;

// Same mapping as Dashboard's protectionChip, which is not exported.
function protectionTone(level: string): "ok" | "fail" | "warn" | "neutral" {
  switch (level) {
    case "green":
      return "ok";
    case "amber":
      return "warn";
    case "red":
      return "fail";
    default:
      return "neutral";
  }
}

// Whether peer cards show their scorecard, remembered per browser.
const FLEET_DETAILS_OPEN_KEY = "bombvault.fleetDetailsOpen";

// An explicit map rather than a template literal, so every lookup is a
// checked TranslationKey.
const DOMAIN_LABEL_KEYS: Record<string, TranslationKey> = {
  containers: "settings.containersEnabled",
  vms: "settings.vmsEnabled",
  flash: "settings.flashEnabled",
  files: "settings.filesEnabled",
  zfs: "settings.zfsEnabled",
  config: "settings.configEnabled",
};

function domainLabelKey(domain: string): TranslationKey {
  return DOMAIN_LABEL_KEYS[domain] ?? "settings.containersEnabled";
}

function protectionLabelKey(level: string): TranslationKey {
  switch (level) {
    case "green":
      return "fleet.protection.green";
    case "amber":
      return "fleet.protection.amber";
    case "red":
      return "fleet.protection.red";
    default:
      return "fleet.protection.none";
  }
}

function PeerScorecard({
  domains,
  t,
  onCheck,
}: {
  domains: DomainStatus[];
  t: T;
  /** Asks the member to check one domain now; absent for a peer that has to
   *  be paired again. */
  onCheck?: (domain: string) => void;
}) {
  const shown = domains.filter((d) => d.enabled && d.protection !== "");
  if (shown.length === 0) {
    return <p className="text-xs text-carbon-textMuted">{t("fleet.noScorecard")}</p>;
  }
  // One grid for all rows, so the badges and the times line up in columns.
  // On a phone a row has no room left for its button, which takes a line of
  // its own under the row.
  return (
    <div
      className={`grid items-center gap-x-3 gap-y-2 ${
        onCheck ? "grid-cols-[auto_auto_minmax(0,1fr)] md:grid-cols-[auto_auto_minmax(0,1fr)_auto]" : "grid-cols-[auto_auto_minmax(0,1fr)]"
      }`}
    >
      {shown.map((d) => (
        <Fragment key={d.domain}>
          <span className="text-xs text-carbon-textSub">{t(domainLabelKey(d.domain))}</span>
          <span>
            <Badge tone={protectionTone(d.protection)}>{t(protectionLabelKey(d.protection))}</Badge>
          </span>
          <span className="text-xs text-carbon-textMuted">
            {d.lastSuccess > 0 && t("fleet.lastBackup").replace("{time}", relativeTime(t, d.lastSuccess))}
          </span>
          {onCheck &&
            (d.domain === "config" ? (
              <span className="max-md:hidden" />
            ) : (
              <span className="max-md:col-span-3 max-md:justify-self-end">
                <Button label={t("fleet.checkNow")} labelKey="fleet.checkNow" tone="neutral" onClick={() => onCheck(d.domain)} />
              </span>
            ))}
        </Fragment>
      ))}
    </div>
  );
}

function meshStatusTone(status: string): "ok" | "fail" | "warn" | "neutral" {
  switch (status) {
    case "accepted":
      return "ok";
    case "declined":
      return "fail";
    default:
      return "warn"; // pending
  }
}

function meshStatusLabelKey(status: string): TranslationKey {
  switch (status) {
    case "accepted":
      return "fleet.mesh.status.accepted";
    case "declined":
      return "fleet.mesh.status.declined";
    default:
      return "fleet.mesh.status.pending";
  }
}

function MeshOfferRow({ offer, t, onChanged }: { offer: MeshOffer; t: T; onChanged: () => void }) {
  const [domain, setDomain] = useState<string>(offer.suggestedDomain || "containers");
  const [busy, setBusy] = useState(false);
  const { push } = useToast();
  const { lang } = useT();
  const { ask, dialog } = useNewTargetQuestion();
  const [shakeAccept, setShakeAccept] = useState(0);
  const [shakeDecline, setShakeDecline] = useState(0);

  async function handleAccept() {
    // The question does a round trip of its own before its dialog appears, and
    // a disabled button is all that keeps a second accept from minting a
    // second target.
    setBusy(true);
    const answer = await ask({
      // The select offers only MESH_DOMAINS, all of them off-site domains.
      domain: domain as OffsiteDomain,
      location: offer.repo,
      name: offer.from || t("fleet.mesh.unknownPeer"),
      moved: false,
    });
    if (!answer.go) {
      setBusy(false);
      return;
    }
    try {
      const res = await acceptMeshOffer(offer.id, domain, answer.alsoExclude ?? undefined);
      if (res.ok) {
        // Accepting creates a credential set with the peer's REST login and
        // an off-site target, so every mounted reader of either list reloads.
        credSetsChanged();
        offsiteTargetsChanged();
        if (answer.alsoExclude) placementChanged();
        onChanged();
      } else {
        push(placementErrorText(t, lang, res, "fleet.mesh.saveError"), "fail");
        setShakeAccept((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.mesh.saveError"), "fail");
      setShakeAccept((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  async function handleDecline() {
    setBusy(true);
    try {
      const res = await declineMeshOffer(offer.id);
      if (res.ok) onChanged();
      else {
        push(res.error ?? t("fleet.mesh.saveError"), "fail");
        setShakeDecline((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.mesh.saveError"), "fail");
      setShakeDecline((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  const pending = offer.status === "pending";

  return (
    <div className="rounded-card bg-carbon-surface2 p-3 flex flex-col gap-2">
      {dialog}
      <div className="flex items-center gap-2 flex-wrap">
        <span className="font-semibold text-carbon-text text-sm truncate max-md:whitespace-normal max-md:wrap-break-word">{offer.from || t("fleet.mesh.unknownPeer")}</span>
        <Badge tone={meshStatusTone(offer.status)}>{t(meshStatusLabelKey(offer.status))}</Badge>
        <span className="text-xs text-carbon-textMuted ms-auto">{relativeTime(t, offer.receivedAt)}</span>
      </div>
      <p dir="ltr" className="text-xs font-mono text-carbon-textMuted truncate text-start max-md:whitespace-normal max-md:break-all">{offer.repo}</p>
      {pending && (
        <div className="flex items-center gap-2 flex-wrap">
          <label className="flex items-center gap-1.5 text-xs text-carbon-textSub">
            {t("fleet.mesh.applyTo")}
            <SelectField
              value={domain}
              onChange={setDomain}
              label={t("fleet.mesh.applyTo")}
              options={MESH_DOMAINS.map((d) => ({ value: d, label: t(domainLabelKey(d)) }))}
              className="rounded-control bg-carbon-surface3 text-carbon-text text-xs px-2 py-1 glim-field-focus-well"
            />
          </label>
          <Button
            key={shakeDecline}
            label={t("fleet.mesh.decline")}
            labelKey="fleet.mesh.decline"
            tone="neutral"
            onClick={() => void handleDecline()}
            disabled={busy}
            className={`inline-flex items-center rounded-pill px-3 py-1.5 text-xs text-carbon-text disabled:opacity-50${
              shakeDecline ? " glim-shake" : ""
            }`}
          />
          <Button
            key={shakeAccept}
            label={t("fleet.mesh.accept")}
            labelKey="fleet.mesh.accept"
            tone="accent"
            onClick={() => void handleAccept()}
            disabled={busy}
            className={`inline-flex items-center rounded-pill bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
              shakeAccept ? " glim-shake" : ""
            }`}
          />
        </div>
      )}
    </div>
  );
}

function ProposeMeshDialog({ peer, t, onClose }: { peer: FleetPeer; t: T; onClose: () => void }) {
  const [domain, setDomain] = useState<string>("containers");
  const [baseUrl, setBaseUrl] = useState("");
  const [sending, setSending] = useState(false);
  const { push } = useToast();
  const [snippet, setSnippet] = useState<(DeploySnippetData & { repo: string }) | null>(null);
  const [shake, setShake] = useState(0);

  // Send stays enabled while the base URL is blank, so the check below can
  // fire. The snippet shown after sending stays on screen to be copied from.
  async function handleSend() {
    if (baseUrl.trim() === "") {
      push(t("fleet.mesh.baseUrlRequired"), "fail");
      setShake((n) => n + 1);
      return;
    }
    setSending(true);
    try {
      const res = await proposeMeshOffer(peer.id, domain, baseUrl.trim());
      if (res.ok && res.snippet) setSnippet(res.snippet);
      else {
        push(res.error ?? t("fleet.mesh.saveError"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.mesh.saveError"), "fail");
      setShake((n) => n + 1);
    } finally {
      setSending(false);
    }
  }

  const inputCls =
    "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus";

  // Centred rather than top-anchored, which would push the heading notch
  // against the viewport edge. The box is capped at 90vh, so it never clips,
  // and the backdrop scrolls when the content grows.
  return createPortal(
    <div className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4" onClick={onClose}>
      {/* The heading notch sits on a non-scrolling shell around the
          scrollable box, as in Receiver.tsx's ReceiverDialog. */}
      <div className="relative w-full max-w-lg">
      {/* px-5 matches the box's p-5 so the notch lands where a Card's does;
          FolderBrowser.tsx explains why the notch has no offset of its own. */}
      <h2 className="flex items-center px-5">
        <Badge tone="heading" size="heading" wrap>{t("fleet.mesh.proposeTitle")}</Badge>
      </h2>
      <div
        role="dialog"
        aria-modal="true"
        aria-label={t("fleet.mesh.proposeTitle")}
        onClick={(e) => e.stopPropagation()}
        className="w-full max-h-[90vh] overflow-y-auto rounded-card bg-carbon-surface p-5 flex flex-col gap-4 shadow-2xl"
      >
        <p className="text-xs text-carbon-textMuted">{t("fleet.mesh.proposeHint").replace("{peer}", peer.name)}</p>

        {!snippet ? (
          <>
            <div className="flex flex-col gap-1.5">
              <label className="text-xs text-carbon-textSub">{t("fleet.mesh.domain")}</label>
              <SelectField
                value={domain}
                onChange={setDomain}
                label={t("fleet.mesh.domain")}
                options={MESH_DOMAINS.map((d) => ({ value: d, label: t(domainLabelKey(d)) }))}
                className={inputCls}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <label className="text-xs text-carbon-textSub">{t("fleet.mesh.baseUrl")}</label>
              <input
                type="text"
                value={baseUrl}
                onChange={(e) => setBaseUrl(e.target.value)}
                spellCheck={false}
                autoComplete="off"
                placeholder="http://192.168.1.50:8000"
                dir="ltr"
                className={`${inputCls} font-mono text-start`}
              />
              <p className="text-caption text-carbon-textMuted">{t("fleet.mesh.baseUrlHint")}</p>
            </div>
            <div className="flex flex-wrap items-center justify-end gap-2 pt-1">
              <Button
                label={t("files.cancel")}
                labelKey="files.cancel"
                tone="neutral"
                onClick={onClose}
                disabled={sending}
              />
              <Button
                key={shake}
                label={t("fleet.mesh.send")}
                labelKey="fleet.mesh.send"
                tone="accent"
                onClick={() => void handleSend()}
                disabled={sending}
                busy={sending}
                title={sending ? t("fleet.mesh.sending") : undefined}
                className={shake ? "glim-shake" : ""}
              />
            </div>
          </>
        ) : (
          <>
            <p className="text-xs text-statusOk">{t("fleet.mesh.sent").replace("{peer}", peer.name)}</p>
            <p className="text-xs text-carbon-textMuted">{t("fleet.mesh.deployNow")}</p>
            <div className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("fleet.mesh.dockerRun")}</span>
              <CopyBlock text={snippet.dockerRun} />
            </div>
            <div className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("fleet.mesh.compose")}</span>
              <CopyBlock text={snippet.compose} />
            </div>
            <div className="flex items-center justify-end pt-1">
              <Button
                label={t("common.close")}
                labelKey="common.close"
                tone="accent"
                onClick={onClose}
              />
            </div>
          </>
        )}
      </div>
      </div>
    </div>,
    document.body,
  );
}

/** InstanceMark is the BombVault logo on the start edge of a card, in the
 *  variant the colour mode needs, as in the sidebar. */
function InstanceMark() {
  const cls = "h-16 md:h-20 w-auto shrink-0";
  return (
    <>
      <img src="/logo.svg" alt="" aria-hidden="true" draggable={false} className={`${cls} block dark:hidden`} />
      <img src="/logo-light.svg" alt="" aria-hidden="true" draggable={false} className={`${cls} hidden dark:block`} />
    </>
  );
}

/** FleetCard is one instance of the group: the logo beside the name with its
 *  badges and a line of facts, the scorecard across the card below, and the
 *  actions along the foot. */
function FleetCard({
  name,
  address,
  self,
  badges,
  facts,
  children,
  actions,
  index,
  t,
}: {
  name: string;
  /** Where the instance is reached, empty when it is not known. */
  address?: string;
  self?: boolean;
  badges: ReactNode;
  facts?: ReactNode;
  children?: ReactNode;
  actions?: ReactNode;
  /** Rainbow position by list index, as for ContainerRow and VMRow. */
  index: number;
  t: T;
}) {
  return (
    <div
      style={{ ...hueVars(index), "--row-i": String(index) } as CSSProperties}
      className="relative flex h-full flex-col overflow-hidden rounded-card bg-carbon-surface glim-hue glim-stagger-row"
      data-self={self ? "" : undefined}
    >
      <div className="flex items-stretch">
        <div className="flex shrink-0 items-center ps-4 pt-4">
          <InstanceMark />
        </div>
        <div className="flex min-w-0 flex-1 flex-col justify-center gap-1 px-5 pt-5">
          <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1.5">
            <span className="min-w-0 truncate font-semibold text-carbon-text max-md:whitespace-normal max-md:wrap-break-word">
              {name}
            </span>
            {self && (
              <span className="shrink-0 text-caption font-semibold uppercase tracking-widest text-carbon-textMuted">
                {t("instances.thisInstance")}
              </span>
            )}
            <span className="ms-auto flex flex-wrap items-center gap-2">{badges}</span>
          </div>
          {address && (
            <p dir="ltr" className="truncate text-start font-mono text-xs text-carbon-textMuted max-md:whitespace-normal max-md:break-all">
              {withoutScheme(address)}
            </p>
          )}
          {facts && <p className="text-xs text-carbon-textMuted wrap-break-word">{facts}</p>}
        </div>
      </div>
      <div className="flex flex-1 flex-col gap-3 p-5">{children}</div>
      {actions && <div className="flex flex-wrap items-center justify-end gap-2 px-5 pb-5">{actions}</div>}
    </div>
  );
}

/** The address as a card shows it, the way it would be typed. */
function withoutScheme(url: string): string {
  return url.replace(/^https?:\/\//, "");
}

function ConnectionBadge({ connected, t }: { connected: boolean; t: T }) {
  return <Badge tone={connected ? "ok" : "fail"} size="large">{connected ? t("instances.connected") : t("instances.notConnected")}</Badge>;
}

function FleetPeerCard({
  peer,
  address,
  t,
  onRefresh,
  index,
}: {
  peer: FleetPeer;
  address?: string;
  t: T;
  onRefresh: () => void;
  index: number;
}) {
  // One key for all cards rather than one per peer, so a renamed or removed
  // peer leaves no entry behind.
  const [open, setOpen] = useState(() => {
    try {
      return localStorage.getItem(FLEET_DETAILS_OPEN_KEY) === "1";
    } catch {
      return false;
    }
  });
  const [removing, setRemoving] = useState(false);

  function toggleDetails() {
    setOpen((v) => {
      const next = !v;
      try {
        localStorage.setItem(FLEET_DETAILS_OPEN_KEY, next ? "1" : "0");
      } catch {
        /* private mode or full quota: not remembered */
      }
      return next;
    });
  }
  const { push } = useToast();
  const [showPropose, setShowPropose] = useState(false);
  // Removing a monitoring entry never contacts the peer and is undone by adding
  // it again, so a two-click inline confirm is enough, as in Receiver.tsx.
  const [confirmRemove, setConfirmRemove] = useState(false);
  const [shakeRemove, setShakeRemove] = useState(0);

  // A check outlasts any request, so the member only confirms it started; the
  // verdict shows in its next scorecard.
  async function handleCheck(domain: string) {
    try {
      const res = await checkFleetPeer(peer.id, domain);
      if (res.ok) push(t("fleet.checkStarted"), "success");
      else push(res.error ?? t("fleet.saveError"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.saveError"), "fail");
    }
  }

  async function handleEnabled(enabled: boolean) {
    try {
      const res = await updateFleetPeer(peer.id, { enabled });
      if (!res.ok) push(res.error ?? t("fleet.saveError"), "fail");
      onRefresh();
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.saveError"), "fail");
    }
  }

  async function handleRemove() {
    setRemoving(true);
    try {
      const res = await deleteFleetPeer(peer.id);
      if (res.ok) {
        onRefresh();
        setConfirmRemove(false);
      } else {
        // The confirm button stays, so it can shake and the user keeps their
        // confirm click for a failure that was not their mistake.
        push(res.error ?? t("fleet.saveError"), "fail");
        setShakeRemove((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.saveError"), "fail");
      setShakeRemove((n) => n + 1);
    } finally {
      setRemoving(false);
    }
  }

  const app = peer.kind === "android";
  const badges = (
    <>
      {app && <Badge tone="neutral">{t("fleet.kindAndroid")}</Badge>}
      {!peer.enabled && <Badge tone="neutral">{t("fleet.monitoringOff")}</Badge>}
      {peer.needsPairing ? (
        <Badge tone="warn">
          {t("pairing.pairAgain")}
          <InfoBubble tip={t("fleet.pairAgainTip")} onAccent />
        </Badge>
      ) : (
        <ConnectionBadge connected={peer.direct || peer.relay} t={t} />
      )}
    </>
  );

  // api.Version already starts with "v".
  const facts = (
    <>
      {peer.lastPollVersion && <span>{peer.lastPollVersion}</span>}
      {peer.lastPollVersion && peer.lastPollAt > 0 && " · "}
      {peer.lastPollAt > 0 && t("fleet.lastPolled").replace("{time}", relativeTime(t, peer.lastPollAt))}
      {peer.lastPollOk === false && peer.lastPollError && <span className="text-statusFail"> · {peer.lastPollError}</span>}
    </>
  );

  const actions = (
    <>
      {!peer.needsPairing && !app && (
        <Button
          label={t("fleet.mesh.proposeButton")}
          labelKey="fleet.mesh.proposeButton"
          tone="neutral"
          onClick={() => setShowPropose(true)}
        />
      )}
      {!peer.needsPairing && !app && (
        <Button
          label={t("fleet.details")}
          labelKey="fleet.details"
          tone="neutral"
          onClick={() => toggleDetails()}
          ariaExpanded={open}
          glyph={<IconDisclosure open={open} />}
        />
      )}
      {/* A text button rather than an icon badge, because the label carries
          the two-click confirm state and a glyph cannot. Neutral like its
          neighbours, with no red of its own. */}
      {confirmRemove ? (
        <Button
          key={shakeRemove}
          label={t("fleet.confirmRemove")}
          labelKey="fleet.confirmRemove"
          tone="neutral"
          onClick={() => void handleRemove()}
          disabled={removing}
          busy={removing}
          title={removing ? t("fleet.removing") : undefined}
          className={shakeRemove ? "glim-shake" : ""}
        />
      ) : (
        <Button label={t("fleet.remove")} labelKey="fleet.remove" tone="neutral" onClick={() => setConfirmRemove(true)} />
      )}
    </>
  );

  return (
    <FleetCard
      name={peer.lastPollInstanceName || peer.name}
      address={address || peer.url}
      badges={badges}
      facts={facts}
      actions={actions}
      index={index}
      t={t}
    >
      {/* A phone backs nothing up, so its card has no scorecard. */}
      {!app && (
        <PeerScorecard
          domains={peer.lastPollDomains}
          t={t}
          onCheck={open && !peer.needsPairing ? (domain) => handleCheck(domain) : undefined}
        />
      )}
      {open && !peer.needsPairing && (
        <ToggleRow checked={peer.enabled} onChange={(v) => void handleEnabled(v)} label={t("fleet.enabledLabel")} />
      )}
      {showPropose && <ProposeMeshDialog peer={peer} t={t} onClose={() => setShowPropose(false)} />}
    </FleetCard>
  );
}

/** How often the page asks who is connected. The answer comes from this
 *  instance's own view of the group, so asking costs no network traffic. */
export const FLEET_REFRESH_MS = 20_000;

/** How old a connected member's scorecard may get before the page fetches it
 *  again over the group. */
export const SCORECARD_STALE_S = 15 * 60;

/** scorecardDue says whether the page should fetch a member's scorecard now. */
export function scorecardDue(peer: FleetPeer, nowS: number): boolean {
  return peer.enabled && !peer.needsPairing && peer.kind !== "android" && (peer.direct || peer.relay) && nowS - peer.lastPollAt >= SCORECARD_STALE_S;
}

interface Self {
  name: string;
  address: string;
  /** In a group at all; outside one the tab is only the way into pairing. */
  active: boolean;
  /** Where each member of the group takes direct calls, by member id. */
  addresses: Record<string, string>;
  version: string;
  domains: DomainStatus[];
  ok: boolean;
}

/** With `embedded` the page is a tab of Instances, which owns the shell and
 *  the heading. */
export function Fleet({ embedded = false }: { embedded?: boolean } = {}) {
  const { t } = useT();
  const navigate = useNavigate();
  const [peers, setPeers] = useState<FleetPeer[]>([]);
  const [self, setSelf] = useState<Self | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [offers, setOffers] = useState<MeshOffer[]>([]);
  const polling = useRef(new Set<string>());

  const loadPeers = useCallback(
    (): Promise<void> =>
      listFleetPeers()
        .then((res) => {
          if (res.ok) {
            setPeers(res.peers ?? []);
            setError(null);
          } else {
            setError(res.error ?? t("fleet.loadError"));
          }
        })
        .catch((err) => setError(err instanceof Error ? err.message : t("fleet.loadError"))),
    [t],
  );

  const loadSelf = useCallback(
    (): Promise<void> =>
      Promise.all([getGroup(), getHealth(), getStatus()])
        .then(([group, health, status]) =>
          setSelf({
            name: group.name,
            address: group.selfAddress,
            active: group.active,
            addresses: Object.fromEntries(group.members.map((m) => [m.id, m.address])),
            version: health.version ?? "",
            domains: status.domains ?? [],
            ok: status.ok,
          }),
        )
        .catch(() => setSelf((prev) => (prev ? { ...prev, ok: false } : prev))),
    [],
  );

  function loadOffers() {
    return listMeshOffers()
      .then((res) => {
        if (res.ok) setOffers(res.offers ?? []);
      })
      .catch(() => undefined);
  }

  useEffect(() => {
    void Promise.all([loadPeers(), loadSelf(), loadOffers()]).finally(() => setLoading(false));
    const refresh = () => {
      if (document.visibilityState !== "visible") return;
      void loadPeers();
      void loadSelf();
    };
    const id = setInterval(refresh, FLEET_REFRESH_MS);
    document.addEventListener("visibilitychange", refresh);
    return () => {
      clearInterval(id);
      document.removeEventListener("visibilitychange", refresh);
    };
  }, [loadPeers, loadSelf]);

  // A member that is connected and has no recent scorecard gets asked for one.
  // A failure lands in the member's row, which the reload then shows.
  useEffect(() => {
    const nowS = Date.now() / 1000;
    for (const p of peers) {
      if (!scorecardDue(p, nowS) || polling.current.has(p.id)) continue;
      polling.current.add(p.id);
      void pollFleetPeer(p.id)
        .catch(() => undefined)
        .finally(() => {
          polling.current.delete(p.id);
          void loadPeers();
        });
    }
  }, [peers, loadPeers]);

  const pendingOffers = offers.filter((o) => o.status === "pending");
  // Nothing to show but this instance: the tab is the way into pairing.
  const empty = !loading && !error && self !== null && !self.active && peers.length === 0;
  const openPairing = () => navigate("/settings/pairing");

  return (
    <div className={embedded ? PAGE_SHELL_TABBED_RESPONSIVE : PAGE_SHELL_RESPONSIVE}>
      {!embedded && <PageTitle>{t("instances.title")}</PageTitle>}

      {loading && <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}
      {error && <p className="text-sm text-statusFail wrap-break-word">{error}</p>}

      {!loading && pendingOffers.length > 0 && (
        <div className="relative bg-carbon-surface rounded-card p-4 flex flex-col gap-3">
          <div>
            {/* The padded card is `relative`, so the notch straddles its edge
                and not this inner div's. */}
            <h2 className="flex items-center">
              <Badge tone="heading" size="heading" wrap hueIndex={0}>{t("fleet.mesh.offersTitle")}</Badge>
            </h2>
            <p className="text-xs text-carbon-textMuted">{t("fleet.mesh.offersHint")}</p>
          </div>
          <div className="flex flex-col gap-2">
            {pendingOffers.map((o) => (
              <MeshOfferRow key={o.id} offer={o} t={t} onChanged={() => void loadOffers()} />
            ))}
          </div>
        </div>
      )}

      {empty && (
        <div className="flex min-h-[50vh] flex-col items-center justify-center">
          <button
            type="button"
            onClick={openPairing}
            className="grid w-full max-w-md grid-cols-[48px_minmax(0,1fr)] items-center gap-4 rounded-card bg-carbon-surface p-5 text-start transition-colors hover:bg-carbon-surface2 glim-field-focus"
          >
            <span className="grid h-12 w-12 place-items-center rounded-control bg-accent text-accentContrast [&>svg]:h-5.5 [&>svg]:w-5.5">
              <IconLink />
            </span>
            <span>
              <span className="block text-lg font-semibold text-carbon-text">{t("pairing.title")}</span>
              <span className="mt-0.5 block text-sm text-carbon-textSub">{t("instances.pairLead")}</span>
            </span>
          </button>
        </div>
      )}

      {!loading && !empty && (self || peers.length > 0) && (
        <>
        <div className="grid grid-cols-[repeat(auto-fill,minmax(min(100%,28rem),1fr))] gap-4 glim-content-fade">
          {self && (
            <FleetCard
              name={self.name}
              address={self.address}
              self
              badges={<ConnectionBadge connected={self.ok} t={t} />}
              facts={self.version || undefined}
              index={0}
              t={t}
            >
              <PeerScorecard domains={self.domains} t={t} />
            </FleetCard>
          )}
          {peers.map((p, i) => (
            <FleetPeerCard
              key={p.id}
              peer={p}
              address={self?.addresses[p.memberId]}
              t={t}
              onRefresh={() => void loadPeers()}
              index={i + 1}
            />
          ))}
        </div>
        <div>
          <Button label={t("pairing.title")} labelKey="pairing.title" glyph={<IconLink />} tone="accent" onClick={openPairing} />
        </div>
        </>
      )}
    </div>
  );
}
