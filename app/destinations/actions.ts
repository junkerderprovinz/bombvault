"use server";

import { revalidatePath } from "next/cache";
import { getDb } from "../../server/db";
import { getConfig } from "../../lib/config";
import { createRepo } from "../../lib/backup-repo";
import { initRepo } from "../../lib/restic";
import { parseDestinationForm } from "./validate";

/**
 * Server action: validate form data, persist the destination (password encrypted
 * at rest), then initialise the restic repo — tolerating "already initialized".
 */
export async function saveDestinationAction(formData: FormData): Promise<void> {
  const raw = {
    name: formData.get("name"),
    repoPath: formData.get("repoPath"),
    password: formData.get("password"),
  };

  const { name, repoPath, password } = parseDestinationForm(raw);

  const repo = createRepo(getDb(), getConfig().APP_KEY);
  repo.createDestination({ name, repoPath, password });

  // Initialise the restic repository. Tolerate "already initialized" so saving
  // the same path twice is idempotent.
  try {
    await initRepo(repoPath, password);
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    // restic exits non-zero but prints "already initialized" when the repo
    // exists. Treat that as a success — the important thing is the password works.
    if (!msg.includes("already initialized") && !msg.includes("already exists")) {
      throw err;
    }
  }

  revalidatePath("/destinations");
}
