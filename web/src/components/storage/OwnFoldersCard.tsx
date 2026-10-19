import { useState } from "react";
import type { OffsiteDomain, StorageLocation } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { DOMAIN_LABEL } from "../../lib/storageLocations";
import { useStorageLocations } from "../../lib/useStorageLocations";
import { Card } from "../../pages/settings/shared";
import { Button } from "../Button";
import { IconForward } from "../glyphs";
import { IconPencil } from "../navGlyphs";
import { OwnFolderWindow } from "./OwnFolderWindow";
import { DomainGlyph, Rows, SettingRow } from "./rows";

/** The sections whose own folder is a home of the location: neither a named
 *  repository nor a target's direct one. */
export function ownFolderSections(location: StorageLocation) {
  return location.sections.filter((s) => s.use === "home" && !s.repoId && !s.targetId);
}

/**
 * OwnFoldersCard lists the sections that write their backups to a storage
 * location first, and makes it the own folder of more of them.
 */
export function OwnFoldersCard({ location, hueIndex }: { location: StorageLocation; hueIndex: number }) {
  const { t } = useT();
  const { locations } = useStorageLocations();
  const [moving, setMoving] = useState<{ domains: OffsiteDomain[]; target?: string } | null>(null);
  const own = ownFolderSections(location);
  // Every section that has an own folder somewhere and not here yet.
  const elsewhere = [
    ...new Set(
      locations.filter((l) => l.id !== location.id).flatMap((l) => ownFolderSections(l).map((s) => s.domain))
    ),
  ];

  return (
    <Card
      title={t("own.card")}
      hint={t("own.cardHint")}
      hueIndex={hueIndex}
      newId="own-folders-card"
      foot={
        elsewhere.length > 0 && (
          <Button
            label={t(own.length > 0 ? "own.addSections" : "own.use")}
            labelKey={null}
            glyph={<IconForward />}
            tone="accent"
            onClick={() => setMoving({ domains: elsewhere, target: location.id })}
            className="glim-btn-wrap"
          />
        )
      }
    >
      {own.length > 0 ? (
        <Rows as="ul">
          {own.map((section) => (
            <SettingRow
              key={section.domain}
              as="li"
              label={t(DOMAIN_LABEL[section.domain])}
              glyph={<DomainGlyph domain={section.domain} />}
              note={
                <code dir="ltr" className="break-all text-start font-mono">
                  {section.where}
                </code>
              }
            >
              <Button
                label={t("storage.add.change")}
                labelKey="storage.add.change"
                glyph={<IconPencil />}
                tone="neutral"
                onClick={() => setMoving({ domains: [section.domain] })}
              />
            </SettingRow>
          ))}
        </Rows>
      ) : (
        <p className="text-sm text-carbon-textSub">{t("own.cardNone")}</p>
      )}
      {moving && <OwnFolderWindow domains={moving.domains} target={moving.target} onClose={() => setMoving(null)} />}
    </Card>
  );
}
