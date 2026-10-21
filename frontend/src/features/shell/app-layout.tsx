import { useQuery } from '@tanstack/react-query';
import { getRouteApi, Link, Outlet, useMatches, useNavigate } from '@tanstack/react-router';
import {
  ChevronsUpDown,
  FileUp,
  Home,
  LogOut,
  ShieldCheck,
  User,
  Users,
  UsersRound,
} from 'lucide-react';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';

import { AppShell } from '@/app/layouts/app-shell';
import { NavLink, NavSection } from '@/app/layouts/nav-link';
import {
  Avatar,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  LanguageSwitcher,
  ThemeToggle,
  TruncatedText,
  useRegisterCommands,
  type PaletteCommand,
} from '@/design-system';
import { hasRole, sessionQuery, useSignOut, type Session } from '@/features/auth/api';
import { useLocalized } from '@/shared/i18n/localized';

import { primaryRole } from './roles';

/** Signed-in frame: navigation by role, the user menu and the page title. */
export function AppLayout({ session: initial }: { session: Session }) {
  const live = useQuery(sessionQuery);
  const session = live.data ?? initial;
  useAppCommands(session);
  return (
    <AppShell
      brand={<Brand session={session} />}
      nav={<AppNav session={session} />}
      user={<UserMenu session={session} />}
      topbar={<PageTitle />}
      actions={
        <>
          <LanguageSwitcher />
          <ThemeToggle />
        </>
      }
    >
      <Outlet />
    </AppShell>
  );
}

function Brand({ session }: { session: Session }) {
  const { t } = useTranslation('common');
  const text = useLocalized();
  return (
    <Link to="/app" className="flex min-w-0 flex-col rounded-md leading-tight">
      <span className="font-display text-base font-semibold text-fg">{t('app.name')}</span>
      <TruncatedText className="text-xs text-fg-subtle">
        {text(session.tenant.short_name) || text(session.tenant.name)}
      </TruncatedText>
    </Link>
  );
}

function AppNav({ session }: { session: Session }) {
  const { t } = useTranslation('app');
  const admin = hasRole(session, 'tenant_admin');
  const manager = hasRole(session, 'innovation_manager');
  return (
    <>
      <NavSection>
        <NavLink to="/app" activeOptions={{ exact: true }} icon={<Home />}>
          {t('nav.home')}
        </NavLink>
        <NavLink to="/app/profile" icon={<User />}>
          {t('nav.profile')}
        </NavLink>
        <NavLink to="/app/team" icon={<UsersRound />}>
          {t('nav.team')}
        </NavLink>
      </NavSection>
      {(admin || manager) && (
        <NavSection label={t('nav.administration')}>
          <NavLink to="/app/admin/members" icon={<Users />}>
            {t('nav.members')}
          </NavLink>
          {admin && (
            <NavLink to="/app/admin/import" icon={<FileUp />}>
              {t('nav.import')}
            </NavLink>
          )}
        </NavSection>
      )}
      <NavSection label={t('nav.settings')}>
        <NavLink to="/app/settings/security" icon={<ShieldCheck />}>
          {t('nav.security')}
        </NavLink>
      </NavSection>
    </>
  );
}

function PageTitle() {
  const { t } = useTranslation('app');
  const matches = useMatches();
  const titleKey = matches.findLast((match) => match.staticData.titleKey)?.staticData.titleKey;
  if (!titleKey) return null;
  return (
    <TruncatedText as="p" className="font-semibold text-fg">
      {t(`titles.${titleKey}`)}
    </TruncatedText>
  );
}

function UserMenu({ session }: { session: Session }) {
  const { t } = useTranslation(['app', 'common']);
  const navigate = useNavigate();
  const signOut = useSignOut();
  const role = primaryRole(session.membership.roles);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className="flex w-full items-center gap-3 rounded-lg p-1.5 text-left transition-colors hover:bg-surface-raised"
          aria-label={t('app:user.menu')}
        >
          <Avatar name={session.user.full_name} src={session.user.avatar_url} size="sm" />
          <span className="flex min-w-0 flex-1 flex-col">
            <TruncatedText className="text-sm font-medium text-fg">
              {session.user.full_name}
            </TruncatedText>
            {role && (
              <TruncatedText className="text-xs text-fg-subtle">
                {t(`common:roles.${role}`)}
              </TruncatedText>
            )}
          </span>
          <ChevronsUpDown className="size-4 text-fg-faint" aria-hidden />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="top" align="start" className="w-60">
        <DropdownMenuLabel className="flex flex-col gap-0.5">
          <span className="text-sm font-medium text-fg">{session.user.full_name}</span>
          <span className="text-xs font-normal break-all text-fg-subtle">{session.user.email}</span>
        </DropdownMenuLabel>
        <DropdownMenuItem
          icon={<User />}
          onSelect={() => {
            void navigate({ to: '/app/profile' });
          }}
        >
          {t('app:nav.profile')}
        </DropdownMenuItem>
        <DropdownMenuItem
          icon={<ShieldCheck />}
          onSelect={() => {
            void navigate({ to: '/app/settings/security' });
          }}
        >
          {t('app:nav.security')}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          icon={<LogOut />}
          onSelect={() => {
            signOut.mutate(undefined, {
              onSettled: () => void navigate({ to: '/login', replace: true }),
            });
          }}
        >
          {t('common:actions.signOut')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/** Pages of the signed-in area in the ⌘K palette. */
function useAppCommands(session: Session) {
  const { t } = useTranslation(['app', 'common']);
  const navigate = useNavigate();
  const admin = hasRole(session, 'tenant_admin');
  const manager = hasRole(session, 'innovation_manager');
  const commands = useMemo<PaletteCommand[]>(() => {
    const group = t('common:commandPalette.groups.navigation');
    const items: PaletteCommand[] = [
      {
        id: 'app.home',
        group,
        label: t('app:nav.home'),
        icon: <Home />,
        perform: () => void navigate({ to: '/app' }),
      },
      {
        id: 'app.profile',
        group,
        label: t('app:nav.profile'),
        icon: <User />,
        perform: () => void navigate({ to: '/app/profile' }),
      },
      {
        id: 'app.team',
        group,
        label: t('app:nav.team'),
        icon: <UsersRound />,
        perform: () => void navigate({ to: '/app/team' }),
      },
      {
        id: 'app.security',
        group,
        label: t('app:nav.security'),
        icon: <ShieldCheck />,
        perform: () => void navigate({ to: '/app/settings/security' }),
      },
    ];
    if (admin || manager) {
      items.push({
        id: 'app.members',
        group,
        label: t('app:nav.members'),
        icon: <Users />,
        perform: () => void navigate({ to: '/app/admin/members' }),
      });
    }
    if (admin) {
      items.push({
        id: 'app.import',
        group,
        label: t('app:nav.import'),
        icon: <FileUp />,
        perform: () => void navigate({ to: '/app/admin/import' }),
      });
    }
    return items;
  }, [t, navigate, admin, manager]);
  useRegisterCommands(commands);
}

const appRoute = getRouteApi('/app');

export function AppRoute() {
  const { session } = appRoute.useRouteContext();
  return <AppLayout session={session} />;
}
