import { assembleReport } from "../server/spike-report";
import { DEFAULT_PROBES } from "../server/host-probes";

// `npm run spike` — run the host-integration probes and print a report. This is
// the REAL-HOST validation artifact: run it inside the container on the real
// Unraid box. It degrades gracefully and exits 0 even when checks fail (a failed
// check is a finding, not a crash); it only exits non-zero on an internal error.
async function main(): Promise<void> {
  const report = await assembleReport(DEFAULT_PROBES);
  // eslint-disable-next-line no-console
  console.log(`BombVault host spike — overall: ${report.overall ? "ALL OK" : "DEGRADED"}`);
  for (const c of report.checks) {
    const status = c.ok ? "OK  " : "FAIL";
    // eslint-disable-next-line no-console
    console.log(`  [${status}] ${c.name}: ${c.ok ? c.detail ?? "" : c.error ?? ""}`);
  }
}

main().catch((err) => {
  // eslint-disable-next-line no-console
  console.error("spike: internal error", err);
  process.exit(1);
});
