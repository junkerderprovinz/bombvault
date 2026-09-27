// Fleet is the read-only view of peer BombVault instances and their cached
// protection scorecards, the same red/amber/green aggregate as the local
// dashboard. Polling GET /api/fleet/status is all this box does to a peer's
// protection data; it never starts a backup, restore or drill there. The
// exception is Mesh off-site: the page can offer this instance's off-site
// storage to a peer (connection details, never backup data) and review offers
// peers sent here. Accepting one makes a storage place for the offered
// domain, since BombVault never hosts storage itself.
//
// Peers are polled by the daily sweep and by "Poll now", not by opening the
// page, because each poll is a round trip to another site.

import { useEffect, useState, type CSSProperties } from "react";
import { createPortal } from "react-dom";
import {
  listFleetPeers,
  createFleetPeer,
  updateFleetPeer,
  deleteFleetPeer,
  pollFleetPeer,
  listMeshOffers,
  proposeMeshOffer,
} from "../lib/api";
import { CopyBlock } from "../components/CopyBlock";
import { MeshOfferRow } from "../components/MeshOfferRow";
import { IconDisclosure } from "../components/IconDisclosure";
import type { FleetPeer, FleetPeerInput, DomainStatus, MeshOffer, DeploySnippetData } from "../lib/api";
import { useT, type TranslationKey } from "../lib/i18n";
import { PAGE_SHELL_RESPONSIVE, PAGE_SHELL_TABBED_RESPONSIVE } from "../lib/pageShell";
import { domainName } from "../lib/placeText";
import { PLACE_DOMAINS } from "../lib/places";
import { SelectField } from "../components/SelectField";
import { relativeTime } from "../lib/reltime";
import { EmptyStateIcon } from "../components/EmptyStateIcon";
import { IconFleet } from "../components/Sidebar";
import { Badge } from "../components/Badge";
import { InfoBubble } from "../components/InfoBubble";
import { RevealInput } from "../components/RevealInput";
import { useReveal } from "../lib/useReveal";
import { useToast } from "../lib/toast";
import { hueVars } from "../lib/appearance";
import { Button } from "../components/Button";
import { WindowActions } from "../components/WindowActions";

import { ToggleRow } from "./settings/shared";
type T = ReturnType<typeof useT>["t"];

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

function PeerScorecard({ domains, t }: { domains: DomainStatus[]; t: T }) {
  const shown = domains.filter((d) => d.enabled && d.protection !== "");
  if (shown.length === 0) {
    return <p className="py-2 text-xs text-carbon-textMuted">{t("fleet.noScorecard")}</p>;
  }
  return (
    <div className="mt-2 flex flex-col gap-1.5">
      {shown.map((d) => (
        <div key={d.domain} className="flex items-center gap-2 flex-wrap">
          <span className="text-xs text-carbon-textSub w-20 shrink-0">{domainName(t, d.domain)}</span>
          <Badge tone={protectionTone(d.protection)}>{t(protectionLabelKey(d.protection))}</Badge>
          {d.lastSuccess > 0 && (
            <span className="text-xs text-carbon-textMuted">
              {t("fleet.lastBackup").replace("{time}", relativeTime(t, d.lastSuccess))}
            </span>
          )}
        </div>
      ))}
    </div>
  );
}

