import { useMemo, useState } from 'react';
import type { AutomationDef } from '../../types/schemas';
import { Card } from '../../components/ui/Card/Card';
import { AutomationFormModal } from './automations/AutomationFormModal';
import { AutomationRow } from './automations/AutomationRow';
import { SuggestedAutomationCard } from './automations/SuggestedAutomationCard';
import {
  SUGGESTED_AUTOMATIONS,
  type SuggestedAutomation,
} from './automations/suggestedAutomations';
import {
  automationDefFromValues,
  automationValuesFromDef,
  automationValuesFromSuggestion,
  emptyAutomationValues,
  type AutomationFormValues,
} from './automations/automationForm';

type AutomationModalState = {
  title: string;
  subtitle?: string;
  submitLabel: string;
  index: number | null;
  initialValues: AutomationFormValues;
};

type AutomationStatus = {
  kind: 'success' | 'error';
  message: string;
};

export function AutomationsCard({
  automations,
  availableProfiles,
  availableStates,
  availableLabels,
  onSave,
}: {
  automations: AutomationDef[];
  availableProfiles: string[];
  availableStates: string[];
  availableLabels: string[];
  onSave: (automations: AutomationDef[]) => Promise<boolean>;
}) {
  const [modalState, setModalState] = useState<AutomationModalState | null>(null);
  const [status, setStatus] = useState<AutomationStatus | null>(null);

  const automationList = useMemo(
    () => [...automations].sort((a, b) => a.id.localeCompare(b.id)),
    [automations],
  );
  const suggestedToShow = useMemo(
    () =>
      SUGGESTED_AUTOMATIONS.filter(
        (suggestion) => !automations.some((item) => item.id === suggestion.id),
      ),
    [automations],
  );

  const openAddModal = () => {
    setModalState({
      title: 'Add Automation',
      subtitle: 'Lightweight cron and event-driven helpers before the full workflow canvas lands.',
      submitLabel: 'Create Automation',
      index: null,
      initialValues: emptyAutomationValues(availableProfiles[0], automations),
    });
  };

  const openEditModal = (index: number, automation: AutomationDef) => {
    setModalState({
      title: `Edit "${automation.id}"`,
      subtitle: 'Update the trigger, selected profile, and automation-specific instructions.',
      submitLabel: 'Save Changes',
      index,
      initialValues: automationValuesFromDef(automation),
    });
  };

  const openTemplateModal = (suggestion: SuggestedAutomation) => {
    if (!availableProfiles.includes(suggestion.profile)) {
      setStatus({
        kind: 'error',
        message: `Template "${suggestion.label}" requires the "${suggestion.profile}" profile.`,
      });
      return;
    }
    setModalState({
      title: `Use "${suggestion.label}" Template`,
      subtitle: suggestion.description,
      submitLabel: 'Create Automation',
      index: null,
      initialValues: automationValuesFromSuggestion(suggestion),
    });
  };

  const saveAutomations = async (nextAutomations: AutomationDef[]) => {
    const seen = new Set<string>();
    for (const automation of nextAutomations) {
      const key = automation.id.trim().toLowerCase();
      if (seen.has(key)) {
        setStatus({ kind: 'error', message: 'Automation IDs must be unique.' });
        return false;
      }
      seen.add(key);
    }
    setStatus(null);
    const ok = await onSave(nextAutomations);
    if (ok) {
      setStatus({
        kind: 'success',
        message: 'Saved to WORKFLOW.md. The daemon will reload automations shortly.',
      });
    }
    return ok;
  };

  return (
    <>
      <Card variant="elevated" padding="none" className="overflow-hidden">
        <Card.Header className="bg-theme-panel-strong mb-0 flex items-center justify-between gap-4 px-5 py-4">
          <div>
            <h2 className="text-theme-text text-sm font-semibold">Automations</h2>
            <p className="text-theme-text-secondary mt-0.5 text-xs">
              Cron and event-driven helper rules layered on top of your agent profiles.
            </p>
          </div>
          <button
            onClick={openAddModal}
            disabled={availableProfiles.length === 0}
            className="bg-theme-accent rounded-[var(--radius-sm)] px-4 py-2 text-sm font-medium whitespace-nowrap text-white transition-colors hover:opacity-90 disabled:opacity-50"
          >
            Add Automation
          </button>
        </Card.Header>

        {availableProfiles.length === 0 && (
          <div className="px-5 py-4">
            <p className="text-theme-danger text-xs">
              Create at least one agent profile before adding automations.
            </p>
          </div>
        )}

        {automationList.length === 0 ? (
          <div className="px-5 py-8">
            <div className="border-theme-line bg-theme-bg-soft rounded-[var(--radius-sm)] border border-dashed px-4 py-6 text-center">
              <p className="text-theme-muted text-xs">No automations configured.</p>
            </div>
          </div>
        ) : (
          <div>
            {automationList.map((automation, index) => (
              <AutomationRow
                key={automation.id}
                automation={automation}
                onEdit={() => {
                  openEditModal(index, automation);
                }}
                onDelete={async () => {
                  await saveAutomations(automationList.filter((_, rowIndex) => rowIndex !== index));
                }}
              />
            ))}
          </div>
        )}

        {suggestedToShow.length > 0 && (
          <Card.Footer className="mt-0 px-5 py-4">
            <p className="text-theme-muted mb-3 text-[11px] font-medium tracking-wider uppercase">
              Start From A Template
            </p>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
              {suggestedToShow.map((suggestion) => (
                <SuggestedAutomationCard
                  key={suggestion.id}
                  suggestion={suggestion}
                  disabled={!availableProfiles.includes(suggestion.profile)}
                  onUse={openTemplateModal}
                />
              ))}
            </div>
          </Card.Footer>
        )}
      </Card>

      {status && (
        <p
          className={
            status.kind === 'error' ? 'text-theme-danger text-sm' : 'text-theme-success text-sm'
          }
        >
          {status.message}
        </p>
      )}

      {modalState && (
        <AutomationFormModal
          key={modalState.initialValues.id}
          isOpen
          title={modalState.title}
          subtitle={modalState.subtitle}
          submitLabel={modalState.submitLabel}
          initialValues={modalState.initialValues}
          availableProfiles={availableProfiles}
          availableStates={availableStates}
          availableLabels={availableLabels}
          onClose={() => {
            setModalState(null);
          }}
          onSubmit={async (values) => {
            const nextAutomation = automationDefFromValues(values);
            const nextAutomations =
              modalState.index === null
                ? [...automationList, nextAutomation]
                : automationList.map((automation, index) =>
                    index === modalState.index ? nextAutomation : automation,
                  );
            return saveAutomations(nextAutomations);
          }}
        />
      )}
    </>
  );
}
