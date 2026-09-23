import { Card } from "../settings/shared";
import { DropdownListbox } from "../../components/DropdownListbox";
import { Flag } from "../../components/Sidebar";
import { useRef, useState } from "react";
import { useT } from "../../lib/i18n";
import { stepIndex } from "../../lib/selectScroll";

// LanguageCard switches the UI language. The choice is stored and applied at
// once, with no Save step.
export function LanguageCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const { lang, setLanguage, languages } = useT();
  const [open, setOpen] = useState(false);
  // On the button rather than a wrapper: DropdownListbox sizes the panel to
  // what this ref measures, and an inline-block wrapper inside a flex column
  // stretches to the card's full width.
  const ref = useRef<HTMLButtonElement>(null);

  const current = languages.find((l) => l.code === lang) ?? languages[0];

  return (
    <Card title={t("settings.language")} hueIndex={hueIndex}>
      <div className="inline-block">
        {/* A fixed width, which the panel takes over from the trigger;
            truncate keeps a long locale name inside it. */}
        <button
          ref={ref}
          type="button"
          aria-label={`${t("language.label")}: ${current.label}`}
          title={`${t("language.label")}: ${current.label}`}
          aria-haspopup="listbox"
          aria-expanded={open}
          onClick={() => setOpen((v) => !v)}
          className="flex items-center gap-2.5 w-48 rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-surface3 transition-colors"
        >
          <Flag code={current.flag} />
          <span className="min-w-0 truncate text-start">{current.label}</span>
        </button>
        <DropdownListbox
          open={open}
          onClose={() => setOpen(false)}
          triggerRef={ref}
          label={t("language.label")}
          // The wheel on the closed trigger steps through the languages,
          // clamped at both ends, so a change does not need a list of 42.
          wheelStep={(delta) => {
            const at = languages.findIndex((l) => l.code === lang);
            const next = stepIndex(languages.length, at < 0 ? 0 : at, delta);
            const picked = languages[next];
            if (picked && picked.code !== lang) setLanguage(picked.code);
          }}
        >
          {languages.map((l) => (
            <button
              key={l.code}
              type="button"
              role="option"
              aria-selected={l.code === lang}
              onClick={() => { setLanguage(l.code); setOpen(false); }}
              className={`flex items-center gap-2.5 w-full px-3 py-2 text-sm text-start transition-colors ${
                l.code === lang
                  ? "bg-carbon-surface3 text-carbon-text"
                  : "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
              }`}
            >
              <Flag code={l.flag} />
              <span>{l.label}</span>
            </button>
          ))}
        </DropdownListbox>
      </div>
    </Card>
  );
}
