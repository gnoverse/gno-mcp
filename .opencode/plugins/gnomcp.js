/**
 * gnomcp plugin for OpenCode.ai
 *
 * Auto-registers the bundled skills directory so the `gno` skill is
 * discoverable without requiring manual symlinks or config edits, and
 * registers the gnomcp MCP server when the binary is installed (PATH or
 * ~/.local/bin, where scripts/install.sh puts it). Without the binary the
 * plugin is skill-only — see INSTALL.md.
 *
 * Domain-specific: this plugin does NOT inject bootstrap context into every
 * session — the `gno` skill is only relevant when the user is working on Gno
 * code. Users invoke the skill explicitly (`/skill gno` or via OpenCode's
 * description-match) when needed.
 */

import { accessSync, constants, statSync } from 'fs';
import os from 'os';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// A directory or non-executable named gnomcp must not shadow a real binary
// further down the candidate list.
const isExecutableFile = (candidate) => {
  try {
    accessSync(candidate, constants.X_OK);
    return statSync(candidate).isFile();
  } catch {
    return false;
  }
};

const findGnomcpBinary = () => {
  const candidates = (process.env.PATH || '')
    .split(path.delimiter)
    .filter(Boolean)
    .map((dir) => path.join(dir, 'gnomcp'));
  candidates.push(path.join(os.homedir(), '.local', 'bin', 'gnomcp'));
  return candidates.find(isExecutableFile);
};

export const GnomcpPlugin = async () => {
  const skillsDir = path.resolve(__dirname, '../../skills');

  return {
    config: async (config) => {
      config.skills = config.skills || {};
      config.skills.paths = config.skills.paths || [];
      if (!config.skills.paths.includes(skillsDir)) {
        config.skills.paths.push(skillsDir);
      }

      // A user-configured "mcp".gnomcp entry in opencode.json wins.
      const binary = findGnomcpBinary();
      if (binary && !(config.mcp && config.mcp.gnomcp)) {
        config.mcp = config.mcp || {};
        config.mcp.gnomcp = { type: 'local', command: [binary], enabled: true };
      }
    },
  };
};
