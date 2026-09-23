// Модульные тесты компонента UploadDocuments.
// Основано на: src/components/UploadDocuments.vue из dokkee-frontend.
// Проверяет фильтрацию по расширению, список файлов, согласие на обработку ПД
// и эмиссию события document-uploaded при старте обработки.
//
// Стек: Vitest + @vue/test-utils + jsdom.

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import UploadDocuments from '@/components/UploadDocuments.vue'

// Стабы дочерних компонентов — их поведение тестируется отдельно.
const stubs = {
  DropMenu: true,
  UploadFilesList: true,
}

describe('UploadDocuments.vue', () => {
  let wrapper

  beforeEach(() => {
    wrapper = mount(UploadDocuments, {
      props: { collapsed: false, visible: true, processing: false },
      global: { stubs },
    })
  })

  it('при старте показывает сообщение о пустом списке', () => {
    expect(wrapper.text()).toContain('Загрузите файлы в поле выше для просмотра')
  })

  it('фильтрует неподдерживаемые форматы (оставляет только pdf/doc/docx)', async () => {
    const pdf  = new File([''], 'contract.pdf', { type: 'application/pdf' })
    const png  = new File([''], 'image.png',    { type: 'image/png' })
    const docx = new File([''], 'letter.docx',  { type: '' })
    const txt  = new File([''], 'readme.txt',   { type: 'text/plain' })

    await wrapper.vm.onFilesAdded([pdf, png, docx, txt])

    expect(wrapper.vm.files).toHaveLength(2)
    expect(wrapper.vm.files.map(f => f.name)).toEqual(['contract.pdf', 'letter.docx'])
  })

  it('удаляет файл из списка по removeFile(index)', async () => {
    const f1 = new File([''], 'a.pdf')
    const f2 = new File([''], 'b.pdf')
    await wrapper.vm.onFilesAdded([f1, f2])

    wrapper.vm.removeFile(0)
    expect(wrapper.vm.files).toHaveLength(1)
    expect(wrapper.vm.files[0].name).toBe('b.pdf')
  })

  it('formatFileSize правильно форматирует значения', () => {
    expect(wrapper.vm.formatFileSize(500)).toBe('500 B')
    expect(wrapper.vm.formatFileSize(2048)).toBe('2.0 KB')
    expect(wrapper.vm.formatFileSize(5_242_880)).toBe('5.0 MB')
  })

  it('truncateFileName обрезает длинные имена', () => {
    const long = 'очень_длинное_имя_файла_договор_с_поставщиком.pdf'
    expect(wrapper.vm.truncateFileName(long, 20)).toBe('очень_длинное_имя_фа..')
  })

  it('startProcessing блокирован пока не отмечено согласие', async () => {
    const f = new File([''], 'ok.pdf')
    await wrapper.vm.onFilesAdded([f])

    wrapper.vm.startProcessing()
    expect(wrapper.emitted('document-uploaded')).toBeFalsy()
  })

  it('эмитит document-uploaded после установки согласия', async () => {
    const f = new File([''], 'ok.pdf')
    await wrapper.vm.onFilesAdded([f])
    wrapper.vm.agreed = true

    wrapper.vm.startProcessing()
    expect(wrapper.emitted('processing-started')).toBeTruthy()
    expect(wrapper.emitted('document-uploaded')).toBeTruthy()
    expect(wrapper.emitted('document-uploaded')[0][0].name).toBe('ok.pdf')
  })

  it('availableFiles уменьшается после каждого startProcessing', async () => {
    const files = [new File([''], 'a.pdf'), new File([''], 'b.pdf')]
    await wrapper.vm.onFilesAdded(files)
    wrapper.vm.agreed = true

    expect(wrapper.vm.availableFiles).toBe(2)
    wrapper.vm.startProcessing()
    expect(wrapper.vm.availableFiles).toBe(1)
  })

  // //возможны изменения// — после интеграции с бэкендом здесь должен
  // появиться тест на обработку ошибки загрузки (axios reject).
  it.todo('при ошибке загрузки показывает уведомление об ошибке')
})
