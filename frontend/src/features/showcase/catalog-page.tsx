import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { getRouteApi, useNavigate } from '@tanstack/react-router';
import { Search, SlidersHorizontal, X } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Button,
  Callout,
  EdgeIllustration,
  EmptyState,
  Field,
  Input,
  Kicker,
  SegmentedControl,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Sheet,
  SheetContent,
  SheetDescription,
  SheetTitle,
  Skeleton,
  Tag,
} from '@/design-system';
import { tenantQuery } from '@/features/auth/api';
import { errorMessage } from '@/shared/api/errors';
import { useDocumentMeta } from '@/shared/hooks/use-document-meta';
import { useFormatters } from '@/shared/i18n/format';
import { useLocalized } from '@/shared/i18n/localized';

import {
  showcaseFacetsQuery,
  showcaseListQuery,
  type ShowcaseCard,
  type ShowcaseFacets,
} from './api';
import { ShowcaseCardSkeleton, ShowcaseCardView } from './card';
import {
  activeFilters,
  INDUSTRIES,
  LIMITS,
  NEEDS,
  STAGES,
  switchScope,
  withFilter,
  withoutFilters,
  type FilterKey,
  type ShowcaseSearch,
} from './logic';
import { QuickView } from './quick-view';
import { SiteFrame } from './site';

const route = getRouteApi('/showcase/');

const ALL = 'all';

/** Tags offered as chips in the filter panel. */
const TOP_TAGS = 12;

