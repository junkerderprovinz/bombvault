import { useEffect, useState } from "react";
import { Badge } from "@glimstone/react/Badge";
import { ConnectionRow } from "@glimstone/react/ConnectionRow";
import { Button } from "../../components/Button";
import { IconKey, IconStar } from "../../components/glyphs";
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
  const [editing, setEditing] = useState<string | null>(null);
  const open = sets.find((s) => s.id === editing);

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

  /** Why a sign-in cannot go, or null when it can. The default goes last,
   *  since every place that names no sign-in falls back to it. */
  function blocker(s: CloudCredSetInfo): string | null {
    const users = s.usedBy ?? [];
    if (users.length > 0) return t("signIns.removeUsed").replace("{list}", () => formatList(lang, users.map((u) => u.name)));
    if (s.default && sets.length > 1) return t("signIns.removeDefault");
    return null;
  }

  /** The answer of an action on one sign-in, said in a toast, and whether it worked. */
  async function act(run: () => Promise<{ ok: boolean; error?: string; warnings?: Parameters<typeof pushSaveWarnings>[2] }>, saved?: string) {
    try {
      const r = await run();
      if (!r.ok) {
        push(r.error ?? t("settings.error"), "fail");
        return false;
      }
      credSetsChanged();
      if (saved) push(saved, "success");
      pushSaveWarnings(push, t, r.warnings);
      return true;
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      return false;
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
        <ul className="flex flex-col gap-2">
          {sets.map((s) => {
            const users = s.usedBy ?? [];
            const name = credSetLabel(t, s);
            const key = s.s3KeyId || s.restUser;
            return (
              <li key={s.id} data-new="sign-in-edit">
                <ConnectionRow
                  name={name}
                  title={name}
                  mark={<IconKey />}
                  status={
                    users.length > 0
                      ? t("signIns.usedBy").replace("{list}", () => formatList(lang, users.map((u) => u.name)))
                      : t("signIns.unused")
                  }
                  badges={
                    <>
                      {s.default && (
                        <Badge tone="ok" size="small">
                          <span className="flex [&_svg]:h-[1em] [&_svg]:w-[1em]">
                            <IconStar />
                          </span>
                          {t("signIns.default")}
                        </Badge>
                      )}
                      {s.s3KeyId && (
                        <Badge tone="neutral" size="small">
                          S3
                        </Badge>
                      )}
                      {s.restUser && (
                        <Badge tone="neutral" size="small">
                          restic REST
                        </Badge>
                      )}
                    </>
                  }
                  text={{ test: t("signIns.test"), edit: t("offsite.targets.edit"), remove: t("offsite.targets.remove") }}
                  onEdit={() => setEditing(s.id)}
                >
                  {key && (
                    <span dir="ltr" title={key} className="block truncate text-start font-mono">
                      {key}
                    </span>
                  )}
                  {s.default && <span>{t("signIns.defaultAlso")}</span>}
                </ConnectionRow>
              </li>
            );
          })}
        </ul>
      )}

      {open && (
        <SignInWindow
          set={open}
          sets={sets}
          onClose={() => setEditing(null)}
          makeDefault={() => act(() => makeDefaultSignIn(open.id), t("settings.saved"))}
          remove={{ blocker: blocker(open), run: () => act(() => removeSignIn(open.id)) }}
        />
      )}
    </Card>
  );
}
