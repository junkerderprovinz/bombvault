import { SelectField } from "../../components/SelectField";
import { Flag } from "../../components/Sidebar";
import { useT } from "../../lib/i18n";

/** LanguagePicker switches the language of the interface. The choice is
 *  stored and applied at once. */
export function LanguagePicker({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const { lang, setLanguage, languages } = useT();
  return (
    // A field's height, like the buttons beside fields, and a fixed width,
    // which the list takes over as its narrowest; truncate keeps a long
    // locale name inside it.
    <SelectField
      value={lang}
      onChange={setLanguage}
      options={languages.map((l) => ({ value: l.code, label: l.label, glyph: <Flag code={l.flag} /> }))}
      label={t("language.label")}
      className="h-[var(--btn-h)] w-48 rounded-control bg-carbon-surface2 px-3 text-sm text-carbon-text hover:bg-carbon-surface3 transition-colors"
    />
  );
}