/** The public catalogue of projects with search, filters and a quick view. */
export function CatalogPage() {
  const { t } = useTranslation(['showcase', 'common']);
  const text = useLocalized();
  const { number } = useFormatters();
  const search = route.useSearch();
  const navigate = useNavigate({ from: '/showcase/' });
  const national = search.scope === 'national';
  const tenant = useQuery(tenantQuery);
  const facets = useQuery(showcaseFacetsQuery(national));
  const list = useInfiniteQuery(showcaseListQuery(search));
  const [quick, setQuick] = useState<ShowcaseCard | null>(null);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const university = text(tenant.data?.short_name) || text(tenant.data?.name);

  useDocumentMeta(
    national ? t('meta.nationalTitle') : university && t('meta.catalogTitle', { university }),
    national ? t('meta.nationalDescription') : t('meta.catalogDescription', { university }),
  );

  const update = (next: ShowcaseSearch) => {
    void navigate({ search: next, resetScroll: false });
  };
  const setFilter = <K extends FilterKey>(key: K, value: ShowcaseSearch[K] | undefined) => {
    update(withFilter(search, key, value));
  };

  const items = list.data?.pages.flatMap((page) => page.items) ?? [];
  const active = activeFilters(search);
  const filtered = active.length > 0 || Boolean(search.q);
  const total = facets.data?.total;

  return (
    <SiteFrame>
      <section
        aria-labelledby="catalog-title"
        className="relative overflow-hidden border-b border-border bg-surface"
      >
        <EdgeIllustration
          name="search"
          placement="top-right"
          className="hidden w-60 translate-y-0 -rotate-6 lg:block"
        />
        <div className="mx-auto grid max-w-page grid-cols-1 items-end gap-8 px-4 py-12 sm:px-6 lg:grid-cols-[minmax(0,1fr)_auto] lg:py-16">
          <div className="flex flex-col gap-5">
            <Kicker>{national ? t('catalog.eyebrow') : university || t('catalog.eyebrow')}</Kicker>
            <h1
              id="catalog-title"
              className="max-w-4xl font-display text-4xl font-extrabold text-balance text-ink sm:text-6xl"
            >
              {national ? t('catalog.titleNational') : t('catalog.title')}
            </h1>
            <p className="max-w-2xl text-lg leading-relaxed text-fg-muted">
              {national
                ? t('catalog.subtitleNational')
                : t('catalog.subtitle', { university: university || t('catalog.eyebrow') })}
            </p>
            <SegmentedControl
              label={t('catalog.scope.label')}
              value={national ? 'national' : 'university'}
              onValueChange={(value) => {
                update(switchScope(search, value === 'national'));
              }}
              options={[
                { value: 'university', label: t('catalog.scope.university') },
                { value: 'national', label: t('catalog.scope.national') },
              ]}
            />
          </div>
          <p className="flex flex-col gap-1 border-l-2 border-primary-500 py-1 pl-6">
            {total === undefined ? (
              <Skeleton className="h-10 w-20" />
            ) : (
              <span className="font-figures text-4xl font-bold text-ink">{number(total)}</span>
            )}
            <span className="text-sm text-fg-muted">
              {t('catalog.inCatalog', { count: total ?? 0 })}
            </span>
          </p>
        </div>
      </section>

      <div className="mx-auto grid max-w-page grid-cols-1 gap-8 px-4 pb-16 sm:px-6 lg:grid-cols-[15rem_1fr] lg:pb-24">
        <aside
          aria-label={t('catalog.filters.title')}
          className="sticky top-20 hidden max-h-[calc(100dvh-6rem)] self-start overflow-y-auto pr-1 pb-4 lg:block"
        >
          <FilterPanel
            search={search}
            facets={facets.data}
            national={national}
            onChange={setFilter}
          />
        </aside>

        <section aria-labelledby="results-title" className="flex min-w-0 flex-col gap-5">
          <h2 id="results-title" className="sr-only">
            {t('catalog.results')}
          </h2>
          <div className="sticky top-16 z-20 -mx-4 flex items-center gap-2 border-b border-border/70 px-4 py-3 max-lg:surface-glass sm:-mx-6 sm:px-6 lg:static lg:mx-0 lg:border-0 lg:p-0">
            <SearchBox
              value={search.q ?? ''}
              onChange={(q) => {
                // Filters may change while typing pauses: build on the latest address.
                void navigate({
                  search: (prev) => ({ ...prev, q: q || undefined }),
                  replace: true,
                  resetScroll: false,
                });
              }}
            />
            <Button
              variant="secondary"
              className="shrink-0 lg:hidden"
              leftIcon={<SlidersHorizontal />}
              onClick={() => {
                setFiltersOpen(true);
              }}
            >
              {active.length > 0
                ? t('catalog.filters.openCount', { count: active.length })
                : t('catalog.filters.open')}
            </Button>
          </div>

          {active.length > 0 && (
            <div className="flex flex-wrap items-center gap-2">
              <span className="sr-only">{t('catalog.filters.active')}</span>
              {active.map(({ key, value }) => {
                const label = filterLabel(key, value);
                return (
                  <Tag
                    key={key}
                    removeLabel={t('catalog.filters.remove', { label })}
                    onRemove={() => {
                      setFilter(key, undefined);
                    }}
                  >
                    {label}
                  </Tag>
                );
              })}
              <Button
                variant="link"
                size="sm"
                onClick={() => {
                  update(withoutFilters(search));
                }}
              >
                {t('catalog.filters.reset')}
              </Button>
            </div>
          )}

          <p aria-live="polite" className="text-sm text-fg-muted">
            {list.isSuccess &&
              items.length > 0 &&
              (list.hasNextPage
                ? !filtered && total !== undefined
                  ? t('catalog.shown', { shown: number(items.length), total: number(total) })
                  : t('catalog.shownSome', { shown: number(items.length) })
                : t('catalog.found', { count: items.length }))}
          </p>

          {list.isError ? (
            <Callout
              tone="danger"
              title={t('catalog.error')}
              action={
                <Button size="sm" variant="secondary" onClick={() => void list.refetch()}>
                  {t('common:actions.retry')}
                </Button>
              }
            >
              {errorMessage(list.error)}
            </Callout>
          ) : list.isPending ? (
            <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 xl:grid-cols-3" aria-busy>
              {Array.from({ length: 6 }, (_, i) => (
                <ShowcaseCardSkeleton key={i} />
              ))}
            </div>
          ) : items.length === 0 ? (
            filtered ? (
              <EmptyState
                illustration="search"
                title={t('catalog.noResults.title')}
                description={t('catalog.noResults.text')}
                action={
                  <Button
                    variant="secondary"
                    onClick={() => {
                      update(national ? { scope: 'national' } : {});
                    }}
                  >
                    {t('catalog.filters.reset')}
                  </Button>
                }
              />
            ) : (
              <EmptyState
                illustration="projects"
                title={t('catalog.empty.title')}
                description={t('catalog.empty.text')}
              />
            )
          ) : (
            <>
              <ul className="grid grid-cols-1 gap-5 sm:grid-cols-2 xl:grid-cols-3">
                {items.map((card, index) => (
                  <li key={`${card.university.slug}/${card.slug}`}>
                    <ShowcaseCardView
                      card={card}
                      external={isExternal(card, tenant.data?.slug)}
                      showUniversity={national}
                      priority={index < 3}
                      onQuickView={() => {
                        setQuick(card);
                      }}
                    />
                  </li>
                ))}
              </ul>
              {list.hasNextPage && (
                <div className="flex justify-center pt-2">
                  <Button
                    variant="secondary"
                    loading={list.isFetchingNextPage}
                    onClick={() => void list.fetchNextPage()}
                  >
                    {t('common:actions.loadMore')}
                  </Button>
                </div>
              )}
            </>
          )}
        </section>
      </div>

      <Sheet open={filtersOpen} onOpenChange={setFiltersOpen}>
        <SheetContent side="right" size="sm">
          <div className="flex flex-col gap-1 border-b border-border px-5 pt-5 pr-14 pb-4">
            <SheetTitle className="text-base font-semibold text-fg">
              {t('catalog.filters.title')}
            </SheetTitle>
            <SheetDescription className="sr-only">{t('catalog.search.label')}</SheetDescription>
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto px-5 py-5">
            <FilterPanel
              search={search}
              facets={facets.data}
              national={national}
              onChange={setFilter}
            />
          </div>
          <div className="flex gap-2 border-t border-border p-4">
            <Button
              variant="secondary"
              className="flex-1"
              disabled={active.length === 0}
              onClick={() => {
                update(withoutFilters(search));
              }}
            >
              {t('catalog.filters.reset')}
            </Button>
            <Button
              className="flex-1"
              onClick={() => {
                setFiltersOpen(false);
              }}
            >
              {t('catalog.filters.show')}
            </Button>
          </div>
        </SheetContent>
      </Sheet>

      <QuickView
        card={quick}
        external={quick ? isExternal(quick, tenant.data?.slug) : false}
        onClose={() => {
          setQuick(null);
        }}
      />
    </SiteFrame>
  );

  function filterLabel(key: FilterKey, value: string): string {
    switch (key) {
      case 'industry':
        return t(`industries.${value as (typeof INDUSTRIES)[number]}`);
      case 'stage':
        return t(`stages.${value as (typeof STAGES)[number]}`);
      case 'need':
        return t(`needs.${value as (typeof NEEDS)[number]}`);
      case 'faculty': {
        const faculty = facets.data?.faculties.find((f) => f.id === value);
        return faculty ? text(faculty.name) : t('catalog.filters.faculty');
      }
      case 'year':
      case 'tag':
        return value;
    }
  }
}

