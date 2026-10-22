import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { Ban, CircleCheck, MoreHorizontal, Search, UserCog } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Avatar,
  Badge,
  Button,
  Callout,
  Checkbox,
  ConfirmDialog,
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  EmptyState,
  IconButton,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  toast,
} from '@/design-system';
import { FormAlert } from '@/features/auth/components';
import type { Role, Session } from '@/features/auth/api';
import { facultiesQuery } from '@/features/structure/api';
import { errorMessage } from '@/shared/api/errors';
import { useDebouncedValue } from '@/shared/hooks/use-debounced-value';
import { useFormatters } from '@/shared/i18n/format';
import { useLocalized } from '@/shared/i18n/localized';
import { compact } from '@/shared/router/search';

import { isRole, membersQuery, ROLES, useUpdateMember, type Member } from './api';

const ANY = 'any';

const STATUS_TONE = { active: 'success', invited: 'info', suspended: 'danger' } as const;

/** Members of the university: search, roles and access. */
export function MembersPage({ session }: { session: Session }) {
  const { t } = useTranslation(['admin', 'common']);
  const canEdit = session.membership.roles.includes('tenant_admin');
  const [query, setQuery] = useState('');
  const [role, setRole] = useState<Role | undefined>();
  const [faculty, setFaculty] = useState<string | undefined>();
  const q = useDebouncedValue(query.trim(), 300);
  const faculties = useQuery(facultiesQuery);
  const text = useLocalized();
  const members = useInfiniteQuery(
    membersQuery(compact({ q: q || undefined, role, faculty_id: faculty })),
  );
  const items = members.data?.pages.flatMap((page) => page.items) ?? [];
  const filtered = q !== '' || role !== undefined || faculty !== undefined;

  const [editing, setEditing] = useState<Member | null>(null);
  const [toggling, setToggling] = useState<Member | null>(null);

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-1">
        <h1 className="font-display text-3xl font-bold text-ink">{t('members.title')}</h1>
        <p className="text-fg-muted">
          {canEdit ? t('members.subtitle') : t('members.subtitleReadOnly')}
        </p>
      </header>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-[minmax(0,2fr)_minmax(0,1fr)_minmax(0,1fr)]">
        <Input
          type="search"
          value={query}
          leading={<Search />}
          maxLength={120}
          aria-label={t('members.search')}
          placeholder={t('members.searchPlaceholder')}
          onChange={(event) => {
            setQuery(event.target.value);
          }}
        />
        <Select
          value={role ?? ANY}
          onValueChange={(value) => {
            setRole(isRole(value) ? value : undefined);
          }}
        >
          <SelectTrigger aria-label={t('members.role')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ANY}>{t('members.anyRole')}</SelectItem>
            {ROLES.map((item) => (
              <SelectItem key={item} value={item}>
                {t(`common:roles.${item}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select
          value={faculty ?? ANY}
          onValueChange={(value) => {
            setFaculty(value === ANY ? undefined : value);
          }}
        >
          <SelectTrigger aria-label={t('members.faculty')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ANY}>{t('members.anyFaculty')}</SelectItem>
            {(faculties.data ?? []).map((item) => (
              <SelectItem key={item.id} value={item.id}>
                {text(item.name)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {members.isError && (
        <Callout
          tone="danger"
          action={
            <Button size="sm" variant="secondary" onClick={() => void members.refetch()}>
              {t('common:actions.retry')}
            </Button>
          }
        >
          {errorMessage(members.error)}
        </Callout>
      )}
      {members.isPending && (
        <div className="flex flex-col gap-2" aria-busy>
          {Array.from({ length: 6 }, (_, index) => (
            <Skeleton key={index} className="h-14 w-full" />
          ))}
        </div>
      )}
      {members.isSuccess && items.length === 0 && (
        <EmptyState
          illustration={filtered ? 'search' : 'team'}
          title={filtered ? t('members.emptyFiltered') : t('members.empty')}
          description={filtered ? t('members.emptyFilteredText') : t('members.emptyText')}
        />
      )}
      {items.length > 0 && (
        <Table label={t('members.table')}>
          <TableHeader>
            <TableRow>
              <TableHead>{t('members.columns.member')}</TableHead>
              <TableHead>{t('members.columns.roles')}</TableHead>
              <TableHead>{t('members.columns.study')}</TableHead>
              <TableHead>{t('members.columns.status')}</TableHead>
              <TableHead>{t('members.columns.lastActive')}</TableHead>
              {canEdit && (
                <TableHead>
                  <span className="sr-only">{t('members.columns.actions')}</span>
                </TableHead>
              )}
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map((member) => (
              <MemberRow
                key={member.user.id}
                member={member}
                canEdit={canEdit}
                self={member.user.id === session.user.id}
                onEditRoles={setEditing}
                onToggleStatus={setToggling}
              />
            ))}
          </TableBody>
        </Table>
      )}
      {members.hasNextPage && (
        <Button
          variant="secondary"
          className="self-center"
          loading={members.isFetchingNextPage}
          onClick={() => void members.fetchNextPage()}
        >
          {t('common:actions.loadMore')}
        </Button>
      )}

      <RolesDialog
        member={editing}
        self={editing?.user.id === session.user.id}
        onOpenChange={(open) => {
          if (!open) setEditing(null);
        }}
      />
      <StatusDialog
        member={toggling}
        onOpenChange={(open) => {
          if (!open) setToggling(null);
        }}
      />
    </div>
  );
}

function MemberRow({
  member,
  canEdit,
  self,
  onEditRoles,
  onToggleStatus,
}: {
  member: Member;
  canEdit: boolean;
  self: boolean;
  onEditRoles: (member: Member) => void;
  onToggleStatus: (member: Member) => void;
}) {
  const { t } = useTranslation(['admin', 'common']);
  const format = useFormatters();
  const text = useLocalized();
  const study = [
    member.user.faculty ? text(member.user.faculty.name) : null,
    member.group?.code ?? null,
    member.course ? t('members.course', { course: member.course }) : null,
  ].filter(Boolean);

  return (
    <TableRow data-testid="member-row">
      <TableCell>
        <div className="flex min-w-56 items-center gap-3">
          <Avatar name={member.user.full_name} src={member.user.avatar_url} size="sm" />
          <div className="flex min-w-0 flex-col">
            <span className="font-medium text-fg">
              {member.user.full_name}
              {self && <span className="ml-1.5 text-xs text-fg-subtle">{t('members.you')}</span>}
            </span>
            <span className="text-xs whitespace-nowrap text-fg-subtle">{member.email}</span>
          </div>
        </div>
      </TableCell>
      <TableCell>
        <ul className="flex min-w-40 flex-wrap gap-1">
          {member.roles.map((role) => (
            <li key={role}>
              <Badge size="sm" tone={role === 'tenant_admin' ? 'gold' : 'neutral'}>
                {t(`common:roles.${role}`)}
              </Badge>
            </li>
          ))}
        </ul>
      </TableCell>
      <TableCell className="min-w-44 text-sm text-fg-muted">
        {study.length > 0 ? study.join(' · ') : '—'}
      </TableCell>
      <TableCell>
        <Badge tone={STATUS_TONE[member.status]} dot>
          {t(`members.status.${member.status}`)}
        </Badge>
      </TableCell>
      <TableCell className="text-sm whitespace-nowrap text-fg-muted">
        {member.last_active_at ? format.relative(member.last_active_at) : '—'}
      </TableCell>
      {canEdit && (
        <TableCell className="text-right">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <IconButton
                size="sm"
                label={t('members.actions', { name: member.user.full_name })}
                icon={<MoreHorizontal />}
              />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              <DropdownMenuItem
                icon={<UserCog />}
                onSelect={() => {
                  onEditRoles(member);
                }}
              >
                {t('members.editRoles')}
              </DropdownMenuItem>
              {!self &&
                (member.status === 'suspended' ? (
                  <DropdownMenuItem
                    icon={<CircleCheck />}
                    onSelect={() => {
                      onToggleStatus(member);
                    }}
                  >
                    {t('members.activate')}
                  </DropdownMenuItem>
                ) : (
                  <DropdownMenuItem
                    destructive
                    icon={<Ban />}
                    onSelect={() => {
                      onToggleStatus(member);
                    }}
                  >
                    {t('members.suspend')}
                  </DropdownMenuItem>
                ))}
            </DropdownMenuContent>
          </DropdownMenu>
        </TableCell>
      )}
    </TableRow>
  );
}

function RolesDialog({
  member,
  self,
  onOpenChange,
}: {
  member: Member | null;
  self: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation('admin');
  return (
    <Dialog open={member !== null} onOpenChange={onOpenChange}>
      <DialogContent size="md">
        <DialogHeader>
          <DialogTitle>{t('members.rolesTitle')}</DialogTitle>
          <DialogDescription>{member?.user.full_name}</DialogDescription>
        </DialogHeader>
        {member && (
          <RolesForm
            member={member}
            self={self}
            onDone={() => {
              onOpenChange(false);
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function RolesForm({
  member,
  self,
  onDone,
}: {
  member: Member;
  self: boolean;
  onDone: () => void;
}) {
  const { t } = useTranslation(['admin', 'common']);
  const update = useUpdateMember();
  const [roles, setRoles] = useState<Role[]>(member.roles);
  const [formError, setFormError] = useState<string | null>(null);

  const toggle = (role: Role, checked: boolean) => {
    setRoles((current) => (checked ? [...current, role] : current.filter((item) => item !== role)));
  };

  const submit = () => {
    setFormError(null);
    update.mutate(
      { userId: member.user.id, body: { roles } },
      {
        onSuccess: () => {
          toast.success(t('members.rolesSaved'));
          onDone();
        },
        onError: (err) => {
          setFormError(errorMessage(err));
        },
      },
    );
  };

  return (
    <form
      className="flex min-h-0 flex-1 flex-col"
      noValidate
      onSubmit={(event) => {
        event.preventDefault();
        submit();
      }}
    >
      <DialogBody className="flex flex-col gap-4">
        <FormAlert message={formError} />
        <fieldset className="flex flex-col gap-1">
          <legend className="sr-only">{t('members.rolesTitle')}</legend>
          {ROLES.map((role) => {
            const id = `role-${role}`;
            const locked = self && role === 'tenant_admin';
            return (
              <div
                key={role}
                className="flex items-start gap-3 rounded-lg px-2 py-2 hover:bg-surface-raised"
              >
                <Checkbox
                  id={id}
                  checked={roles.includes(role)}
                  disabled={locked}
                  onCheckedChange={(checked) => {
                    toggle(role, checked === true);
                  }}
                  className="mt-0.5"
                />
                <label htmlFor={id} className="flex flex-col gap-0.5">
                  <span className="text-sm font-medium text-fg">{t(`common:roles.${role}`)}</span>
                  <span className="text-xs text-fg-subtle">{t(`members.roleHints.${role}`)}</span>
                </label>
              </div>
            );
          })}
        </fieldset>
        {self && <Callout tone="info">{t('members.selfAdmin')}</Callout>}
        {roles.length === 0 && (
          <p className="text-sm text-danger" role="alert">
            {t('common:validation.minItems', { count: 1 })}
          </p>
        )}
      </DialogBody>
      <DialogFooter>
        <Button type="button" variant="secondary" onClick={onDone}>
          {t('common:actions.cancel')}
        </Button>
        <Button type="submit" loading={update.isPending} disabled={roles.length === 0}>
          {t('common:actions.save')}
        </Button>
      </DialogFooter>
    </form>
  );
}

function StatusDialog({
  member,
  onOpenChange,
}: {
  member: Member | null;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation(['admin', 'common']);
  const update = useUpdateMember();
  const suspending = member?.status !== 'suspended';
  const name = member?.user.full_name ?? '';
  return (
    <ConfirmDialog
      open={member !== null}
      onOpenChange={onOpenChange}
      tone={suspending ? 'danger' : 'primary'}
      title={
        suspending ? t('members.suspendTitle', { name }) : t('members.activateTitle', { name })
      }
      description={suspending ? t('members.suspendText') : t('members.activateText')}
      confirmLabel={suspending ? t('members.suspend') : t('members.activate')}
      loading={update.isPending}
      onConfirm={() => {
        if (!member) return;
        update.mutate(
          { userId: member.user.id, body: { status: suspending ? 'suspended' : 'active' } },
          {
            onSuccess: () => {
              toast.success(suspending ? t('members.suspended') : t('members.activated'));
              onOpenChange(false);
            },
            onError: (err) => {
              toast.error(errorMessage(err));
            },
          },
        );
      }}
    />
  );
}

const membersRoute = getRouteApi('/app/admin/members');

export function MembersRoute() {
  const { session } = membersRoute.useRouteContext();
  return <MembersPage session={session} />;
}
