import { useQuery } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { Archive, Crown, LogOut, Search, UserPlus, X } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Avatar,
  Badge,
  Button,
  Checkbox,
  ConfirmDialog,
  Field,
  IconButton,
  Input,
  SegmentedControl,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Separator,
  Sheet,
  SheetContent,
  SheetDescription,
  SheetTitle,
  toast,
  TruncatedText,
} from '@/design-system';
import { errorMessage } from '@/shared/api/errors';
import { useLocalized } from '@/shared/i18n/localized';

import {
  canEdit,
  INDUSTRIES,
  peopleQuery,
  PROJECT_LIMITS,
  useAddMember,
  useArchiveProject,
  useRemoveMember,
  useUpdateMember,
  useUpdateProject,
  type Industry,
  type Project,
  type ProjectMember,
} from './api';

interface ProjectSettingsSheetProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  project: Project;
  etag: string | undefined;
  meId: string;
  innovationManager: boolean;
}

/** The project card, its team and the dangerous actions (leave, hand over, archive). */
export function ProjectSettingsSheet({
  open,
  onOpenChange,
  project,
  etag,
  meId,
  innovationManager,
}: ProjectSettingsSheetProps) {
  const { t } = useTranslation('incubator');
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent size="md">
        <div className="flex flex-col gap-1 border-b border-border px-5 pt-5 pr-14 pb-4">
          <SheetTitle className="text-base font-semibold text-fg">{t('settings.title')}</SheetTitle>
          <SheetDescription className="text-sm text-fg-muted">{project.title}</SheetDescription>
        </div>
        <div className="flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto px-5 py-5">
          {open && (
            <>
              <CardForm key={etag} project={project} etag={etag} />
              <Separator />
              <Team project={project} meId={meId} innovationManager={innovationManager} />
              <Separator />
              <Danger
                project={project}
                meId={meId}
                innovationManager={innovationManager}
                onDone={() => {
                  onOpenChange(false);
                }}
              />
            </>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}

function CardForm({ project, etag }: { project: Project; etag: string | undefined }) {
  const { t } = useTranslation(['incubator', 'common']);
  const update = useUpdateProject(project.id);
  const editable = canEdit(project);
  const [title, setTitle] = useState(project.title);
  const [tagline, setTagline] = useState(project.tagline);
  const [industry, setIndustry] = useState<Industry>(project.industry);
  const [hide, setHide] = useState(project.hide_sensitive);
  const tooShort = title.trim().length < PROJECT_LIMITS.title[0];
  const changed =
    title !== project.title ||
    tagline !== project.tagline ||
    industry !== project.industry ||
    hide !== project.hide_sensitive;
  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault();
        if (tooShort) return;
        update.mutate(
          {
            body: { title: title.trim(), tagline: tagline.trim(), industry, hide_sensitive: hide },
            etag,
          },
          {
            onSuccess: () => {
              toast.success(t('settings.saved'));
            },
            onError: (err) => {
              toast.error(errorMessage(err));
            },
          },
        );
      }}
    >
      <Field
        label={t('create.name')}
        required
        error={tooShort ? t('common:validation.required') : undefined}
      >
        <Input
          value={title}
          maxLength={PROJECT_LIMITS.title[1]}
          disabled={!editable}
          onChange={(e) => {
            setTitle(e.target.value);
          }}
        />
      </Field>
      <Field
        label={t('create.tagline')}
        optional
        counter={{ value: tagline.length, max: PROJECT_LIMITS.tagline }}
      >
        <Input
          value={tagline}
          maxLength={PROJECT_LIMITS.tagline}
          disabled={!editable}
          onChange={(e) => {
            setTagline(e.target.value);
          }}
        />
      </Field>
      <Field label={t('create.industry')} required>
        <Select
          value={industry}
          disabled={!editable}
          onValueChange={(v) => {
            setIndustry(v as Industry);
          }}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {INDUSTRIES.map((value) => (
              <SelectItem key={value} value={value}>
                {t(`industries.${value}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>
      <label className="flex items-start gap-3 text-sm">
        <Checkbox
          checked={hide}
          disabled={!editable}
          onCheckedChange={(v) => {
            setHide(v === true);
          }}
        />
        <span className="flex flex-col gap-0.5">
          <span className="font-medium text-fg">{t('settings.hideSensitive')}</span>
          <span className="text-fg-subtle">{t('settings.hideSensitiveHint')}</span>
        </span>
      </label>
      {editable && (
        <Button type="submit" className="self-start" disabled={!changed} loading={update.isPending}>
          {t('common:actions.save')}
        </Button>
      )}
    </form>
  );
}

function Team({
  project,
  meId,
  innovationManager,
}: {
  project: Project;
  meId: string;
  innovationManager: boolean;
}) {
  const { t } = useTranslation('incubator');
  const owner = project.access === 'owner';
  const moderator = project.access === 'manager' && innovationManager;
  const manageSupervisor = owner || moderator;
  const [adding, setAdding] = useState(false);
  return (
    <section aria-labelledby="team" className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-2">
        <h3 id="team" className="text-sm font-semibold text-ink">
          {t('settings.team')}
        </h3>
        {(owner || manageSupervisor) && !project.archived && !adding && (
          <Button
            size="sm"
            variant="secondary"
            leftIcon={<UserPlus />}
            onClick={() => {
              setAdding(true);
            }}
          >
            {t('settings.add')}
          </Button>
        )}
      </div>
      {adding && (
        <AddMember
          project={project}
          supervisorOnly={!owner}
          onClose={() => {
            setAdding(false);
          }}
        />
      )}
      <ul className="flex flex-col gap-2">
        {project.members.map((member) => (
          <MemberRow
            key={member.user.id}
            project={project}
            member={member}
            me={member.user.id === meId}
            owner={owner}
            moderator={moderator}
          />
        ))}
      </ul>
    </section>
  );
}

function MemberRow({
  project,
  member,
  me,
  owner,
  moderator,
}: {
  project: Project;
  member: ProjectMember;
  me: boolean;
  owner: boolean;
  /** An innovation manager outside the team: hands over a project whose owner is gone. */
  moderator: boolean;
}) {
  const { t } = useTranslation('incubator');
  const update = useUpdateMember(project.id);
  const remove = useRemoveMember(project.id);
  const [title, setTitle] = useState(member.title);
  const [confirm, setConfirm] = useState<'remove' | 'transfer' | null>(null);
  const canTitle = (me || owner) && !project.archived;
  const canTransfer =
    ((owner && !me) || moderator) && member.role === 'member' && !project.archived;
  const canRemove = owner && !me && member.role !== 'owner' && !project.archived;
  return (
    <li className="flex flex-col gap-2 rounded-xl border border-border p-3">
      <div className="flex items-center gap-3">
        <Avatar name={member.user.full_name} src={member.user.avatar_url} size="sm" />
        <div className="flex min-w-0 flex-1 flex-col">
          <TruncatedText className="text-sm font-medium text-fg">
            {member.user.full_name}
          </TruncatedText>
          <span className="text-xs text-fg-subtle">{t(`settings.role.${member.role}`)}</span>
        </div>
        {member.role === 'owner' && (
          <Badge size="sm" tone="gold" icon={<Crown />}>
            {t('settings.role.owner')}
          </Badge>
        )}
        {(canTransfer || canRemove) && (
          <span className="flex">
            {canTransfer && (
              <IconButton
                size="sm"
                label={t('settings.transfer')}
                icon={<Crown />}
                onClick={() => {
                  setConfirm('transfer');
                }}
              />
            )}
            {canRemove && (
              <IconButton
                size="sm"
                label={t('settings.remove')}
                icon={<X />}
                onClick={() => {
                  setConfirm('remove');
                }}
              />
            )}
          </span>
        )}
      </div>
      {member.role !== 'supervisor' && (
        <form
          className="flex items-center gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            update.mutate(
              { userId: member.user.id, body: { title: title.trim() } },
              {
                onSuccess: () => {
                  toast.success(t('settings.saved'));
                },
                onError: (err) => {
                  toast.error(errorMessage(err));
                },
              },
            );
          }}
        >
          <Input
            inputSize="sm"
            value={title}
            maxLength={PROJECT_LIMITS.memberTitle}
            placeholder={t('settings.titlePlaceholder')}
            aria-label={t('settings.memberTitle', { name: member.user.full_name })}
            disabled={!canTitle}
            onChange={(e) => {
              setTitle(e.target.value);
            }}
          />
          {canTitle && title !== member.title && (
            <Button size="sm" variant="secondary" type="submit" loading={update.isPending}>
              {t('settings.saveTitle')}
            </Button>
          )}
        </form>
      )}
      <ConfirmDialog
        open={confirm !== null}
        onOpenChange={(v) => {
          if (!v) setConfirm(null);
        }}
        title={
          confirm === 'transfer'
            ? t('settings.transferTitle', { name: member.user.full_name })
            : t('settings.removeTitle', { name: member.user.full_name })
        }
        description={
          confirm === 'transfer'
            ? owner
              ? t('settings.transferText')
              : t('settings.transferModeratorText')
            : t('settings.removeText')
        }
        confirmLabel={confirm === 'transfer' ? t('settings.transfer') : t('settings.remove')}
        tone={confirm === 'remove' ? 'danger' : 'primary'}
        loading={update.isPending || remove.isPending}
        onConfirm={() => {
          const done = {
            onSuccess: () => {
              setConfirm(null);
            },
            onError: (err: unknown) => {
              toast.error(errorMessage(err));
            },
          };
          if (confirm === 'transfer') {
            update.mutate({ userId: member.user.id, body: { role: 'owner' } }, done);
          } else {
            remove.mutate(member.user.id, done);
          }
        }}
      />
    </li>
  );
}

function AddMember({
  project,
  supervisorOnly,
  onClose,
}: {
  project: Project;
  supervisorOnly: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation('incubator');
  const text = useLocalized();
  const [role, setRole] = useState<'member' | 'supervisor'>(
    supervisorOnly ? 'supervisor' : 'member',
  );
  const [q, setQ] = useState('');
  const add = useAddMember(project.id);
  const people = useQuery({
    ...peopleQuery(q.trim(), role === 'supervisor' ? 'supervisor' : undefined),
    enabled: q.trim().length >= 2,
  });
  const taken = new Set(project.members.map((m) => m.user.id));
  return (
    <div className="flex flex-col gap-3 rounded-xl border border-primary-border bg-primary-soft p-3">
      <div className="flex items-center justify-between gap-2">
        {supervisorOnly ? (
          <span className="text-sm font-medium text-fg">{t('settings.addSupervisor')}</span>
        ) : (
          <SegmentedControl
            size="sm"
            label={t('settings.addAs')}
            value={role}
            onValueChange={setRole}
            options={[
              { value: 'member', label: t('settings.role.member') },
              { value: 'supervisor', label: t('settings.role.supervisor') },
            ]}
          />
        )}
        <IconButton size="sm" label={t('settings.closeAdd')} icon={<X />} onClick={onClose} />
      </div>
      <Input
        autoFocus
        value={q}
        leading={<Search />}
        placeholder={t('settings.search')}
        aria-label={t('settings.search')}
        onChange={(e) => {
          setQ(e.target.value);
        }}
      />
      {q.trim().length >= 2 && people.data && (
        <ul className="flex max-h-64 flex-col gap-1 overflow-y-auto">
          {people.data.length === 0 && (
            <li className="px-2 py-1.5 text-sm text-fg-subtle">{t('settings.nobody')}</li>
          )}
          {people.data.map((person) => (
            <li key={person.id}>
              <button
                type="button"
                disabled={taken.has(person.id) || add.isPending}
                className="flex w-full items-center gap-3 rounded-lg px-2 py-1.5 text-left hover:bg-surface disabled:opacity-60"
                onClick={() => {
                  add.mutate(
                    { user_id: person.id, role },
                    {
                      onSuccess: () => {
                        toast.success(t('settings.added', { name: person.full_name }));
                        setQ('');
                      },
                      onError: (err) => {
                        toast.error(errorMessage(err));
                      },
                    },
                  );
                }}
              >
                <Avatar name={person.full_name} src={person.avatar_url} size="xs" />
                <span className="flex min-w-0 flex-1 flex-col">
                  <TruncatedText className="text-sm text-fg">{person.full_name}</TruncatedText>
                  {person.faculty && (
                    <TruncatedText className="text-xs text-fg-subtle">
                      {text(person.faculty.name)}
                    </TruncatedText>
                  )}
                </span>
                {taken.has(person.id) && (
                  <Badge size="sm" tone="neutral">
                    {t('settings.inTeam')}
                  </Badge>
                )}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function Danger({
  project,
  meId,
  innovationManager,
  onDone,
}: {
  project: Project;
  meId: string;
  innovationManager: boolean;
  onDone: () => void;
}) {
  const { t } = useTranslation('incubator');
  const navigate = useNavigate();
  const remove = useRemoveMember(project.id);
  const archive = useArchiveProject(project.id);
  const [confirm, setConfirm] = useState<'leave' | 'archive' | null>(null);
  const member = project.access === 'member' || project.access === 'supervisor';
  const canArchive =
    !project.archived &&
    (project.access === 'owner' || (project.access === 'manager' && innovationManager));
  if (!member && !canArchive) return null;
  return (
    <section className="flex flex-col items-start gap-2">
      {member && (
        <Button
          variant="ghost"
          leftIcon={<LogOut />}
          onClick={() => {
            setConfirm('leave');
          }}
        >
          {t('settings.leave')}
        </Button>
      )}
      {canArchive && (
        <Button
          variant="ghost"
          className="text-danger"
          leftIcon={<Archive />}
          onClick={() => {
            setConfirm('archive');
          }}
        >
          {t('settings.archive')}
        </Button>
      )}
      <ConfirmDialog
        open={confirm !== null}
        onOpenChange={(v) => {
          if (!v) setConfirm(null);
        }}
        title={confirm === 'archive' ? t('settings.archiveTitle') : t('settings.leaveTitle')}
        description={confirm === 'archive' ? t('settings.archiveText') : t('settings.leaveText')}
        confirmLabel={confirm === 'archive' ? t('settings.archive') : t('settings.leave')}
        tone="danger"
        loading={remove.isPending || archive.isPending}
        onConfirm={() => {
          const onError = (err: unknown) => {
            toast.error(errorMessage(err));
          };
          if (confirm === 'archive') {
            archive.mutate(undefined, {
              onSuccess: () => {
                setConfirm(null);
                onDone();
                toast.success(t('settings.archived'));
              },
              onError,
            });
          } else {
            remove.mutate(meId, {
              onSuccess: () => {
                setConfirm(null);
                onDone();
                void navigate({ to: '/app/projects' });
              },
              onError,
            });
          }
        }}
      />
    </section>
  );
}
