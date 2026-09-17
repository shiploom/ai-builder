import { createMDX } from 'fumadocs-mdx/next';

const withMDX = createMDX();

// Project pages (e.g. GitHub Pages under /<repo>/) need prefixed links.
// Local default is unprefixed; CI sets PAGES_BASE_PATH=/ai-builder.
const basePath = process.env.PAGES_BASE_PATH || '';

export default withMDX({
  reactStrictMode: true,
  output: 'export', // static site: deploy the out/ directory anywhere
  ...(basePath ? { basePath } : {}),
});
