// Pure report assembly for the host-integration spike. Each Probe returns a
// ProbeResult; assembleReport runs them all, converts any thrown error into a
// failed check, and never throws. This is the unit-tested core — the real probe
// implementations (host-probes.ts) are validated by the user on the real host.
export interface ProbeResult {
  name: string;
  ok: boolean;
  detail?: string;
  error?: string;
}

export type Probe = () => Promise<ProbeResult>;

export interface SpikeReport {
  generatedAt: number;
  overall: boolean;
  checks: ProbeResult[];
}

export async function assembleReport(probes: Probe[]): Promise<SpikeReport> {
  const checks: ProbeResult[] = [];
  for (const probe of probes) {
    try {
      checks.push(await probe());
    } catch (err) {
      checks.push({
        name: "unknown",
        ok: false,
        error: err instanceof Error ? err.message : String(err),
      });
    }
  }
  return {
    generatedAt: Date.now(),
    overall: checks.every((c) => c.ok),
    checks,
  };
}
