// Mesh off-site: an instance offers its own off-site storage to a member of
// the group (connection details, never backup data), and reviews the offers
// members sent here. Accepting one creates an ordinary credential set and
// off-site target, since BombVault never hosts storage itself.
import { useState } from "react";

import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { CopyBlock } from "../../components/CopyBlock";
import { InfoBubble } from "../../components/InfoBubble";
import { SelectField } from "../../components/SelectField";
import { useNewTargetQuestion } from "../../components/placement/NewTargetQuestion";
import { acceptMeshOffer, declineMeshOffer, proposeMeshOffer } from "../../lib/api";
import type { DeploySnippetData, FleetPeer, MeshOffer, OffsiteDomain } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { placementErrorText } from "../../lib/placementCodes";
import { placementChanged } from "../../lib/placementEvents";
import { relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { credSetsChanged } from "../../lib/useCloudCredSets";
import { offsiteTargetsChanged } from "../../lib/useOffsiteTargets";
import { InstanceWindow } from "./InstanceWindow";
import { domainLabelKey } from "./PeerScorecard";

type T = ReturnType<typeof useT>["t"];

const MESH_DOMAINS = ["containers", "vms", "flash", "config", "files", "zfs"] as const;

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
    <div className="rounded-card bg-carbon-background p-3 flex flex-col gap-2">
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
            className={`inline-flex items-center rounded-pill px-3 py-1.5 text-xs font-medium disabled:opacity-50${
              shakeAccept ? " glim-shake" : ""
            }`}
          />
        </div>
      )}
    </div>
  );
}

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

  return (
    <InstanceWindow
      title={t("fleet.mesh.proposeTitle")}
      onClose={onClose}
      footer={
        !snippet ? (
          <>
            <Button label={t("files.cancel")} labelKey="files.cancel" tone="neutral" onClick={onClose} disabled={sending} />
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
        )
      }
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
            <label className="flex items-center gap-1.5 text-xs text-carbon-textSub">
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
            <CopyBlock text={snippet.dockerRun} />
          </div>
          <div className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("fleet.mesh.compose")}</span>
            <CopyBlock text={snippet.compose} />
          </div>
        </>
      )}
    </InstanceWindow>
  );
}

/** MeshOffers lists the storage offers that wait for an answer, under the
 *  line that says what accepting one does. */
export function MeshOffers({ offers, onChanged }: { offers: MeshOffer[]; onChanged: () => void }) {
  const { t } = useT();
  if (offers.length === 0) return null;
  return (
    <div className="flex flex-col gap-2">
      <span className="flex items-center gap-1.5 text-xs font-semibold text-carbon-textSub">
        {t("fleet.mesh.offersTitle")}
        <InfoBubble tip={t("fleet.mesh.offersHint")} />
      </span>
      {offers.map((o) => (
        <MeshOfferRow key={o.id} offer={o} t={t} onChanged={onChanged} />
      ))}
    </div>
  );
}