// Exported for components/WindowActions.dom.test.tsx.
export function ProposeMeshDialog({ peer, t, onClose }: { peer: FleetPeer; t: T; onClose: () => void }) {
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
  // against the viewport edge. The window is capped at 90vh, so it never clips,
  // and its fields scroll inside it when they outgrow it.
  return createPortal(
    <div className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4" onClick={onClose}>
      {/* The heading notch sits on a shell around the window, as in
          Receiver.tsx's ReceiverDialog. */}
      <div className="relative w-full max-w-lg">
      {/* px-5 matches the body's padding so the notch lands where a Card's
          does; FolderBrowser.tsx explains why the notch has no offset of its
          own. */}
      <h2 className="flex items-center px-5">
        <Badge tone="heading" size="heading" wrap>{t("fleet.mesh.proposeTitle")}</Badge>
      </h2>
      <div
        role="dialog"
        aria-modal="true"
        aria-label={t("fleet.mesh.proposeTitle")}
        onClick={(e) => e.stopPropagation()}
        className="flex max-h-[90vh] w-full flex-col rounded-card bg-carbon-surface shadow-2xl"
      >
        <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-5 pt-5 pb-1">
          <p className="text-xs text-carbon-textMuted">{t("fleet.mesh.proposeHint").replace("{peer}", peer.name)}</p>

          {!snippet ? (
            <>
              <div className="flex flex-col gap-1.5">
                <label className="text-xs text-carbon-textSub">{t("fleet.mesh.domain")}</label>
                <SelectField
                  value={domain}
                  onChange={setDomain}
                  label={t("fleet.mesh.domain")}
                  options={PLACE_DOMAINS.map((d) => ({ value: d, label: domainName(t, d) }))}
                  className={inputCls}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <label className="flex items-center gap-1 text-xs text-carbon-textSub">
                  {t("fleet.mesh.baseUrl")}
                  <InfoBubble tip={t("fleet.mesh.baseUrlHint")} />
                </label>
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
              </div>
            </>
          ) : (
            <>
              <p className="text-xs text-statusOk">{t("fleet.mesh.sent").replace("{peer}", peer.name)}</p>
              <p className="text-xs text-carbon-textMuted">{t("fleet.mesh.deployNow")}</p>
              <div className="flex flex-col gap-1">
                <span className="text-xs text-carbon-textSub">{t("fleet.mesh.dockerRun")}</span>
                <CopyBlock text={snippet.dockerRun} t={t} />
              </div>
              <div className="flex flex-col gap-1">
                <span className="text-xs text-carbon-textSub">{t("fleet.mesh.compose")}</span>
                <CopyBlock text={snippet.compose} t={t} />
              </div>
            </>
          )}
        </div>

        <WindowActions>
          {!snippet ? (
            <>
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
            </>
          ) : (
            <Button label={t("common.close")} labelKey="common.close" tone="accent" onClick={onClose} />
          )}
        </WindowActions>
      </div>
      </div>
    </div>,
    document.body,
  );
}

