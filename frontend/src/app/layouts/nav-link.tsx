import { createLink } from '@tanstack/react-router';
import { use, type ComponentProps, type ReactNode } from 'react';

import { cn } from '@/design-system/lib/cn';
import { Tooltip } from '@/design-system/components/tooltip';

import { SidebarCollapsed } from './sidebar-context';

interface NavAnchorProps extends ComponentProps<'a'> {
  icon: ReactNode;
  badge?: ReactNode;
  children: string;
}

function NavAnchor({ icon, badge, children, className, ...props }: NavAnchorProps) {
  const collapsed = use(SidebarCollapsed);
  const anchor = (
    <a
      aria-label={collapsed ? children : undefined}
      className={cn(
        'group relative flex h-9 items-center gap-3 rounded-lg px-2 text-sm font-medium text-fg-muted transition-colors',
        'hover:bg-primary-soft/60 hover:text-ink',
        // The active entry of the university's site: a teal wash with a hairline,
        // a bar at the sidebar's edge and the icon on a teal tile.
        'data-[status=active]:bg-linear-to-r data-[status=active]:from-primary-soft data-[status=active]:to-primary-soft/35',
        'data-[status=active]:font-semibold data-[status=active]:text-ink data-[status=active]:ring-1 data-[status=active]:ring-primary/15 data-[status=active]:ring-inset',
        'before:absolute before:inset-y-2 before:-left-3 before:w-0.75 before:rounded-r-sm before:bg-linear-to-b before:from-primary-500 before:to-primary before:opacity-0 data-[status=active]:before:opacity-100',
        collapsed && 'justify-center px-0',
        className,
      )}
      {...props}
    >
      <span className="grid size-7 shrink-0 place-items-center rounded-md transition-colors group-hover:text-primary-text group-data-[status=active]:bg-linear-to-br group-data-[status=active]:from-primary group-data-[status=active]:to-primary-500 group-data-[status=active]:text-white group-data-[status=active]:shadow-sm [&_svg]:size-4">
        {icon}
      </span>
      {!collapsed && <span className="flex-1 truncate">{children}</span>}
      {!collapsed && badge}
    </a>
  );
  return collapsed ? (
    <Tooltip content={children} side="right">
      {anchor}
    </Tooltip>
  ) : (
    anchor
  );
}

/** Sidebar entry with a type-checked route (`to`) and active styling. */
export const NavLink = createLink(NavAnchor);

export function NavSection({ label, children }: { label?: string; children: ReactNode }) {
  const collapsed = use(SidebarCollapsed);
  return (
    <div className="flex flex-col gap-0.5">
      {label && !collapsed && (
        <p className="px-2.5 pt-4 pb-1.5 text-3xs font-bold tracking-label text-fg-subtle uppercase">
          {label}
        </p>
      )}
      {label && collapsed && <span aria-hidden className="mx-3 my-2 h-px bg-border" />}
      {children}
    </div>
  );
}
