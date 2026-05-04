import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://itervox.dev',
  integrations: [
    starlight({
      title: 'Itervox',
      logo: {
        src: './src/assets/logo.svg',
        replacesTitle: true,
      },
      description: 'Autonomous agents that narrate the fix. From issue to PR, every step visible.',
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/vnovick/itervox' },
      ],
      customCss: ['./src/styles/custom.css'],
      sidebar: [
        { label: 'Getting Started', slug: 'getting-started' },
        { label: 'Configuration', slug: 'configuration' },
        {
          label: 'Guides',
          items: [
            { label: "What's New in v0.2.0", slug: 'guides/whats-new-v020' },
            { label: 'Linear Setup', slug: 'guides/linear-setup' },
            { label: 'GitHub Issues', slug: 'guides/github-issues' },
            { label: 'Agent Profiles', slug: 'guides/agent-profiles' },
            { label: 'Automations', slug: 'guides/automations' },
            { label: 'Skills Inventory', slug: 'guides/skills-inventory' },
            { label: 'Remote Access & Mobile', slug: 'guides/remote-access' },
          ],
        },
        { label: 'CLI Reference', slug: 'cli' },
        { label: 'API Reference', slug: 'api-reference' },
      ],
    }),
  ],
});
