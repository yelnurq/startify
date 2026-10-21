import {
  ArrowRight,
  AtSign,
  Bookmark,
  Download,
  Globe,
  MoreHorizontal,
  Plus,
  Share2,
  Sparkles,
  Trash2,
  Trophy,
} from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Button,
  Checkbox,
  Field,
  IconButton,
  Input,
  modKey,
  Radio,
  RadioGroup,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Slider,
  Switch,
  Textarea,
} from '@/design-system';

import { Section, Specimen } from '../section';

export function ButtonsSection({ index }: { index: number }) {
  const { t } = useTranslation('design');
  return (
    <Section id="buttons" index={index} title={t('buttons.title')} lead={t('buttons.lead')}>
      <Specimen label={t('buttons.variants')} bodyClassName="flex flex-wrap items-center gap-3">
        <Button leftIcon={<Plus />}>{t('buttons.primary')}</Button>
        <Button variant="secondary">{t('buttons.secondary')}</Button>
        <Button variant="soft" leftIcon={<Sparkles />}>
          {t('buttons.soft')}
        </Button>
        <Button variant="outline">{t('buttons.outline')}</Button>
        <Button variant="ghost">{t('buttons.ghost')}</Button>
        <Button variant="danger" leftIcon={<Trash2 />}>
          {t('buttons.danger')}
        </Button>
        <Button variant="gold" leftIcon={<Trophy />}>
          {t('buttons.gold')}
        </Button>
        <Button variant="link">{t('buttons.link')}</Button>
      </Specimen>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Specimen label={t('buttons.sizes')} bodyClassName="flex flex-wrap items-center gap-3">
          <Button size="sm">{t('buttons.small')}</Button>
          <Button size="md">{t('buttons.medium')}</Button>
          <Button size="lg" rightIcon={<ArrowRight />}>
            {t('buttons.large')}
          </Button>
        </Specimen>
        <Specimen label={t('buttons.states')} bodyClassName="flex flex-wrap items-center gap-3">
          <Button loading>{t('buttons.loading')}</Button>
          <Button disabled>{t('buttons.disabled')}</Button>
          <Button variant="secondary" leftIcon={<Download />}>
            {t('buttons.download')}
          </Button>
        </Specimen>
      </div>
      <Specimen label={t('buttons.iconButtons')} bodyClassName="flex flex-wrap items-center gap-2">
        <IconButton label={t('buttons.share')} icon={<Share2 />} variant="secondary" />
        <IconButton label={t('buttons.favorite')} icon={<Bookmark />} variant="secondary" />
        <IconButton label={t('buttons.more')} icon={<MoreHorizontal />} />
        <IconButton
          label={t('buttons.ask')}
          icon={<Sparkles />}
          variant="soft"
          shortcut={[modKey(), 'J']}
        />
      </Specimen>
    </Section>
  );
}

const INDUSTRIES = ['agro', 'edtech', 'fintech', 'logistics', 'medtech', 'ecology'] as const;

export function FormsSection({ index }: { index: number }) {
  const { t } = useTranslation('design');
  const [title, setTitle] = useState('AgroSense');
  const [problem, setProblem] = useState('');
  const [email, setEmail] = useState('aigerim@kazutb');
  const [score, setScore] = useState([7]);
  const [visibility, setVisibility] = useState('university');
  const emailInvalid = !/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(email);
  return (
    <Section id="forms" index={index} title={t('forms.title')} lead={t('forms.lead')}>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Specimen label={t('forms.textFields')} bodyClassName="flex flex-col gap-5">
          <Field
            label={t('forms.projectTitle')}
            hint={t('forms.projectTitleHint')}
            required
            counter={{ value: title.length, max: 40 }}
          >
            <Input
              value={title}
              placeholder={t('forms.projectTitlePlaceholder')}
              onChange={(e) => {
                setTitle(e.target.value);
              }}
            />
          </Field>
          <Field label={t('forms.problem')} counter={{ value: problem.length, max: 600 }}>
            <Textarea
              value={problem}
              rows={3}
              placeholder={t('forms.problemPlaceholder')}
              onChange={(e) => {
                setProblem(e.target.value);
              }}
            />
          </Field>
          <Field label={t('forms.email')} error={emailInvalid ? t('forms.emailError') : undefined}>
            <Input
              type="email"
              value={email}
              leading={<AtSign />}
              onChange={(e) => {
                setEmail(e.target.value);
              }}
            />
          </Field>
          <Field label={t('forms.website')} optional>
            <Input type="url" placeholder={t('forms.websitePlaceholder')} leading={<Globe />} />
          </Field>
        </Specimen>
        <Specimen label={t('forms.choices')} bodyClassName="flex flex-col gap-6">
          <Field label={t('forms.industry')}>
            <Select defaultValue="agro">
              <SelectTrigger>
                <SelectValue placeholder={t('forms.industryPlaceholder')} />
              </SelectTrigger>
              <SelectContent>
                {INDUSTRIES.map((key) => (
                  <SelectItem key={key} value={key}>
                    {t(`forms.industries.${key}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          <fieldset className="flex flex-col gap-3">
            <legend className="mb-3 text-sm font-medium text-fg">{t('forms.visibility')}</legend>
            <RadioGroup value={visibility} onValueChange={setVisibility}>
              {(['private', 'university', 'public'] as const).map((value) => (
                <label key={value} className="flex items-center gap-3 text-sm text-fg">
                  <Radio value={value} />
                  {t(`forms.visibilityOptions.${value}`)}
                </label>
              ))}
            </RadioGroup>
          </fieldset>
          <div className="flex flex-col gap-3">
            <label className="flex items-center justify-between gap-4 text-sm text-fg">
              {t('forms.notify')}
              <Switch defaultChecked />
            </label>
            <label className="flex items-center justify-between gap-4 text-sm text-fg">
              {t('forms.hideDetails')}
              <Switch />
            </label>
          </div>
          <label className="flex items-start gap-3 text-sm text-fg">
            <Checkbox defaultChecked className="mt-0.5" />
            {t('forms.consent')}
          </label>
          <div className="flex flex-col gap-3">
            <div className="flex items-center justify-between text-sm">
              <span id="score-label" className="font-medium text-fg">
                {t('forms.score')}
              </span>
              <span className="font-semibold text-primary-text tabular-nums">
                {t('forms.scoreValue', { value: score[0] ?? 0 })}
              </span>
            </div>
            <Slider
              value={score}
              onValueChange={setScore}
              max={10}
              step={1}
              thumbLabels={[t('forms.score')]}
              aria-labelledby="score-label"
            />
          </div>
          <Field label={t('forms.faculty')}>
            <Input disabled value={t('forms.facultyValue')} readOnly />
          </Field>
        </Specimen>
      </div>
    </Section>
  );
}
