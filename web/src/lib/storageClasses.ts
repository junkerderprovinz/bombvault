/** The restore-readable S3 storage classes, in the order the pickers offer them.
 *  The server keeps the authoritative list (restic.AllowedStorageClasses) and
 *  refuses anything else on save.
 *
 *  Glacier Flexible Retrieval and Deep Archive are left out because they break
 *  `restic restore`, so a backup written to one cannot be got back.
 */
export const STORAGE_CLASSES = [
  "STANDARD",
  "STANDARD_IA",
  "ONEZONE_IA",
  "INTELLIGENT_TIERING",
  "GLACIER_IR",
] as const;
