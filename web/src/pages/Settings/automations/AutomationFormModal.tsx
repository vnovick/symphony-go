import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Modal, ModalFooter } from '../../../components/ui/modal';
import { inputCls } from '../formStyles';
import { AutomationEditorFields } from './AutomationEditorFields';
import { automationFormSchema, type AutomationFormValues } from './automationForm';

export function AutomationFormModal({
  isOpen,
  title,
  subtitle,
  submitLabel,
  initialValues,
  availableProfiles,
  availableStates,
  availableLabels,
  onClose,
  onSubmit,
}: {
  isOpen: boolean;
  title: string;
  subtitle?: string;
  submitLabel: string;
  initialValues: AutomationFormValues;
  availableProfiles: string[];
  availableStates: string[];
  availableLabels: string[];
  onClose: () => void;
  onSubmit: (values: AutomationFormValues) => Promise<boolean>;
}) {
  const {
    register,
    handleSubmit,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<AutomationFormValues>({
    resolver: zodResolver(automationFormSchema),
    defaultValues: initialValues,
  });

  const values = watch();

  const submit = handleSubmit(async (nextValues) => {
    const ok = await onSubmit(nextValues);
    if (ok) {
      onClose();
    }
  });

  return (
    <Modal isOpen={isOpen} onClose={onClose} showCloseButton padded className="mx-4 my-8 max-w-5xl">
      <div className="space-y-5">
        <div>
          <h2 className="text-theme-text text-lg font-semibold">{title}</h2>
          {subtitle && <p className="text-theme-text-secondary mt-1 text-sm">{subtitle}</p>}
        </div>

        <form
          onSubmit={(event) => {
            void submit(event);
          }}
          className="space-y-4"
        >
          <div>
            <label className="text-theme-text-secondary mb-2 block text-xs font-medium tracking-wider uppercase">
              Automation ID
            </label>
            <input
              {...register('id')}
              autoFocus
              placeholder="automation-name"
              className={`${inputCls} font-mono text-sm`}
            />
            {errors.id && (
              <p role="alert" className="text-theme-danger mt-1 text-xs">
                {errors.id.message}
              </p>
            )}
          </div>

          <AutomationEditorFields
            values={values}
            availableProfiles={availableProfiles}
            availableStates={availableStates}
            availableLabels={availableLabels}
            onEnabledChange={(value) => {
              setValue('enabled', value, { shouldValidate: true });
            }}
            onProfileChange={(value) => {
              setValue('profile', value, { shouldValidate: true });
            }}
            onInstructionsChange={(value) => {
              setValue('instructions', value);
            }}
            onTriggerTypeChange={(value) => {
              setValue('triggerType', value, { shouldValidate: true });
              if (value !== 'issue_entered_state') {
                setValue('triggerState', '');
              }
              if (value !== 'input_required') {
                setValue('inputContextRegex', '');
                setValue('autoResume', false);
              }
            }}
            onTriggerStateChange={(value) => {
              setValue('triggerState', value, { shouldValidate: true });
            }}
            onCronChange={(value) => {
              setValue('cron', value, { shouldValidate: true });
            }}
            onTimezoneChange={(value) => {
              setValue('timezone', value);
            }}
            onMatchModeChange={(value) => {
              setValue('matchMode', value, { shouldValidate: true });
            }}
            onStatesChange={(value) => {
              setValue('states', value, { shouldValidate: true });
            }}
            onLabelsAnyChange={(value) => {
              setValue('labelsAny', value, { shouldValidate: true });
            }}
            onIdentifierRegexChange={(value) => {
              setValue('identifierRegex', value);
            }}
            onLimitChange={(value) => {
              setValue('limit', value, { shouldValidate: true });
            }}
            onInputContextRegexChange={(value) => {
              setValue('inputContextRegex', value);
            }}
            onAutoResumeChange={(value) => {
              setValue('autoResume', value, { shouldValidate: true });
            }}
          />

          {errors.profile && (
            <p role="alert" className="text-theme-danger text-xs">
              {errors.profile.message}
            </p>
          )}
          {errors.cron && (
            <p role="alert" className="text-theme-danger text-xs">
              {errors.cron.message}
            </p>
          )}
          {errors.triggerState && (
            <p role="alert" className="text-theme-danger text-xs">
              {errors.triggerState.message}
            </p>
          )}
          {errors.limit && (
            <p role="alert" className="text-theme-danger text-xs">
              {errors.limit.message}
            </p>
          )}

          <ModalFooter>
            <button
              type="button"
              onClick={onClose}
              className="border-theme-line text-theme-text-secondary rounded-[var(--radius-sm)] border px-4 py-2 text-sm transition-colors hover:opacity-80"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="bg-theme-accent rounded-[var(--radius-sm)] px-4 py-2 text-sm font-medium text-white transition-colors disabled:opacity-50"
            >
              {isSubmitting ? 'Saving…' : submitLabel}
            </button>
          </ModalFooter>
        </form>
      </div>
    </Modal>
  );
}