function FleetPeerCard({
  peer,
  t,
  onRefresh,
  onEdit,
  index,
}: {
  peer: FleetPeer;
  t: T;
  onRefresh: () => void;
  onEdit: () => void;
  /** Rainbow position by list index, as for ContainerRow and VMRow. */
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
  const [polling, setPolling] = useState(false);
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
  const [shakePoll, setShakePoll] = useState(0);
  const [shakeRemove, setShakeRemove] = useState(0);

  // Any answer from the server reloads the list, so the poll badge shows the
  // outcome the server recorded.
  async function handlePoll() {
    setPolling(true);
    try {
      const res = await pollFleetPeer(peer.id);
      if (!res.ok) {
        push(res.error ?? t("fleet.saveError"), "fail");
        setShakePoll((n) => n + 1);
      }
      onRefresh();
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.saveError"), "fail");
      setShakePoll((n) => n + 1);
    } finally {
      setPolling(false);
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

  const pollTone: "ok" | "fail" | "neutral" = peer.lastPollOk === null ? "neutral" : peer.lastPollOk ? "ok" : "fail";
  const pollLabel =
    peer.lastPollOk === null ? t("fleet.pollNever") : peer.lastPollOk ? t("fleet.pollOk") : t("fleet.pollFailed");

  return (
    <div
      style={{ ...hueVars(index), "--row-i": String(index) } as CSSProperties}
      // The same hued shell as ContainerRow, without glim-active: polling and
      // proposing are quick requests, not a tracked job.
      className="relative overflow-hidden bg-carbon-surface rounded-card p-4 flex flex-col gap-3 glim-hue glim-stagger-row"
    >
      <div className="flex items-start gap-3 flex-wrap">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-semibold text-carbon-text text-sm truncate max-md:whitespace-normal max-md:wrap-break-word">
              {peer.lastPollInstanceName || peer.name}
            </span>
            {!peer.enabled && <Badge tone="neutral">{t("fleet.monitoringOff")}</Badge>}
            <Badge tone={pollTone}>{pollLabel}</Badge>
          </div>
          <p dir="ltr" className="mt-1 text-xs font-mono text-carbon-textMuted truncate text-start max-md:whitespace-normal max-md:break-all">{peer.url}</p>
        </div>
        {/* api.Version already starts with "v". */}
        {peer.lastPollVersion && (
          <span className="text-xs text-carbon-textMuted shrink-0">{peer.lastPollVersion}</span>
        )}
      </div>

      {peer.lastPollAt > 0 && (
        <p className="text-xs text-carbon-textMuted">
          {t("fleet.lastPolled").replace("{time}", relativeTime(t, peer.lastPollAt))}
          {peer.lastPollOk === false && peer.lastPollError && (
            <span className="text-statusFail"> · {peer.lastPollError}</span>
          )}
        </p>
      )}

      <div className="flex items-center gap-3 flex-wrap">
        <Button
          key={shakePoll}
          label={t("fleet.pollNow")}
          labelKey="fleet.pollNow"
          title={polling ? t("fleet.polling") : undefined}
          tone="accent"
          onClick={() => void handlePoll()}
          disabled={polling}
          busy={polling}
          className={`inline-flex items-center gap-1.5 rounded-pill bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50${
            shakePoll ? " glim-shake" : ""
          }`}
        />

        <div className="ms-auto flex items-center gap-2 flex-wrap max-md:w-full max-md:justify-end">
          <Button
            label={t("fleet.mesh.proposeButton")}
            labelKey="fleet.mesh.proposeButton"
            tone="neutral"
            onClick={() => setShowPropose(true)}
          />
          <Button
            label={t("fleet.details")}
            labelKey="fleet.details"
            tone="neutral"
            onClick={() => toggleDetails()}
            glyph={<IconDisclosure open={open} />}
          />
          <Button
            label={t("fleet.edit")}
            labelKey="fleet.edit"
            tone="neutral"
            onClick={onEdit}
          />
          {/* A text button rather than an icon badge, because the label
              carries the two-click confirm state and a glyph cannot. Neutral
              like its neighbours, with no red of its own. */}
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
            <Button
              label={t("fleet.remove")}
              labelKey="fleet.remove"
              tone="neutral"
              onClick={() => setConfirmRemove(true)}
            />
          )}
        </div>
      </div>

      {open && (
        <div className="rounded-card bg-carbon-background px-3 py-2">
          <p className="text-xs font-medium text-carbon-textSub">{t("fleet.scorecardTitle")}</p>
          <PeerScorecard domains={peer.lastPollDomains} t={t} />
        </div>
      )}
      {showPropose && <ProposeMeshDialog peer={peer} t={t} onClose={() => setShowPropose(false)} />}
    </div>
  );
}

// Exported for components/WindowActions.dom.test.tsx.
export function FleetDialog({
  initial,
  t,
  onClose,
  onSaved,
}: {
  /** null = create; a peer = edit that peer. */
  initial: FleetPeer | null;
  t: T;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { push } = useToast();
  const [name, setName] = useState(initial?.name ?? "");
  const [url, setUrl] = useState(initial?.url ?? "");
  const [token, setToken] = useState("");
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [saving, setSaving] = useState(false);
  const revealToken = useReveal();
  const [shake, setShake] = useState(0);

  const editing = initial !== null;
  const canSave = name.trim() !== "" && url.trim() !== "" && (token.trim() !== "" || editing) && !saving;

  // The dialog closes on success, so every outcome is reported as a toast.
  async function handleSave() {
    if (name.trim() === "") {
      push(t("fleet.nameRequired"), "fail");
      setShake((n) => n + 1);
      return;
    }
    if (url.trim() === "") {
      push(t("fleet.urlRequired"), "fail");
      setShake((n) => n + 1);
      return;
    }
    setSaving(true);
    const input: FleetPeerInput = {
      name: name.trim(),
      url: url.trim(),
      token: token.trim(),
      enabled,
      sortOrder: initial?.sortOrder ?? 0,
    };
    try {
      const res = editing ? await updateFleetPeer(initial.id, input) : await createFleetPeer(input);
      if (res.ok) {
        push(t("settings.saved"), "success");
        onSaved();
      } else {
        push(res.error ?? t("fleet.saveError"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.saveError"), "fail");
      setShake((n) => n + 1);
    } finally {
      setSaving(false);
    }
  }

  const inputCls =
    "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus";

  // Centred and split into shell and window like ProposeMeshDialog.
  return createPortal(
    <div
      className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4"
      onClick={onClose}
    >
      <div className="relative w-full max-w-lg">
      <h2 className="flex items-center px-5">
        <Badge tone="heading" size="heading" wrap>{editing ? t("fleet.editTitle") : t("fleet.addTitle")}</Badge>
      </h2>
      <div
        role="dialog"
        aria-modal="true"
        aria-label={editing ? t("fleet.editTitle") : t("fleet.addTitle")}
        onClick={(e) => e.stopPropagation()}
        className="flex max-h-[90vh] w-full flex-col rounded-card bg-carbon-surface shadow-2xl"
      >
        <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-5 pt-5 pb-1">
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-carbon-textSub">{t("fleet.name")}</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              spellCheck={false}
              autoComplete="off"
              placeholder="tower"
              className={inputCls}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <label className="flex items-center gap-1 text-xs text-carbon-textSub">
              {t("fleet.url")}
              <InfoBubble tip={t("fleet.urlHint")} />
            </label>
            <input
              type="text"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              spellCheck={false}
              autoComplete="off"
              placeholder="https://192.168.1.50:3443"
              dir="ltr"
              className={`${inputCls} font-mono text-start`}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <label className="flex items-center gap-1 text-xs text-carbon-textSub">
              {t("fleet.token")}
              <InfoBubble tip={t("fleet.tokenHint")} />
            </label>
            <RevealInput
              {...revealToken}
              value={token}
              onChange={(e) => setToken(e.target.value)}
              spellCheck={false}
              autoComplete="off"
              placeholder={editing ? t("fleet.tokenKeep") : "a1b2c3…"}
              wrapperClassName="w-full"
              className={`${inputCls} font-mono`}
            />
          </div>

          {/* ToggleRow puts the words at the start and the switch at the end,
              like every setting row. */}
          <ToggleRow checked={enabled} onChange={setEnabled} label={t("fleet.enabledLabel")} />
        </div>

        <WindowActions>
          <Button
            label={t("files.cancel")}
            labelKey="files.cancel"
            tone="neutral"
            onClick={onClose}
            disabled={saving}
          />
          <Button
            key={shake}
            label={t("settings.save")}
            labelKey="settings.save"
            tone="accent"
            onClick={() => void handleSave()}
            disabled={!canSave}
            busy={saving}
            title={saving ? t("common.saving") : undefined}
            className={shake ? "glim-shake" : ""}
          />
        </WindowActions>
      </div>
      </div>
    </div>,
    document.body,
  );
}

/** With `embedded` the page is a tab of Instances, which owns the shell and
 *  the heading; the subtitle stays. */
export function Fleet({ embedded = false }: { embedded?: boolean } = {}) {
  const { t } = useT();
  const [peers, setPeers] = useState<FleetPeer[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // null = closed; "new" = create dialog; a row = edit dialog for that peer.
  const [dialog, setDialog] = useState<"new" | FleetPeer | null>(null);
  const [offers, setOffers] = useState<MeshOffer[]>([]);

  function loadPeers() {
    return listFleetPeers()
      .then((res) => {
        if (res.ok) {
          setPeers(res.peers ?? []);
          setError(null);
        } else {
          setError(res.error ?? t("fleet.loadError"));
        }
      })
      .catch((err) => setError(err instanceof Error ? err.message : t("fleet.loadError")));
  }

  function loadOffers() {
    return listMeshOffers()
      .then((res) => {
        if (res.ok) setOffers(res.offers ?? []);
      })
      .catch(() => undefined);
  }

  useEffect(() => {
    void Promise.all([loadPeers(), loadOffers()]).finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const pendingOffers = offers.filter((o) => o.status === "pending");

  // The empty state has its own Add button, so the header one waits for the
  // first peer.
  const showEmptyState = !loading && !error && peers.length === 0;

  // Heading notches take hues in render order. The offers card and the empty
  // state can show together (a peer offered storage before any peer is
  // watched), so neither can assume hue 0. Both calls sit directly in this
  // component's JSX, so a notch that does not render takes no hue.
  let hueSeq = 0;
  const nextHue = () => hueSeq++;

  return (
    <div className={embedded ? PAGE_SHELL_TABBED_RESPONSIVE : PAGE_SHELL_RESPONSIVE}>
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          {!embedded && <h1 className="text-2xl font-semibold text-carbon-text">{t("fleet.title")}</h1>}
          <p className="mt-1 text-sm text-carbon-textSub">{t("fleet.subtitle")}</p>
        </div>
        {!showEmptyState && (
          <Button
            label={t("fleet.addPeer")}
            labelKey="fleet.addPeer"
            tone="accent"
            onClick={() => setDialog("new")}
            className="shrink-0"
          />
        )}
      </div>

      {loading && <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>}
      {error && <p className="text-sm text-statusFail wrap-break-word">{error}</p>}

      {!loading && pendingOffers.length > 0 && (
        <div className="relative bg-carbon-surface rounded-card p-4 flex flex-col gap-3">
          <div>
            {/* The padded card is `relative`, so the notch straddles its edge
                and not this inner div's. */}
            <h2 className="flex items-center">
              <Badge tone="heading" size="heading" wrap hueIndex={nextHue()}>{t("fleet.mesh.offersTitle")}</Badge>
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

      {/* insetStart corrects the notch in a centred card, see Badge.tsx. */}
      {showEmptyState && (() => {
        // One hue for the notch and the card, so glim-hue gives the Add
        // button the same accent.
        const emptyHue = nextHue();
        return (
          <div
            className="relative glim-notch-card glim-hue bg-carbon-surface rounded-card p-6 text-center flex flex-col items-center gap-3"
            style={hueVars(emptyHue) as CSSProperties}
          >
            <h2 className="flex items-center">
              <Badge tone="heading" size="heading" wrap hueIndex={emptyHue} insetStart={6}>
                {t("fleet.emptyTitle")}
                <InfoBubble tip={t("fleet.empty")} onAccent />
              </Badge>
            </h2>
            <EmptyStateIcon icon={IconFleet} />
            <Button
              label={t("fleet.addPeer")}
              labelKey="fleet.addPeer"
              tone="accent"
              onClick={() => setDialog("new")}
            />
          </div>
        );
      })()}

      {!loading && peers.length > 0 && (
        <div className="flex flex-col gap-3 glim-content-fade">
          {peers.map((p, i) => (
            <FleetPeerCard
              key={p.id}
              peer={p}
              t={t}
              onRefresh={() => void loadPeers()}
              onEdit={() => setDialog(p)}
              index={i}
            />
          ))}
        </div>
      )}

      {dialog !== null && (
        <FleetDialog
          initial={dialog === "new" ? null : dialog}
          t={t}
          onClose={() => setDialog(null)}
          onSaved={() => {
            setDialog(null);
            void loadPeers();
          }}
        />
      )}
    </div>
  );
}
