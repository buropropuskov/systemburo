import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import TemplatePatternField from '../TemplatePatternField.vue'

const TOKENS = [
  { key: 'год', label: 'Год', group: 'Дата', example: '2026', dir_allowed: true, file_allowed: true },
  { key: 'тип', label: 'Тип вложения', group: 'Вложение', example: 'Заявка', dir_allowed: false, file_allowed: true },
]

function mountField(props = {}) {
  return mount(TemplatePatternField, {
    props: {
      modelValue: '',
      tokens: TOKENS,
      scope: 'dir',
      label: 'Шаблон папки заявки',
      defaultTemplate: '{год}',
      ...props,
    },
  })
}

describe('TemplatePatternField', () => {
  it('палитра фильтруется по scope - "{тип}" недопустим в шаблоне папки', () => {
    const w = mountField({ scope: 'dir' })
    const chips = w.findAll('[data-testid="tpf-chip"]').map(c => c.text())
    expect(chips).toContain('Год')
    expect(chips).not.toContain('Тип вложения')
  })

  it('для шаблона файла палитра показывает "{тип}"', () => {
    const w = mountField({ scope: 'file' })
    const chips = w.findAll('[data-testid="tpf-chip"]').map(c => c.text())
    expect(chips).toContain('Тип вложения')
  })

  // @mousedown.prevent не даёт полю потерять фокус/выделение - вставка идёт по
  // реальной позиции курсора, а не в конец строки.
  it('клик по чипу вставляет плейсхолдер по позиции курсора', async () => {
    const w = mountField({ modelValue: 'AB' })
    const input = w.find('[data-testid="tpf-input"]').element
    input.setSelectionRange(1, 1)
    await w.find('[data-testid="tpf-chip"]').trigger('mousedown')
    expect(w.emitted('update:modelValue')[0]).toEqual(['A{год}B'])
  })

  it('"Стандартный" сбрасывает поле к шаблону по умолчанию', async () => {
    const w = mountField({ modelValue: 'моя раскладка' })
    await w.find('[data-testid="tpf-reset"]').trigger('click')
    expect(w.emitted('update:modelValue')[0]).toEqual(['{год}'])
  })

  it('подсвечивает неизвестный плейсхолдер клиентской проверкой, не дожидаясь ответа бэка', () => {
    const w = mountField({ modelValue: 'X{неизвестно}Y' })
    const seg = w.find('.tpf__seg--unknown')
    expect(seg.exists()).toBe(true)
    expect(seg.text()).toBe('{неизвестно}')
  })

  it('известный токен с претензией из props.problems подсвечивается как warn', () => {
    const w = mountField({
      modelValue: '{тип}',
      scope: 'file',
      problems: [{ token: 'тип', reason: 'не может использоваться здесь' }],
    })
    expect(w.find('.tpf__seg--warn').exists()).toBe(true)
    expect(w.find('.tpf__seg--unknown').exists()).toBe(false)
  })

  it('известный уместный токен подсвечивается нейтрально (ok)', () => {
    const w = mountField({ modelValue: '{год}' })
    expect(w.find('.tpf__seg--ok').exists()).toBe(true)
  })

  it('показывает превью раскладки из props.previewText', () => {
    const w = mountField({ previewText: '2026/Мегобари' })
    expect(w.find('[data-testid="tpf-preview"]').text()).toContain('2026/Мегобари')
  })

  it('disabled блокирует поле, чипы и кнопку сброса', () => {
    const w = mountField({ disabled: true })
    expect(w.find('[data-testid="tpf-input"]').attributes('disabled')).toBeDefined()
    expect(w.find('[data-testid="tpf-reset"]').attributes('disabled')).toBeDefined()
  })
})
