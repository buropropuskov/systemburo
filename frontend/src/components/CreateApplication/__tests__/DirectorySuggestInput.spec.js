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
// matched = null значит «не проверяли»: так бэк отвечает на коротком вводе и на
// наименовании без букв, и форма в этом случае молчит.
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

  it('окно с пунктом «Создать» вылезает, когда похожих записей нет', async () => {
    const wrapper = mountInput({
      editable: true,
      fetcher: vi.fn().mockResolvedValue(answer({ canonical: 'ООО "Бебра"', matched: false })),
    })

    await type(wrapper, 'ооо "бебра')

    // Раньше при пустой выдаче окно не показывалось вовсе, и человек не видел ни вариантов,
    // ни того, что именно уйдёт в справочник.
    expect(wrapper.find('[data-testid="create-organization-list"]').exists()).toBe(true)
    const create = wrapper.find('[data-testid="create-organization-option-create"]')
    expect(create.exists()).toBe(true)
    // Пункт выглядит как обычный вариант: только само наименование, без подписей.
    expect(create.text()).toBe('ООО "Бебра"')
  })

  it('пункт «Создать» ставит каноничное написание, не привязывая запись', async () => {
    const wrapper = mountInput({
      editable: true,
      fetcher: vi.fn().mockResolvedValue(answer({ canonical: 'ООО "Бебра"', matched: false })),
    })

    await type(wrapper, 'ооо "бебра')
    await wrapper.get('[data-testid="create-organization-option-create"]').trigger('mousedown')

    expect(wrapper.emitted('update:modelValue').at(-1)).toEqual(['ООО "Бебра"'])
    // Записи в справочнике нет, значит и id нет: подача уйдёт наименованием.
    expect(wrapper.emitted('select').at(-1)).toEqual([null])
    expect(wrapper.find('[data-testid="create-organization-list"]').exists()).toBe(false)
  })

  it('пункт «Создать» соседствует с найденными записями и берётся с клавиатуры', async () => {
    const item = { id: 4, name: 'ООО "Бебра Групп"' }
    const wrapper = mountInput({
      editable: true,
      fetcher: vi.fn().mockResolvedValue(answer({ items: [item], canonical: 'ООО "Бебра"', matched: false })),
    })

    await type(wrapper, 'ооо "бебра')
    expect(wrapper.findAll('[data-testid="create-organization-option"]')).toHaveLength(1)
    expect(wrapper.find('[data-testid="create-organization-option-create"]').exists()).toBe(true)

    // Стрелка вниз дважды: первая позиция - найденная запись, вторая - пункт создания.
    const input = wrapper.get('input')
    await input.trigger('keydown.down')
    await input.trigger('keydown.down')
    await input.trigger('keydown.enter')

    expect(wrapper.emitted('update:modelValue').at(-1)).toEqual(['ООО "Бебра"'])
    expect(wrapper.emitted('select').at(-1)).toEqual([null])
  })

  it('пункта «Создать» нет на одном-двух символах', async () => {
    const wrapper = mountInput({
      editable: true,
      fetcher: vi.fn().mockResolvedValue(answer({ canonical: 'Ма', matched: false })),
    })

    await type(wrapper, 'м')
    expect(wrapper.find('[data-testid="create-organization-option-create"]').exists()).toBe(false)

    await type(wrapper, 'ма')
    expect(wrapper.find('[data-testid="create-organization-option-create"]').exists()).toBe(false)
  })

  it('принятый пункт «Создать» не всплывает повторно при возврате в поле', async () => {
    const wrapper = mountInput({
      editable: true,
      fetcher: vi.fn().mockResolvedValue(answer({ canonical: 'ООО "Бебра"', matched: false })),
    })

    await type(wrapper, 'ооо "бебра')
    await wrapper.get('[data-testid="create-organization-option-create"]').trigger('mousedown')
    await wrapper.setProps({ modelValue: 'ООО "Бебра"' })

    await wrapper.get('input').trigger('focus')
    expect(wrapper.find('[data-testid="create-organization-option-create"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="create-organization-list"]').exists()).toBe(false)
  })

  it('пункта «Создать» нет, когда наименование в справочнике уже есть', async () => {
    const item = { id: 5, name: 'ООО Демо-Партнёр' }
    const wrapper = mountInput({
      editable: true,
      fetcher: vi.fn().mockResolvedValue(answer({ items: [item], canonical: item.name, matched: true })),
    })

    await type(wrapper, 'демо-партнёр')

    expect(wrapper.find('[data-testid="create-organization-option"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="create-organization-option-create"]').exists()).toBe(false)
  })

  it('пункта «Создать» нет у наименования, из которого запись не выйдет', async () => {
    const wrapper = mountInput({
      editable: true,
      fetcher: vi.fn().mockResolvedValue(answer({ canonical: '---', degenerate: true, matched: null })),
    })

    await type(wrapper, '---')

    expect(wrapper.find('[data-testid="create-organization-option-create"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="create-organization-list"]').exists()).toBe(false)
  })

  it('пункта «Создать» нет, пока ответ не пришёл', async () => {
    const wrapper = mountInput({
      editable: true,
      fetcher: vi.fn().mockImplementation(() => new Promise(() => {})),
    })

    await type(wrapper, 'ооо "бебра')

    expect(wrapper.find('[data-testid="create-organization-option-create"]').exists()).toBe(false)
  })

  it('без права канон не применяется и окно не открывается', async () => {
    const fetcher = vi.fn().mockResolvedValue(answer({ canonical: 'ООО "Братишк"' }))
    const wrapper = mountInput({ editable: false, modelValue: 'ооо "братишк', fetcher })

    await wrapper.get('input').trigger('blur')

    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(wrapper.find('[data-testid="create-organization-list"]').exists()).toBe(false)
  })
})
