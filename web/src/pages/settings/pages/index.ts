import type { ComponentType } from "react";
import type { SettingsPageId } from "../settingsPages";
import { AppsPage } from "./AppsPage";
import { GeneralPage } from "./GeneralPage";
import { InfoPage } from "./InfoPage";
import { InstancesPage } from "./InstancesPage";
import { IntegrationsPage } from "./IntegrationsPage";
import { IntegrityPage } from "./IntegrityPage";
import { LookPage } from "./LookPage";
import { NotificationsPage } from "./NotificationsPage";
import { PairingPage } from "./PairingPage";
import { RestorePage } from "./RestorePage";
import { SchedulesPage } from "./SchedulesPage";
import { SecurityPage } from "./SecurityPage";
import { StoragePage } from "./StoragePage";
import { SystemPage } from "./SystemPage";

/** The component behind each page id. */
export const PAGE_COMPONENTS: Record<SettingsPageId, ComponentType> = {
  general: GeneralPage,
  look: LookPage,
  schedules: SchedulesPage,
  storage: StoragePage,
  restore: RestorePage,
  notifications: NotificationsPage,
  integrity: IntegrityPage,
  security: SecurityPage,
  instances: InstancesPage,
  pairing: PairingPage,
  integrations: IntegrationsPage,
  apps: AppsPage,
  system: SystemPage,
  info: InfoPage,
};
