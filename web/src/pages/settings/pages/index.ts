import type { ComponentType } from "react";
import type { SettingsPageId } from "../settingsPages";
import { AppsPage } from "./AppsPage";
import { CloudPage } from "./CloudPage";
import { ContainersPage } from "./ContainersPage";
import { GeneralPage } from "./GeneralPage";
import { IntegrationsPage } from "./IntegrationsPage";
import { IntegrityPage } from "./IntegrityPage";
import { LookPage } from "./LookPage";
import { NotificationsPage } from "./NotificationsPage";
import { OffsitePage } from "./OffsitePage";
import { PairingPage } from "./PairingPage";
import { RetentionPage } from "./RetentionPage";
import { SchedulesPage } from "./SchedulesPage";
import { SecurityPage } from "./SecurityPage";
import { StoragePage } from "./StoragePage";
import { SystemPage } from "./SystemPage";

/** The component behind each page id. */
export const PAGE_COMPONENTS: Record<SettingsPageId, ComponentType> = {
  general: GeneralPage,
  look: LookPage,
  storage: StoragePage,
  retention: RetentionPage,
  schedules: SchedulesPage,
  containers: ContainersPage,
  offsite: OffsitePage,
  cloud: CloudPage,
  notifications: NotificationsPage,
  integrity: IntegrityPage,
  security: SecurityPage,
  pairing: PairingPage,
  integrations: IntegrationsPage,
  apps: AppsPage,
  system: SystemPage,
};
