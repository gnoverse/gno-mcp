/**
 * gnomcp plugin for OpenCode.ai
 *
 * Auto-registers the bundled skills directory so the `gno` skill is
 * discoverable without requiring manual symlinks or config edits.
 *
 * Domain-specific: this plugin does NOT inject bootstrap context into every
 * session — the `gno` skill is only relevant when the user is working on Gno
 * code. Users invoke the skill explicitly (`/skill gno` or via OpenCode's
 * description-match) when needed.
 */

import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export const GnomcpPlugin = async () => {
  const skillsDir = path.resolve(__dirname, '../../skills');

  return {
    config: async (config) => {
      config.skills = config.skills || {};
      config.skills.paths = config.skills.paths || [];
      if (!config.skills.paths.includes(skillsDir)) {
        config.skills.paths.push(skillsDir);
      }
    },
  };
};
