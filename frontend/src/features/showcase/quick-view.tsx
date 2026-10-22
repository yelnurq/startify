import { useQuery } from '@tanstack/react-query';
import { ArrowRight, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import {
  Badge,
  Button,
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetTitle,
  SkeletonText,
} from '@/design-system';
import { useLocalized } from '@/shared/i18n/localized';

import { showcaseProjectQuery, type ShowcaseCard } from './api';
import { CardCover, ProjectLink, ScoreBadge, UniversityChip } from './card';
import { excerpt } from './logic';

interface QuickViewProps {
  card: ShowcaseCard | null;
  external: boolean;
  onClose: () => void;
}

/** A project at a glance without leaving the catalogue. */
export function QuickView({ card, external, onClose }: QuickViewProps) {
  return (
    <Sheet
      open={card !== null}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <SheetContent side="right" size="md" hideClose>
        {card && <QuickViewBody card={card} external={external} />}
      </SheetContent>
    </Sheet>
  );
}

function QuickViewBody({ card, external }: { card: ShowcaseCard; external: boolean }) {
  const { t } = useTranslation(['showcase', 'common']);
  const text = useLocalized();
  // Pages of other universities live on their own sites: only the card is known here.
  const page = useQuery({ ...showcaseProjectQuery(card.slug), enabled: !external });
  const sections = page.data?.sections;
  return (
    <>
      <SheetClose asChild>
        <Button
          variant="secondary"
          size="icon-sm"
          aria-label={t('common:actions.close')}
          className="absolute top-3 right-3 z-10 shadow-md"
        >
          <X aria-hidden />
        </Button>
      </SheetClose>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="aspect-video overflow-hidden bg-surface-sunken">
          <CardCover title={card.title} industry={card.industry} coverUrl={card.cover_url} />
        </div>
        <div className="flex flex-col gap-5 p-5">
          <div className="flex flex-col gap-3">
            <div className="flex flex-wrap items-center gap-1.5">
              <Badge tone="primary" size="sm">
                {t(`industries.${card.industry}`)}
              </Badge>
              <Badge size="sm">{t(`stages.${card.stage}`)}</Badge>
              <ScoreBadge score={card.score} level={card.level} size="sm" />
            </div>
            <SheetTitle className="pr-10 font-display text-2xl leading-tight font-bold text-ink">
              {card.title}
            </SheetTitle>
            <SheetDescription className="text-base leading-relaxed text-fg-muted">
              {card.tagline || t('quick.description')}
            </SheetDescription>
            <div className="flex flex-wrap items-center gap-2 text-sm text-fg-subtle">
              <UniversityChip university={card.university} />
              {text(card.faculty) && <span>{text(card.faculty)}</span>}
              <span className="tabular-nums">{card.year}</span>
            </div>
          </div>

          {external ? (
            <p className="text-sm text-fg-muted">{t('quick.external')}</p>
          ) : page.isPending ? (
            <SkeletonText lines={5} />
          ) : (
            sections && (
              <dl className="flex flex-col gap-4">
                {sections.problem && (
                  <div className="flex flex-col gap-1">
                    <dt className="text-sm font-semibold text-fg">{t('quick.problem')}</dt>
                    <dd className="text-sm leading-relaxed text-fg-muted">
                      {excerpt(sections.problem, 320)}
                    </dd>
                  </div>
                )}
                {sections.solution && (
                  <div className="flex flex-col gap-1">
                    <dt className="text-sm font-semibold text-fg">{t('quick.solution')}</dt>
                    <dd className="text-sm leading-relaxed text-fg-muted">
                      {excerpt(sections.solution, 320)}
                    </dd>
                  </div>
                )}
              </dl>
            )
          )}

          {card.needs.length > 0 && (
            <div className="flex flex-col gap-2">
              <p className="text-sm font-semibold text-fg">{t('quick.needs')}</p>
              <ul className="flex flex-wrap gap-1.5">
                {card.needs.map((need) => (
                  <li key={need}>
                    <Badge tone="gold">{t(`needsText.${need}`)}</Badge>
                  </li>
                ))}
              </ul>
            </div>
          )}

          {card.tags.length > 0 && (
            <ul className="flex flex-wrap gap-1.5">
              {card.tags.map((tag) => (
                <li key={tag}>
                  <Badge size="sm">{tag}</Badge>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
      <div className="border-t border-border p-4">
        <Button asChild block>
          <ProjectLink card={card} external={external}>
            {t('quick.open')}
            <ArrowRight aria-hidden />
          </ProjectLink>
        </Button>
      </div>
    </>
  );
}
