// GlimStone's Version and Components cards (reference/react/AboutCard.tsx,
// InfoVersionCard and InfoComponentsCard) on this app's Card and Button. What
// they say and decide comes from @glimstone/core.
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
  type VersionCardText,
} from "@glimstone/core";
import { Button } from "../../components/Button";
import { glyphFor } from "../../components/glyphFor";
import { IconWhatsNew } from "../../components/glyphs";
import { InfoBubble } from "../../components/InfoBubble";
import { GLIMSTONE_VERSION } from "../../lib/glimstoneVersion";
import { Card } from "./shared";

/** A number that reads as plain text and opens the newest release of its project. */
export function VersionNumber({ row, unreleased }: { row: ComponentRow; unreleased: (version: string) => string }) {
  const { shown, released, href } = numberLink(row);
  if (!href) return <span className="glim-num glim-version-number">{released ? shown : unreleased(shown)}</span>;
  return (
    <a href={href} target="_blank" rel="noopener noreferrer" className="glim-num glim-version-number">
      {shown}
    </a>
  );
}

/**
 * InfoVersionCard is the first card of the Info page: the app with its
 * number, then a last row with where the updates stand and the buttons that
 * go with it. `state` is null until something was asked, since a container
 * here asks GitHub only when the person presses Check for updates.
 */
export function InfoVersionCard({
  text,
  name,
  version,
  repoUrl,
  route,
  state,
  onCheck,
  onUpdate,
  guideUrl,
  onWhatsNew,
  unseen = 0,
  newId,
  hueIndex,
}: {
  text: VersionCardText;
  name: string;
  version: string;
  repoUrl?: string;
  route: UpdateRoute;
  state: UpdateState | null;
  onCheck?: () => void;
  onUpdate?: () => void;
  guideUrl?: string;
  onWhatsNew: () => void;
  unseen?: number;
  /** The change in lib/whatsNew.ts the What's new button is. */
  newId?: string;
  hueIndex?: number;
}) {
  const face = state && updateLine(state, text);
  const action = state && updateAction(route, state, text);
  const check = checkButton(route, state ?? { kind: "current" });
  const press = () => {
    if (action?.kind === "guide") {
      if (guideUrl) window.open(guideUrl, "_blank", "noopener,noreferrer");
    } else onUpdate?.();
  };
  return (
    <Card title={text.title} hint={withApp(text.hint, name)} hueIndex={hueIndex}>
      <div className="glim-version-head">
        <span className="glim-version-app">{name}</span>
        {version && <VersionNumber row={{ name, version, repoUrl }} unreleased={text.unreleased} />}
      </div>
      <div className="glim-version-foot">
        <p className="glim-version-state" data-tone={face?.tone} role="status">
          {face && <span className="glim-version-line">{face.line}</span>}
          {face?.note && <span className="glim-version-note">{face.note}</span>}
        </p>
        <div className="glim-version-actions">
          <span data-new={newId} className="flex">
            <Button
              label={whatsNewLabel(text, unseen)}
              labelKey="version.whatsNew"
              glyph={
                <span className="inline-flex items-center gap-1.5">
                  <span className="glim-version-dot" data-unseen={unseen > 0 ? "" : undefined} aria-hidden="true" />
                  <IconWhatsNew />
                </span>
              }
              keepLabel
              onClick={onWhatsNew}
            />
          </span>
          {check && onCheck && (
            <Button label={text.check} labelKey="version.check" busy={check.busy} disabled={check.busy} onClick={onCheck} />
          )}
          {action && (
            <Button
              label={action.label}
              labelKey={action.labelKey}
              glyph={glyphFor(action.labelKey)}
              tone={action.kind === "guide" ? "neutral" : "accent"}
              onClick={press}
            />
          )}
        </div>
      </div>
    </Card>
  );
}

/**
 * InfoComponentsCard lists every version the app is made of or brings along,
 * in GlimStone's fixed order, with what each one is in the (i) beside its name.
 */
export function InfoComponentsCard({
  text,
  name,
  rows,
  hueIndex,
}: {
  text: ComponentsCardText;
  name: string;
  rows: ComponentRow[];
  hueIndex?: number;
}) {
  // The design language's row names the copied files this app runs, which
  // can trail the package by a release.
  const design: ComponentRow = { ...designRow(text), version: GLIMSTONE_VERSION };
  return (
    <Card title={text.title} hint={withApp(text.hint, name)} hueIndex={hueIndex}>
      <ul className="glim-version-rows">
        {componentRows([...rows, design]).map((row) => (
          <li key={row.name} className="glim-version-row">
            <span className="glim-version-name">
              <span className="glim-version-part">{row.name}</span>
              {row.hint && <InfoBubble tip={row.hint} />}
            </span>
            {row.status && (
              <span className="glim-version-status" data-tone={row.tone}>
                {row.status}
              </span>
            )}
            {row.version && <VersionNumber row={row} unreleased={text.unreleased} />}
          </li>
        ))}
      </ul>
    </Card>
  );
}
