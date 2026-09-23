// A muted, oversized copy of the page's own nav icon above an empty-state
// message, so the empty state echoes the tab it is on.
export function EmptyStateIcon({ icon: Icon }: { icon: React.ComponentType }) {
  return (
    <div className="text-carbon-textMuted opacity-40 [&_svg]:h-10 [&_svg]:w-10">
      <Icon />
    </div>
  );
}
