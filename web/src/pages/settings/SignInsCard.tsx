import { useEffect, useState } from "react";
import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import { IconStar } from "../../components/glyphs";
import { InfoBubble } from "../../components/InfoBubble";
import { IconCheck } from "../../components/navGlyphs";
import { SignInWindow } from "../../components/storage/SignInWindow";
import {
  dismissSignInsNote,
  getCloudCredSets,
  makeDefaultSignIn,
  removeSignIn,
  type CloudCredSetInfo,
  type SignInsNote,
} from "../../lib/api";
import { useT } from "../../lib/i18n";
import { formatList } from "../../lib/placement";
import { pushSaveWarnings } from "../../lib/placementCodes";
import { DOMAIN_LABEL } from "../../lib/storageLocations";
import { useToast } from "../../lib/toast";
import { credSetLabel, credSetsChanged, useCloudCredSets } from "../../lib/useCloudCredSets";
import { Card } from "./shared";

/** The note on what the shared cloud credentials became, as one paragraph. */
function noteText(t: ReturnType<typeof useT>["t"], lang: string, note: SignInsNote): string {
  const parts: string[] = [];
  if (note.setName) {
    parts.push(t(note.merged ? "signIns.note.merged" : "signIns.note.created").replace(/\{name\}/g, () => note.setName));
  }
  if (note.duplicates > 0) parts.push(t("signIns.note.duplicates", note.duplicates));
  if (note.linked.length > 0) {
    const list = formatList(lang, note.linked.map((domain) => t(DOMAIN_LABEL[domain])));
    parts.push(t("signIns.note.linked").replace("{list}", () => list));
  }
  return parts.join(" ");
}

/**
 * SignInsCard lists the saved sign-ins: the keys BombVault signs in to S3
 * services and restic REST servers with. A sign-in comes into being when a
 * storage location is added, and the default serves every place that names
 * none.
 */
