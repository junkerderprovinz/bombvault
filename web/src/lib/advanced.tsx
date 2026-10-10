import type { ReactNode } from "react";

export function AdvancedProvider({ children }: { children: ReactNode }) {
  return <>{children}</>;
}

export function useAdvanced() {
  return { advanced: true };
}

export function Advanced({ when = true, children }: { when?: boolean; children: ReactNode }) {
  return when ? <>{children}</> : null;
}
