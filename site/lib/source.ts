import { docs } from '@/.source/server';
import { loader } from 'fumadocs-core/source';

// Mirrors next.config.mjs basePath at build time (NEXT_PUBLIC_* is inlined);
// empty locally, "/ai-builder" for project-pages deploys.
const basePath = process.env.NEXT_PUBLIC_BASE_PATH || '';

export const source = loader({
  baseUrl: `${basePath}/docs`,
  source: docs.toFumadocsSource(),
});
