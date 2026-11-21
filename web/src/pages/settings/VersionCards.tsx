// GlimStone's Version and Components cards (InfoVersionCard and
// InfoComponentsCard in its AboutCard.tsx), drawn with BombVault's own card,
// rows and buttons. The words and what each update state shows come from
// @glimstone/core, so they read the same as in every other app.
import {
  checkButton,
  componentRows,
  designRow,
  numberLink,
  updateAction,
  updateLine,
  whatsNewLabel,
  withApp,
  type ComponentRow,
  type ComponentsCardText,
  type UpdateRoute,
  type UpdateState,
  type UpdateTone,
  type VersionCardText,
} from "@glimstone/core";
import { Button } from "../../components/Button";
import { IconUpdateAvailable, IconWhatsNew } from "../../components/glyphs";
import { IconSync } from "../../components/navGlyphs";
import { Rows, SettingRow } from "../../components/storage/rows";
import { GLIMSTONE_VERSION } from "../../lib/glimstoneVersion";
import { useT } from "../../lib/i18n";
import { Card } from "./shared";

const TONE: Record<UpdateTone, string> = {
  neutral: "",
  ok: "text-statusOk",
  warn: "text-statusWarn",
  fail: "text-statusFail",
};

/**
 * A number that reads as plain text and opens the newest release of its
 * project. It takes the accent's ink under the pointer and never an underline,
 * so it does not pass for a button. An empty version is one that could not be
 * read.
 */
function VersionNumber({ row, unreleased, className }: { row: ComponentRow; unreleased: (version: string) => string; className: string }) {
  const { t } = useT();
  if (!row.version) return <span className="text-sm text-carbon-textMuted">{t("tile.info.versionUnknown")}</span>;
  const { shown, released, href } = numberLink(row);
  if (!href) return <span className={className}>{released ? shown : unreleased(shown)}</span>;
  return (
    // Under a coarse pointer the padding widens the target and the negative
    // margin takes it back, so the row keeps its height.
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className={`${className} no-underline hover:text-accentText focus-visible:text-accentText pointer-coarse:-m-2.5 pointer-coarse:p-2.5`}
    >
      {shown}
    </a>
  );
}

/**
 * VersionCard is the app with its number and one last row: where the updates
 * stand at its start, and "What's new", "Check for updates" while there is
 * something to look for, and the one action at its end.
 */
export function VersionCard({
  text,
  name,
  version,
  repoUrl,
  route,
  state,
  onCheck,
  guideUrl,
  onWhatsNew,
  unseen,
  hueIndex,
}: {
  text: VersionCardText;
  name: string;
  /** Empty until the server has said which version runs. */
  version: string;
  repoUrl: string;
  route: UpdateRoute;
  /** Where the updates stand. The page finds out, the card shows it. */
  state: UpdateState;
  onCheck: () => void;
  /** Where the instructions for updating the container live. */
  guideUrl: string;
  onWhatsNew: () => void;
  unseen: number;
  hueIndex?: number;
}) {
  const face = updateLine(state, text);
  const action = updateAction(route, state, text);
  const check = checkButton(route, state);

  return (
    <Card title={text.title} hint={withApp(text.hint, name)} hueIndex={hueIndex}>
      <div className="flex items-baseline justify-between gap-4">
        <span className="min-w-0 text-body font-semibold text-carbon-text">{name}</span>
        <VersionNumber row={{ name, version, repoUrl }} unreleased={text.unreleased} className="flex-none text-body tabular-nums text-carbon-text" />
      </div>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-3">
        <p role="status" className="m-0 flex min-w-0 flex-[1_1_8rem] flex-wrap items-baseline gap-x-3 gap-y-1 text-sm text-carbon-textSub">
          <span className={TONE[face.tone]}>{face.line}</span>
          {face.note && <span className="text-carbon-textMuted wrap-break-word">{face.note}</span>}
        </p>
        <div className="ms-auto flex flex-wrap items-end justify-end gap-2">
          <span data-new="settings-version" className="flex">
            <Button
              label={whatsNewLabel(text, unseen)}
              labelKey="version.whatsNew"
              glyph={
                <span className="inline-flex items-center gap-1.5">
                  <i aria-hidden="true" className={`glim-new-dot glim-new-dot-still${unseen > 0 ? "" : " glim-new-dot-seen"}`} />
                  <IconWhatsNew />
                </span>
              }
              keepLabel
              onClick={onWhatsNew}
            />
          </span>
          {check && <Button label={text.check} labelKey="version.check" glyph={<IconSync />} busy={check.busy} disabled={check.busy} onClick={onCheck} />}
          {action && (
            <Button
              label={action.label}
              labelKey={action.labelKey}
              glyph={<IconUpdateAvailable />}
              onClick={() => window.open(guideUrl, "_blank", "noopener,noreferrer")}
            />
          )}
        </div>
      </div>
    </Card>
  );
}

/**
 * ComponentsCard is every other version BombVault ships, in GlimStone's fixed
 * order: the programs it brings along by name, then the design language. What
 * a component is sits in the (i) beside its name.
 */
export function ComponentsCard({ text, name, rows, hueIndex }: { text: ComponentsCardText; name: string; rows: ComponentRow[]; hueIndex?: number }) {
  // The web part is ported by hand, so its row names the version of the copies
  // it runs rather than the npm package's.
  const design = { ...designRow(text), version: GLIMSTONE_VERSION };
  return (
    <Card title={text.title} hint={withApp(text.hint, name)} hueIndex={hueIndex}>
      <Rows>
        {componentRows([...rows, design]).map((row) => (
          <SettingRow key={row.name} label={row.name} hint={row.hint}>
            <VersionNumber row={row} unreleased={text.unreleased} className="flex-none text-sm tabular-nums text-carbon-textSub" />
          </SettingRow>
        ))}
      </Rows>
    </Card>
  );
}
