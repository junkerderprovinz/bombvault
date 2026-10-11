// Takes a recovery kit as a file, picked or dropped, and the age identity of
// one that is sealed.
import { useRef, useState, type DragEvent, type RefObject } from "react";
import { useT } from "../../lib/i18n";
import { Button } from "../Button";
import { IconUpload } from "../glyphs";
import { InfoBubble } from "../InfoBubble";

// A kit is a page of text. Anything far beyond that is another file.
const MAX_BYTES = 1024 * 1024;

/**
 * KitPicker hands the text of the chosen file to its caller. It keeps the
 * name and nothing of the content, which holds a key unless the kit is sealed.
 */
export function KitPicker({
  name,
  busy = false,
  onPick,
}: {
  /** The file that was chosen last, null before the first. */
  name: string | null;
  busy?: boolean;
  onPick: (text: string, name: string) => void;
}) {
  const { t } = useT();
  const input = useRef<HTMLInputElement>(null);
  const [over, setOver] = useState(false);
  const [refused, setRefused] = useState<string | null>(null);

  async function take(file: File | undefined) {
    if (!file) return;
    if (file.size > MAX_BYTES) {
      setRefused(t("kit.tooLarge"));
      return;
    }
    setRefused(null);
    onPick(await file.text(), file.name);
  }

  function drop(e: DragEvent) {
    e.preventDefault();
    setOver(false);
    void take(e.dataTransfer.files[0]);
  }

  return (
    <div
      data-testid="kit-drop"
      onDragOver={(e) => {
        e.preventDefault();
        setOver(true);
      }}
      onDragLeave={() => setOver(false)}
      onDrop={drop}
      className={`flex flex-wrap items-center gap-3 rounded-card border-2 border-dashed p-4 transition-colors ${
        over ? "border-accent bg-accentSoft" : "border-carbon-border"
      }`}
    >
      <input
        ref={input}
        type="file"
        hidden
        data-testid="kit-file"
        accept=".md,.age,.txt,text/markdown,text/plain"
        onChange={(e) => {
          void take(e.target.files?.[0]);
          // The same file can be chosen again after a refusal.
          e.target.value = "";
        }}
      />
      <Button
        label={t("kit.choose")}
        labelKey={null}
        glyph={<IconUpload />}
        onClick={() => input.current?.click()}
        disabled={busy}
        busy={busy}
      />
      <span className="min-w-0 flex-1 text-xs text-carbon-textMuted wrap-anywhere">
        {name ? (
          <span dir="ltr" className="font-mono text-carbon-text">
            {name}
          </span>
        ) : (
          t("kit.drop")
        )}
      </span>
      {refused && <p className="basis-full text-xs text-statusFail">{refused}</p>}
    </div>
  );
}

/**
 * KitIdentityField asks for the age identity a sealed kit was sealed for. The
 * field is bound to no state: takeIdentity reads it for one request and
 * empties it, so the secret is held nowhere else.
 */
export function KitIdentityField({ inputRef, onEnter }: { inputRef: RefObject<HTMLInputElement | null>; onEnter?: () => void }) {
  const { t } = useT();
  return (
    <label className="flex flex-col gap-1 text-xs text-carbon-textSub">
      <span className="flex items-center gap-1">
        {t("kit.identity")}
        <InfoBubble tip={t("kit.identityHint")} />
      </span>
      <input
        ref={inputRef}
        type="password"
        dir="ltr"
        autoComplete="off"
        spellCheck={false}
        placeholder="AGE-SECRET-KEY-1…"
        onKeyDown={(e) => {
          if (e.key === "Enter") onEnter?.();
        }}
        className="rounded-control bg-carbon-surface2 px-3 py-2 text-start font-mono text-sm text-carbon-text glim-field-focus"
      />
    </label>
  );
}

export function takeIdentity(inputRef: RefObject<HTMLInputElement | null>): string {
  const field = inputRef.current;
  if (!field) return "";
  const identity = field.value.trim();
  field.value = "";
  return identity;
}
