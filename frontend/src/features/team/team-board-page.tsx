import { useInfiniteQuery } from '@tanstack/react-query';
import { getRouteApi, useNavigate } from '@tanstack/react-router';
import { Plus, Search, X } from 'lucide-react';
import { useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Button,
  Callout,
  EmptyState,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  SkeletonCard,
  Tabs,
  TabsList,
  TabsTrigger,
} from '@/design-system';
import { errorMessage } from '@/shared/api/errors';
import { useDebouncedValue } from '@/shared/hooks/use-debounced-value';
import { compact } from '@/shared/router/search';

import { isTeamRole, TEAM_ROLES, teamRequestsQuery, type TeamRequest, type TeamRole } from './api';
import { RequestCard } from './request-card';
import { RequestDialog, RespondDialog, ResponsesSheet } from './request-dialogs';

export interface TeamSearch {
  tab?: 'mine';
  role?: TeamRole;
  /** Opens the form for a new request (links from the dashboard). */
  new?: boolean;
}

interface TeamBoardPageProps {
  search: TeamSearch;
  onSearchChange: (search: TeamSearch) => void;
}

const ANY = 'any';

/** «Ищу в команду»: requests for teammates, replies and the author's own requests. */
export function TeamBoardPage({ search, onSearchChange }: TeamBoardPageProps) {
  const { t } = useTranslation(['team', 'common']);
  const mine = search.tab === 'mine';
  const [query, setQuery] = useState('');
  const [skill, setSkill] = useState('');
  const debouncedQuery = useDebouncedValue(query.trim(), 350);
  const debouncedSkill = useDebouncedValue(skill.trim(), 350);

  const [creating, setCreating] = useState(Boolean(search.new));
  const [editing, setEditing] = useState<TeamRequest | null>(null);
  const [responding, setResponding] = useState<TeamRequest | null>(null);
  const [viewing, setViewing] = useState<TeamRequest | null>(null);

  const update = (patch: Partial<TeamSearch>) => {
    onSearchChange(compact({ ...search, ...patch }));
  };

  const filters = mine
    ? { mine: true }
    : compact({
        q: debouncedQuery || undefined,
        role: search.role,
        skill: debouncedSkill || undefined,
      });
  const requests = useInfiniteQuery(teamRequestsQuery(filters));
  const items = requests.data?.pages.flatMap((page) => page.items) ?? [];
  const filtered =
    !mine && (debouncedQuery !== '' || search.role !== undefined || debouncedSkill !== '');

  const resetFilters = () => {
    setQuery('');
    setSkill('');
    onSearchChange(compact({ tab: search.tab }));
  };

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex max-w-2xl flex-col gap-1">
          <h1 className="font-display text-3xl font-semibold text-fg">{t('board.title')}</h1>
          <p className="text-fg-muted">{t('board.subtitle')}</p>
        </div>
        <Button
          leftIcon={<Plus />}
          onClick={() => {
            setCreating(true);
          }}
        >
          {t('board.create')}
        </Button>
      </header>

      <Tabs
        value={mine ? 'mine' : 'all'}
        onValueChange={(value) => {
          update({ tab: value === 'mine' ? 'mine' : undefined });
        }}
      >
        <TabsList>
          <TabsTrigger value="all">{t('board.tabs.all')}</TabsTrigger>
          <TabsTrigger value="mine">{t('board.tabs.mine')}</TabsTrigger>
        </TabsList>
      </Tabs>

      {!mine && (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-[minmax(0,2fr)_minmax(0,1fr)_minmax(0,1fr)_auto]">
          <Input
            type="search"
            value={query}
            leading={<Search />}
            maxLength={120}
            aria-label={t('board.search')}
            placeholder={t('board.searchPlaceholder')}
            onChange={(event) => {
              setQuery(event.target.value);
            }}
          />
          <Select
            value={search.role ?? ANY}
            onValueChange={(value) => {
              update({ role: isTeamRole(value) ? value : undefined });
            }}
          >
            <SelectTrigger aria-label={t('board.role')}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ANY}>{t('board.anyRole')}</SelectItem>
              {TEAM_ROLES.map((role) => (
                <SelectItem key={role} value={role}>
                  {t(`roles.${role}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Input
            value={skill}
            maxLength={40}
            aria-label={t('board.skill')}
            placeholder={t('board.skillPlaceholder')}
            onChange={(event) => {
              setSkill(event.target.value);
            }}
          />
          {filtered && (
            <Button variant="ghost" leftIcon={<X />} onClick={resetFilters}>
              {t('board.reset')}
            </Button>
          )}
        </div>
      )}

      {requests.isPending && (
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-2" aria-busy>
          <SkeletonCard />
          <SkeletonCard />
        </div>
      )}
      {requests.isError && (
        <Callout
          tone="danger"
          action={
            <Button size="sm" variant="secondary" onClick={() => void requests.refetch()}>
              {t('common:actions.retry')}
            </Button>
          }
        >
          {errorMessage(requests.error)}
        </Callout>
      )}
      {requests.isSuccess && items.length === 0 && (
        <EmptyState
          illustration={filtered ? 'search' : mine ? 'idea' : 'team'}
          title={
            filtered ? t('board.emptyFiltered') : mine ? t('board.emptyMine') : t('board.empty')
          }
          description={
            filtered
              ? t('board.emptyFilteredText')
              : mine
                ? t('board.emptyMineText')
                : t('board.emptyText')
          }
          action={
            filtered ? (
              <Button variant="secondary" onClick={resetFilters}>
                {t('board.reset')}
              </Button>
            ) : (
              <Button
                leftIcon={<Plus />}
                onClick={() => {
                  setCreating(true);
                }}
              >
                {t('board.create')}
              </Button>
            )
          }
        />
      )}
      {items.length > 0 && (
        <ul className="grid grid-cols-1 gap-4 xl:grid-cols-2">
          {items.map((request) => (
            <li key={request.id}>
              <RequestCard
                linked
                request={request}
                onRespond={setResponding}
                onEdit={setEditing}
                onShowResponses={setViewing}
              />
            </li>
          ))}
        </ul>
      )}
      {requests.hasNextPage && (
        <Button
          variant="secondary"
          className="self-center"
          loading={requests.isFetchingNextPage}
          onClick={() => void requests.fetchNextPage()}
        >
          {t('common:actions.loadMore')}
        </Button>
      )}

      <RequestDialog
        open={creating || editing !== null}
        request={editing ?? undefined}
        onOpenChange={(open) => {
          if (open) return;
          setCreating(false);
          setEditing(null);
          if (search.new) update({ new: undefined });
        }}
      />
      <RespondDialog
        request={responding}
        onOpenChange={(open) => {
          if (!open) setResponding(null);
        }}
      />
      <ResponsesSheet
        request={viewing}
        onOpenChange={(open) => {
          if (!open) setViewing(null);
        }}
      />
    </div>
  );
}

const teamRoute = getRouteApi('/app/team/');

/** `/app/team`: the tab and role filter live in the address. */
export function TeamRoute() {
  const search = teamRoute.useSearch();
  const navigate = useNavigate({ from: '/app/team/' });
  const onSearchChange = useCallback(
    (next: TeamSearch) => {
      void navigate({ search: next, replace: true });
    },
    [navigate],
  );
  return <TeamBoardPage search={search} onSearchChange={onSearchChange} />;
}
