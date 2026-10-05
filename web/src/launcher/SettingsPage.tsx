import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { ColorPickerSwatch } from "../components/ColorPickerPopover";
import { IconTipButton } from "../components/IconTipButton";
import { InfoBubble } from "../components/InfoBubble";
import { IconCopy, IconTrash } from "../components/navGlyphs";
import { BrandMark, ReadmeButton } from "../components/ReadmeButton";
import { Toggle } from "../components/Toggle";
import { AboutContent } from "../pages/settings/AboutCard";
import { IconResetArrow } from "../pages/settings/AccentCard";
import { DEFAULT_ACCENT, DEFAULT_ACCENT_PRESETS, getAccent, getAccentPresets, setAccent, setAccentPresets } from "../lib/accent";
import { DOCKER_SVG, UNRAID_SVG, ZIP_SVG } from "../lib/appMarks";
import { RAINBOW, getRainbow, hueVars, setRainbow, type RainbowState } from "../lib/appearance";
import { copyText } from "../lib/clipboard";
import { applyStoredDisco, discoTap, getDisco, setDisco } from "../lib/disco";
import { useT, type TranslationKey } from "../lib/i18n";
import { MOTION_INTENSITIES, getMotionIntensity, setMotionIntensity, stormTap, type MotionIntensity } from "../lib/motion";
import { SHAPES, getShape, leafTap, setShape, type Shape } from "../lib/shape";
import { getResolvedTheme, getTheme, setTheme, type ResolvedTheme } from "../lib/theme";
import { useConfirm } from "../lib/useConfirm";
import type { AppInfo } from "./bridge";
import { Flag } from "./flags";
import { PageTop } from "./PageTop";
import { FOLLOWED_EVENT, following as isFollowing, keepAsOwn, setFollowing } from "./look";

type T = ReturnType<typeof useT>["t"];

const REPO = "https://github.com/junkerderprovinz/bombvault";
const PAYPAL = "https://www.paypal.com/donate/?hosted_button_id=76FVV52TKXTUS";

/** The source of this build where it is a release, the newest otherwise. A
 *  test build counts 0.0.N and has no tag. */
function sourceZip(version: string): string {
  return /^[1-9]\d*\.\d+\.\d+$/.test(version) ? `${REPO}/archive/refs/tags/v${version}.zip` : `${REPO}/archive/refs/heads/main.zip`;
}

function open(url: string) {
  window.open(url, "_blank", "noopener,noreferrer");
}

/** A card with its title in the notch on its top edge, spaced like the
 *  settings cards of KnightLoader's app. */
function Section({ title, hue, info, children }: { title: string; hue: number; info?: string; children: ReactNode }) {
  return (
    <section
      className="relative glim-notch-card glim-hue flex flex-col gap-1 rounded-card bg-carbon-surface px-4 pb-3.5 pt-6"
      style={hueVars(hue) as CSSProperties}
    >
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap hueIndex={hue}>
          {title}
          {info && <InfoBubble tip={info} onAccent />}
        </Badge>
      </h2>
      {children}
    </section>
  );
}

/** A label with an optional line under it, and its control at the end. */
function Row({ label, sub, info, dim, children }: { label: string; sub?: string; info?: string; dim?: boolean; children?: ReactNode }) {
  return (
    <div className="flex items-center gap-3 py-2">
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="flex items-center gap-1.5">
          <span className={`text-sm ${dim ? "text-carbon-textMuted" : "text-carbon-text"}`}>{label}</span>
          {info && <InfoBubble tip={info} />}
        </span>
        {sub && <span className="text-[11px] leading-4 text-carbon-textMuted">{sub}</span>}
      </div>
      {children}
    </div>
  );
}

/** A switch at its own rainbow position among the switches of its card. */
function Switch({ label, checked, onChange, hue }: { label: string; checked: boolean; onChange: (on: boolean) => void; hue: number }) {
  return (
    <span className="glim-hue flex" style={hueVars(hue) as CSSProperties}>
      <Toggle hideLabel label={label} checked={checked} onChange={onChange} />
    </span>
  );
}

/** The small caption over a picker. */
function Axis({ children }: { children: ReactNode }) {
  return <span className="mb-1.5 mt-3 text-[11px] tracking-[0.6px] text-carbon-textSub">{children}</span>;
}

