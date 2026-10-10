// How a saved destination is reached, as far as the server shows it, and the
// patch that changes it.

import type { BackendOption, DestinationConnectionEdit, Provider, RcloneSection } from "./api";

/** What signs a location in, without the secrets. */
export interface StoredSignIn {
  s3KeyId?: string;
  s3Region?: string;
  restUser?: string;
}

export interface StoredConnection {
  /** The form's fields whose values the server shows. */
  values: Record<string, string>;
  /** The fields that hold a value the server keeps to itself. */
  hidden: string[];
  /** The bucket or folder, "" where the address has none. */
  dir: string;
}

/**
 * storedConnection reads a destination's connection back into the fields its
 * provider's form has. An rclone remote gives them from its section of the
 * config, an S3 service and a rest-server from the address and the
 * credentials the destination signs in with.
 */
export function storedConnection(
  provider: Provider,
  where: string,
  signIn: StoredSignIn,
  section: RcloneSection | undefined
): StoredConnection {
  switch (provider.route) {
    case "path":
      return { values: { path: where }, hidden: [], dir: "" };
    case "rest":
      return {
        values: { url: where.replace(/^rest:/, ""), user: signIn.restUser ?? "" },
        hidden: ["pass"],
        dir: "",
      };
    case "s3": {
      const [, endpoint = "", dir = ""] = /^s3:((?:https?:\/\/)?[^/]+)\/?(.*)$/.exec(where) ?? [];
      return {
        values: { endpoint, region: signIn.s3Region ?? "", access_key_id: signIn.s3KeyId ?? "" },
        hidden: ["secret_access_key"],
        dir,
      };
    }
    case "rclone": {
      const options = section?.options ?? [];
      return {
        values: Object.fromEntries(options.filter((o) => o.value !== undefined).map((o) => [o.name, o.value ?? ""])),
        hidden: options.filter((o) => o.hidden).map((o) => o.name),
        dir: where.replace(/^rclone:[^:]*:/, ""),
      };
    }
  }
}

/**
 * connectionPatch is what a filled-in form changes. A field left as stored is
 * left out, and a secret is sent only when one was typed: an empty one keeps
 * the stored secret.
 */
export function connectionPatch(
  fields: BackendOption[],
  secret: (option: BackendOption) => boolean,
  stored: StoredConnection,
  typed: Record<string, string>,
  dir: string
): DestinationConnectionEdit {
  const settings: Record<string, string> = {};
  for (const field of fields) {
    const value = typed[field.name];
    if (value === undefined) continue;
    if (secret(field) ? value !== "" : value.trim() !== (stored.values[field.name] ?? "")) settings[field.name] = value;
  }
  const edit: DestinationConnectionEdit = {};
  if (Object.keys(settings).length > 0) edit.settings = settings;
  if (dir.trim().replace(/^\/+|\/+$/g, "") !== stored.dir) edit.dir = dir.trim();
  return edit;
}
