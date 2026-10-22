import type { IncubatorTemplate, StepStatus, TemplateStage, TemplateStep } from './api';
import type { TemplateField } from './values';

export interface FlatStep extends TemplateStep {
  stageIndex: number;
}

/** Every step in order, with the index of its stage. */
export function flattenSteps(template: IncubatorTemplate): FlatStep[] {
  return template.stages.flatMap((stage, stageIndex) =>
    stage.steps.map((step) => ({ ...step, stageIndex })),
  );
}

export function findStep(template: IncubatorTemplate, key: string): FlatStep | undefined {
  return flattenSteps(template).find((step) => step.key === key);
}

export function stageOf(template: IncubatorTemplate, key: string): TemplateStage | undefined {
  return template.stages.find((stage) => stage.steps.some((step) => step.key === key));
}

export function neighbours(
  template: IncubatorTemplate,
  key: string,
): { previous?: FlatStep; next?: FlatStep } {
  const steps = flattenSteps(template);
  const index = steps.findIndex((step) => step.key === key);
  return { previous: steps[index - 1], next: steps[index + 1] };
}

export function isDone(status: StepStatus | undefined): boolean {
  return status === 'completed' || status === 'approved';
}

/** Fields a mentor suggestion can be written into (text-like answers). */
export function suggestible(field: TemplateField): boolean {
  return (
    field.type === 'text' ||
    field.type === 'textarea' ||
    field.type === 'list' ||
    field.type === 'url'
  );
}