/** Flush segments in a shared groove, the chosen one filled in its position's colour. */
function Well<V extends string>({ label, options, value, onPick }: { label: string; options: { value: V; label: string }[]; value: V; onPick: (v: V) => void }) {
  return (
    <div role="radiogroup" aria-label={label} className="flex gap-0.5 rounded-pill bg-carbon-surface2 p-[3px]">
      {options.map((o, i) => {
        const on = o.value === value;
        return (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={on}
            onClick={() => onPick(o.value)}
            style={hueVars(i) as CSSProperties}
            className={`glim-hue min-w-0 flex-1 truncate rounded-pill px-1.5 py-[7px] text-xs font-medium glim-field-focus ${
              on ? "bg-accent text-accentContrast" : "text-carbon-textSub"
            }`}
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}

/**
 * One colour in a row of swatches, which share the row's width up to 32px
 * each. A swatch with onEdit opens the colour picker; one with onChoose is
 * chosen with a press.
 */
function Swatch({
  hex,
  selected,
  label,
  onEdit,
  onChoose,
}: {
  hex: string;
  selected: boolean;
  label: string;
  onEdit?: (hex: string) => void;
  onChoose?: () => void;
}) {
  return (
    // A ring in the ink, a gap in the card's colour, then the colour: the
    // layers are inset by percentages so the ring keeps its share at any size.
    <span className={`relative aspect-square max-w-8 flex-1 rounded-pill ${selected ? "bg-carbon-text" : ""}`}>
      <span className={`absolute inset-[6%] flex rounded-pill p-[7%] ${selected ? "bg-carbon-surface" : ""}`}>
        {onEdit ? (
          <ColorPickerSwatch value={hex} onChange={onEdit} label={label} className="h-full w-full rounded-pill" />
        ) : (
          <button
            type="button"
            aria-label={label}
            onClick={onChoose}
            className="h-full w-full rounded-pill glim-field-focus"
            style={{ background: hex }}
          />
        )}
      </span>
    </span>
  );
}

function SwatchReset({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <span className="relative aspect-square max-w-8 flex-1">
      <IconTipButton
        tip={label}
        onClick={onClick}
        className="absolute inset-[6%] flex items-center justify-center rounded-pill bg-carbon-surface2 text-carbon-textSub glim-field-focus"
      >
        <IconResetArrow />
      </IconTipButton>
    </span>
  );
}

/**
 * SettingsPage is the app's settings, card for card the ones KnightLoader's
 * app has: language, this device, appearance, motion, the server it runs on,
 * a report for problems, about, and the danger zone.
 */
export function SettingsPage({
  t,
  app,
  onBack,
  onLanguage,
  onRename,
  onFollow,
  onRemoveAll,
}: {
  t: T;
  app: AppInfo;
  onBack: () => void;
  onLanguage: () => void;
  onRename: (name: string) => void;
  /** Asks the first server for its look, after following was switched on. */
  onFollow: () => void;
  onRemoveAll: () => void;
}) {
  const { lang, languages } = useT();
  const { confirm, confirmDialog } = useConfirm();
  const [name, setName] = useState(app.deviceName);
  const [copied, setCopied] = useState(false);
  const copiedTimer = useRef(0);
  useEffect(() => () => window.clearTimeout(copiedTimer.current), []);

  const [follows, setFollows] = useState(isFollowing);
  const [theme, setThemeLocal] = useState<ResolvedTheme>(getResolvedTheme);
  const [shape, setShapeLocal] = useState<Shape>(getShape);
  const [accent, setAccentLocal] = useState(getAccent);
  const [presets, setPresetsLocal] = useState(getAccentPresets);
  const [rainbow, setRainbowLocal] = useState<RainbowState>(getRainbow);
  const [disco, setDiscoLocal] = useState(getDisco);
  const [motion, setMotionLocal] = useState<MotionIntensity>(getMotionIntensity);
  // Found is this page's state rather than storage: leave with another value
  // chosen and the hidden entry is gone until the gesture is made again.
  const [leafFound, setLeafFound] = useState(false);
  const [stormFound, setStormFound] = useState(false);
  const [discoFound, setDiscoFound] = useState(false);
  const leafTaps = useRef({ taps: 0 });
  const stormTaps = useRef({ taps: 0 });
  const discoTaps = useRef({ taps: 0, last: 0 });

  // A followed look, or the phone's own given back, replaces what the page shows.
  useEffect(() => {
    const reread = () => {
      setThemeLocal(getResolvedTheme());
      setShapeLocal(getShape());
      setAccentLocal(getAccent());
      setPresetsLocal(getAccentPresets());
      setRainbowLocal(getRainbow());
    };
    window.addEventListener(FOLLOWED_EVENT, reread);
    return () => window.removeEventListener(FOLLOWED_EVENT, reread);
  }, []);

  /** A change made here makes the look this phone's own. */
  function takeOver() {
    if (!follows) return;
    keepAsOwn();
    setFollows(false);
  }

  function updateRainbow(patch: Partial<RainbowState>) {
    setRainbowLocal(setRainbow(patch));
    applyStoredDisco();
    takeOver();
  }

  function chooseAccent(hex: string) {
    setAccent(hex);
    setAccentLocal(hex);
    takeOver();
  }

  const reducedMotion = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false;
  const current = languages.find((l) => l.code === lang) ?? languages[0];
  // A slot is chosen when it holds the accent; the first match wins, so two
  // slots mixed to one colour do not both light up.
  const chosenSlot = presets.findIndex((hex) => hex.toLowerCase() === accent.toLowerCase());

  // What a report needs and nothing more: no address, no name, no phrase.
  const report = [
    `app:      ${app.version} (versionCode ${app.versionCode})`,
    `platform: android ${app.android}`,
    `language: ${lang}`,
    `look:     theme=${getResolvedTheme()}${getTheme() === "system" ? " (device)" : ""} accent=${follows ? "instance" : "local"} rainbow=${rainbow.on ? "on" : "off"}`,
  ].join("\n");

  const rename = () => {
    const next = name.trim();
    if (next !== app.deviceName) onRename(next);
  };

  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-xl flex-col gap-10 px-4 pb-8 pt-2">
      <PageTop t={t} title={t("settings.title")} onBack={onBack} />

      <Section title={t("settings.language")} hue={0}>
        <button type="button" onClick={onLanguage} className="flex items-center gap-3 py-2 text-start glim-field-focus">
          <span className="min-w-0 flex-1 text-sm text-carbon-text">{t("settings.language")}</span>
          <Flag code={current.flag} />
          <span className="truncate text-sm text-carbon-textMuted">{current.label}</span>
        </button>
      </Section>

      <Section title={t("launcher.device")} hue={4} info={t("launcher.deviceHint")}>
        <input
          aria-label={t("launcher.deviceName")}
          value={name}
          onChange={(e) => setName(e.target.value)}
          onBlur={rename}
          onKeyDown={(e) => {
            if (e.key === "Enter") e.currentTarget.blur();
          }}
          placeholder={app.model}
          autoCorrect="off"
          enterKeyHint="done"
          className="my-1 w-full rounded-control bg-carbon-surface2 px-3.5 py-3 text-sm text-carbon-text placeholder:text-carbon-textMuted glim-field-focus"
        />
      </Section>

      <Section title={t("settings.tab.look")} hue={1}>
        {/* Following is a state, so it is a switch. Off gives the phone's own
            look back; on asks the first server again. */}
        <Row label={t("launcher.follow")} sub={follows ? t("launcher.followOn") : t("launcher.followOff")}>
          <Switch
            label={t("launcher.follow")}
            checked={follows}
            onChange={(on) => {
              setFollowing(on);
              setFollows(on);
              if (on) onFollow();
            }}
            hue={0}
          />
        </Row>

        {/* Dimmed while following: the options show the server's look, and a
            press takes it over on this phone, which turns the switch off. */}
        <div className={`flex flex-col ${follows ? "opacity-40" : ""}`}>
          <Axis>{t("settings.theme")}</Axis>
          <Well
            label={t("settings.theme")}
            options={[
              { value: "light", label: t("theme.light") },
              { value: "dark", label: t("theme.dark") },
            ]}
            value={theme}
            onPick={(v: ResolvedTheme) => {
              setTheme(v);
              setThemeLocal(v);
              takeOver();
            }}
          />
          <Axis>{t("settings.shape")}</Axis>
          <Well
            label={t("settings.shape")}
            options={[...SHAPES, ...(leafFound || shape === "leaf" ? (["leaf"] as const) : [])].map((s) => ({
              value: s as Shape,
              label: t(`settings.shape.${s}` as TranslationKey),
            }))}
            value={shape}
            onPick={(id) => {
              const leaf = leafTap(leafTaps.current, id, shape);
              if (leaf) setLeafFound(true);
              const next = (leaf ?? id) as Shape;
              setShapeLocal(next);
              setShape(next);
              takeOver();
            }}
          />

          {/* Label and swatches on one line. While the rainbow owns the
              colours the swatches dim and stay pressable, since the accent
              still paints everything without a position. */}
          <div className="mb-0.5 mt-3 flex items-center gap-3">
            <span className="flex shrink-0 items-center gap-1.5">
              <span className={`text-sm ${rainbow.on ? "text-carbon-textMuted" : "text-carbon-text"}`}>{t("settings.accentColor")}</span>
              {rainbow.on && <InfoBubble tip={t("settings.accentRainbowHint")} />}
            </span>
            <span className={`flex flex-1 items-center justify-end gap-0.5 ${rainbow.on ? "opacity-40" : ""}`}>
              {presets.map((hex, i) => (
                <Swatch
                  key={i}
                  hex={hex}
                  selected={i === chosenSlot}
                  label={`${t("settings.accentPreset")} ${i + 1}`}
                  // One press chooses; a press on the one chosen opens its colour.
                  onChoose={() => chooseAccent(hex)}
                  onEdit={
                    i === chosenSlot
                      ? (v) => {
                          const next = presets.slice();
                          next[i] = v;
                          setPresetsLocal(setAccentPresets(next));
                          chooseAccent(v);
                        }
                      : undefined
                  }
                />
              ))}
              <SwatchReset
                label={t("settings.accentReset")}
                onClick={() => {
                  setPresetsLocal(setAccentPresets(DEFAULT_ACCENT_PRESETS));
                  chooseAccent(DEFAULT_ACCENT);
                }}
              />
            </span>
          </div>
          <Row label={t("settings.rainbow")}>
            <Switch
              label={t("settings.rainbow")}
              checked={rainbow.on}
              onChange={(on) => {
                updateRainbow({ on });
                if (discoTap(discoTaps.current, on, { now: Date.now() })) {
                  setDiscoFound(true);
                  setDisco(true);
                  setDiscoLocal(true);
                }
              }}
              hue={1}
            />
          </Row>
          {/* Disco hangs off the rainbow and stays hidden until found. Its
              value is this phone's own, like the motion level. */}
          {rainbow.on && (discoFound || disco) && (
            <Row label={t("settings.disco")} info={t("settings.discoHint")}>
              <Switch
                label={t("settings.disco")}
                checked={disco}
                onChange={(on) => {
                  setDisco(on);
                  setDiscoLocal(on);
                }}
                hue={2}
              />
            </Row>
          )}
          {rainbow.on && (
            <div className="mb-0.5 mt-3 flex items-center gap-3">
              <span className="shrink-0 text-sm text-carbon-text">{t("settings.rainbowPaletteLabel")}</span>
              <span className="flex flex-1 items-center justify-end gap-0.5">
                {rainbow.palette.map((hex, i) => (
                  <Swatch
                    key={i}
                    hex={hex}
                    selected={false}
                    label={`${t("settings.rainbowPalette")} ${i + 1}`}
                    onEdit={(v) => {
                      const next = rainbow.palette.slice();
                      next[i] = v;
                      updateRainbow({ palette: next });
                    }}
                  />
                ))}
                <SwatchReset label={t("settings.rainbowPaletteReset")} onClick={() => updateRainbow({ palette: RAINBOW })} />
              </span>
            </div>
          )}
        </div>
      </Section>

      {/* While the phone asks for less motion the bubble says so, or somebody
          picks the liveliest level, sees nothing move and reports a bug. The
          storm is not stopped by the system and says nothing. */}
      <Section
        title={t("settings.motion")}
        hue={5}
        info={reducedMotion && motion !== "storm" ? `${t("settings.motionHint")} ${t("launcher.motionReduced")}` : t("settings.motionHint")}
      >
        <div className="my-1">
          <Well
            label={t("settings.motion")}
            options={[...MOTION_INTENSITIES, ...(stormFound || motion === "storm" ? (["storm"] as const) : [])].map((m) => ({
              value: m as MotionIntensity,
              label: t(`settings.motion.${m}` as TranslationKey),
            }))}
            value={motion}
            onPick={(id) => {
              const storm = stormTap(stormTaps.current, id, motion);
              if (storm) setStormFound(true);
              const next = (storm ?? id) as MotionIntensity;
              setMotionLocal(next);
              setMotionIntensity(next);
            }}
          />
        </div>
      </Section>

      {/* The forms of BombVault this app is not. A server is the only one. */}
      <Section title={t("launcher.appsServer")} hue={7} info={t("launcher.appsServerHint")}>
        <div className="glim-readme-btn-rows my-1">
          <ReadmeButton
            tile="glim-tile-unraid"
            parts={[{ name: "Unraid", sub: t("launcher.appsUnraidSub"), href: `${REPO}#7-install-on-unraid` }]}
            mark={<BrandMark svg={UNRAID_SVG} />}
          />
          <ReadmeButton
            tile="glim-tile-docker"
            parts={[{ name: "Docker", sub: t("launcher.appsComposeSub"), href: `${REPO}/blob/main/deploy/docker-compose.generic.yml` }]}
            mark={<BrandMark svg={DOCKER_SVG} />}
            markClass="glim-docker-mark"
          />
          <ReadmeButton
            tile="glim-tile-zip"
            parts={[{ name: t("launcher.appsSource"), sub: t("launcher.appsZip"), href: sourceZip(app.version) }]}
            mark={<BrandMark svg={ZIP_SVG} />}
          />
        </div>
      </Section>

      <Section title={t("launcher.problems")} hue={2} info={t("launcher.problemsHint")}>
        <pre
          dir="ltr"
          className="my-1 whitespace-pre-wrap break-words rounded-control bg-carbon-surface2 p-3 font-mono text-[11px] leading-[17px] text-carbon-textSub select-text"
        >
          {report}
        </pre>
        <div className="mb-2.5 mt-1.5">
          <Button
            label={copied ? t("launcher.problemsCopied") : t("launcher.problemsCopy")}
            labelKey={copied ? "launcher.problemsCopied" : "launcher.problemsCopy"}
            glyph={<IconCopy />}
            tone="accent"
            className="glim-btn-key"
            onClick={() =>
              void copyText(report).then((ok) => {
                if (!ok) return;
                setCopied(true);
                window.clearTimeout(copiedTimer.current);
                copiedTimer.current = window.setTimeout(() => setCopied(false), 2000);
              })
            }
          />
        </div>
      </Section>

      <Section title={t("launcher.about")} hue={3}>
        <div className="my-1 flex flex-col gap-4">
          <AboutContent
            app={{
              version: app.version,
              paypal: () => open(PAYPAL),
              privacy: { label: t("launcher.privacy"), href: `${REPO}/blob/main/android/PRIVACY.md` },
            }}
          />
        </div>
      </Section>

      {/* No red on the button: what warns is the question it asks. Leaving
          the group alone is on the pairing page. */}
      <Section title={t("launcher.dangerZone")} hue={6}>
        <div className="my-1">
          <Button
            label={t("launcher.removeAll")}
            labelKey="launcher.removeAll"
            glyph={<IconTrash />}
            tone="neutral"
            onClick={() =>
              void confirm(t("launcher.removeAllConfirm"), { confirmKey: "launcher.removeAllButton" }).then((ok) => {
                if (ok) onRemoveAll();
              })
            }
            className="glim-btn-key w-full"
          />
        </div>
      </Section>
      {confirmDialog}
    </main>
  );
}

/** LanguagePage lists every language, the one in use marked. */
export function LanguagePage({ t, onBack }: { t: T; onBack: () => void }) {
  const { lang, languages, setLanguage } = useT();
  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-xl flex-col gap-4 px-4 pb-8 pt-2">
      <header className="flex flex-col gap-1">
        <button type="button" onClick={onBack} className="self-start text-xs text-carbon-textMuted glim-field-focus">
          ‹ {t("settings.title")}
        </button>
        <h1 className="text-xl font-semibold text-carbon-text">{t("settings.language")}</h1>
      </header>
      <div className="flex flex-col gap-1.5">
        {languages.map((l) => {
          const chosen = l.code === lang;
          return (
            <button
              key={l.code}
              type="button"
              aria-current={chosen || undefined}
              onClick={() => {
                setLanguage(l.code);
                onBack();
              }}
              className={`flex items-center gap-2 rounded-card px-4 py-3.5 text-start glim-field-focus ${chosen ? "bg-carbon-surface2" : "bg-carbon-surface"}`}
            >
              <span className="w-[30px] shrink-0">
                <Flag code={l.flag} />
              </span>
              <span className="min-w-0 flex-1 truncate text-sm text-carbon-text">{l.label}</span>
              {chosen && (
                <span aria-hidden className="text-sm font-bold text-accentText">
                  ✓
                </span>
              )}
            </button>
          );
        })}
      </div>
    </main>
  );
}
