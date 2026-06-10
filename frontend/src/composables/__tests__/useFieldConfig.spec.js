import { describe, it, expect } from 'vitest';
import { ref } from 'vue';
import { useFieldConfig } from '../useFieldConfig.js';

// H-5b (#529): единый composable потребления field-config для секций формы подачи.
// Дефолт (нет строки конфига) = поле видимо и обязательно -> текущее поведение форм.

describe('useFieldConfig', () => {
  it('пустой конфиг: любое поле видимо и обязательно', () => {
    const { fieldVisible, fieldRequired } = useFieldConfig({});
    expect(fieldVisible('any')).toBe(true);
    expect(fieldRequired('any')).toBe(true);
  });

  it('нет строки для ключа -> дефолт true/true', () => {
    const { fieldVisible, fieldRequired } = useFieldConfig({ foo: { visible: false, required: false } });
    expect(fieldVisible('missing')).toBe(true);
    expect(fieldRequired('missing')).toBe(true);
  });

  it('читает visible/required из строки конфига', () => {
    const { fieldVisible, fieldRequired } = useFieldConfig({
      hidden: { visible: false, required: true },
      optional: { visible: true, required: false },
    });
    expect(fieldVisible('hidden')).toBe(false);
    expect(fieldRequired('hidden')).toBe(true);
    expect(fieldVisible('optional')).toBe(true);
    expect(fieldRequired('optional')).toBe(false);
  });

  // Уровень вызова: хелпер читает источник на каждый вызов. Реактивность Vue
  // (перерисовка компонента при смене пропа) покрыта DateRangeSectionFieldConfig.spec.js.
  it('принимает геттер и читает источник на каждый вызов', () => {
    let cfg = { x: { visible: true, required: true } };
    const { fieldVisible } = useFieldConfig(() => cfg);
    expect(fieldVisible('x')).toBe(true);
    cfg = { x: { visible: false, required: true } };
    expect(fieldVisible('x')).toBe(false);
  });

  it('принимает ref и читает актуальное значение', () => {
    const cfg = ref({ x: { visible: true, required: false } });
    const { fieldRequired } = useFieldConfig(cfg);
    expect(fieldRequired('x')).toBe(false);
    cfg.value = { x: { visible: true, required: true } };
    expect(fieldRequired('x')).toBe(true);
  });

  it('источник null/undefined не падает -> дефолт', () => {
    const { fieldVisible } = useFieldConfig(() => null);
    expect(fieldVisible('x')).toBe(true);
  });
});
