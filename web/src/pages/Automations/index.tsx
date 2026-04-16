import PageMeta from '../../components/common/PageMeta';
import { Card } from '../../components/ui/Card/Card';
import { AutomationsCard } from '../Settings/AutomationsCard';
import { useSettingsPageData } from '../Settings/useSettingsPageData';

export default function Automations() {
  const {
    automations,
    automationProfileOptions,
    trackerStateOptions,
    automationLabelOptions,
    setAutomations,
  } = useSettingsPageData();

  return (
    <>
      <PageMeta
        title="Itervox | Automations"
        description="Itervox automations — cron and event-driven helpers that will evolve into canvas"
      />
      <div className="w-full max-w-none space-y-8">
        <div>
          <h1 className="text-theme-text text-2xl font-bold tracking-tight">Automations</h1>
          <p className="text-theme-muted mt-1 text-sm">
            Configure cron and event-driven helper runs. This page is the stepping stone toward the
            future Canvas workflow surface.
          </p>
        </div>

        <Card variant="elevated" className="space-y-2">
          <p className="text-theme-text text-sm font-medium">Automation scope</p>
          <p className="text-theme-muted text-sm leading-relaxed">
            Use this page for practical automations today: scheduled QA checks, backlog review, and
            helper agents that react to input-required events. The broader visual workflow canvas
            will build on top of this surface later.
          </p>
        </Card>

        <AutomationsCard
          automations={automations}
          availableProfiles={automationProfileOptions}
          availableStates={trackerStateOptions}
          availableLabels={automationLabelOptions}
          onSave={setAutomations}
        />
      </div>
    </>
  );
}
