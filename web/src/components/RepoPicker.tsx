import { useEffect, useState } from "react";
import { listRepos, type NamedRepo } from "../lib/api";
import { useT } from "../lib/i18n";
import { offQualifier } from "../lib/placement";
import { placeErrorText } from "../lib/placeText";
import { ensurePlaceRepo, listPlaces, type Place } from "../lib/places";
import { reposChanged } from "../lib/useNamedRepos";
import { useToast } from "../lib/toast";
import { InfoBubble } from "./InfoBubble";
import { SelectField } from "./SelectField";

// RepoPicker chooses the repository a ZFS item's backups go to. Containers,
// VMs and folder sets choose theirs in the placement row. The empty value, the
// domain repository, is named, because a blank entry reads as an unanswered
// question.
//
// Beside the repositories it offers every place with a folder for ZFS
// datasets that has no repository there yet; picking one makes it. A
// repository a place made for another domain is left out, since its folder
// belongs to that domain.
//
// Locked once the item has backups: they stay where they were written, so
// pointing the item elsewhere would split its history. The server refuses the
// change as well.
export function RepoPicker({
  value,
  onChange,
  locked = false,
  disabled = false,
}: {
  /** The chosen repository's id, or "" for the domain repository. */
  value: string;
  onChange: (next: string) => void;
  /** True once the item has backups: the choice is frozen with a reason. */
  locked?: boolean;
  disabled?: boolean;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const [repos, setRepos] = useState<NamedRepo[]>([]);
  const [places, setPlaces] = useState<Place[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [making, setMaking] = useState(false);

  useEffect(() => {
    let alive = true;
    Promise.all([listRepos(), listPlaces().catch(() => null)])
      .then(([r, p]) => {
        if (!alive) return;
        setRepos(r.ok ? (r.repos ?? []) : []);
        setPlaces(p?.ok ? (p.places ?? []) : []);
        setLoaded(true);
      })
      .catch(() => {
        if (alive) setLoaded(true);
      });
    return () => {
      alive = false;
    };
  }, []);

  const forZFS = (r: NamedRepo) => !r.placeId || r.placeDomain === "" || r.placeDomain === "zfs";
  const toMake = places.filter(
    (p) =>
      p.enabled &&
      "zfs" in p.folders &&
      !p.usage.homeDomains.includes("zfs") &&
      !repos.some((r) => r.placeId === p.id && forZFS(r))
  );

  // A switched-off repository cannot be picked, but the one already chosen
  // stays listed, or the item would appear to be on the domain repository.
  const options = [
    { value: "", label: t("zfs.repoPlaceholder") },
    ...repos
      .filter((r) => (r.enabled && forZFS(r)) || r.id === value)
      .map((r) => ({
        value: r.id,
        // "off", not "not in use": this item may well be on it.
        label: r.enabled ? r.name : `${r.name} (${offQualifier(t)})`,
        disabled: !r.enabled && r.id !== value,
      })),
    ...toMake.map((p) => ({ value: `place:${p.id}`, label: p.name, disabled: false })),
  ];
  // A stored id whose repository is gone is listed by its id. The item is not
  // on the domain repository, and its next backup will fail.
  if (value !== "" && !repos.some((r) => r.id === value)) {
    options.push({ value, label: value, disabled: false });
  }

  async function pick(next: string) {
    if (!next.startsWith("place:")) {
      onChange(next);
      return;
    }
    setMaking(true);
    try {
      const res = await ensurePlaceRepo(next.slice("place:".length), "zfs");
      if (!res.ok || res.repoId === undefined) {
        push(placeErrorText(t, lang, res, "settings.error"), "fail");
        return;
      }
      reposChanged();
      const made = await listRepos();
      if (made.ok) setRepos(made.repos ?? []);
      onChange(res.repoId);
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    } finally {
      setMaking(false);
    }
  }

  return (
    <div className="flex flex-col gap-1">
      <label className="flex items-center gap-1 text-xs text-carbon-textSub">
        {t("zfs.repo")}
        <InfoBubble tip={t("zfs.repoPickHint")} />
        {locked && <InfoBubble tip={t("zfs.repoLocked")} />}
      </label>
      <SelectField
        value={value}
        onChange={(next) => void pick(next)}
        options={options}
        label={t("zfs.repo")}
        disabled={disabled || locked || !loaded || making}
        className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus text-start"
      />
    </div>
  );
}
