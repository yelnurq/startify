import { Link } from '@tanstack/react-router';
import { CircleCheck, Inbox, Lock, LockOpen, MoreHorizontal, Pencil, Send } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Avatar,
  Badge,
  Button,
  Card,
  cn,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  IconButton,
  Tag,
  toast,
} from '@/design-system';
import { errorMessage } from '@/shared/api/errors';
import { useFormatters } from '@/shared/i18n/format';
import { useLocalized } from '@/shared/i18n/localized';

import { useUpdateTeamRequest, type TeamRequest } from './api';

const LONG_TEXT = 280;

interface RequestCardProps {
  request: TeamRequest;
  /** The title links to the request page (on the board). */
  linked?: boolean;
  onRespond: (request: TeamRequest) => void;
  onEdit: (request: TeamRequest) => void;
  onShowResponses: (request: TeamRequest) => void;
}

export function RequestCard({
  request,
  linked = false,
  onRespond,
  onEdit,
  onShowResponses,
}: RequestCardProps) {
  const { t } = useTranslation(['team', 'common']);
  const format = useFormatters();
  const text = useLocalized();
  const update = useUpdateTeamRequest();
  const [expanded, setExpanded] = useState(false);
  const closed = request.status === 'closed';
  const long = request.description.length > LONG_TEXT;
  const titleId = `request-${request.id}`;

  const setStatus = (status: TeamRequest['status']) => {
    update.mutate(
      { id: request.id, body: { status } },
      {
        onSuccess: () => {
          toast.success(status === 'closed' ? t('card.closedToast') : t('card.reopenedToast'));
        },
        onError: (err) => {
          toast.error(errorMessage(err));
        },
      },
    );
  };

  return (
    <Card
      className={cn('h-full gap-4 p-5', closed && 'bg-surface-raised')}
      aria-labelledby={titleId}
      data-testid="team-request"
    >
      <div className="flex items-start gap-3">
        <Avatar name={request.author.full_name} src={request.author.avatar_url} size="sm" />
        <div className="flex min-w-0 flex-1 flex-col">
          <p className="text-sm font-medium text-fg">{request.author.full_name}</p>
          <p className="text-xs text-fg-subtle">
            {request.author.faculty ? `${text(request.author.faculty.name)} · ` : ''}
            <time dateTime={request.created_at}>{format.relative(request.created_at)}</time>
          </p>
        </div>
        {request.is_mine && (closed || !linked) && (
          <Badge tone={closed ? 'neutral' : 'success'} dot className="shrink-0">
            {closed ? t('card.statusClosed') : t('card.statusOpen')}
          </Badge>
        )}
        {request.is_mine && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <IconButton size="sm" label={t('card.actions')} icon={<MoreHorizontal />} />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              <DropdownMenuItem
                icon={<Pencil />}
                onSelect={() => {
                  onEdit(request);
                }}
              >
                {t('common:actions.edit')}
              </DropdownMenuItem>
              {closed ? (
                <DropdownMenuItem
                  icon={<LockOpen />}
                  onSelect={() => {
                    setStatus('open');
                  }}
                >
                  {t('card.reopen')}
                </DropdownMenuItem>
              ) : (
                <DropdownMenuItem
                  icon={<Lock />}
                  onSelect={() => {
                    setStatus('closed');
                  }}
                >
                  {t('card.close')}
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>

      <div className="flex flex-col gap-2">
        <h3 id={titleId} className="text-lg leading-snug font-semibold text-fg">
          {linked ? (
            <Link
              to="/app/team/$requestId"
              params={{ requestId: request.id }}
              className="rounded-sm hover:text-primary-text hover:underline"
            >
              {request.title}
            </Link>
          ) : (
            request.title
          )}
        </h3>
        <p
          className={cn(
            'text-sm leading-relaxed whitespace-pre-line text-fg-muted',
            long && !expanded && 'line-clamp-4',
          )}
        >
          {request.description}
        </p>
        {long && (
          <Button
            variant="link"
            size="sm"
            className="self-start"
            aria-expanded={expanded}
            onClick={() => {
              setExpanded((value) => !value);
            }}
          >
            {expanded ? t('card.less') : t('card.more')}
          </Button>
        )}
      </div>

      <div className="flex flex-col gap-2">
        <ul className="flex flex-wrap gap-1.5" aria-label={t('card.roles')}>
          {request.roles_needed.map((role) => (
            <li key={role}>
              <Badge tone="primary">{t(`roles.${role}`)}</Badge>
            </li>
          ))}
        </ul>
        {request.skills.length > 0 && (
          <ul className="flex flex-wrap gap-1.5" aria-label={t('card.skills')}>
            {request.skills.map((skill) => (
              <li key={skill}>
                <Tag className="h-7 text-xs">{skill}</Tag>
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="mt-auto flex flex-wrap items-center justify-between gap-3 border-t border-border pt-4">
        <span className="text-sm text-fg-subtle">
          {t('card.responses', { count: request.responses_count })}
        </span>
        {request.is_mine ? (
          <Button
            size="sm"
            variant="soft"
            leftIcon={<Inbox />}
            onClick={() => {
              onShowResponses(request);
            }}
          >
            {t('card.showResponses')}
          </Button>
        ) : request.has_responded ? (
          <Badge tone="success" icon={<CircleCheck />}>
            {t('card.responded')}
          </Badge>
        ) : closed ? (
          <Badge tone="neutral">{t('card.statusClosed')}</Badge>
        ) : (
          <Button
            size="sm"
            leftIcon={<Send />}
            onClick={() => {
              onRespond(request);
            }}
          >
            {t('card.respond')}
          </Button>
        )}
      </div>
    </Card>
  );
}
