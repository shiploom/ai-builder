#!/usr/bin/env node
// sync-docs.mjs — single-source docs pipeline (docs/ -> site/content/docs/).
//
// Usage: node scripts/sync-docs.mjs [--check] [--self-test]
//   default: regenerate content/docs/*.mdx from ../docs/*.md
//   --check: exit 2 if any generated file differs (CI drift gate)
//   --self-test: run escaper unit checks, exit 2 on failure
//
// MDX treats bare <word> in prose as JSX, so prose outside code spans is
// entity-escaped. Fenced blocks and inline `code` are passed through
// byte-identical (so `shiploom run <workflow>` keeps rendering literally).

import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const srcDir = join(root, '..', 'docs');
const outDir = join(root, 'content', 'docs');

const PAGES = [
  ['install.md', 'install.mdx', 'Install', 'Install Shiploom via pipx, uvx, or source'],
  ['release.md', 'release.mdx', 'Release process', 'Versioning, tagging, signing, and publishing'],
  ['manual.md', 'manual.mdx', 'Consumer manual', 'Task-oriented paths J1–J4'],
  ['policy-cookbook.md', 'policy-cookbook.mdx', 'Policy cookbook', 'Default-deny packs, gates, and approvals'],
  ['brownfield-playbook.md', 'brownfield-playbook.mdx', 'Brownfield playbook', 'Map, impact, characterize, change, verify'],
  ['cost-dashboard.md', 'cost-dashboard.mdx', 'Cost / escape dashboard', 'The metrics contract and collector'],
];

/** Escape bare <word> outside fenced blocks and inline code spans. */
export function escapeProse(text) {
  const lines = text.split('\n');
  let inFence = false;
  return lines
    .map((line) => {
      if (/^(`{3,}|~{3,})/.test(line.trim())) {
        inFence = !inFence;
        return line;
      }
      if (inFence) return line;
      // Split on inline code spans; escape prose segments only.
      const parts = line.split('`');
      for (let i = 0; i < parts.length; i += 2) {
        parts[i] = parts[i].replace(/<([A-Za-z][A-Za-z0-9_-]*)>/g, '&lt;$1&gt;');
      }
      return parts.join('`');
    })
    .join('\n');
}

function render(name, title, description) {
  const body = readFileSync(join(srcDir, name), 'utf8').replace(/\s+$/, '') + '\n';
  return `---\ntitle: ${title}\ndescription: ${description}\n---\n\n${escapeProse(body)}`;
}

function selfTest() {
  const cases = [
    ['plain <workflow> prose', 'plain &lt;workflow&gt; prose'],
    ['`shiploom run <workflow>` stays', '`shiploom run <workflow>` stays'],
    ['```\n<tag> fenced\n```\na <b> c', '```\n<tag> fenced\n```\na &lt;b&gt; c'],
    ['no tags here', 'no tags here'],
    ['a <b> c `d <e>` f <g>', 'a &lt;b&gt; c `d <e>` f &lt;g&gt;'],
  ];
  let failed = 0;
  for (const [input, want] of cases) {
    const got = escapeProse(input);
    if (got !== want) {
      failed += 1;
      process.stderr.write(`self-test FAIL\n  in:   ${JSON.stringify(input)}\n  want: ${JSON.stringify(want)}\n  got:  ${JSON.stringify(got)}\n`);
    }
  }
  return failed;
}

function main() {
  const args = new Set(process.argv.slice(2));
  if (args.has('--self-test')) {
    const failed = selfTest();
    process.stdout.write(failed === 0 ? 'sync-docs self-test: ok\n' : `sync-docs self-test: ${failed} failure(s)\n`);
    process.exit(failed === 0 ? 0 : 2);
  }
  mkdirSync(outDir, { recursive: true });
  let drifted = [];
  for (const [name, out, title, description] of PAGES) {
    const rendered = render(name, title, description);
    const dest = join(outDir, out);
    let current = null;
    try {
      current = readFileSync(dest, 'utf8');
    } catch {
      current = null;
    }
    if (current !== rendered) {
      if (args.has('--check')) {
        drifted.push(out);
      } else {
        writeFileSync(dest, rendered);
      }
    }
  }
  if (args.has('--check')) {
    if (drifted.length > 0) {
      process.stderr.write(`docs drift: ${drifted.join(', ')} (run npm run docs:sync)\n`);
      process.exit(2);
    }
    process.stdout.write('docs in sync\n');
  }
}

main();
