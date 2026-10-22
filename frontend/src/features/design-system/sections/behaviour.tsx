import {
  Archive,
  CalendarClock,
  Command as CommandIcon,
  Copy,
  FileDown,
  Info,
  MoreHorizontal,
  PartyPopper,
  Pencil,
  Send,
  Trash2,
} from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Badge,
  Button,
  Callout,
  celebrateStage,
  ConfirmDialog,
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Field,
  Input,
  Kbd,
  modKey,
  Popover,
  PopoverContent,
  PopoverTrigger,
  Progress,
  ProgressRing,
  ScoreGauge,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Sheet,
  SheetContent,
  SheetDescription,
  SheetTitle,
  SheetTrigger,
  SkeletonCard,
  Spinner,
  toast,
  Tooltip,
  useCommandPalette,
} from '@/design-system';
import { useFormatters } from '@/shared/i18n/format';

import { sampleProjects } from '../sample-data';
import { Section, Specimen } from '../section';

// Fixed at load time: rendering must stay pure.
const DEADLINE = new Date(Date.now() + 5 * 24 * 3600 * 1000);

export function OverlaysSection({ index }: { index: number }) {
  const { t } = useTranslation('design');
  const format = useFormatters();
  const openPalette = useCommandPalette((s) => s.setOpen);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const project = sampleProjects[0];

  return (
    <Section id="overlays" index={index} title={t('overlays.title')} lead={t('overlays.lead')}>
      <Specimen label={t('overlays.label')} bodyClassName="flex flex-wrap items-center gap-3">
        <Dialog>
          <DialogTrigger asChild>
            <Button leftIcon={<Send />}>{t('overlays.invite')}</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('overlays.inviteTitle')}</DialogTitle>
              <DialogDescription>{t('overlays.inviteDescription')}</DialogDescription>
            </DialogHeader>
            <DialogBody className="flex flex-col gap-4">
              <Field label={t('overlays.inviteEmail')} required>
                <Input type="email" placeholder={t('overlays.inviteEmailPlaceholder')} />
              </Field>
              <Field label={t('overlays.role')}>
                <Select defaultValue="designer">
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {(['developer', 'designer', 'marketer'] as const).map((role) => (
                      <SelectItem key={role} value={role}>
                        {t(`overlays.roles.${role}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
            </DialogBody>
            <DialogFooter>
              <Button
                onClick={() => {
                  toast.success(t('overlays.inviteSent'));
                }}
              >
                {t('overlays.send')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <Button
          variant="secondary"
          leftIcon={<Trash2 />}
          onClick={() => {
            setConfirmOpen(true);
          }}
        >
          {t('overlays.deleteDraft')}
        </Button>
        <ConfirmDialog
          open={confirmOpen}
          onOpenChange={setConfirmOpen}
          tone="danger"
          title={t('overlays.confirmTitle')}
          description={t('overlays.confirmDescription')}
          confirmLabel={t('overlays.confirmAction')}
          loading={deleting}
          onConfirm={() => {
            setDeleting(true);
            window.setTimeout(() => {
              setDeleting(false);
              setConfirmOpen(false);
              toast(t('overlays.deleted'), {
                action: {
                  label: t('feedback.undo'),
                  onClick: () => {
                    toast.success(t('overlays.restored'));
                  },
                },
              });
            }, 700);
          }}
        />

        <Sheet>
          <SheetTrigger asChild>
            <Button variant="secondary">{t('overlays.quickView')}</Button>
          </SheetTrigger>
          <SheetContent>
            <div className={`h-40 shrink-0 bg-linear-to-br ${project?.cover ?? ''}`} />
            <div className="flex flex-col gap-3 p-6">
              <SheetTitle className="font-display text-2xl font-bold text-ink">
                {project?.name}
              </SheetTitle>
              <SheetDescription className="text-sm leading-relaxed text-fg-muted">
                {t('cards.projects.agro.tagline')}
              </SheetDescription>
              <div className="flex gap-2">
                <Badge tone="primary">{t('cards.projects.agro.stage')}</Badge>
                <Badge tone="neutral">{t('cards.projects.agro.industry')}</Badge>
              </div>
            </div>
          </SheetContent>
        </Sheet>

        <Popover>
          <PopoverTrigger asChild>
            <Button variant="secondary" leftIcon={<CalendarClock />}>
              {t('overlays.deadline')}
            </Button>
          </PopoverTrigger>
          <PopoverContent className="flex flex-col gap-2">
            <p className="font-semibold text-fg">{t('overlays.deadlineTitle')}</p>
            <p className="text-fg-muted">
              {t('overlays.deadlineText', {
                date: format.dateTime(DEADLINE, { dateStyle: 'long' }),
              })}
            </p>
            <Progress value={72} size="sm" label={t('overlays.deadlineTitle')} />
          </PopoverContent>
        </Popover>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="secondary" size="icon" aria-label={t('overlays.menu')}>
              <MoreHorizontal aria-hidden />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start">
            <DropdownMenuItem icon={<Pencil />} shortcut="E">
              {t('overlays.menuItems.edit')}
            </DropdownMenuItem>
            <DropdownMenuItem icon={<Copy />}>{t('overlays.menuItems.duplicate')}</DropdownMenuItem>
            <DropdownMenuItem icon={<FileDown />}>
              {t('overlays.menuItems.export')}
            </DropdownMenuItem>
            <DropdownMenuItem icon={<Archive />}>
              {t('overlays.menuItems.archive')}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem icon={<Trash2 />} destructive>
              {t('overlays.menuItems.delete')}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>

        <Tooltip content={t('overlays.tooltipText')}>
          <Button variant="ghost" size="icon" aria-label={t('overlays.tooltip')}>
            <Info aria-hidden />
          </Button>
        </Tooltip>

        <Button
          variant="soft"
          leftIcon={<CommandIcon />}
          onClick={() => {
            openPalette(true);
          }}
        >
          {t('overlays.palette')}
          <span className="ml-1 flex gap-0.5 pointer-coarse:hidden">
            <Kbd>{modKey()}</Kbd>
            <Kbd>K</Kbd>
          </span>
        </Button>
      </Specimen>
    </Section>
  );
}

export function FeedbackSection({ index }: { index: number }) {
  const { t } = useTranslation('design');
  return (
    <Section id="feedback" index={index} title={t('feedback.title')} lead={t('feedback.lead')}>
      <Specimen label={t('feedback.toasts')} bodyClassName="flex flex-wrap gap-3">
        <Button
          variant="secondary"
          onClick={() => {
            toast.success(t('feedback.saved'), { description: t('feedback.savedDescription') });
          }}
        >
          {t('feedback.showSuccess')}
        </Button>
        <Button
          variant="secondary"
          onClick={() => {
            toast(t('feedback.commentDeleted'), {
              action: {
                label: t('feedback.undo'),
                onClick: () => {
                  toast.success(t('feedback.commentRestored'));
                },
              },
            });
          }}
        >
          {t('feedback.showUndo')}
        </Button>
        <Button
          variant="secondary"
          onClick={() => {
            toast.error(t('feedback.saveFailed'), {
              description: t('feedback.saveFailedDescription'),
            });
          }}
        >
          {t('feedback.showError')}
        </Button>
        <Button
          variant="secondary"
          onClick={() => {
            toast.info(t('feedback.evaluationReady'), {
              description: t('feedback.evaluationReadyDescription'),
            });
          }}
        >
          {t('feedback.showInfo')}
        </Button>
      </Specimen>
      <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
        <Callout tone="ai" title={t('feedback.calloutAiTitle')}>
          {t('feedback.calloutAi')}
        </Callout>
        <Callout tone="info" title={t('feedback.calloutInfoTitle')}>
          {t('feedback.calloutInfo')}
        </Callout>
        <Callout tone="success" title={t('feedback.calloutSuccessTitle')}>
          {t('feedback.calloutSuccess')}
        </Callout>
        <Callout tone="warning" title={t('feedback.calloutWarningTitle')}>
          {t('feedback.calloutWarning')}
        </Callout>
        <Callout
          tone="danger"
          title={t('feedback.calloutDangerTitle')}
          className="lg:col-span-2"
          action={
            <Button size="sm" variant="secondary">
              {t('feedback.retry')}
            </Button>
          }
        >
          {t('feedback.calloutDanger')}
        </Callout>
      </div>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)]">
        <Specimen label={t('feedback.progress')} bodyClassName="flex flex-col gap-5">
          {[
            { label: t('feedback.stage1'), value: 100, tone: 'success' as const },
            { label: t('feedback.stage2'), value: 62, tone: 'brand' as const },
            { label: t('feedback.stage3'), value: 15, tone: 'brand' as const },
          ].map((row) => (
            <div key={row.label} className="flex flex-col gap-2">
              <div className="flex justify-between text-sm">
                <span className="text-fg">{row.label}</span>
                <span className="font-semibold text-fg-muted tabular-nums">{row.value}%</span>
              </div>
              <Progress value={row.value} tone={row.tone} label={row.label} />
            </div>
          ))}
          <div className="flex items-center gap-5 pt-1">
            <ProgressRing value={62} label={t('feedback.overall')} size={64}>
              {62}%
            </ProgressRing>
            <div className="flex items-center gap-2 text-sm text-fg-muted">
              <Spinner className="text-brand" />
              {t('feedback.saving')}
            </div>
          </div>
          <Button
            variant="gold"
            leftIcon={<PartyPopper />}
            className="self-start"
            onClick={() => {
              celebrateStage();
              toast.success(t('feedback.stageDone'));
            }}
          >
            {t('feedback.celebrate')}
          </Button>
        </Specimen>
        <Specimen label={t('feedback.skeleton')} bodyClassName="bg-bg">
          <SkeletonCard />
        </Specimen>
      </div>
    </Section>
  );
}

export function GaugeSpecimen() {
  const { t } = useTranslation('design');
  return (
    <ScoreGauge
      value={74}
      caption={t('data.gaugeLevel')}
      label={t('data.gaugeLabel', { value: 74, level: t('data.gaugeLevel') })}
    />
  );
}
