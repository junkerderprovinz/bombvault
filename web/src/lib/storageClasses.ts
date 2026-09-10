/** The restore-readable S3 storage classes, in the order the pickers offer them.
 *
 *  One list for the whole app. It stood written out three times — the shared
 *  cloud credentials, a credential set, and an off-site target — and a
 *  whitelist copied three times is a whitelist that will be extended twice.
 *  The server keeps the authoritative copy (restic.AllowedStorageClasses) and
 *  refuses anything else on save; this is what the interface offers.
 *
 *  Deep-archive tiers (Glacier Flexible Retrieval, Deep Archive) are missing on
 *  purpose: they break `restic restore`, so a backup written to one is not a
 *  backup you can get back.
 */
export const STORAGE_CLASSES = [
  "STANDARD",
  "STANDARD_IA",
  "ONEZONE_IA",
  "INTELLIGENT_TIERING",
  "GLACIER_IR",
] as const;
