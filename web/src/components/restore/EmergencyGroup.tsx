// The way into an emergency through the group: another instance keeps the
// sealed recovery kit of the install that is gone. This server asks for it, a
// person at that instance allows it, and the recovery code opens it here.
import { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { emergencyFromGroup, listOfferedDeposits, type EmergencyState, type OfferedDeposit } from "../../lib/api";
import { RELEASE_OVER, recoveryCodeComplete, recoveryCodeShown, refusalText } from "../../lib/emergency";
import { useT } from "../../lib/i18n";
import { formatRecent } from "../../lib/reltime";
import { Button } from "../Button";
import { IconBack, IconKey, IconLink, IconRetry } from "../glyphs";
import { Rows, SettingRow } from "../storage/rows";
import { EMERGENCY_FIELD, Note, Section } from "./EmergencyParts";

/** A request this server sent for a deposit. It waits on the instance that
 *  keeps the deposit, also while this window is closed. */
export interface AskedDeposit {
  memberId: string;
  memberName: string;
  ownerId: string;
  ownerName: string;
}

const ASKED_KEY = "bv-emergency-asked";

// The answer comes from a person, so a few seconds between two looks lose
// nothing, and each look is a call to the other instance.
const POLL_MS = 5000;

/** The request that was waiting when the window was last open, without the
 *  recovery code, which is kept nowhere. */
export function askedDeposit(): AskedDeposit | null {
  try {
    const stored = JSON.parse(window.localStorage.getItem(ASKED_KEY) ?? "null") as Partial<AskedDeposit> | null;
    if (!stored || typeof stored.memberId !== "string" || typeof stored.ownerId !== "string") return null;
    return {
      memberId: stored.memberId,
      memberName: String(stored.memberName ?? ""),
      ownerId: stored.ownerId,
      ownerName: String(stored.ownerName ?? ""),
    };
  } catch {
    return null;
  }
}

function remember(asked: AskedDeposit | null) {
  try {
    if (asked) window.localStorage.setItem(ASKED_KEY, JSON.stringify(asked));
    else window.localStorage.removeItem(ASKED_KEY);
  } catch {
    // Without storage the request is asked for again after the window closes.
  }
}

type Notice = { code?: string; said?: string } | { over: keyof typeof RELEASE_OVER };

export function EmergencyGroup({
  active,
  footer,
  onEmergency,
  onPairing,
}: {
  /** This install is in a group, null until the server has said so. */
  active: boolean | null;
  /** The place in the window's foot for the request and the way back to the
   *  list. */
  footer: HTMLElement | null;
  onEmergency: (state: EmergencyState) => void;
  /** Leads to the page where this install joins a group. */
  onPairing?: () => void;
}) {
  const { t, lang } = useT();
  const [deposits, setDeposits] = useState<OfferedDeposit[] | null>(null);
  const [silent, setSilent] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [asked, setAsked] = useState<AskedDeposit | null>(askedDeposit);
  const [chosen, setChosen] = useState<AskedDeposit | null>(asked);
  const [waiting, setWaiting] = useState(false);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);

  // The recovery code lives in its field alone. It is read from there for
  // each call and goes with the field when the window closes.
  const codeField = useRef<HTMLInputElement>(null);
  const [complete, setComplete] = useState(false);

  const load = useCallback(() => {
    setLoading(true);
    setLoadError(null);
    listOfferedDeposits()
      .then((res) => {
        if (res.ok) {
          setDeposits(res.deposits ?? []);
          setSilent(res.silent ?? []);
        } else setLoadError(res.error ?? t("settings.error"));
      })
      .catch((err: unknown) => setLoadError(err instanceof Error ? err.message : String(err)))
      .finally(() => setLoading(false));
  }, [t]);

  useEffect(() => {
    if (active && chosen === null && deposits === null) load();
  }, [active, chosen, deposits, load]);

  function forget() {
    setAsked(null);
    remember(null);
    setWaiting(false);
  }

  function emptyCode() {
    if (codeField.current) codeField.current.value = "";
    setComplete(false);
  }

  const calling = useRef(false);
  async function send(ask: boolean, polling = false) {
    if (!chosen || calling.current) return;
    calling.current = true;
    if (!polling) setBusy(true);
    try {
      const res = await emergencyFromGroup(chosen.memberId, chosen.ownerId, codeField.current?.value ?? "", ask);
      if (res.ok && res.emergency?.session) {
        forget();
        emptyCode();
        onEmergency(res.emergency);
      } else if (res.ok && res.release === "asked") {
        setAsked(chosen);
        remember(chosen);
        setWaiting(true);
        setNotice(null);
      } else if (res.ok) {
        forget();
        setNotice({ over: res.release === "expired" || res.release === "released" ? res.release : "unasked" });
      } else if (res.code === "deposit-code") {
        // The deposit is held on this server once it was allowed, so the
        // code can be typed again without another request.
        setAsked(chosen);
        remember(chosen);
        setWaiting(false);
        emptyCode();
        codeField.current?.focus();
        setNotice({ code: res.code });
      } else if (polling && (res.code === "member-gone" || res.code === "member-silent")) {
        setNotice({ code: res.code });
      } else {
        if (res.code === "release-declined" || res.code === "deposit-none") forget();
        setWaiting(false);
        setNotice({ code: res.code, said: res.error });
      }
    } catch (err) {
      if (!polling) setNotice({ said: err instanceof Error ? err.message : String(err) });
    } finally {
      calling.current = false;
      setBusy(false);
    }
  }

  const look = useRef(send);
  look.current = send;
  useEffect(() => {
    if (!waiting) return;
    const timer = window.setInterval(() => {
      if (document.visibilityState === "visible") void look.current(false, true);
    }, POLL_MS);
    return () => window.clearInterval(timer);
  }, [waiting]);

  function chooseAnother() {
    forget();
    emptyCode();
    setChosen(null);
    setNotice(null);
  }

  if (active === null) return <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>;
  if (!active) {
    return (
      <Section title={t("emergency.way.group")}>
        <p className="text-sm text-carbon-text">{t("emergency.group.noGroup")}</p>
        {onPairing && (
          <div>
            <Button label={t("emergency.group.toPairing")} labelKey={null} glyph={<IconLink />} tone="accent" onClick={onPairing} />
          </div>
        )}
      </Section>
    );
  }

  if (!chosen) {
    return (
      <Section title={t("emergency.group.deposits")}>
        {loading && <p className="text-sm text-carbon-textMuted">{t("emergency.group.asking")}</p>}
        {loadError && <Note tone="fail">{loadError}</Note>}
        {deposits !== null && !loading && deposits.length === 0 && <p className="text-sm text-carbon-text">{t("emergency.group.none")}</p>}
        {deposits !== null && deposits.length > 0 && (
          <Rows>
            {deposits.map((d) => (
              <div key={`${d.memberId}:${d.ownerId}`} data-testid="emergency-deposit" className="flex flex-wrap items-center gap-3 py-3 first:pt-0 last:pb-0">
                <div className="flex min-w-0 flex-1 basis-48 flex-col">
                  <span className="text-sm font-semibold text-carbon-text wrap-anywhere">{d.ownerName}</span>
                  <span className="text-xs text-carbon-textMuted wrap-anywhere">
                    {t("emergency.group.deposit")
                      .replace("{keeper}", () => d.memberName)
                      .replace("{when}", () => formatRecent(Math.floor(Date.parse(d.updatedAt) / 1000), lang))}
                  </span>
                  {!d.newest && <span className="text-xs text-statusWarn wrap-anywhere">{t("emergency.group.older")}</span>}
                </div>
                <Button
                  label={t("emergency.group.choose")}
                  labelKey={null}
                  glyph={<IconKey />}
                  onClick={() => {
                    setChosen({ memberId: d.memberId, memberName: d.memberName, ownerId: d.ownerId, ownerName: d.ownerName });
                    setNotice(null);
                  }}
                />
              </div>
            ))}
          </Rows>
        )}
        {silent.length > 0 && (
          <p className="text-xs leading-relaxed text-carbon-textMuted">
            {t("emergency.group.silent").replace("{list}", () => new Intl.ListFormat(lang, { type: "conjunction" }).format(silent))}
          </p>
        )}
        {!loading && (
          <div>
            <Button label={t("emergency.group.again")} labelKey={null} glyph={<IconRetry />} tone="subtle" onClick={load} />
          </div>
        )}
      </Section>
    );
  }

  const wrongCode = notice !== null && "code" in notice && notice.code === "deposit-code";
  const named = (key: "emergency.group.waiting" | "emergency.group.resumeText" | "emergency.group.chosen") =>
    t(key)
      .replace("{keeper}", () => chosen.memberName)
      .replace("{owner}", () => chosen.ownerName);

  return (
    <Section title={t("emergency.way.group")}>
      <p className="text-sm text-carbon-text">{named("emergency.group.chosen")}</p>
      {waiting && (
        <Note tone="neutral">
          <span role="status">{named("emergency.group.waiting")}</span>
        </Note>
      )}
      {!waiting && asked && notice === null && <Note tone="neutral">{named("emergency.group.resumeText")}</Note>}
      {notice !== null &&
        ("over" in notice ? (
          <Note tone="warn">{t(RELEASE_OVER[notice.over] ?? "emergency.release.unasked")}</Note>
        ) : (
          <Note tone={wrongCode ? "warn" : "fail"}>{refusalText(t, notice.code, notice.said)}</Note>
        ))}

      <SettingRow label={t("emergency.group.code")} hint={t("emergency.group.codeHint")} htmlFor="emergency-group-code">
        <input
          id="emergency-group-code"
          ref={codeField}
          type="text"
          dir="ltr"
          autoComplete="off"
          autoCapitalize="characters"
          spellCheck={false}
          placeholder="XXXXX-XXXXX-XXXXX-XXXXX"
          disabled={waiting}
          onChange={(e) => {
            e.target.value = recoveryCodeShown(e.target.value);
            setComplete(recoveryCodeComplete(e.target.value));
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter" && complete && !busy) void send(asked === null);
          }}
          className={`w-72 max-w-full ${EMERGENCY_FIELD} tracking-widest disabled:opacity-60`}
        />
      </SettingRow>

      {footer &&
        createPortal(
          <>
            <Button label={t("emergency.group.other")} labelKey={null} glyph={<IconBack />} tone="subtle" onClick={chooseAnother} disabled={busy} />
            {waiting ? (
              <Button label={t("emergency.group.stop")} labelKey="emergency.group.stop" onClick={() => setWaiting(false)} />
            ) : (
              <Button
                label={t(asked === null ? "emergency.group.ask" : wrongCode ? "emergency.group.tryCode" : "emergency.group.resume")}
                labelKey={null}
                glyph={<IconKey />}
                tone="accent"
                onClick={() => void send(asked === null)}
                disabled={busy || !complete}
                busy={busy}
              />
            )}
          </>,
          footer
        )}
    </Section>
  );
}
