import fs from 'node:fs/promises';
import path from 'node:path';

/**
 * Vite plugin: scan src/**\/*.vue для v-permission-scope использований и
 * генерация frontend/src/generated/permission-keys.json. Запускается в
 * buildStart (build mode); в dev mode запускается один раз при первом
 * импорте plugin (Vite serve dev) -- этого достаточно потому что новые
 * ключи всё равно добавляются в код вручную.
 *
 * Поддерживаемые формы:
 *   v-permission-scope="'action.delete.employee'"
 *   v-permission-scope:hide="'action.foo'"
 *   v-permission-scope="{ key: 'action.foo' }"
 *   <PermissionScope key="action.foo">
 *
 * Не парсим динамические ключи (computed-выражения) -- такие ключи
 * нужно явно перечислить в frontend/src/constants/permissionKeys.js.
 */
const STATIC_KEY_REGEXES = [
  /v-permission-scope(?::\w+)?\s*=\s*["']'([^']+)'["']/g,
  /v-permission-scope(?::\w+)?\s*=\s*\{\s*key\s*:\s*['"]([^'"]+)['"]/g,
  /<PermissionScope[^>]*\s+:?key\s*=\s*["']([^"']+)["']/g,
];

export default function permissionsKeysPlugin(options = {}) {
  const { srcDir = 'src', outFile = 'src/generated/permission-keys.json' } = options;
  return {
    name: 'systemburo-permissions-keys',
    apply: 'build',
    async buildStart() {
      try {
        const keys = await scan(srcDir);
        const outPath = path.resolve(process.cwd(), outFile);
        await fs.mkdir(path.dirname(outPath), { recursive: true });
        await fs.writeFile(
          outPath,
          JSON.stringify({ generated: new Date().toISOString(), keys }, null, 2) + '\n',
        );
        this.info?.(`permission keys generated: ${keys.length} entries -> ${outFile}`);
      } catch (err) {
        this.warn?.(`permissions key scan failed: ${err.message}`);
      }
    },
  };
}

async function scan(srcDir) {
  const found = new Set();
  const root = path.resolve(process.cwd(), srcDir);
  const stack = [root];
  while (stack.length > 0) {
    const dir = stack.pop();
    let entries;
    try {
      entries = await fs.readdir(dir, { withFileTypes: true });
    } catch {
      continue;
    }
    for (const entry of entries) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        if (entry.name === 'generated' || entry.name === '__tests__') continue;
        stack.push(full);
      } else if (entry.isFile() && (entry.name.endsWith('.vue') || entry.name.endsWith('.js') || entry.name.endsWith('.ts'))) {
        const content = await fs.readFile(full, 'utf8');
        for (const re of STATIC_KEY_REGEXES) {
          re.lastIndex = 0;
          let m;
          while ((m = re.exec(content)) !== null) {
            found.add(m[1]);
          }
        }
      }
    }
  }
  return Array.from(found).sort();
}
