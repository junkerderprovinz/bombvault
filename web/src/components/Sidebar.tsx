import { NavLink } from "react-router-dom";
import type { Settings } from "../lib/api";
import { useT } from "../lib/i18n";

interface SidebarProps {
  settings: Settings | null;
}

interface NavItem {
  to: string;
  label: string;
  icon: React.ReactNode;
  disabled?: boolean;
  comingSoon?: boolean;
}

// Simple inline SVG icons (monochrome, 20×20)
function IconDashboard() {
  return (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none" className="shrink-0">
      <rect x="2" y="2" width="7" height="7" rx="1.5" fill="currentColor" />
      <rect x="11" y="2" width="7" height="7" rx="1.5" fill="currentColor" opacity=".6" />
      <rect x="2" y="11" width="7" height="7" rx="1.5" fill="currentColor" opacity=".6" />
      <rect x="11" y="11" width="7" height="7" rx="1.5" fill="currentColor" opacity=".4" />
    </svg>
  );
}

// Docker whale mark — Simple Icons path, scaled to 20×20 viewport
function IconContainers() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M13.983 11.078h2.119a.186.186 0 0 0 .186-.185V9.006a.186.186 0 0 0-.186-.186h-2.119a.185.185 0 0 0-.185.185v1.888c0 .102.083.185.185.185m-2.954-5.43h2.118a.186.186 0 0 0 .186-.186V3.574a.186.186 0 0 0-.186-.185h-2.118a.185.185 0 0 0-.185.185v1.888c0 .103.082.185.185.185m0 2.716h2.118a.187.187 0 0 0 .186-.186V6.29a.186.186 0 0 0-.186-.185h-2.118a.185.185 0 0 0-.185.185v1.887c0 .102.082.185.185.185m-2.93 0h2.12a.186.186 0 0 0 .184-.186V6.29a.185.185 0 0 0-.185-.185H8.1a.185.185 0 0 0-.185.185v1.887c0 .102.083.185.185.185m-2.964 0h2.119a.186.186 0 0 0 .185-.186V6.29a.185.185 0 0 0-.185-.185H5.136a.186.186 0 0 0-.186.185v1.887c0 .102.084.185.186.185m5.893 2.715h2.118a.186.186 0 0 0 .186-.185V9.006a.186.186 0 0 0-.186-.186h-2.118a.185.185 0 0 0-.185.185v1.888c0 .102.082.185.185.185m-2.93 0h2.12a.185.185 0 0 0 .184-.185V9.006a.185.185 0 0 0-.184-.186h-2.12a.185.185 0 0 0-.184.185v1.888c0 .102.083.185.185.185m-2.964 0h2.119a.185.185 0 0 0 .185-.185V9.006a.185.185 0 0 0-.184-.186h-2.12a.186.186 0 0 0-.186.185v1.888c0 .102.084.185.186.185m-2.92 0h2.12a.185.185 0 0 0 .184-.185V9.006a.185.185 0 0 0-.184-.186h-2.12a.185.185 0 0 0-.185.185v1.888c0 .102.083.185.185.185M23.763 9.89c-.065-.051-.672-.51-1.954-.51-.338.001-.676.03-1.01.087-.248-1.7-1.653-2.53-1.716-2.566l-.344-.199-.226.327c-.284.438-.49.922-.612 1.43-.23.97-.09 1.882.403 2.661-.595.332-1.55.413-1.744.42H.751a.751.751 0 0 0-.75.75c-.007 1.73.425 3.43 1.25 4.977.892 1.679 2.22 2.922 3.836 3.592 1.973.799 5.146.985 7.325.985 1.815.001 3.626-.19 5.392-.573 2.483-.556 4.649-1.932 6.2-3.967a15.024 15.024 0 0 0 2.203-5.09c.048-.165.087-.336.122-.512.054-.234.086-.473.095-.714a4.81 4.81 0 0 0-.66-2.352" />
    </svg>
  );
}

