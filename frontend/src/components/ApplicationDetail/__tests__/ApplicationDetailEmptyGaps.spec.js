/**
 * Замок против лишних отступов от пустых блоков в детали заявки (#1587).
 *
 * Обёртки ветки и вопросов рендерятся всегда, а содержимое приходит по данным:
 * пустой список пересылок оставлял div высотой 0, и gap колонки считал его за
 * блок - между сообщением и вопросами выходило 20px вместо 10px. Тот же класс
 * ошибки внутри секций: последний видимый блок держал зазор своим margin-bottom,
 * и когда следующий скрывался (нет согласующих, заявка отозвана, нет
 * принявшего), margin складывался с padding секции в пустоту снизу.
 *
 * Проверки статические: раскладку jsdom не считает, а правила легко потерять при
 * рефакторинге стилей - именно так они и появились.
 */
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

const DIR = path.resolve(__dirname, '..');
const detail = fs.readFileSync(path.join(DIR, 'ApplicationDetail.vue'), 'utf8');
const confirmation = fs.readFileSync(path.join(DIR, 'ApplicationConfirmation.vue'), 'utf8');

/** Тело правила для селектора, без учёта переносов. */
function rule(src, selector) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const found = src.match(new RegExp(`${escaped}\\s*\\{([^}]*)\\}`));
  return found ? found[1].replace(/\s+/g, ' ').trim() : null;
}

describe('пустые блоки не занимают место', () => {
  it('пустая обёртка в центральной колонке убрана из раскладки', () => {
    expect(rule(detail, '.detail-main-column > *:empty')).toMatch(/display:\s*none/);
  });

  it('пустая обёртка в секции статуса убрана из раскладки', () => {
    expect(rule(detail, '.application-status-section > *:empty')).toMatch(/display:\s*none/);
  });
});

describe('зазор держит gap, а не margin последнего блока', () => {
  it('секция статуса - вертикальный стек с gap', () => {
    const body = rule(detail, '.application-status-section');
    expect(body).toMatch(/display:\s*flex/);
    expect(body).toMatch(/flex-direction:\s*column/);
    expect(body).toMatch(/gap:\s*15px/);
  });

  it('заголовок статуса не несёт собственный margin-bottom', () => {
    expect(rule(detail, '.status-header')).not.toMatch(/margin-bottom/);
  });

  it('последний блок согласования не даёт отступа снизу', () => {
    expect(rule(confirmation, '.confirmation-section > *:last-child'))
      .toMatch(/margin-bottom:\s*0/);
  });
});
