import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils';

const getDocument = vi.fn();
const GlobalWorkerOptions = { workerPort: null };
const sharedWorker = { destroy: vi.fn() };
const PDFWorker = { create: vi.fn(() => sharedWorker) };

vi.mock('pdfjs-dist', () => ({
  getDocument: (...a) => getDocument(...a),
  GlobalWorkerOptions,
  PDFWorker,
}));
// ?worker отдаёт конструктор Worker - подменяем безобидной заглушкой.
vi.mock('pdfjs-dist/build/pdf.worker.min.mjs?worker', () => ({
  default: class {
    postMessage() {}

    terminate() {}

    addEventListener() {}

    removeEventListener() {}
  },
}));

import PdfDocumentViewer from '../PdfDocumentViewer.vue';

enableAutoUnmount(afterEach);

function makePdf(numPages) {
  return {
    numPages,
    getPage: vi.fn(async () => ({
      getViewport: ({ scale }) => ({ width: 100 * scale, height: 200 * scale }),
      render: () => ({ promise: Promise.resolve() }),
    })),
  };
}

function deferred() {
  return Promise.withResolvers();
}

function makeTask(promise = Promise.resolve(makePdf(1))) {
  return { promise, destroy: vi.fn(async () => {}) };
}

function makeBlob() {
  return { arrayBuffer: async () => new ArrayBuffer(8) };
}

