import { Command } from 'cmdk';
import { CornerDownLeft, Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { cn } from '../lib/cn';

import { useCommandPalette, type PaletteCommand } from './command-palette';
import { Kbd } from './kbd';

/** The palette itself; loaded the first time it opens. */
export function CommandPaletteDialog() {
  const { t } = useTranslation();
  const open = useCommandPalette((s) => s.open);
  const setOpen = useCommandPalette((s) => s.setOpen);
  const commands = useCommandPalette((s) => s.commands);

  const groups = new Map<string, PaletteCommand[]>();
  for (const command of Object.values(commands)) {
    groups.set(command.group, [...(groups.get(command.group) ?? []), command]);
  }

  return (
    <Command.Dialog
      open={open}
      onOpenChange={setOpen}
      label={t('commandPalette.title')}
      overlayClassName="fixed inset-0 z-50 bg-overlay backdrop-blur-[2px] data-[state=open]:animate-fade-in"
      contentClassName={cn(
        'fixed top-[12vh] left-1/2 z-50 w-[min(40rem,calc(100vw-2rem))] -translate-x-1/2 overflow-hidden rounded-2xl border border-border bg-surface shadow-lg',
        'data-[state=open]:animate-scale-in',
      )}
    >
      <div className="flex items-center gap-3 border-b border-border px-4">
        <Search className="size-4.5 shrink-0 text-fg-faint" aria-hidden />
        <Command.Input
          placeholder={t('commandPalette.placeholder')}
          className="h-14 flex-1 bg-transparent text-base text-fg outline-none placeholder:text-fg-subtle"
        />
        <Kbd>Esc</Kbd>
      </div>
      <Command.List className="max-h-[min(60vh,26rem)] overflow-y-auto overscroll-contain p-2">
        <Command.Empty className="px-4 py-10 text-center text-sm text-fg-subtle">
          {t('commandPalette.empty')}
        </Command.Empty>
        {[...groups.entries()].map(([group, items]) => (
          <Command.Group
            key={group}
            heading={group}
            className="[&_[cmdk-group-heading]]:px-3 [&_[cmdk-group-heading]]:pt-3 [&_[cmdk-group-heading]]:pb-1.5 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:font-medium [&_[cmdk-group-heading]]:text-fg-subtle"
          >
            {items.map((command) => (
              <Command.Item
                key={command.id}
                value={`${command.label} ${command.id}`}
                keywords={command.keywords}
                onSelect={() => {
                  setOpen(false);
                  command.perform();
                }}
                className="group flex h-11 cursor-pointer items-center gap-3 rounded-lg px-3 text-sm text-fg data-[selected=true]:bg-primary-soft data-[selected=true]:text-primary-soft-fg [&_svg]:size-4 [&_svg]:shrink-0"
              >
                <span className="text-fg-subtle group-data-[selected=true]:text-primary-soft-fg">
                  {command.icon}
                </span>
                <span className="flex-1 truncate">{command.label}</span>
                {command.hint && <span className="text-xs text-fg-subtle">{command.hint}</span>}
                <CornerDownLeft
                  className="opacity-0 group-data-[selected=true]:opacity-60"
                  aria-hidden
                />
              </Command.Item>
            ))}
          </Command.Group>
        ))}
      </Command.List>
      <div className="flex items-center gap-4 border-t border-border bg-surface-raised/60 px-4 py-2.5 text-xs text-fg-subtle">
        <span className="flex items-center gap-1.5">
          <Kbd>↑</Kbd>
          <Kbd>↓</Kbd>
          {t('commandPalette.hintNavigate')}
        </span>
        <span className="flex items-center gap-1.5">
          <Kbd>↵</Kbd>
          {t('commandPalette.hintSelect')}
        </span>
        <span className="flex items-center gap-1.5">
          <Kbd>Esc</Kbd>
          {t('commandPalette.hintClose')}
        </span>
      </div>
    </Command.Dialog>
  );
}
