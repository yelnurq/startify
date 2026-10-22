import { useQueries, useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { Building2, Mail, Send, X } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Badge,
  Button,
  Callout,
  EmptyState,
  SkeletonCard,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  toast,
  type Tone,
} from '@/design-system';
import { errorMessage } from '@/shared/api/errors';
import { useFormatters } from '@/shared/i18n/format';

import {
  contactRequestsQuery,
  useModerateContact,
  type ContactRequest,
  type ContactStatus,
} from './api';

const TABS: ContactStatus[] = ['pending', 'forwarded', 'rejected'];

const STATUS_TONE: Record<ContactStatus, Tone> = {
  pending: 'warning',
  forwarded: 'success',
  rejected: 'neutral',
};

/** Requests from the showcase that wait for the innovation office. */
export function ContactsPage() {
  const { t } = useTranslation('showcase');
  const [tab, setTab] = useState<ContactStatus>('pending');
  const counts = useQueries({ queries: TABS.map((status) => contactRequestsQuery(status)) });
  return (
    <div className="mx-auto flex w-full max-w-4xl flex-col gap-6">
      <header className="flex flex-col gap-1">
        <h1 className="font-display text-3xl font-semibold text-fg">{t('contacts.title')}</h1>
        <p className="text-fg-muted">{t('contacts.subtitle')}</p>
      </header>
      <Tabs
        value={tab}
        onValueChange={(value) => {
          setTab(value as ContactStatus);
        }}
      >
        <TabsList>
          {TABS.map((status, i) => (
            <TabsTrigger key={status} value={status} count={counts[i]?.data?.length}>
              {t(`contacts.tabs.${status}`)}
            </TabsTrigger>
          ))}
        </TabsList>
        {TABS.map((status) => (
          <TabsContent key={status} value={status}>
            <ContactList status={status} />
          </TabsContent>
        ))}
      </Tabs>
    </div>
  );
}

function ContactList({ status }: { status: ContactStatus }) {
  const { t } = useTranslation('showcase');
  const list = useQuery(contactRequestsQuery(status));
  if (list.isPending) {
    return (
      <div className="flex flex-col gap-4" aria-busy>
        <SkeletonCard />
        <SkeletonCard />
      </div>
    );
  }
  if (list.isError) return <Callout tone="danger">{errorMessage(list.error)}</Callout>;
  if (list.data.length === 0) {
    return (
      <EmptyState
        illustration="inbox"
        title={status === 'pending' ? t('contacts.empty.pending') : t('contacts.empty.other')}
        description={
          status === 'pending' ? t('contacts.empty.pendingText') : t('contacts.empty.otherText')
        }
      />
    );
  }
  return (
    <ul className="flex flex-col gap-4">
      {list.data.map((contact) => (
        <li key={contact.id}>
          <ContactCard contact={contact} />
        </li>
      ))}
    </ul>
  );
}

function ContactCard({ contact }: { contact: ContactRequest }) {
  const { t } = useTranslation('showcase');
  const { dateTime } = useFormatters();
  const moderate = useModerateContact();
  const act = (forward: boolean) => {
    moderate.mutate(
      { contactId: contact.id, forward },
      {
        onSuccess: () => {
          toast.success(forward ? t('contacts.forwarded') : t('contacts.rejected'));
        },
        onError: (err) => {
          toast.error(errorMessage(err));
        },
      },
    );
  };
  return (
    <article className="flex flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 flex-col gap-1">
          <p className="text-xs text-fg-subtle">
            {t('contacts.project')}:{' '}
            <Link
              to="/app/projects/$projectId/showcase"
              params={{ projectId: contact.project_id }}
              className="font-medium text-primary-text hover:underline"
            >
              {contact.project_title}
            </Link>
          </p>
          <h2 className="text-base font-semibold text-fg">{contact.name}</h2>
          <p className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-fg-muted">
            <a
              href={`mailto:${contact.email}`}
              className="inline-flex items-center gap-1.5 hover:text-fg"
            >
              <Mail className="size-4" aria-hidden />
              {contact.email}
            </a>
            {contact.organization && (
              <span className="inline-flex items-center gap-1.5">
                <Building2 className="size-4" aria-hidden />
                {contact.organization}
              </span>
            )}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-1.5">
          <Badge tone="info" size="sm">
            {t(`contactKinds.${contact.kind}`)}
          </Badge>
          <Badge tone={STATUS_TONE[contact.status]} size="sm" dot>
            {t(`contactStatus.${contact.status}`)}
          </Badge>
        </div>
      </div>
      <p className="text-sm leading-relaxed whitespace-pre-line text-fg">{contact.message}</p>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <span className="text-xs text-fg-subtle">
          {t('contacts.received', { time: dateTime(contact.created_at) })}
        </span>
        {contact.status === 'pending' && (
          <div className="flex flex-wrap gap-2">
            <Button
              size="sm"
              leftIcon={<Send />}
              loading={moderate.isPending && moderate.variables.forward}
              disabled={moderate.isPending}
              onClick={() => {
                act(true);
              }}
            >
              {t('contacts.forward')}
            </Button>
            <Button
              size="sm"
              variant="secondary"
              leftIcon={<X />}
              loading={moderate.isPending && !moderate.variables.forward}
              disabled={moderate.isPending}
              onClick={() => {
                act(false);
              }}
            >
              {t('contacts.reject')}
            </Button>
          </div>
        )}
      </div>
    </article>
  );
}
