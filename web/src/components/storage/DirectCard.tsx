import { deleteRepo, type NamedRepo, type StorageLocation } from "../../lib/api";
import { idleDirect, inUseText, repoDisplayName } from "../../lib/directRepo";
import { useT } from "../../lib/i18n";
import { placementErrorText } from "../../lib/placementCodes";
import { DOMAIN_LABEL } from "../../lib/storageLocations";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { reposChanged, useNamedRepos } from "../../lib/useNamedRepos";
import { storageLocationsChanged } from "../../lib/useStorageLocations";
import { Card } from "../../pages/settings/shared";
import { Button } from "../Button";
import { IconTrash } from "../navGlyphs";
import { DomainGlyph, Rows, SettingRow } from "./rows";

/**
 * DirectCard lists the sections that back up straight to a storage location,
 * each through its target's direct repository. Such a repository takes every
 * setting from the location, so a row has none of its own; one that nothing
 * backs up to any more can be removed.
 */
export function DirectCard({ location, hueIndex }: { location: StorageLocation; hueIndex: number }) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const repos = useNamedRepos();
  const homes = location.sections.filter((section) => section.use === "home" && section.targetId && section.repoId);
  if (homes.length === 0) return null;

  async function remove(repo: NamedRepo) {
    if (!(await confirm(`${repoDisplayName(t, repo.name, repo.directOf)} - ${repo.repo}`, { confirmKey: "offsite.targets.remove" }))) return;
    const r = await deleteRepo(repo.id);
    if (!r.ok) {
      push(placementErrorText(t, lang, r, "settings.error"), "fail");
      return;
    }
    reposChanged();
    storageLocationsChanged();
  }

  return (
    <Card title={t("direct.card")} hint={t("direct.cardHint")} hueIndex={hueIndex}>
      {confirmDialog}
      <Rows as="ul">
        {homes.map((section) => {
          const repo = repos.find((r) => r.id === section.repoId);
          const name = t(DOMAIN_LABEL[section.domain]);
          const idle = repo !== undefined && idleDirect(repo);
          // Entries that follow the section's default are not counted among
          // those kept here, so the default gets its own line.
          const byDefault = repo?.defaultFor?.some((domain) => domain === section.domain) ?? false;
          return (
            <SettingRow
              key={section.repoId}
              as="li"
              label={section.enabled ? name : t("placement.off").replace("{name}", () => name)}
              glyph={<DomainGlyph domain={section.domain} />}
              note={
                repo && (
                  <>
                    {(!byDefault || repo.inUse !== 0) && <span className="block">{inUseText(t, repo)}</span>}
                    {byDefault && <span className="block">{t("repos.byDefault").replace("{section}", () => name)}</span>}
                    {idle && <span className="block">{t("repos.directIdle")}</span>}
                  </>
                )
              }
            >
              {idle && (
                <Button
                  label={t("offsite.targets.remove")}
                  labelKey="offsite.targets.remove"
                  glyph={<IconTrash />}
                  tone="neutral"
                  onClick={() => void remove(repo)}
                />
              )}
            </SettingRow>
          );
        })}
      </Rows>
    </Card>
  );
}
