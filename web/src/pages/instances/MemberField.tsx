// MemberField picks the instance a receiver or pull source belongs to, from
// the members of this instance's pairing group, and offers the repository
// locations that member reports so nobody has to copy one across by hand.
// The member's restic password never reaches the browser: the server fetches
// it over the group when the form is saved.
import { useEffect, useState } from "react";
import { SelectField } from "../../components/SelectField";
import { getGroup, memberRepos, type GroupMember, type MemberRepo } from "../../lib/api";
import type { useT } from "../../lib/i18n";

type T = ReturnType<typeof useT>["t"];

const inputCls = "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus";

export function MemberField({
  t,
  label,
  value,
  onChange,
  keepOption,
  onPickLocation,
}: {
  t: T;
  label: string;
  /** The chosen member id, or "" for none or keep. */
  value: string;
  onChange: (id: string) => void;
  /** Offers "keep the current pairing" as the empty choice, for editing a
   *  row that is already paired. */
  keepOption: boolean;
  onPickLocation: (location: string) => void;
}) {
  const [members, setMembers] = useState<GroupMember[] | null>(null);
  const [repos, setRepos] = useState<MemberRepo[]>([]);

  useEffect(() => {
    getGroup()
      .then((g) => setMembers(g.ok ? g.members : []))
      .catch(() => setMembers([]));
  }, []);

  useEffect(() => {
    if (value === "") {
      setRepos([]);
      return;
    }
    let live = true;
    memberRepos(value)
      .then((res) => {
        if (live) setRepos(res.ok ? (res.repos ?? []) : []);
      })
      .catch(() => {
        if (live) setRepos([]);
      });
    return () => {
      live = false;
    };
  }, [value]);

  if (members === null) return null;

  const options = [
    { value: "", label: keepOption ? t("pairing.memberKeep") : t("pairing.memberChoose") },
    ...members.map((m) => ({
      value: m.id,
      label: `${m.name || m.id} (${m.direct ? t("pairing.direct") : t("pairing.viaRelay")})`,
    })),
  ];

  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-xs text-carbon-textSub">{label}</label>
      {members.length === 0 && !keepOption ? (
        <p className="text-caption text-carbon-textMuted">{t("pairing.memberNone")}</p>
      ) : (
        <SelectField value={value} onChange={onChange} label={label} options={options} className={inputCls} />
      )}
      <p className="text-caption text-carbon-textMuted">{t("pairing.memberHint")}</p>
      {repos.length > 0 && (
        <div className="flex flex-col gap-1">
          <span className="text-caption text-carbon-textSub">{t("pairing.memberRepos")}</span>
          <ul className="flex flex-col gap-1">
            {repos.map((r, i) => (
              <li key={i}>
                <button
                  type="button"
                  onClick={() => onPickLocation(r.location)}
                  className="w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-start hover:bg-carbon-surface3 glim-field-focus"
                >
                  <span className="block text-caption text-carbon-textSub">{r.name || r.domain}</span>
                  <span dir="ltr" className="block font-mono text-xs text-carbon-text break-all">
                    {r.location}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