// Desktop/monitor icon — matches Unraid's "VMs" tab glyph (screen + stand + base)
function IconVM() {
  return (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none" className="shrink-0" aria-hidden="true">
      <rect x="2" y="3" width="16" height="10" rx="1.5" stroke="currentColor" strokeWidth="1.5" />
      <path d="M7 17h6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <path d="M10 13v4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

function IconFlash() {
  return (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none" className="shrink-0">
      <path d="M11 2L4 11h6l-1 7 7-9h-6l1-7z" fill="currentColor" />
    </svg>
  );
}

function IconSettings() {
  // Standard 8-tooth cog/gear — conventional settings symbol
  return (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 0 1-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 0 1 .947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 0 1 2.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 0 1 2.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 0 1 .947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 0 1-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 0 1-2.287-.947zM10 13a3 3 0 1 0 0-6 3 3 0 0 0 0 6z"
      />
    </svg>
  );
}

// Calendar/list icon for Jobs
function IconJobs() {
  return (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none" className="shrink-0" aria-hidden="true">
      <rect x="3" y="4" width="14" height="13" rx="1.5" stroke="currentColor" strokeWidth="1.5" />
      <path d="M7 2v4M13 2v4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <path d="M3 8h14" stroke="currentColor" strokeWidth="1.5" />
      <path d="M6 12h8M6 15h5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

const navBase =
  "flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-colors duration-150 select-none";
const navActive =
  "bg-accent text-accentContrast";
const navInactive =
  "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text";
const navDisabled =
  "text-carbon-textMuted cursor-default opacity-50";

function NavItem({ to, label, icon, disabled, comingSoon }: NavItem) {
  if (disabled) {
    return (
      <div className="relative group">
        <div className={`${navBase} ${navDisabled}`}>
          {icon}
          <span>{label}</span>
        </div>
        {comingSoon && (
          <div className="absolute left-full ml-2 top-1/2 -translate-y-1/2 z-50 whitespace-nowrap rounded-md bg-carbon-surface2 border border-carbon-border px-2 py-1 text-xs text-carbon-textSub opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none">
            Coming soon
          </div>
        )}
      </div>
    );
  }
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        `${navBase} ${isActive ? navActive : navInactive}`
      }
    >
      {icon}
      <span>{label}</span>
    </NavLink>
  );
}

export function Sidebar({ settings }: SidebarProps) {
  const { t } = useT();
  const flashEnabled = settings?.flashEnabled ?? false;

  return (
    <aside className="flex flex-col w-56 shrink-0 h-full bg-carbon-surface border-r border-carbon-border">
      {/* Logo / brand */}
      <div className="flex items-center gap-2.5 px-4 py-5 border-b border-carbon-border">
        <div className="w-7 h-7 rounded-lg bg-carbon-surface3 text-carbon-text flex items-center justify-center">
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
            <circle cx="8" cy="8" r="5" stroke="currentColor" strokeWidth="1.5" />
            <path d="M8 5v3l2 1.5" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" />
          </svg>
        </div>
        <span className="text-carbon-text font-semibold text-sm tracking-wide">
          BombVault
        </span>
      </div>

      {/* Navigation */}
      <nav className="flex flex-col gap-1 p-3 flex-1">
        <NavItem
          to="/dashboard"
          label={t("nav.dashboard")}
          icon={<IconDashboard />}
        />
        <NavItem
          to="/containers"
          label={t("nav.containers")}
          icon={<IconContainers />}
        />
        <NavItem
          to="/jobs"
          label={t("nav.jobs")}
          icon={<IconJobs />}
        />
        <NavItem
          to="/vms"
          label={t("nav.vms")}
          icon={<IconVM />}
        />
        <NavItem
          to="/flash"
          label={t("nav.flash")}
          icon={<IconFlash />}
          disabled={!flashEnabled}
          comingSoon
        />

        <div className="mt-auto pt-3 border-t border-carbon-border">
          <NavItem
            to="/settings"
            label={t("nav.settings")}
            icon={<IconSettings />}
          />
        </div>
      </nav>
    </aside>
  );
}
