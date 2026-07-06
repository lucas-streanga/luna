#!/usr/bin/env node
/**
 * sync-luna-grammar.mjs — lives in the astro repo at scripts/.
 *
 * Syncs src/lib/shiki-luna.ts from the luna repo so this site never maintains
 * its own fork of the grammar. Pure Node (built-in fetch, Node >= 18): no git,
 * no curl — both are absent from node:22-slim, so this runs identically inside
 * the container, on a Linux host, and on Apple-silicon podman.
 *
 * Wire via package.json:
 *
 *   "scripts": {
 *     "predev":   "node scripts/sync-luna-grammar.mjs",
 *     "prebuild": "node scripts/sync-luna-grammar.mjs"
 *   }
 *
 * Offline-safe: keeps the existing copy (with a warning) when GitHub is
 * unreachable; fails only if no copy exists at all. Every fetch carries a
 * 10s timeout so a dead network can never hang `npm run dev` (whose hang is a
 * hung container, given the image's CMD). Recommend COMMITTING the synced
 * file: fresh clones and offline CI always build, and upstream grammar
 * changes surface as reviewable diffs.
 */

import { readFileSync, writeFileSync, mkdirSync, existsSync } from 'node:fs';
import { dirname } from 'node:path';

const OWNER_REPO = 'lucas-streanga/luna';
const BRANCH = 'master';
const FILE = 'tooling/shiki-luna.ts';
const DEST = 'src/lib/shiki-luna.ts';

const RAW_URL = `https://raw.githubusercontent.com/${OWNER_REPO}/${BRANCH}/${FILE}`;
const SHA_URL = `https://api.github.com/repos/${OWNER_REPO}/commits/${BRANCH}`;

const TIMEOUT_MS = 10_000;

async function get(url, accept) {
  const res = await fetch(url, {
    signal: AbortSignal.timeout(TIMEOUT_MS),
    headers: accept ? { accept } : {},
  });
  if (!res.ok) throw new Error(`${url}: HTTP ${res.status}`);
  return res.text();
}

try {
  const body = await get(RAW_URL);

  // Best-effort rev for the stamp; rate limits or API failures must not
  // block the sync itself.
  let rev = 'unknown';
  try {
    rev = (await get(SHA_URL, 'application/vnd.github.sha')).trim().slice(0, 12);
  } catch {
    /* stamp stays 'unknown' */
  }

  const stamped =
    `// GENERATED: synced from ${OWNER_REPO} (${FILE} @ ${rev})\n` +
    `// by scripts/sync-luna-grammar.mjs — edit in the luna repo, never here.\n` +
    body;

  mkdirSync(dirname(DEST), { recursive: true });

  // Skip the write when nothing changed, so watch-mode tooling stays quiet.
  const current = existsSync(DEST) ? readFileSync(DEST, 'utf8') : null;
  if (current !== null && current.replace(/@ \w+\)/, '') === stamped.replace(/@ \w+\)/, '')) {
    console.log(`sync-luna-grammar: ${DEST} already current (${rev})`);
  } else {
    writeFileSync(DEST, stamped);
    console.log(`sync-luna-grammar: wrote ${DEST} from ${OWNER_REPO}@${rev}`);
  }
} catch (err) {
  if (existsSync(DEST)) {
    console.warn(
      `sync-luna-grammar: WARNING — could not reach ${OWNER_REPO} ` +
      `(${err.message}); keeping existing ${DEST}`,
    );
  } else {
    console.error(
      `sync-luna-grammar: ERROR — could not reach ${OWNER_REPO} ` +
      `(${err.message}) and no existing ${DEST} exists`,
    );
    process.exit(1);
  }
}
