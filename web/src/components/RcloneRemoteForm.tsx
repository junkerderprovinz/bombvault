import { useId, useState } from "react";

import { addRcloneRemote, type RcloneRemoteForm as Form } from "../lib/api";
import { useT } from "../lib/i18n";
import { useToast } from "../lib/toast";

import { Button } from "./Button";
import { SelectField } from "./SelectField";
import { Rows, SettingRow } from "./storage/rows";

type RemoteType = "smb" | "webdav";

/**
 * Adds an SMB or WebDAV destination from a form instead of a hand-written
 * rclone config.
 *
 * BombVault could always reach both, as long as the operator wrote an INI
 * section into the paste box below. The documented alternative is worse than it
 * looks: mounting the share on Unraid and pointing a backup path at it puts the
 * repository on a CIFS mount, which restic's own documentation advises against.
 * So this form is not only the easier route, it is the sounder one.
 *
 * NFS is left out. Neither rclone nor restic has an NFS
 * backend, and a form that cannot work would be worse than telling the user
 * NFS still needs a host mount.
 */
export function RcloneRemoteForm({
  t,
  onAdded,
}: {
  t: ReturnType<typeof useT>["t"];
  /** Called with the finished repository location, so the page can offer it. */
  onAdded?: (location: string) => void;
}) {
  const { push } = useToast();
  const fid = useId();
  const [type, setType] = useState<RemoteType>("smb");
  const [name, setName] = useState("");
  const [host, setHost] = useState("");
  const [share, setShare] = useState("");
  const [url, setUrl] = useState("");
  const [vendor, setVendor] = useState("nextcloud");
  const [user, setUser] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [location, setLocation] = useState("");

  async function submit() {
    setBusy(true);
    setLocation("");
    try {
      const form: Form = { name, type, user, password };
      if (type === "smb") {
        form.host = host;
        form.share = share;
      } else {
        form.url = url;
        form.vendor = vendor;
      }
      const res = await addRcloneRemote(form);
      if (!res.ok) {
        // Server text verbatim by design: the API answers English and is not
        // translated client-side. Its most common message here names the field
        // that is wrong, which is more useful than a generic failure.
        push(res.error ?? t("rcloneRemote.failed"), "fail");
        return;
      }
      // The password is not kept around once it has served its purpose.
      setPassword("");
      setLocation(res.location ?? "");
      if (res.location) onAdded?.(res.location);
    } catch (e) {
      push(e instanceof Error ? e.message : String(e), "fail");
    } finally {
      setBusy(false);
    }
  }

  const text = "w-72 max-w-full rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus";
  const select = "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus";

  return (
    <div className="flex flex-col gap-3">
      <Rows>
        <SettingRow label={t("rcloneRemote.type")} htmlFor={`${fid}-type`}>
          <SelectField
            id={`${fid}-type`}
            value={type}
            label={t("rcloneRemote.type")}
            options={[
              { value: "smb" as const, label: t("rcloneRemote.typeSmb") },
              { value: "webdav" as const, label: t("rcloneRemote.typeWebdav") },
            ]}
            onChange={(next) => setType(next)}
            className={select}
          />
        </SettingRow>
        <SettingRow label={t("rcloneRemote.name")} htmlFor={`${fid}-name`}>
          <input id={`${fid}-name`} className={text} value={name} onChange={(e) => setName(e.target.value)} />
        </SettingRow>

        {type === "smb" ? (
          <>
            <SettingRow label={t("rcloneRemote.host")} htmlFor={`${fid}-host`}>
              <input id={`${fid}-host`} className={text} value={host} onChange={(e) => setHost(e.target.value)} />
            </SettingRow>
            <SettingRow label={t("rcloneRemote.share")} htmlFor={`${fid}-share`}>
              <input id={`${fid}-share`} className={text} value={share} onChange={(e) => setShare(e.target.value)} />
            </SettingRow>
          </>
        ) : (
          <>
            <SettingRow label={t("rcloneRemote.url")} htmlFor={`${fid}-url`}>
              <input id={`${fid}-url`} className={text} value={url} onChange={(e) => setUrl(e.target.value)} />
            </SettingRow>
            <SettingRow label={t("rcloneRemote.vendor")} htmlFor={`${fid}-vendor`}>
              <SelectField
                id={`${fid}-vendor`}
                value={vendor}
                label={t("rcloneRemote.vendor")}
                options={[
                  { value: "nextcloud", label: "Nextcloud" },
                  { value: "owncloud", label: "ownCloud" },
                  { value: "sharepoint", label: "SharePoint" },
                  { value: "other", label: t("rcloneRemote.vendorOther") },
                ]}
                onChange={(next) => setVendor(next)}
                className={select}
              />
            </SettingRow>
          </>
        )}

        <SettingRow label={t("rcloneRemote.user")} htmlFor={`${fid}-user`}>
          <input id={`${fid}-user`} className={text} value={user} onChange={(e) => setUser(e.target.value)} />
        </SettingRow>
        <SettingRow label={t("rcloneRemote.password")} htmlFor={`${fid}-password`}>
          <input
            id={`${fid}-password`}
            className={text}
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </SettingRow>
      </Rows>

      <Button
        label={t("rcloneRemote.add")}
        labelKey="rcloneRemote.add"
        tone="accent"
        busy={busy}
        onClick={() => void submit()}
        className="self-start"
      />

      {location && (
        <div className="rounded-control bg-carbon-surface2 px-3 py-2">
          <p className="text-xs text-carbon-textSub">{t("rcloneRemote.useThisPath")}</p>
          <code className="text-sm text-carbon-text break-all">{location}</code>
        </div>
      )}
    </div>
  );
}
