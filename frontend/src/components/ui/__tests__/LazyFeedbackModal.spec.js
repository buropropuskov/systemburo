import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { defineComponent, ref, onMounted, watch } from 'vue';
import { mount, flushPromises } from '@vue/test-utils';
import LazyFeedbackModal from '../LazyFeedbackModal.vue';
import { loadFeedbackModal } from '@/utils/feedbackModalLoader';

const { notify } = vi.hoisted(() => ({ notify: vi.fn() }));
vi.mock('@/utils/feedbackModalLoader', () => ({ loadFeedbackModal: vi.fn() }));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify }) }));

let wrapper, initial;
const Window = defineComponent({
  props: { show: Boolean, autoFocus: Boolean },
  emits: ['update:show', 'submitted'],
  setup(props) {
    const message = ref('');
    onMounted(() => { initial.push(props.show); });
    watch(() => props.show, show => initial.push(show));
    return { message };
  },
  template: '<section v-show="show"><input v-model="message"><button @click="$emit(\'update:show\', false)">Close</button><button @click="$emit(\'submitted\')">Submit</button></section>',
});

beforeEach(() => {
  vi.clearAllMocks();
  initial = [];
  loadFeedbackModal.mockResolvedValue({ default: Window });
});
afterEach(() => wrapper?.unmount());

describe('LazyFeedbackModal', () => {
  it('does not request the window until it is opened; mounts closed before showing it', async () => {
    wrapper = mount(LazyFeedbackModal, { props: { show: false } });
    expect(loadFeedbackModal).not.toHaveBeenCalled();
    await wrapper.setProps({ show: true });
    await flushPromises();
    expect(loadFeedbackModal).toHaveBeenCalledOnce();
    expect(initial).toEqual([false, true]);
  });
  it('keeps the same instance and text for repeated openings and forwards props/events', async () => {
    const submitted = vi.fn();
    wrapper = mount(LazyFeedbackModal, { props: { show: true }, attrs: { autoFocus: false, onSubmitted: submitted } });
    await flushPromises();
    const input = wrapper.get('input').element;
    await wrapper.get('input').setValue('Сохранённый текст');
    await wrapper.setProps({ show: false });
    await wrapper.setProps({ show: true });
    await flushPromises();
    expect(wrapper.get('input').element).toBe(input);
    expect(wrapper.get('input').element.value).toBe('Сохранённый текст');
    expect(wrapper.findComponent(Window).props('autoFocus')).toBe(false);
    expect(loadFeedbackModal).toHaveBeenCalledOnce();
    await wrapper.get('button').trigger('click');
    expect(wrapper.emitted('update:show')).toEqual([[false]]);
    await wrapper.findAll('button')[1].trigger('click');
    expect(submitted).toHaveBeenCalledOnce();
  });
  it('does not open a window if it was closed while the module was loading', async () => {
    let resolve;
    loadFeedbackModal.mockReturnValue(new Promise(done => { resolve = done; }));
    wrapper = mount(LazyFeedbackModal, { props: { show: true } });
    await wrapper.setProps({ show: false });
    resolve({ default: Window });
    await flushPromises();
    expect(wrapper.findComponent(Window).exists()).toBe(false);
    expect(notify).not.toHaveBeenCalled();
  });
  it('reports a load failure through the existing notification store and allows another request', async () => {
    loadFeedbackModal.mockRejectedValueOnce(new Error('Unavailable module'));
    wrapper = mount(LazyFeedbackModal, { props: { show: true } });
    await flushPromises();
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
    expect(wrapper.emitted('update:show')).toEqual([[false]]);
    await wrapper.setProps({ show: false });
    await wrapper.setProps({ show: true });
    await flushPromises();
    expect(wrapper.findComponent(Window).props('show')).toBe(true);
    expect(loadFeedbackModal).toHaveBeenCalledTimes(2);
  });
  it('ignores a rejected request after unmount', async () => {
    let reject;
    loadFeedbackModal.mockReturnValue(new Promise((_, fail) => { reject = fail; }));
    wrapper = mount(LazyFeedbackModal, { props: { show: true } });
    wrapper.unmount();
    reject(new Error('Unavailable module'));
    await flushPromises();
    expect(notify).not.toHaveBeenCalled();
  });
});
