import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import DirectorySuggestInput from '../DirectorySuggestInput.vue'

/**
 * Поле наименования справочника с подсказками (#1437): гейт по праву, дебаунс запроса,
 * связь выбранной записи с текстом и разрыв этой связи при ручной правке.
 */

function mountInput(props = {}) {
  return mount(DirectorySuggestInput, {
    props: {
      label: 'Организация / Отдел',
      testid: 'create-organization',
      fetcher: vi.fn().mockResolvedValue([]),
      ...props,
    },
  })
}

// Ввод + прокрутка дебаунса до запроса и его разрешения.
async function type(wrapper, value) {
  const input = wrapper.get('input')
  input.element.value = value
  await input.trigger('input')
  await vi.advanceTimersByTimeAsync(300)
}

describe('DirectorySuggestInput', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('без права поле только для чтения и подсказки не запрашиваются', async () => {
    const fetcher = vi.fn().mockResolvedValue([{ id: 1, name: 'ООО "Максима Групп"' }])
    const wrapper = mountInput({ editable: false, modelValue: 'ООО "Победа"', fetcher })

    expect(wrapper.get('input').attributes('readonly')).toBeDefined()

    await type(wrapper, 'максима')
    expect(fetcher).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="create-organization-list"]').exists()).toBe(false)
  })

  it('запрос уходит только с трёх символов', async () => {
    const fetcher = vi.fn().mockResolvedValue([])
    const wrapper = mountInput({ editable: true, fetcher })

    await type(wrapper, 'ма')
    expect(fetcher).not.toHaveBeenCalled()

    await type(wrapper, 'мак')
    expect(fetcher).toHaveBeenCalledWith('мак')
  })

  it('выбор подсказки подставляет наименование и отдаёт запись наверх', async () => {
    const item = { id: 7, name: 'ООО "Максима Групп"' }
    const wrapper = mountInput({ editable: true, fetcher: vi.fn().mockResolvedValue([item]) })

    await type(wrapper, 'максима')
    const options = wrapper.findAll('[data-testid="create-organization-option"]')
    expect(options).toHaveLength(1)

    await options[0].trigger('mousedown')

    expect(wrapper.emitted('update:modelValue').at(-1)).toEqual([item.name])
    expect(wrapper.emitted('select').at(-1)).toEqual([item])
    expect(wrapper.find('[data-testid="create-organization-list"]').exists()).toBe(false)
  })

  it('ручная правка рвёт связь с выбранной записью', async () => {
    const wrapper = mountInput({ editable: true, fetcher: vi.fn().mockResolvedValue([]) })

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
      .mockResolvedValueOnce([{ id: 2, name: 'ООО "Победа"' }])
    const wrapper = mountInput({ editable: true, fetcher })

    await type(wrapper, 'поб')
    await type(wrapper, 'победа')

    // Первый запрос отвечает уже после второго - его результат должен быть отброшен.
    resolveFirst([{ id: 1, name: 'ООО "Побег"' }])
    await vi.advanceTimersByTimeAsync(10)

    const names = wrapper.findAll('[data-testid="create-organization-option"]').map((o) => o.text())
    expect(names).toEqual(['ООО "Победа"'])
  })

  it('blur не съедает выбор подсказки', async () => {
    const item = { id: 9, name: 'ЧОП "АРЕС"' }
    const wrapper = mountInput({ editable: true, fetcher: vi.fn().mockResolvedValue([item]) })

    await type(wrapper, 'арес')
    await wrapper.get('input').trigger('blur')
    // Клик по подсказке приходит после blur: список обязан дожить до выбора.
    await vi.advanceTimersByTimeAsync(100)
    const option = wrapper.find('[data-testid="create-organization-option"]')
    expect(option.exists()).toBe(true)

    await option.trigger('mousedown')
    expect(wrapper.emitted('select').at(-1)).toEqual([item])
  })
})
