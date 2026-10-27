import type { ReactNode } from 'react';

import { Button, type ButtonProps } from './button';
import { Tooltip } from './tooltip';

interface IconButtonProps extends Omit<
  ButtonProps,
  'children' | 'leftIcon' | 'rightIcon' | 'size'
> {
  /** Accessible name, also shown as the tooltip. */
  label: string;
  icon: ReactNode;
  size?: 'sm' | 'md';
  shortcut?: string[];
  tooltipSide?: 'top' | 'right' | 'bottom' | 'left';
}

/** Icon-only button: always named for screen readers and explained by a tooltip. */
export function IconButton({
  label,
  icon,
  size = 'md',
  variant = 'ghost',
  shortcut,
  tooltipSide,
  ...props
}: IconButtonProps) {
  return (
    <Tooltip content={label} shortcut={shortcut} side={tooltipSide}>
      <Button
        aria-label={label}
        variant={variant}
        size={size === 'sm' ? 'icon-sm' : 'icon'}
        {...props}
      >
        {icon}
      </Button>
    </Tooltip>
  );
}
