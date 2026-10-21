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
        'group relative flex h-9 items-center gap-3 rounded-lg px-2.5 text-sm font-medium text-fg-muted transition-colors',
        'hover:bg-surface-raised hover:text-fg',
        'data-[status=active]:bg-primary-soft data-[status=active]:text-primary-soft-fg',
        collapsed && 'justify-center px-0',
        className,
      )}
      {...props}
    >
      <span className="flex shrink-0 [&_svg]:size-4.5">{icon}</span>
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
        <p className="px-2.5 pt-3 pb-1.5 text-xs font-medium text-fg-subtle">{label}</p>
      )}
      {label && collapsed && <span aria-hidden className="mx-3 my-2 h-px bg-border" />}
      {children}
    </div>
  );
}
