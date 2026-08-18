// Empty-state icon — a muted, oversized rendering of the page's own nav icon,
// shown above an empty-state message (Containers/VMs/Files/Fleet/Receiver).
// Reuses the exported Sidebar nav icon components rather than a separate icon
// set, so the empty state visually echoes the tab you're already on.
export function EmptyStateIcon({ icon: Icon }: { icon: React.ComponentType }) {
  return (
    <div className="text-carbon-textMuted opacity-40 [&_svg]:h-10 [&_svg]:w-10">
      <Icon />
    </div>
  );
}
