// Модульные тесты компонента DropMenu (drag-and-drop загрузка файлов).
// Основано на: src/components/DropMenu.vue из dokkee-frontend.
// Стек: Vitest + @vue/test-utils.

import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import DropMenu from '@/components/DropMenu.vue'

describe('DropMenu.vue', () => {
  it('отображает инструкцию загрузки и скрытый input[type=file]', () => {
    const wrapper = mount(DropMenu)
    expect(wrapper.text()).toContain('Загрузите файл(-ы) сюда')
    expect(wrapper.find('input[type="file"]').exists()).toBe(true)
  })

  it('эмитит событие files-added при drop', async () => {
    const wrapper = mount(DropMenu)
    const file = new File(['fake pdf'], 'contract.pdf', { type: 'application/pdf' })

    await wrapper.trigger('drop', {
      dataTransfer: { files: [file] },
    })

    expect(wrapper.emitted('files-added')).toBeTruthy()
    const payload = wrapper.emitted('files-added')[0][0]
    expect(payload).toHaveLength(1)
    expect(payload[0].name).toBe('contract.pdf')
  })

  it('эмитит событие files-added при выборе через input', async () => {
    const wrapper = mount(DropMenu)
    const file = new File(['txt'], 'note.txt', { type: 'text/plain' })

    const input = wrapper.find('input[type="file"]')
    // //возможны изменения// — JSDOM не позволяет напрямую присваивать files,
    // поэтому эмулируем через замену свойства
    Object.defineProperty(input.element, 'files', { value: [file] })
    await input.trigger('change')

    expect(wrapper.emitted('files-added')).toBeTruthy()
    expect(wrapper.emitted('files-added')[0][0][0].name).toBe('note.txt')
  })

  it('переключает класс dragover при dragover и dragleave', async () => {
    const wrapper = mount(DropMenu)
    const zone = wrapper.find('.drop-menu__zone')

    expect(zone.classes()).not.toContain('drop-menu__zone--dragover')

    await zone.trigger('dragover')
    expect(zone.classes()).toContain('drop-menu__zone--dragover')

    await zone.trigger('dragleave')
    expect(zone.classes()).not.toContain('drop-menu__zone--dragover')
  })

  it('клик по зоне открывает диалог выбора файла (триггерит input.click)', async () => {
    const wrapper = mount(DropMenu)
    const input = wrapper.find('input[type="file"]')
    const clickSpy = vi.spyOn(input.element, 'click')

    await wrapper.find('.drop-menu__zone').trigger('click')
    expect(clickSpy).toHaveBeenCalled()
  })
})
