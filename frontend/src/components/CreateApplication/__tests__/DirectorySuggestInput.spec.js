import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import DirectorySuggestInput from '../DirectorySuggestInput.vue'

/**
 * Поле наименования справочника с подсказками (#1437): гейт по праву, дебаунс запроса,
 * связь выбранной записи с текстом и разрыв этой связи при ручной правке, подстановка
 * каноничного оформления и предупреждение о записи, которой в справочнике нет.
 *
 * Ответ подсказок - { items, canonical, matched }: канон и признак совпадения считает
 * бэк, фронт только применяет. Фикстуры строим по этой форме, иначе тест зеленел бы на
 * контракте, которого нет.
 */

// answer собирает ответ подсказок; по умолчанию наименования в справочнике нет.
function answer({ items = [], canonical = '', matched = false, degenerate = false } = {}) {
  return { items, canonical, matched, degenerate }
}

function mountInput(props = {}) {
  return mount(DirectorySuggestInput, {
    props: {
      label: 'Организация / Отдел',
      testid: 'create-organization',
      modelValue: '',
      fetcher: vi.fn().mockResolvedValue(answer()),
      ...props,
    },
  })
}

// Ввод + прокрутка дебаунса до запроса и его разрешения. Поле управляемое: в приложении
// текст возвращается пропом от родителя, и без этой синхронизации компонент видел бы
// пустое значение - тест проверял бы состояние, которого в проде не бывает.
async function type(wrapper, value) {
  const input = wrapper.get('input')
  input.element.value = value
  await input.trigger('input')
  await wrapper.setProps({ modelValue: value })
  await vi.advanceTimersByTimeAsync(300)
}

