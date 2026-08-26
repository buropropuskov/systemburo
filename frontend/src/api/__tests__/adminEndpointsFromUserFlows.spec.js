import fs from 'node:fs';
import path from 'node:path';
import { describe, it, expect } from 'vitest';
import { backendRoutes, gatesForPath } from './helpers/backendRouteGates';
import { userFlowModules, apiCallsIn, SRC_ROOT } from './helpers/userFlowModules';
import { SILENT_403_PREFIXES } from '../client';
import registry from './adminEndpointRegistry.json';

/**
 * Замок против тоста «Недостаточно прав» на действии, которого человек не совершал.
 *
 * Экран, открытый любому вошедшему, не должен фоном звать метод, закрытый правом:
 * арендатор открывал форму подачи заявки и получал тост из-за `/users/all` (#1928).
 * Лечится либо гейтом самого элемента (тогда запроса не будет), либо `silent403` -
 * когда отказ штатный и экран без этих данных работает.
 */

const relative = (file) => path.relative(SRC_ROOT, file).split(path.sep).join('/');

/**
 * Вызовы, у которых путь приходит переменной-параметром и статически не читается.
 * Каждый разобран руками; список растёт - разбирать новый, иначе замок перестаёт
 * видеть часть запросов экрана и молчит про них как про чистые.
 */
const OPAQUE_CALLS = {
  'components/CreateApplication/CreateApplication.vue:1408':
    'loadDefaultApprovers гоняет общий collect(url) по /organizations/:id/users и /companies/:id/users - оба открыты любому вошедшему',
};

function silenced(call) {
  if (/silent403/.test(call.args)) return true;
  return SILENT_403_PREFIXES.some((p) => call.path === p || call.path.startsWith(`${p}/`));
}

/**
 * Функции обёрток `src/api/*.js`, которые бьют в закрытый правом метод и при этом
 * шумят на отказ. Молчащие (silent403 или путь из тихого списка) в счёт не идут:
 * тост они не поднимают, гейтить вызывающий элемент незачем.
 *
 * @returns {Map<string, string[]>} `api/users.js::getUsers` -> права
 */
function gatedApiFunctions() {
  const found = new Map();
  const dir = path.join(SRC_ROOT, 'api');
  for (const name of fs.readdirSync(dir)) {
    if (!name.endsWith('.js')) continue;
    const text = fs.readFileSync(path.join(dir, name), 'utf8');
    const marks = [
      ...text.matchAll(/export\s+(?:async\s+)?function\s+(\w+)/g),
      ...text.matchAll(/export\s+const\s+(\w+)\s*=\s*(?:async\s*)?\(/g),
    ].sort((a, b) => a.index - b.index);

    marks.forEach((mark, i) => {
      const body = text.slice(mark.index, i + 1 < marks.length ? marks[i + 1].index : text.length);
      const gates = [...new Set(apiCallsIn(body)
        .filter((call) => call.path && !silenced(call))
        .flatMap((call) => gatesForPath(call.path, call.method)))];
      if (gates.length) found.set(`api/${name}::${mark[1]}`, gates);
    });
  }
  return found;
}

/** @returns {Map<string, string[]>} модуль пользовательского экрана -> импортированные из `@/api` имена */
function apiImportsIn(text) {
  const byModule = new Map();
  for (const m of text.matchAll(/import\s*\{([^}]*)\}\s*from\s*['"](?:@\/|\.{1,2}\/)?((?:\.{2}\/)*api\/[\w-]+)['"]/g)) {
    const names = m[1].split(',').map((n) => n.trim().split(/\s+as\s+/)[0]).filter(Boolean);
    const module = `${m[2].replace(/^(\.\.\/)*/, '')}.js`;
    byModule.set(module, [...(byModule.get(module) ?? []), ...names]);
  }
  return byModule;
}

describe('вызовы закрытых правом методов с пользовательских экранов', () => {
  const modules = userFlowModules();

  it('парсеры живы: роуты бэкенда и граф экранов разобраны', () => {
    // Молчаливо сломавшийся парсер сделал бы замок вечно зелёным - проверяем якорями.
    expect(backendRoutes().length).toBeGreaterThan(300);
    expect(gatesForPath('/users/all')).toContain('page.admin.users');
    expect(gatesForPath('/applications/user')).toEqual([]);
    expect(modules.length).toBeGreaterThan(100);
    expect(modules.some((f) => relative(f) === 'components/CreateApplication/CreateApplication.vue')).toBe(true);
  });

  it('запрос с непрозрачным путём разобран руками', () => {
    const opaque = [];
    for (const file of modules) {
      for (const call of apiCallsIn(fs.readFileSync(file, 'utf8'))) {
        if (call.path) continue;
        const key = `${relative(file)}:${call.line}`;
        if (!OPAQUE_CALLS[key]) opaque.push(`${key} -> apiRequest(${call.expression})`);
      }
    }
    expect(opaque, [
      'Путь запроса собран в переменной и статически не читается - проверить, куда он ведёт,',
      'и описать в OPAQUE_CALLS (либо передать путь литералом).',
    ].join('\n')).toEqual([]);
  });

  it('прямой запрос в закрытый метод либо не делается, либо молчит про 403', () => {
    const loud = [];
    for (const file of modules) {
      const text = fs.readFileSync(file, 'utf8');
      for (const call of apiCallsIn(text)) {
        if (!call.path) continue;
        const gates = gatesForPath(call.path, call.method);
        if (!gates.length || silenced(call)) continue;
        loud.push(`${relative(file)}:${call.line} ${call.method} ${call.path} (закрыт: ${gates.join(', ')})`);
      }
    }
    expect(loud, [
      'Экран открыт любому вошедшему, а запрос закрыт правом: человек без права увидит',
      'тост «Недостаточно прав» на действии, которого не совершал.',
      'Лечится гейтом элемента (тогда запроса нет) либо silent403, если отказ штатный.',
    ].join('\n')).toEqual([]);
  });

  it('обёртка из src/api, закрытая правом, вызывается с пользовательского экрана только по записи в реестре', () => {
    const gated = gatedApiFunctions();
    const unregistered = [];
    for (const file of modules) {
      const text = fs.readFileSync(file, 'utf8');
      for (const [module, names] of apiImportsIn(text)) {
        for (const name of names) {
          const key = `${module}::${name}`;
          if (!gated.has(key) || registry[key]) continue;
          unregistered.push(`${relative(file)} зовёт ${key} (закрыт: ${gated.get(key).join(', ')})`);
        }
      }
    }
    expect(unregistered, [
      'Обёртка бьёт в закрытый правом метод и импортируется экраном, открытым всем.',
      'Опиши в adminEndpointRegistry.json, чем гейтится сам вызов (или убери его с экрана).',
    ].join('\n')).toEqual([]);
  });

  it('записи реестра объясняют гейт и не протухли', () => {
    const gated = gatedApiFunctions();
    for (const [key, entry] of Object.entries(registry)) {
      expect(gated.has(key), `${key}: метод больше не закрыт правом - запись в реестре лишняя`).toBe(true);
      expect(entry.gate?.length, `${key}: пустое объяснение гейта`).toBeGreaterThan(20);
    }
  });
});