/** A card of another university links to that university's site. */
function isExternal(card: ShowcaseCard, tenantSlug: string | undefined): boolean {
  return tenantSlug !== undefined && card.university.slug !== tenantSlug;
}

/** The search field: typing updates the address after a pause. */
function SearchBox({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const { t } = useTranslation(['showcase', 'common']);
  const [draft, setDraft] = useState(value);
  const [synced, setSynced] = useState(value);
  const timer = useRef<number | undefined>(undefined);
  // Back, forward and "reset" change the address: the field follows it, but
  // keeps what is being typed when only spaces differ.
  if (value !== synced) {
    setSynced(value);
    if (value !== draft.trim()) setDraft(value);
  }
  useEffect(
    () => () => {
      window.clearTimeout(timer.current);
    },
    [],
  );
  const change = (next: string, delay: number) => {
    setDraft(next);
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => {
      onChange(next.trim());
    }, delay);
  };
  return (
    <div className="min-w-0 flex-1">
      <label htmlFor="showcase-search" className="sr-only">
        {t('catalog.search.label')}
      </label>
      <Input
        id="showcase-search"
        type="search"
        enterKeyHint="search"
        value={draft}
        maxLength={LIMITS.search}
        placeholder={t('catalog.search.placeholder')}
        inputSize="lg"
        className="border-t-2 border-t-primary-500"
        leading={<Search aria-hidden />}
        trailing={
          draft ? (
            <Button
              variant="ghost"
              size="icon-sm"
              className="-mr-1.5"
              aria-label={t('common:actions.reset')}
              onClick={() => {
                change('', 0);
              }}
            >
              <X aria-hidden />
            </Button>
          ) : undefined
        }
        onChange={(event) => {
          change(event.target.value, 300);
        }}
      />
    </div>
  );
}

