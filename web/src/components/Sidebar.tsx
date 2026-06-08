import { NavLink } from "react-router-dom";
import type { Settings } from "../lib/api";
import type { useT } from "../lib/i18n";

type T = ReturnType<typeof useT>["t"];

interface SidebarProps {
  t: T;
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

function IconContainers() {
  return (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none" className="shrink-0">
      <rect x="2" y="4" width="16" height="4" rx="1" fill="currentColor" />
      <rect x="2" y="10" width="16" height="4" rx="1" fill="currentColor" opacity=".7" />
      <rect x="2" y="16" width="10" height="2" rx="1" fill="currentColor" opacity=".4" />
    </svg>
  );
}

function IconVM() {
  return (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none" className="shrink-0">
      <rect x="2" y="3" width="16" height="11" rx="1.5" stroke="currentColor" strokeWidth="1.5" fill="none" />
      <path d="M7 17h6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <path d="M10 14v3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <path d="M6 9l2.5-2.5L11 9l3-3" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round" />
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
  return (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none" className="shrink-0">
      <circle cx="10" cy="10" r="2.5" stroke="currentColor" strokeWidth="1.5" />
      <path
        d="M10 2v2M10 16v2M2 10h2M16 10h2M4.22 4.22l1.42 1.42M14.36 14.36l1.42 1.42M4.22 15.78l1.42-1.42M14.36 5.64l1.42-1.42"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
      />
    </svg>
  );
}

const navBase =
  "flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-colors duration-150 select-none";
const navActive =
  "bg-carbon-surface3 text-carbon-text";
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

export function Sidebar({ t, settings }: SidebarProps) {
  const vmsEnabled = settings?.vmsEnabled ?? false;
  const flashEnabled = settings?.flashEnabled ?? false;

  return (
    <aside className="flex flex-col w-56 shrink-0 h-full bg-carbon-surface border-r border-carbon-border">
      {/* Logo / brand */}
      <div className="flex items-center gap-2.5 px-4 py-5 border-b border-carbon-border">
        <div className="w-7 h-7 rounded-lg bg-carbon-surface3 flex items-center justify-center">
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
            <circle cx="8" cy="8" r="5" stroke="#f4f4f4" strokeWidth="1.5" />
            <path d="M8 5v3l2 1.5" stroke="#f4f4f4" strokeWidth="1.3" strokeLinecap="round" />
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
          to="/vms"
          label={t("nav.vms")}
          icon={<IconVM />}
          disabled={!vmsEnabled}
          comingSoon
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