describe('PdfDocumentViewer', () => {
  beforeEach(() => {
    getDocument.mockReset();
    GlobalWorkerOptions.workerPort = null;
    PDFWorker.create.mockClear();
    sharedWorker.destroy.mockClear();
  });

  it('рендерит по canvas на каждую страницу PDF и настраивает воркер', async () => {
    getDocument.mockReturnValue(makeTask(Promise.resolve(makePdf(3))));
    const wrapper = mount(PdfDocumentViewer, { props: { blob: makeBlob() } });
    await flushPromises();
    await flushPromises();

    expect(getDocument).toHaveBeenCalledTimes(1);
    expect(GlobalWorkerOptions.workerPort).toBeTruthy();
    expect(PDFWorker.create).toHaveBeenCalledWith({ port: GlobalWorkerOptions.workerPort });
    expect(getDocument).toHaveBeenCalledWith({ data: expect.any(Uint8Array), worker: sharedWorker });
    expect(wrapper.findAll('canvas.pdf-viewer__page')).toHaveLength(3);
    expect(wrapper.emitted('loaded')?.[0]).toEqual([3]);
  });

  it('показывает ошибку и эмитит error, если PDF не открылся', async () => {
    const task = makeTask(Promise.reject(new Error('bad pdf')));
    getDocument.mockReturnValue(task);
    const wrapper = mount(PdfDocumentViewer, { props: { blob: makeBlob() } });
    await flushPromises();
    await flushPromises();

    expect(wrapper.text()).toContain('Не удалось показать документ');
    expect(wrapper.emitted('error')).toBeTruthy();
    expect(wrapper.findAll('canvas.pdf-viewer__page')).toHaveLength(0);
    expect(task.destroy).toHaveBeenCalledTimes(1);
  });

  it('без blob ничего не грузит', async () => {
    const wrapper = mount(PdfDocumentViewer, { props: { blob: null } });
    await flushPromises();

    expect(getDocument).not.toHaveBeenCalled();
    expect(wrapper.findAll('canvas').length).toBe(0);
  });

  it('перерендеривает при смене blob и не плодит canvas от старого документа', async () => {
    const oldTask = makeTask(Promise.resolve(makePdf(2)));
    getDocument.mockReturnValue(oldTask);
    const wrapper = mount(PdfDocumentViewer, { props: { blob: makeBlob() } });
    await flushPromises();
    await flushPromises();
    expect(wrapper.findAll('canvas.pdf-viewer__page')).toHaveLength(2);

    const newTask = makeTask();
    getDocument.mockReturnValue(newTask);
    await wrapper.setProps({ blob: makeBlob() });
    await flushPromises();
    await flushPromises();

    expect(getDocument).toHaveBeenCalledTimes(2);
    expect(wrapper.findAll('canvas.pdf-viewer__page')).toHaveLength(1);
    expect(oldTask.destroy).toHaveBeenCalledTimes(1);
    expect(newTask.destroy).not.toHaveBeenCalled();
  });

  it('отменяет pending load без ожидания и игнорирует его поздний результат', async () => {
    const pending = deferred();
    const cleanup = deferred();
    const oldTask = makeTask(pending.promise);
    oldTask.destroy.mockReturnValue(cleanup.promise);
    const newTask = makeTask();
    getDocument.mockReturnValueOnce(oldTask).mockReturnValueOnce(newTask);
    const wrapper = mount(PdfDocumentViewer, { props: { blob: makeBlob() } });
    await flushPromises();

    await wrapper.setProps({ blob: makeBlob() });
    await flushPromises();
    expect(oldTask.destroy).toHaveBeenCalledTimes(1);
    expect(wrapper.emitted('loaded')).toEqual([[1]]);
    expect(wrapper.findAll('canvas')).toHaveLength(1);

    const staleDoc = makePdf(3);
    pending.resolve(staleDoc);
    cleanup.resolve();
    await flushPromises();
    expect(staleDoc.getPage).not.toHaveBeenCalled();
    expect(oldTask.destroy).toHaveBeenCalledTimes(1);
    expect(newTask.destroy).not.toHaveBeenCalled();
    expect(sharedWorker.destroy).not.toHaveBeenCalled();
    expect(wrapper.emitted('loaded')).toEqual([[1]]);
    expect(wrapper.findAll('canvas')).toHaveLength(1);
  });

  it('после поздней ошибки старой загрузки сохраняет актуальный документ', async () => {
    const pending = deferred();
    const oldTask = makeTask(pending.promise);
    const newTask = makeTask();
    getDocument.mockReturnValueOnce(oldTask).mockReturnValueOnce(newTask);
    const wrapper = mount(PdfDocumentViewer, { props: { blob: makeBlob() } });
    await flushPromises();
    await wrapper.setProps({ blob: makeBlob() });
    await flushPromises();
    pending.reject(new Error('Loading aborted'));
    await flushPromises();
    expect(newTask.destroy).not.toHaveBeenCalled();
    expect(wrapper.emitted('error')).toBeUndefined();
    expect(wrapper.findAll('canvas')).toHaveLength(1);
  });

  it('не создаёт загрузку после устаревшего arrayBuffer', async () => {
    const pending = deferred();
    getDocument.mockReturnValue(makeTask());
    const wrapper = mount(PdfDocumentViewer, {
      props: { blob: { arrayBuffer: () => pending.promise } },
    });
    await flushPromises();
    await wrapper.setProps({ blob: makeBlob() });
    await flushPromises();
    pending.resolve(new ArrayBuffer(8));
    await flushPromises();
    expect(getDocument).toHaveBeenCalledTimes(1);
    expect(wrapper.emitted('loaded')).toEqual([[1]]);
  });

  it('не добавляет canvas после устаревшего getPage', async () => {
    const pending = deferred();
    const oldDoc = makePdf(1);
    oldDoc.getPage.mockReturnValue(pending.promise);
    const oldTask = makeTask(Promise.resolve(oldDoc));
    getDocument.mockReturnValueOnce(oldTask).mockReturnValueOnce(makeTask());
    const wrapper = mount(PdfDocumentViewer, { props: { blob: makeBlob() } });
    await flushPromises();
    await wrapper.setProps({ blob: makeBlob() });
    await flushPromises();
    const page = { render: vi.fn() };
    pending.resolve(page);
    await flushPromises();
    expect(page.render).not.toHaveBeenCalled();
    expect(wrapper.findAll('canvas')).toHaveLength(1);
    expect(wrapper.emitted('loaded')).toEqual([[1]]);
  });

  it.each(['resolve', 'reject'])('отменяет незавершённый render и игнорирует его %s', async (outcome) => {
    const pending = deferred();
    const oldDoc = makePdf(2);
    oldDoc.getPage.mockResolvedValue({
      getViewport: ({ scale }) => ({ width: 100 * scale, height: 200 * scale }),
      render: () => ({ promise: pending.promise }),
    });
    const oldTask = makeTask(Promise.resolve(oldDoc));
    const newTask = makeTask();
    getDocument.mockReturnValueOnce(oldTask).mockReturnValueOnce(newTask);
    const wrapper = mount(PdfDocumentViewer, { props: { blob: makeBlob() } });
    await flushPromises();
    const oldCanvas = wrapper.find('canvas').element;
    await wrapper.setProps({ blob: makeBlob() });
    await flushPromises();
    expect(oldTask.destroy).toHaveBeenCalledTimes(1);
    expect(wrapper.find('canvas').element).not.toBe(oldCanvas);
    pending[outcome](new Error('Rendering cancelled'));
    await flushPromises();
    expect(oldDoc.getPage).toHaveBeenCalledTimes(1);
    expect(wrapper.findAll('canvas')).toHaveLength(1);
    expect(wrapper.emitted('loaded')).toEqual([[1]]);
    expect(wrapper.emitted('error')).toBeUndefined();
    expect(newTask.destroy).not.toHaveBeenCalled();
  });

  it('очищает документ при ошибке рендера и позволяет повторить', async () => {
    const doc = makePdf(1);
    doc.getPage.mockRejectedValue(new Error('bad page'));
    const task = makeTask(Promise.resolve(doc));
    getDocument.mockReturnValueOnce(task).mockReturnValueOnce(makeTask());
    const wrapper = mount(PdfDocumentViewer, { props: { blob: makeBlob() } });
    await flushPromises();
    expect(task.destroy).toHaveBeenCalledTimes(1);
    expect(wrapper.emitted('error')).toHaveLength(1);
    await wrapper.find('button').trigger('click');
    await flushPromises();
    expect(wrapper.emitted('loaded')).toEqual([[1]]);
    expect(task.destroy).toHaveBeenCalledTimes(1);
  });

  it.each([false, true])('очищает загрузку при unmount (pending=%s)', async (isPending) => {
    const pending = deferred();
    const task = makeTask(isPending ? pending.promise : Promise.resolve(makePdf(1)));
    getDocument.mockReturnValue(task);
    const onLoaded = vi.fn();
    const wrapper = mount(PdfDocumentViewer, { props: { blob: makeBlob(), onLoaded } });
    await flushPromises();
    const emittedBefore = onLoaded.mock.calls.length;
    wrapper.unmount();
    expect(task.destroy).toHaveBeenCalledTimes(1);
    pending.resolve(makePdf(2));
    await flushPromises();
    expect(onLoaded).toHaveBeenCalledTimes(emittedBefore);
    expect(sharedWorker.destroy).not.toHaveBeenCalled();
  });

  it('не запускает getDocument после unmount во время arrayBuffer', async () => {
    const pending = deferred();
    const wrapper = mount(PdfDocumentViewer, {
      props: { blob: { arrayBuffer: () => pending.promise } },
    });
    await flushPromises();
    wrapper.unmount();
    pending.resolve(new ArrayBuffer(8));
    await flushPromises();
    expect(getDocument).not.toHaveBeenCalled();
  });

  it('при blob=null очищает страницы и loadingTask', async () => {
    const task = makeTask();
    getDocument.mockReturnValue(task);
    const wrapper = mount(PdfDocumentViewer, { props: { blob: makeBlob() } });
    await flushPromises();
    await wrapper.setProps({ blob: null });
    expect(task.destroy).toHaveBeenCalledTimes(1);
    expect(wrapper.findAll('canvas')).toHaveLength(0);
    expect(wrapper.text()).not.toContain('Загрузка');
  });

  it('очистка одного экземпляра не мешает соседнему и повторному открытию', async () => {
    const tasks = [makeTask(), makeTask(), makeTask()];
    tasks.forEach((task) => getDocument.mockReturnValueOnce(task));
    const first = mount(PdfDocumentViewer, { props: { blob: makeBlob() } });
    await flushPromises();
    const second = mount(PdfDocumentViewer, { props: { blob: makeBlob() } });
    await flushPromises();
    const port = GlobalWorkerOptions.workerPort;
    first.unmount();
    expect(tasks[0].destroy).toHaveBeenCalledTimes(1);
    expect(tasks[1].destroy).not.toHaveBeenCalled();
    expect(second.findAll('canvas')).toHaveLength(1);
    const reopened = mount(PdfDocumentViewer, { props: { blob: makeBlob() } });
    await flushPromises();
    expect(reopened.emitted('loaded')).toEqual([[1]]);
    expect(GlobalWorkerOptions.workerPort).toBe(port);
    expect(getDocument.mock.calls.every(([options]) => options.worker === sharedWorker)).toBe(true);
    expect(sharedWorker.destroy).not.toHaveBeenCalled();
  });

  it('при unmount отменяет активный рендер и игнорирует его rejection', async () => {
    const pending = deferred();
    const doc = makePdf(2);
    doc.getPage.mockResolvedValue({
      getViewport: ({ scale }) => ({ width: 100 * scale, height: 200 * scale }),
      render: () => ({ promise: pending.promise }),
    });
    const task = makeTask(Promise.resolve(doc));
    getDocument.mockReturnValue(task);
    const wrapper = mount(PdfDocumentViewer, { props: { blob: makeBlob() } });
    await flushPromises();
    wrapper.unmount();
    expect(task.destroy).toHaveBeenCalledTimes(1);
    pending.reject(new Error('Rendering cancelled'));
    await flushPromises();
    expect(doc.getPage).toHaveBeenCalledTimes(1);
    expect(wrapper.emitted('loaded')).toBeUndefined();
    expect(wrapper.emitted('error')).toBeUndefined();
  });
});