interface FilterPanelProps {
  search: ShowcaseSearch;
  facets: ShowcaseFacets | undefined;
  national: boolean;
  onChange: <K extends FilterKey>(key: K, value: ShowcaseSearch[K] | undefined) => void;
}

/** Filters with counts; the same panel sits in the sidebar and in the phone sheet. */
function FilterPanel({ search, facets, national, onChange }: FilterPanelProps) {
  const { t } = useTranslation('showcase');
  const text = useLocalized();
  const count = (list: { key: string; count: number }[] | undefined, key: string) =>
    list?.find((f) => f.key === key)?.count ?? 0;
  const option = (label: string, n: number) => t('catalog.filters.option', { label, count: n });
  const industries = INDUSTRIES.filter(
    (key) => count(facets?.industries, key) > 0 || search.industry === key,
  );
  const years = (facets?.years ?? []).map((y) => Number(y.key)).filter(Number.isInteger);
  const tags = (facets?.tags ?? []).slice(0, TOP_TAGS);

  return (
    <div className="flex flex-col gap-5">
      <FacetSelect
        label={t('catalog.filters.industry')}
        value={search.industry}
        options={industries.map((key) => ({
          value: key,
          label: option(t(`industries.${key}`), count(facets?.industries, key)),
        }))}
        onChange={(value) => {
          onChange('industry', value as ShowcaseSearch['industry']);
        }}
      />
      <FacetSelect
        label={t('catalog.filters.stage')}
        value={search.stage}
        options={STAGES.map((key) => ({
          value: key,
          label: option(t(`stages.${key}`), count(facets?.stages, key)),
        }))}
        onChange={(value) => {
          onChange('stage', value as ShowcaseSearch['stage']);
        }}
      />
      {!national && (
        <FacetSelect
          label={t('catalog.filters.faculty')}
          value={search.faculty}
          options={(facets?.faculties ?? []).map((f) => ({
            value: f.id,
            label: option(text(f.name), f.count),
          }))}
          onChange={(value) => {
            onChange('faculty', value);
          }}
        />
      )}
      <FacetSelect
        label={t('catalog.filters.year')}
        value={search.year === undefined ? undefined : String(search.year)}
        options={years.map((year) => ({
          value: String(year),
          label: option(String(year), count(facets?.years, String(year))),
        }))}
        onChange={(value) => {
          onChange('year', value === undefined ? undefined : Number(value));
        }}
      />
      <fieldset className="flex flex-col gap-2">
        <legend className="mb-2 text-sm font-medium text-fg">{t('catalog.filters.needs')}</legend>
        <div className="flex flex-wrap gap-2">
          {NEEDS.map((need) => {
            const selected = search.need === need;
            const n = count(facets?.needs, need);
            return (
              <Tag
                key={need}
                selected={selected}
                onToggle={() => {
                  onChange('need', selected ? undefined : need);
                }}
              >
                {t(`needs.${need}`)}
                <span className="text-xs text-fg-subtle tabular-nums">{n}</span>
              </Tag>
            );
          })}
        </div>
      </fieldset>
      {tags.length > 0 && (
        <fieldset className="flex flex-col gap-2">
          <legend className="mb-2 text-sm font-medium text-fg">{t('catalog.filters.tags')}</legend>
          <div className="flex flex-wrap gap-2">
            {tags.map(({ key, count: n }) => {
              const selected = search.tag === key;
              return (
                <Tag
                  key={key}
                  selected={selected}
                  onToggle={() => {
                    onChange('tag', selected ? undefined : key);
                  }}
                >
                  {key}
                  <span className="text-xs text-fg-subtle tabular-nums">{n}</span>
                </Tag>
              );
            })}
          </div>
        </fieldset>
      )}
    </div>
  );
}

function FacetSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string | undefined;
  options: { value: string; label: string }[];
  onChange: (value: string | undefined) => void;
}) {
  const { t } = useTranslation('showcase');
  return (
    <Field label={label}>
      <Select
        value={value ?? ALL}
        onValueChange={(next) => {
          onChange(next === ALL ? undefined : next);
        }}
      >
        <SelectTrigger>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ALL}>{t('catalog.filters.any')}</SelectItem>
          {options.map((o) => (
            <SelectItem key={o.value} value={o.value}>
              {o.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </Field>
  );
}
