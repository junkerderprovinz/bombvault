import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { ConfirmDialog } from "../ConfirmDialog";
import { ConfirmSheet } from "../mobile/ConfirmSheet";
import { TestButton, VerdictLine } from "../TestButton";
import {
  createDirectRepo,
  getDirectRepo,
  testDirectLocation,
  type DirectSuggestion,
  type NamedRepo,
  type OkEnvelope,
} from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { placementErrorText } from "../../lib/placementCodes";
import { useToast } from "../../lib/toast";
import { useDialogKeys } from "../../lib/useConfirm";
import { useIsDesktop } from "../../lib/useMediaQuery";
import { useTestVerdict } from "../../lib/useTestVerdict";
import { reposChanged } from "../../lib/useNamedRepos";

export type DirectRepoResult =
  | { kind: "created"; repo: NamedRepo }
  | { kind: "remembered"; name: string; location: string };

const NOTE_KEYS: Record<Exclude<DirectSuggestion["note"], "">, TranslationKey> = {
  "bucket-root": "directRepo.bucketRoot",
  "path-needed": "directRepo.pathNeeded",
};

const FIELD = "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus";

/** DirectRepoDialog sets up a target's direct repository. Nothing is created
 *  before "Create and use"; in "remember" mode nothing is created at all, the
 *  place is only handed back for the folder set that will use it. */
export function DirectRepoDialog({
  target,
  mode,
  onDone,
  onClose,
}: {
  target: { id: string; name: string };
  mode: "create" | "remember";
  onDone: (result: DirectRepoResult) => void;
  onClose: () => void;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const dialogRef = useRef<HTMLDivElement>(null);
  const creating = useRef(false);
  const locationTouched = useRef(false);
  const [name, setName] = useState("");
  const [location, setLocation] = useState("");
  const [note, setNote] = useState<DirectSuggestion["note"]>("");
  const verdict = useTestVerdict(location.trim(), t("settings.error"));
  const isDesktop = useIsDesktop();
  useDialogKeys(isDesktop, dialogRef, onClose);

  useEffect(() => {
    let alive = true;
    void getDirectRepo(target.id)
      .then((r) => {
        if (!alive) return;
        if (!r.ok) {
          push(placementErrorText(t, lang, r, "settings.error"), "fail");
          return;
        }
        if (!r.suggestion || locationTouched.current) return;
        setLocation(r.suggestion.location);
        setNote(r.suggestion.note);
      })
      .catch((err: unknown) => {
        if (alive) push(err instanceof Error ? err.message : t("settings.error"), "fail");
      });
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [target.id]);

  const withTarget = (text: string) => text.replace(/\{target\}/g, () => target.name);

  function test() {
    void verdict.run(async () => {
      let r: OkEnvelope & { initialized?: boolean };
      try {
        r = await testDirectLocation(target.id, location.trim());
      } catch (err) {
        r = { ok: false, error: err instanceof Error ? err.message : undefined };
      }
      if (!r.ok) {
        const error = placementErrorText(t, lang, r, "settings.error");
        return { ok: false, reason: t("directRepo.testFailed").replace("{error}", () => error) };
      }
      return { ok: true, note: r.initialized ? t("directRepo.testExisting") : t("directRepo.testEmpty") };
    });
  }

  async function confirm() {
    if (mode === "remember") {
      onDone({ kind: "remembered", name: name.trim(), location: location.trim() });
      return;
    }
    if (creating.current) return;
    creating.current = true;
    let r: OkEnvelope & { repo?: NamedRepo };
    try {
      r = await createDirectRepo(target.id, name.trim(), location.trim());
    } catch (err) {
      r = { ok: false, error: err instanceof Error ? err.message : undefined };
    } finally {
      creating.current = false;
    }
    if (!r.ok || !r.repo) {
      push(placementErrorText(t, lang, r, "settings.error"), "fail");
      return;
    }
    reposChanged();
    onDone({ kind: "created", repo: r.repo });
  }

  const surface = {
    title: withTarget(t("directRepo.title")),
    message: withTarget(t("directRepo.intro")),
    confirmLabel: t("directRepo.addAndUse"),
    confirmLabelKey: "directRepo.addAndUse",
    cancelLabel: t("common.cancel"),
    cancelTone: "neutral" as const,
    extra: (
      <div className="flex flex-col gap-3">
        <label className="flex flex-col gap-1 text-xs text-carbon-textSub">
          {t("repos.name")}
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={withTarget(t("placement.homeDirect"))}
            spellCheck={false}
            autoComplete="off"
            className={FIELD}
          />
        </label>
        <label className="flex flex-col gap-1 text-xs text-carbon-textSub">
          {t("repos.location")}
          <input
            value={location}
            onChange={(e) => {
              locationTouched.current = true;
              setLocation(e.target.value);
            }}
            spellCheck={false}
            autoComplete="off"
            dir="ltr"
            className={`${FIELD} font-mono text-start`}
          />
        </label>
        {note !== "" && <p className="text-xs text-carbon-textSub">{withTarget(t(NOTE_KEYS[note]))}</p>}
        {mode === "remember" && <p className="text-xs text-carbon-textMuted">{t("directRepo.draftNote")}</p>}
        <VerdictLine verdict={verdict.verdict} />
        <div className="flex items-center gap-2 flex-wrap">
          <TestButton
            label={t("directRepo.test")}
            labelKey="directRepo.test"
            test={verdict}
            onClick={test}
            disabled={location.trim() === ""}
          />
        </div>
      </div>
    ),
    onConfirm: () => void confirm(),
    onCancel: onClose,
  };
  // The sheet takes no ref: BottomSheet handles Escape and the Tab trap itself.
  return createPortal(
    isDesktop ? <ConfirmDialog ref={dialogRef} {...surface} /> : <ConfirmSheet {...surface} />,
    document.body
  );
}
