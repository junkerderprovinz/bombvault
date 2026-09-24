import { useEffect, useState } from "react";
import { listRepos, type NamedRepo } from "../lib/api";
import { useT } from "../lib/i18n";
import { InfoBubble } from "./InfoBubble";
import { SelectField } from "./SelectField";

// RepoPicker chooses the repository an item's backups go to, for containers,
// VMs and folder sets alike. Repositories are defined once in Settings and
// picked here. The empty value, the domain repository, is named, because a
// blank entry reads as an unanswered question.
//
// Locked once the item has backups: they stay where they were written, so
// pointing the item elsewhere would split its history. The server refuses the
// change as well.
export function RepoPicker({
  value,
  onChange,
  locked = false,
  hintKey = "repos.itemHint",
  labelKey = "repos.itemLabel",
  defaultLabelKey = "repos.itemDefault",
  lockedKey = "repos.itemLocked",
  disabled = false,
}: {
  /** The chosen repository's id, or "" for the domain repository. */
  value: string;
  onChange: (next: string) => void;
  /** True once the item has backups: the choice is frozen with a reason. */
  locked?: boolean;
  hintKey?: "repos.itemHint" | "files.repoHint" | "zfs.repoHint";
  labelKey?: "repos.itemLabel" | "files.repo" | "zfs.repo";
  defaultLabelKey?: "repos.itemDefault" | "files.repoPlaceholder" | "zfs.repoPlaceholder";
  /** Why the choice is frozen. A folder set says it in its own words ("delete
   *  this set's backups first, or create a new set"), which is more use than
   *  the generic sentence. */
  lockedKey?: "repos.itemLocked" | "files.repoLocked" | "zfs.repoLocked";
  disabled?: boolean;
}) {
  const { t } = useT();
  const [repos, setRepos] = useState<NamedRepo[]>([]);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    let alive = true;
    listRepos()
      .then((r) => {
        if (!alive) return;
        setRepos(r.ok ? (r.repos ?? []) : []);
        setLoaded(true);
      })
      .catch(() => {
        if (alive) setLoaded(true);
      });
    return () => {
      alive = false;
    };
  }, []);

  // A switched-off repository cannot be picked, but the one already chosen
  // stays listed, or the item would appear to be on the domain repository.
  const options = [
    { value: "", label: t(defaultLabelKey) },
    ...repos
      .filter((r) => r.enabled || r.id === value)
      .map((r) => ({
        value: r.id,
        // "off", not "not in use": this item may well be on it.
        label: r.enabled ? r.name : `${r.name} (${t("repos.off")})`,
        disabled: !r.enabled && r.id !== value,
      })),
  ];
  // A stored id whose repository is gone is listed by its id. The item is not
  // on the domain repository, and its next backup will fail.
  if (value !== "" && !repos.some((r) => r.id === value)) {
    options.push({ value, label: value, disabled: false });
  }

  return (
    <div className="flex flex-col gap-1">
      <label className="flex items-center gap-1 text-xs text-carbon-textSub">
        {t(labelKey)}
        <InfoBubble tip={t(hintKey)} />
        {locked && <InfoBubble tip={t(lockedKey)} />}
      </label>
      <SelectField
        value={value}
        onChange={onChange}
        options={options}
        label={t(labelKey)}
        disabled={disabled || locked || !loaded}
        className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus text-start"
      />
    </div>
  );
}