export function SignInsCard({ hueIndex }: { hueIndex?: number }) {
  const { t, lang } = useT();
  const { push } = useToast();
  const sets = useCloudCredSets();
  const [note, setNote] = useState<SignInsNote | null>(null);
  const [editing, setEditing] = useState<CloudCredSetInfo | null>(null);
  const [confirmRemove, setConfirmRemove] = useState<string | null>(null);
  const [removingId, setRemovingId] = useState<string | null>(null);
  const [defaultingId, setDefaultingId] = useState<string | null>(null);
  // A failed action shakes the button that caused it, keyed per row.
  const [shake, setShake] = useState<Record<string, number>>({});

  useEffect(() => {
    let alive = true;
    getCloudCredSets()
      .then((res) => {
        if (alive && res.ok) setNote(res.note ?? null);
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);

  function bumpShake(key: string) {
    setShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }

  /** Why a sign-in cannot go, or null when it can. The default goes last,
   *  since every place that names no sign-in falls back to it. */
  function blocker(s: CloudCredSetInfo): string | null {
    const users = s.usedBy ?? [];
    if (users.length > 0) return t("signIns.removeUsed").replace("{list}", () => formatList(lang, users.map((u) => u.name)));
    if (s.default && sets.length > 1) return t("signIns.removeDefault");
    return null;
  }

  async function remove(s: CloudCredSetInfo) {
    if (confirmRemove !== s.id) {
      setConfirmRemove(s.id);
      return;
    }
    setRemovingId(s.id);
    try {
      const r = await removeSignIn(s.id);
      if (r.ok) {
        credSetsChanged();
        pushSaveWarnings(push, t, r.warnings);
      } else {
        push(r.error ?? t("settings.error"), "fail");
        bumpShake(`remove:${s.id}`);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      bumpShake(`remove:${s.id}`);
    } finally {
      setRemovingId(null);
      setConfirmRemove(null);
    }
  }

  async function makeDefault(s: CloudCredSetInfo) {
    setDefaultingId(s.id);
    try {
      const r = await makeDefaultSignIn(s.id);
      if (!r.ok) {
        push(r.error ?? t("settings.error"), "fail");
        bumpShake(`default:${s.id}`);
        return;
      }
      credSetsChanged();
      push(t("settings.saved"), "success");
      pushSaveWarnings(push, t, r.warnings);
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      bumpShake(`default:${s.id}`);
    } finally {
      setDefaultingId(null);
    }
  }

  function dismiss() {
    setNote(null);
    void dismissSignInsNote().catch(() => undefined);
  }

  return (
    <Card title={t("signIns.title")} hint={t("signIns.hint")} hueIndex={hueIndex} newId="settings-sign-ins">
      {note && (
        <div role="status" className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-card bg-statusOkBg px-3 py-2">
          <span className="inline-flex text-statusOk">
            <IconCheck />
          </span>
          <span className="flex min-w-0 flex-1 flex-col gap-0.5 text-sm text-carbon-text">
            <span className="flex items-center gap-1.5 font-semibold">
              {t("signIns.note.title")}
              {noteText(t, lang, note) && <InfoBubble tip={noteText(t, lang, note)} />}
            </span>
            <span className="text-xs text-carbon-textSub">{t("signIns.note.text")}</span>
          </span>
          <Button label={t("common.gotIt")} labelKey="common.gotIt" tone="neutral" onClick={dismiss} />
        </div>
      )}

      {sets.length === 0 && <span className="text-xs text-carbon-textMuted">{t("signIns.none")}</span>}

      {sets.length > 0 && (
        <ul className="flex flex-col divide-y divide-carbon-border/60">
          {sets.map((s) => {
            const users = s.usedBy ?? [];
            const why = blocker(s);
            return (
              <li key={s.id} className="flex flex-col py-3 first:pt-0 last:pb-0">
                {/* On a phone the actions go under the name, which beside them
                    shrinks to a column one character wide. */}
                <div className="flex items-start justify-between gap-3 max-md:flex-col">
                  <div className="flex min-w-0 flex-col gap-1">
                    <span className="flex flex-wrap items-center gap-2 text-sm text-carbon-text">
                      <span className="min-w-0 truncate max-md:whitespace-normal max-md:wrap-anywhere">{credSetLabel(t, s)}</span>
                      {s.s3KeyId && <Badge tone="neutral" size="small">S3</Badge>}
                      {s.restUser && <Badge tone="neutral" size="small">restic REST</Badge>}
                      {s.default && (
                        <Badge tone="ok" size="small">
                          <span className="flex [&_svg]:h-[1em] [&_svg]:w-[1em]">
                            <IconStar />
                          </span>
                          {t("signIns.default")}
                        </Badge>
                      )}
                    </span>
                    {(s.s3KeyId || s.restUser) && (
                      <span dir="ltr" className="break-all text-start font-mono text-xs text-carbon-textMuted">
                        {s.s3KeyId || s.restUser}
                      </span>
                    )}
                    <span className="text-xs text-carbon-textSub wrap-break-word">
                      {users.length > 0
                        ? t("signIns.usedBy").replace("{list}", () => formatList(lang, users.map((u) => u.name)))
                        : t("signIns.unused")}
                    </span>
                    {s.default && <span className="text-xs text-carbon-textSub wrap-break-word">{t("signIns.defaultAlso")}</span>}
                  </div>
                  <div className="flex shrink-0 flex-wrap items-center gap-2">
                    {!s.default && (
                      <span data-new="sign-in-make-default" className="flex">
                        <Button
                          key={shake[`default:${s.id}`] || 0}
                          label={t("signIns.makeDefault")}
                          labelKey="signIns.makeDefault"
                          onClick={() => void makeDefault(s)}
                          disabled={defaultingId !== null}
                          busy={defaultingId === s.id}
                          className={shake[`default:${s.id}`] ? "glim-shake" : ""}
                        />
                      </span>
                    )}
                    <span data-new="sign-in-edit" className="flex">
                      <Button label={t("offsite.targets.edit")} labelKey="offsite.targets.edit" tone="neutral" onClick={() => setEditing(s)} />
                    </span>
                    {/* A destructive action gets no red of its own in this app. */}
                    <Button
                      key={shake[`remove:${s.id}`] || 0}
                      label={t(confirmRemove === s.id ? "offsite.targets.confirmRemove" : "offsite.targets.remove")}
                      labelKey={confirmRemove === s.id ? "offsite.targets.confirmRemove" : "offsite.targets.remove"}
                      tone="neutral"
                      onClick={() => void remove(s)}
                      disabled={why !== null || removingId === s.id}
                      busy={removingId === s.id}
                      hint={why ?? undefined}
                      className={shake[`remove:${s.id}`] ? "glim-shake" : ""}
                    />
                  </div>
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {editing && <SignInWindow set={editing} sets={sets} onClose={() => setEditing(null)} />}
    </Card>
  );
}
