import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://infrashift.github.io',
  base: '/chit',
  integrations: [
    starlight({
      title: 'Chit',
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/infrashift/chit',
        },
      ],
      editLink: {
        baseUrl: 'https://github.com/infrashift/chit/edit/main/docs/',
      },
      customCss: ['./src/styles/custom.css'],
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            { label: 'Installation', slug: 'getting-started/installation' },
            { label: 'Configuration', slug: 'getting-started/configuration' },
            { label: 'Quick Start', slug: 'getting-started/quick-start' },
            { label: 'Manual Acceptance Testing', slug: 'getting-started/manual-acceptance-testing' },
          ],
        },
        {
          label: 'Architecture',
          items: [
            { label: 'Overview', slug: 'architecture/overview' },
            { label: 'WebSocket', slug: 'architecture/websocket' },
            { label: 'Pub/Sub', slug: 'architecture/pubsub' },
          ],
        },
        {
          label: 'Deployment',
          items: [
            { label: 'Podman Kube', slug: 'deployment/podman-kube' },
            { label: 'Ory Stack', slug: 'deployment/ory-stack' },
            { label: 'User Management', slug: 'deployment/user-management' },
            { label: 'Headless Claude Code', slug: 'deployment/headless-claude' },
          ],
        },
        {
          label: 'API Reference',
          items: [
            { label: 'REST API', slug: 'api-reference/rest-api' },
            { label: 'WebSocket Protocol', slug: 'api-reference/websocket-protocol' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'Configuration', slug: 'reference/configuration' },
            { label: 'Database Schema', slug: 'reference/database-schema' },
            { label: 'MCP and Slash Commands', slug: 'reference/mcp-and-commands' },
          ],
        },
        {
          label: 'TUI Client',
          items: [
            { label: 'Overview', slug: 'tui' },
            { label: 'Installation', slug: 'tui/getting-started/installation' },
            { label: 'Configuration', slug: 'tui/getting-started/configuration' },
            { label: 'Quick Start', slug: 'tui/getting-started/quick-start' },
            { label: 'Themes', slug: 'tui/guides/themes' },
            { label: 'Custom Themes', slug: 'tui/guides/custom-themes' },
            { label: 'User Acceptance Testing', slug: 'tui/tutorials/user-acceptance-testing' },
            { label: 'Keybindings', slug: 'tui/reference/keybindings' },
            { label: 'Architecture', slug: 'tui/reference/architecture' },
            { label: 'Project Structure', slug: 'tui/reference/project-structure' },
          ],
        },
        { label: 'Roadmap', slug: 'roadmap' },
        { label: 'Changelog', slug: 'changelog' },
      ],
    }),
  ],
});
