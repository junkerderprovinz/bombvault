import { useCallback, useEffect, useState } from "react";
import { Button } from "../../components/Button";
import { ConfirmDialog } from "../../components/ConfirmDialog";
import {
  deletePasskey,
  passkeyStatus,
  passkeysAvailableInBrowser,
  registerPasskey,
  type PasskeyStatusResponse,
  type PasskeyView,
} from "../../lib/api";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { Card } from "./shared";

// PasskeyCard lets the operator sign in with a key on a phone, laptop or
// security stick instead of typing the password.
//
// A passkey is bound to a domain, and the browser refuses the exchange on a
// bare IP or an untrusted certificate. BombVault's default installation is
// exactly that, so most people opening this card cannot use the feature, and
// the card says why before the browser answers with "NotAllowedError".
//
// The password stays: a passkey is an additional way in, because a lost phone
// must not lock anyone out of the tool that recovers everything else. A key
// also belongs to one address (registered through the proxy, it does not exist
// over the IP), so the list says which address each key is for.
export function PasskeyCard({
  /** Whether a login password exists at all. Without one there is nothing to
   *  add a second way into, and the server refuses the registration. */
  passwordSet,
  hueIndex,
}: {
  passwordSet: boolean;
  hueIndex?: number;
}) {
  const { t } = useT();
  const { push } = useToast();
  const [status, setStatus] = useState<PasskeyStatusResponse | null>(null);
  const [busy, setBusy] = useState(false);
  const [name, setName] = useState("");
  const [adding, setAdding] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<PasskeyView | null>(null);

  const reload = useCallback(async () => {
    try {
      setStatus(await passkeyStatus());
    } catch {
      // Non-fatal: the card then shows the feature as unavailable, which is
      // also what an unreachable server means for it.
      setStatus(null);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  const browserOK = passkeysAvailableInBrowser();
  const addressOK = status?.supported === true;
  const keys = status?.passkeys ?? [];

  async function add() {
    setBusy(true);
    try {
      const res = await registerPasskey(name.trim() || t("auth.passkeyDefaultName"));
      if (res.ok) {
        push(t("auth.passkeyAdded"), "success");
        setName("");
        setAdding(false);
        await reload();
      } else {
        push(res.error ?? t("auth.passkeyFailed"), "fail");
      }
    } catch (err) {
      // The browser's own refusal lands here: a cancelled prompt, a timeout, an
      // authenticator that declined. Its message is the useful one.
      push(err instanceof Error ? err.message : t("auth.passkeyFailed"), "fail");
    } finally {
      setBusy(false);
    }
  }

  async function remove(p: PasskeyView) {
    setBusy(true);
    try {
      const res = await deletePasskey(p.id);
      if (res.ok) {
        push(t("auth.passkeyRemoved"), "success");
        await reload();
      } else {
        push(res.error ?? t("common.removeFailed"), "fail");
      }
    } finally {
      setBusy(false);
      setPendingDelete(null);
    }
  }

  return (
    <Card title={t("auth.passkeys")} hint={t("auth.passkeysHint")} hueIndex={hueIndex}>
      <div className="flex items-center gap-2">
        <span
          className={`inline-block h-2 w-2 rounded-full ${keys.length > 0 ? "bg-statusOkSolid" : "bg-carbon-textMuted"}`}
        />
        <span className="text-sm text-carbon-text">
          {keys.length > 0
            ? t("auth.passkeysOn").replace("{n}", String(keys.length))
            : t("auth.passkeysOff")}
        </span>
      </div>

      {!passwordSet && (
        <p className="text-sm text-carbon-textSub">{t("auth.passkeyNeedsPassword")}</p>
      )}

      {/* Shown whenever this address cannot carry a passkey, which on a stock
          Unraid installation is always: the template opens https://[IP]:3443
          with a certificate that covers only localhost. The text is translated
          rather than the server's English reason, because it is the paragraph
          that explains the whole feature, not an error in a toast. */}
      {passwordSet && !addressOK && (
        <div className="rounded-card bg-statusWarnBgSoft px-3 py-2.5 text-sm text-carbon-text leading-relaxed">
          <p className="font-medium">{t("auth.passkeyNotHere")}</p>
          <p className="mt-1 text-carbon-textSub">{t("auth.passkeyNeedsDomain")}</p>
        </div>
      )}

      {passwordSet && addressOK && !browserOK && (
        <p className="text-sm text-statusWarn">{t("auth.passkeyNoBrowser")}</p>
      )}

      {/* Every key is listed, including the ones bound to another address:
          hiding those would make a key somebody registered look lost. */}
      {keys.length > 0 && (
        <ul className="flex flex-col gap-2">
          {keys.map((p) => (
            <li
              key={p.id}
              className="flex items-center justify-between gap-3 rounded-control bg-carbon-surface2 px-3 py-2"
            >
              <div className="flex min-w-0 flex-col">
                <span className="truncate text-sm text-carbon-text">{p.name}</span>
                {/* Two independent facts, joined by a separator. Whether the
                    second appears depends on the authenticator, so neither
                    sentence can carry the punctuation itself. */}
                <span className="truncate text-xs text-carbon-textSub">
                  {p.usableHere
                    ? t("auth.passkeyUsableHere")
                    : t("auth.passkeyOtherAddress").replace("{host}", p.rpId)}
                  {!p.backedUp && <> · {t("auth.passkeyNotSynced")}</>}
                </span>
              </div>
              <Button
                label={t("common.delete")}
                labelKey="common.delete"
                tone="neutral"
                onClick={() => setPendingDelete(p)}
                disabled={busy}
                hueIndex={hueIndex}
              />
            </li>
          ))}
        </ul>
      )}

      {/* The name is the operator's own label and means nothing to the
          protocol, so an empty one is filled in rather than refused. */}
      {passwordSet && addressOK && browserOK && !adding && (
        <Button
          label={t("auth.passkeyAdd")}
          labelKey="auth.passkeyAdd"
          tone="accent"
          onClick={() => setAdding(true)}
          disabled={busy}
          hueIndex={hueIndex}
          className="self-start"
        />
      )}

      {adding && (
        <div className="flex flex-col gap-2">
          <label htmlFor="bv-passkey-name" className="text-xs text-carbon-textSub">
            {t("auth.passkeyNameLabel")}
          </label>
          <input
            id="bv-passkey-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t("auth.passkeyNamePlaceholder")}
            className="w-64 rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus"
          />
          <div className="flex items-center gap-3">
            <Button
              label={t("common.cancel")}
              labelKey="common.cancel"
              tone="neutral"
              onClick={() => {
                setAdding(false);
                setName("");
              }}
              hueIndex={hueIndex}
            />
            <Button
              label={t("auth.passkeyCreate")}
              labelKey="auth.passkeyCreate"
              tone="accent"
              onClick={() => void add()}
              disabled={busy}
              busy={busy}
              hueIndex={hueIndex}
            />
          </div>
        </div>
      )}

      {pendingDelete && (
        <ConfirmDialog
          title={t("auth.passkeyRemoveTitle")}
          message={t("auth.passkeyRemoveConfirm").replace("{name}", pendingDelete.name)}
          confirmLabel={t("common.delete")}
          confirmLabelKey="common.delete"
          cancelLabel={t("common.cancel")}
          onConfirm={() => void remove(pendingDelete)}
          onCancel={() => setPendingDelete(null)}
        />
      )}
    </Card>
  );
}