describe('DirectorySuggestInput', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('без права поле только для чтения и подсказки не запрашиваются', async () => {
    const fetcher = vi.fn().mockResolvedValue(answer({ items: [{ id: 1, name: 'ООО "Максима Групп"' }] }))
    const wrapper = mountInput({ editable: false, modelValue: 'ООО "Победа"', fetcher })

    expect(wrapper.get('input').attributes('readonly')).toBeDefined()

    await type(wrapper, 'максима')
    expect(fetcher).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="create-organization-list"]').exists()).toBe(false)
  })

  it('запрос уходит с первого символа: канон нужен и до порога подсказок', async () => {
    const fetcher = vi.fn().mockResolvedValue(answer())
    const wrapper = mountInput({ editable: true, fetcher })

    await type(wrapper, 'ма')
    // Канон и признак совпадения нужны и на коротком вводе, поэтому запрос уходит сразу;
    // порог выдачи самих подсказок держит бэк.
    expect(fetcher).toHaveBeenCalledWith('ма')

    await type(wrapper, 'мак')
    expect(fetcher).toHaveBeenCalledWith('мак')
  })

  it('выбор подсказки подставляет наименование и отдаёт запись наверх', async () => {
    const item = { id: 7, name: 'ООО "Максима Групп"' }
    const wrapper = mountInput({ editable: true, fetcher: vi.fn().mockResolvedValue(answer({ items: [item], matched: true, canonical: item.name })) })

    await type(wrapper, 'максима')
    const options = wrapper.findAll('[data-testid="create-organization-option"]')
    expect(options).toHaveLength(1)

    await options[0].trigger('mousedown')

    expect(wrapper.emitted('update:modelValue').at(-1)).toEqual([item.name])
    expect(wrapper.emitted('select').at(-1)).toEqual([item])
    expect(wrapper.find('[data-testid="create-organization-list"]').exists()).toBe(false)
  })

  it('ручная правка рвёт связь с выбранной записью', async () => {
    const wrapper = mountInput({ editable: true, fetcher: vi.fn().mockResolvedValue(answer()) })

    await type(wrapper, 'ООО Ромашка')

    expect(wrapper.emitted('select').at(-1)).toEqual([null])
  })

  it('сбой подсказок не ломает ввод', async () => {
    const fetcher = vi.fn().mockRejectedValue(new Error('сеть'))
    const wrapper = mountInput({ editable: true, fetcher })

    await type(wrapper, 'максима')

    expect(wrapper.find('[data-testid="create-organization-list"]').exists()).toBe(false)
    expect(wrapper.emitted('update:modelValue').at(-1)).toEqual(['максима'])
  })

  it('устаревший ответ не затирает подсказки текущего запроса', async () => {
    let resolveFirst
    const fetcher = vi.fn()
      .mockImplementationOnce(() => new Promise((resolve) => { resolveFirst = resolve }))
      .mockResolvedValueOnce(answer({ items: [{ id: 2, name: 'ООО "Победа"' }], matched: true }))
    const wrapper = mountInput({ editable: true, fetcher })

    await type(wrapper, 'поб')
    await type(wrapper, 'победа')

    // Первый запрос отвечает уже после второго - его результат должен быть отброшен.
    resolveFirst(answer({ items: [{ id: 1, name: 'ООО "Побег"' }] }))
    await vi.advanceTimersByTimeAsync(10)

    const names = wrapper.findAll('[data-testid="create-organization-option"]').map((o) => o.text())
    expect(names).toEqual(['ООО "Победа"'])
  })

  it('blur не съедает выбор подсказки', async () => {
    const item = { id: 9, name: 'ЧОП "АРЕС"' }
    const wrapper = mountInput({ editable: true, fetcher: vi.fn().mockResolvedValue(answer({ items: [item], matched: true, canonical: item.name })) })

    await type(wrapper, 'арес')
    await wrapper.get('input').trigger('blur')
    // Клик по подсказке приходит после blur: список обязан дожить до выбора.
    await vi.advanceTimersByTimeAsync(100)
    const option = wrapper.find('[data-testid="create-organization-option"]')
    expect(option.exists()).toBe(true)

    await option.trigger('mousedown')
    expect(wrapper.emitted('select').at(-1)).toEqual([item])
  })

  it('канон оформления подставляется при потере фокуса', async () => {
    const fetcher = vi.fn().mockResolvedValue(answer({ canonical: 'ООО "Братишк"' }))
    const wrapper = mountInput({ editable: true, fetcher })

    await type(wrapper, 'ооо "братишк')
    // Во время набора поле не трогаем: подстановка дёргала бы каретку на каждой букве.
    expect(wrapper.emitted('update:modelValue').at(-1)).toEqual(['ооо "братишк'])

    await wrapper.get('input').trigger('blur')
    expect(wrapper.emitted('update:modelValue').at(-1)).toEqual(['ООО "Братишк"'])
  })

  it('канон от прошлого ввода не подставляется в изменённый текст', async () => {
    const fetcher = vi.fn()
      .mockResolvedValueOnce(answer({ canonical: 'ООО "Братишк"' }))
      .mockImplementationOnce(() => new Promise(() => {}))
    const wrapper = mountInput({ editable: true, fetcher })

    await type(wrapper, 'ооо "братишк')
    // Второй ответ ещё не пришёл - канон предыдущего запроса к этому тексту не относится.
    const input = wrapper.get('input')
    input.element.value = 'зао ромашка'
    await input.trigger('input')
    await vi.advanceTimersByTimeAsync(300)

    await input.trigger('blur')
    expect(wrapper.emitted('update:modelValue').at(-1)).toEqual(['зао ромашка'])
  })

  it('предупреждает, что наименования нет в справочнике', async () => {
    const wrapper = mountInput({
      editable: true,
      fetcher: vi.fn().mockResolvedValue(answer({ canonical: 'ООО "Братишк"', matched: false })),
    })

    await type(wrapper, 'ооо "братишк')

    const notice = wrapper.find('[data-testid="create-organization-notice"]')
    expect(notice.exists()).toBe(true)
    expect(notice.text()).toContain('Нет в справочнике - будет создано и отправлено на проверку')
  })

  it('о вырожденном наименовании говорит, что запись не создать', async () => {
    const wrapper = mountInput({
      editable: true,
      fetcher: vi.fn().mockResolvedValue(answer({ canonical: '""', degenerate: true })),
    })

    await type(wrapper, '""')

    const notice = wrapper.find('[data-testid="create-organization-notice"]')
    expect(notice.exists()).toBe(true)
    // Подача такой ввод отклоняет, поэтому обещать проверку нельзя.
    expect(notice.text()).toContain('Укажите наименование')
  })

  it('о существующем наименовании не предупреждает', async () => {
    const item = { id: 3, name: 'ООО Демо-Партнёр' }
    const wrapper = mountInput({
      editable: true,
      fetcher: vi.fn().mockResolvedValue(answer({ items: [item], canonical: item.name, matched: true })),
    })

    await type(wrapper, 'демо-партнёр')

    expect(wrapper.find('[data-testid="create-organization-notice"]').exists()).toBe(false)
  })

  it('без права ни канон, ни предупреждение не применяются', async () => {
    const fetcher = vi.fn().mockResolvedValue(answer({ canonical: 'ООО "Братишк"' }))
    const wrapper = mountInput({ editable: false, modelValue: 'ооо "братишк', fetcher })

    await wrapper.get('input').trigger('blur')

    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(wrapper.find('[data-testid="create-organization-notice"]').exists()).toBe(false)
  })
})
