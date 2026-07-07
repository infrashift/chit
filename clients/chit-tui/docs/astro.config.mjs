import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://infrashift.github.io',
  base: '/chit-tui',
  integrations: [
    starlight({
      title: 'Chit TUI',
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/infrashift/chit-tui',
        },
      ],
      editLink: {
        baseUrl: 'https://github.com/infrashift/chit-tui/edit/main/docs/',
      },
      customCss: ['./src/styles/custom.css'],
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            { label: 'Installation', slug: 'getting-started/installation' },
            { label: 'Quick Start', slug: 'getting-started/quick-start' },
            { label: 'Configuration', slug: 'getting-started/configuration' },
          ],
        },
        {
          label: 'Guides',
          items: [
            { label: 'Themes', slug: 'guides/themes' },
            { label: 'Custom Themes', slug: 'guides/custom-themes' },
          ],
        },
        {
          label: 'Tutorials',
          items: [
            { label: 'User Acceptance Testing', slug: 'tutorials/user-acceptance-testing' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'Keybindings', slug: 'reference/keybindings' },
            { label: 'Architecture', slug: 'reference/architecture' },
            { label: 'Project Structure', slug: 'reference/project-structure' },
          ],
        },
      ],
    }),
  ],
});
