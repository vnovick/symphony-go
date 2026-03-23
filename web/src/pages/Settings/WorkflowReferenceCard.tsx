export function WorkflowReferenceCard() {
  return (
    <div className="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-gray-800 dark:bg-white/[0.03]">
      <div className="border-b border-gray-100 bg-gray-50 px-6 py-4 dark:border-gray-800 dark:bg-gray-900/40">
        <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-300">
          WORKFLOW.md profile syntax
        </h2>
      </div>
      <div className="px-6 py-5">
        <pre className="overflow-x-auto rounded-xl bg-gray-100 p-4 font-mono text-xs leading-relaxed text-gray-800 dark:bg-gray-900 dark:text-gray-200">{`agent:
  command: claude          # default command for unassigned issues
  max_concurrent_agents: 3
  profiles:
    fast:
      command: claude --model claude-haiku-4-5-20251001
      prompt: "Fast executor for simple, well-scoped tasks."
    codex-research:
      command: codex --model gpt-5.2-codex
      backend: codex
      prompt: "Long-horizon coding and investigation agent."
    researcher:
      command: claude --model claude-opus-4-6
      prompt: "Deep research and analysis agent."`}</pre>
        <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">
          Changes are hot-reloaded without restarting. Assign profiles to issues manually from the
          dashboard.
        </p>
      </div>
    </div>
  );
}
