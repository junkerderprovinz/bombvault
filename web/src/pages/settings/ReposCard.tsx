import { useEffect, useRef, useState } from "react";
import { Card } from "./shared";
import { Button } from "../../components/Button";
import { LocationRow } from "../../components/storage/LocationList";
import { Rows, SettingRow } from "../../components/storage/rows";
import { useToast } from "../../lib/toast";
import { createRepo, listRepos, type NamedRepo } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { keptText } from "../../lib/directRepo";
import { placementErrorText } from "../../lib/placementCodes";
import { reposChanged } from "../../lib/useNamedRepos";
import { storageLocationsChanged, useStorageLocationsOnDemand } from "../../lib/useStorageLocations";

// ReposCard is where the named repositories of #204 are written down: a
// location written once and picked per item, instead of the same bucket path
// typed into ten containers. Each row opens the repository's own page, which
// holds its settings and its removal. A direct repository is listed with the
// storage location of its target, so the card holds only the ones made here.
//
// Two refusals carry the weight of a repository, and both come from the same
// fact: nothing ever re-homes a backup that has been written. The location of
// a repository in use cannot move, and one in use cannot be deleted. The
// in-use count is on every row, so the reason is visible before the attempt.
export function ReposCard({ hueIndex }: { hueIndex?: number }) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { locations } = useStorageLocationsOnDemand();
  const [repos, setRepos] = useState<NamedRepo[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [name, setName] = useState("");
  const [repo, setRepo] = useState("");
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);
  const nameRef = useRef<HTMLInputElement>(null);

  // A failed list leaves the card empty and loaded, the same answer the picker
  // gives, rather than stuck on its loading state with nothing to say why.
  async function reload() {
    try {
      const r = await listRepos();
      setRepos(r.ok ? (r.repos ?? []) : []);
    } catch {
      setRepos([]);
    } finally {
      setLoaded(true);
    }
  }

  useEffect(() => {
    void reload();
  }, []);

  async function add() {
    if (busy) return;
    setBusy(true);
    try {
      const r = await createRepo({ name: name.trim(), repo: repo.trim(), enabled: true });
      if (!r.ok) {
        push(placementErrorText(t, lang, r, "settings.error"), "fail");
        setShake((n) => n + 1);
        return;
      }
      setName("");
      setRepo("");
      nameRef.current?.focus();
      reposChanged();
      storageLocationsChanged();
      await reload();
    } finally {
      setBusy(false);
    }
  }

  const rows = repos.flatMap((r) => {
    const location = r.companionOf === "" ? locations.find((l) => l.id === `repo:${r.id}`) : undefined;
    return location ? [{ repo: r, location }] : [];
  });

  return (
    <Card
      title={t("repos.title")}
      hint={t("repos.intro")}
      hueIndex={hueIndex}
      foot={
        <Button
          key={shake}
          label={t("repos.add")}
          labelKey="repos.add"
          tone="accent"
          onClick={() => void add()}
          disabled={busy || name.trim() === "" || repo.trim() === ""}
        />
      }
    >
      <div className="flex flex-col gap-3">
        {loaded && rows.length === 0 && <p className="text-xs text-carbon-textMuted">{t("repos.empty")}</p>}

        {rows.length > 0 && (
          <ul className="flex flex-col divide-y divide-carbon-border/60">
            {rows.map(({ repo: r, location }) => (
              <LocationRow key={r.id} location={location} note={r.companionLost ? `${keptText(t, lang, r)} · ${t("repos.companionLost")}` : keptText(t, lang, r)} />
            ))}
          </ul>
        )}

        <div className="flex flex-col gap-2 pt-1">
          <Rows>
            <SettingRow label={t("repos.name")} htmlFor="repo-name">
              <input
                id="repo-name"
                ref={nameRef}
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t("repos.namePlaceholder")}
                className="w-72 max-w-full rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
              />
            </SettingRow>
            <SettingRow label={t("repos.location")} hint={t("repos.locationHint")} htmlFor="repo-location">
              <input
                id="repo-location"
                type="text"
                value={repo}
                onChange={(e) => setRepo(e.target.value)}
                placeholder={t("repos.locationPlaceholder")}
                dir="ltr"
                spellCheck={false}
                className="w-72 max-w-full rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus text-start"
              />
            </SettingRow>
          </Rows>
        </div>
      </div>
    </Card>
  );
}
