import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  integrations: [
    starlight({
      title: 'eyeVesa Docs',
      description: 'Identity and trust layer documentation for the agentic economy.',
      social: {
        github: 'https://github.com/Hafizaljohari/eyeVesa',
      },
      sidebar: [
        {
          label: 'Start Here',
          items: [
            { label: 'Overview', slug: 'overview' },
            { label: 'Quickstart', slug: 'guides/quickstart' },
            { label: 'Architecture', slug: 'guides/architecture' },
          ],
        },
        {
          label: 'SDKs',
          items: [
            { label: 'SDKs', slug: 'sdk' },
            { label: 'TypeScript SDK', slug: 'sdk/typescript' },
            { label: 'Python SDK', slug: 'sdk/python' },
            { label: 'Airport SDK', slug: 'sdk/airport' },
            { label: 'Transactions', slug: 'sdk/transactions' },
            { label: 'Errors', slug: 'sdk/errors' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'Airport', slug: 'guides/airport' },
            { label: 'CLI', slug: 'reference/cli' },
          ],
        },
      ],
    }),
  ],
});
