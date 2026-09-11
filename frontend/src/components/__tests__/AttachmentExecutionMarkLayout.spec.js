import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

/**
 * Замок на раскладку блока отметки исполнения (#2446).
 *
 * jsdom не считает геометрию, поэтому «кнопка и строка сводки слиплись в одну
 * строку» тестом компонента не поймать - на стенде это выглядело так, будто отступа
 * между ними нет вовсе. Причина была в инлайновых элементах: вертикальный margin им
 * ничего не даёт, и сводка встала СПРАВА от кнопки.
 *
 * Поэтому сторожим сами стили: контейнер обязан выкладывать содержимое колонкой и
 * держать промежуток.
 */
const sfc = readFileSync(
  resolve(process.cwd(), 'src/components/AttachmentExecutionMark.vue'),
  'utf8',
);
const styles = sfc.slice(sfc.indexOf('<style')).replace(/\/\*[\s\S]*?\*\//g, '');
const rule = (selector) => {
  const i = styles.indexOf(`${selector} {`);
  return i === -1 ? '' : styles.slice(i, styles.indexOf('}', i));
};

describe('AttachmentExecutionMark - раскладка', () => {
  it('кнопка, сводка и список идут колонкой, а не в строку', () => {
    const container = rule('.execution-mark');
    expect(container).toContain('display: flex');
    expect(container).toContain('flex-direction: column');
  });

  it('между элементами есть промежуток', () => {
    const container = rule('.execution-mark');
    const gap = /gap:\s*(\d+)px/.exec(container);
    expect(gap, 'у контейнера нет gap - элементы слипнутся').not.toBeNull();
    expect(Number(gap[1])).toBeGreaterThanOrEqual(6);
  });

  it('содержимое прижато к левому краю, а не растянуто на всю ширину карточки', () => {
    expect(rule('.execution-mark')).toContain('align-items: flex-start');
  });
});
