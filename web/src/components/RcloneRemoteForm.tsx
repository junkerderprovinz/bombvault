import { useState } from "react";

import { addRcloneRemote, type RcloneRemoteForm as Form } from "../lib/api";
import { useT } from "../lib/i18n";
import { useToast } from "../lib/toast";

import { Button } from "./Button";
import { SelectField } from "./SelectField";

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

  const field = "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full glim-field-focus";

  return (
    <div className="flex flex-col gap-3">
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <label className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textSub">{t("rcloneRemote.type")}</span>
          <SelectField
            value={type}
            label={t("rcloneRemote.type")}
            options={[
              { value: "smb" as const, label: t("rcloneRemote.typeSmb") },
              { value: "webdav" as const, label: t("rcloneRemote.typeWebdav") },
            ]}
            onChange={(next) => setType(next)}
            className={field}
          />
        </label>
        <label className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textSub">{t("rcloneRemote.name")}</span>
          <input className={field} value={name} onChange={(e) => setName(e.target.value)} />
        </label>

        {type === "smb" ? (
          <>
            <label className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("rcloneRemote.host")}</span>
              <input className={field} value={host} onChange={(e) => setHost(e.target.value)} />
            </label>
            <label className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("rcloneRemote.share")}</span>
              <input className={field} value={share} onChange={(e) => setShare(e.target.value)} />
            </label>
          </>
        ) : (
          <>
            <label className="flex flex-col gap-1 sm:col-span-2">
              <span className="text-xs text-carbon-textSub">{t("rcloneRemote.url")}</span>
              <input className={field} value={url} onChange={(e) => setUrl(e.target.value)} />
            </label>
            <label className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("rcloneRemote.vendor")}</span>
              <SelectField
                value={vendor}
                label={t("rcloneRemote.vendor")}
                options={[
                  { value: "nextcloud", label: "Nextcloud" },
                  { value: "owncloud", label: "ownCloud" },
                  { value: "sharepoint", label: "SharePoint" },
                  { value: "other", label: t("rcloneRemote.vendorOther") },
                ]}
                onChange={(next) => setVendor(next)}
                className={field}
              />
            </label>
          </>
        )}

        <label className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textSub">{t("rcloneRemote.user")}</span>
          <input className={field} value={user} onChange={(e) => setUser(e.target.value)} />
        </label>
        <label className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textSub">{t("rcloneRemote.password")}</span>
          <input
            className={field}
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </label>
      </div>

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
