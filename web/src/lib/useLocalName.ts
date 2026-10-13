import { useContext } from "react";
import { LocationsContext } from "./attention";
import { useT } from "./i18n";
import { localName } from "./storageLocations";

/** useLocalNames names each section's own folder the way the storage pages do. */
export function useLocalNames(): (domain: string) => string {
  const { t } = useT();
  const locations = useContext(LocationsContext);
  return (domain) => localName(t, locations, domain);
}

/** useLocalName is what the storage pages call one section's own folder. */
export function useLocalName(domain: string): string {
  return useLocalNames()(domain);
}
