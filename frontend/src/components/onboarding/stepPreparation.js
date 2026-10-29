import { applyReveal } from './reveal';
import { waitForPageSettled } from './pageSettled';
import { findTourTarget, isTourTargetVisible } from './targetResolver';
import { readStepOwner, waitForStepOwner } from './ownerReadiness';

/** A form may keep the same semantic target while its requested blank changes. */
export function waitSelectorOf(step) { return step?.waitFor || step?.element; }

function blocked(step, reason) {
  return Object.assign(new Error('Learning step is not ready'), { stepId: step?.id, reason });
}

const FIRST_TARGET_TIMEOUT = 4000;
const REVEAL_TARGET_TIMEOUT = 9000;
// Цель, приезжающая ДАННЫМИ уже открытого узла: девять секунд ей ни к чему - у
// заявки без дополнения её нет вовсе, и подряд такие шаги вешали тур (#2584).
const DATA_TARGET_TIMEOUT = 2500;
// Потолок ожидания загрузки данных страницы. Дольше держать человека на «Далее»
// нельзя, а списки бюро на живом стенде приезжают за полторы-две секунды.
const PAGE_SETTLE_TIMEOUT = 5000;

/**
 * Сколько ждать цель шага, которому нужно раскрытие узла.
 *
 * @param {{ reveal?: { open?: string } }} step
 * @returns {number}
 */
function targetTimeoutFor(step) {
  return step?.reveal?.open ? REVEAL_TARGET_TIMEOUT : FIRST_TARGET_TIMEOUT;
}

export function createStepPreparation({
  store, watchers, applyDemoAttachment, beforePrepare, waitSelectorOf, waitForElement, ensureInView, demoFallback,
  prepareTarget = async (element, block, operation) => {
    await ensureInView(element, block, operation.signal);
    return element;
  },
}) {
  /**
   * Перед переходом на шаг готовим DOM: ставим демо-вложение и дожидаемся
   * появления целевого элемента - иначе driver подсветит пустоту, если данные
   * ещё грузятся (баг: шаг всплывал раньше, чем элемент попадал в DOM).
   *
   * Опциональный шаг (`optional`, напр. доп.поля «при наличии»): если элемента
   * нет за короткий таймаут - возвращаем false, и onNextClick пропускает шаг.
   *
   * Шаг с демо-скриншотом (`demo`) не пропускаем никогда: у нового пользователя
   * система пуста (ни заявок, ни вложений), и молчаливый пропуск отнимал бы у него
   * ровно то, ради чего тур и заведён. Вместо подсветки показываем скриншот - об
   * этом и говорит STEP_DEMO_FALLBACK.
   *
   * @param {number} globalIndex
   * @returns {Promise<boolean|string>} false = пропустить шаг, STEP_DEMO_FALLBACK =
   *   показать без подсветки со скриншотом, true = вести шаг как обычно
   */
  return async function prepareStep(globalIndex, operation, { initial = false } = {}) {
    if (!operation.isCurrent()) return;
    beforePrepare(globalIndex);
    // Тур уже двинулся - наблюдение прошлого шага снимаем сразу. Иначе оно
    // срабатывало на узле, который открывает СЛЕДУЮЩИЙ шаг (карточку заявки), и
    // тур перескакивал через него.
    watchers.stopAll();
    const step = store.steps[globalIndex];
    const emptyResult = () => step.demo || initial || !step.optional ? demoFallback : false;
    async function ownerReady(key, availability) {
      const state = await waitForStepOwner(key, { timeout: PAGE_SETTLE_TIMEOUT, signal: operation.signal, availability });
      if (!operation.isCurrent() || state.kind === 'cancelled') return state;
      if (['error', 'forbidden', 'timeout'].includes(state.kind)) throw blocked(step, state.kind);
      return state;
    }
    if (step.dataPrerequisite) {
      const prerequisite = await ownerReady(step.dataPrerequisite);
      if (!operation.isCurrent() || prerequisite.kind === 'cancelled') return;
      if (prerequisite.kind === 'empty') return emptyResult();
    }
    if (step.demoScene) await store.prepareCenterDemo(step.demoScene, operation);
    if (!operation.isCurrent()) return;
    if (step.reveal?.prepare) {
      await applyReveal([{ ...step, reveal: { ...step.reveal, open: step.reveal.prepare } }], 0, { closeOthers: false, signal: operation.signal });
      const preparedOwner = await ownerReady(step.dataOwner, step.dataAvailability);
      if (!operation.isCurrent() || preparedOwner.kind === 'cancelled') return;
      if (preparedOwner.kind === 'empty') return emptyResult();
    }
    const attachmentChanged = applyDemoAttachment(globalIndex);
    const revealed = await applyReveal(store.steps, globalIndex, { closeOthers: false, signal: operation.signal });
    if (!operation.isCurrent()) return;
    if (!step?.element) return true;
    const owner = await ownerReady(step.dataOwner, step.dataAvailability);
    if (!operation.isCurrent() || owner.kind === 'cancelled') return;
    if (owner.kind === 'empty' && !findTourTarget(waitSelectorOf(step))) {
      return emptyResult();
    }
    // Необязательный шаг ждём коротко (700 мс): нет цели - значит её тут нет
    // (#1771). Шагу с `waitsData` даём 2,5 с - цель приезжает данными узла (#2584).
    // Полное ожидание - когда раскрывали узел, меняли бланк или шаг обязателен.
    // Сперва даём странице договорить с сервером - иначе состав зависит от сети.
    const scope = () => findTourTarget(step.dataReady || step.waitFor || step.element) || document;
    const settled = await waitForPageSettled(PAGE_SETTLE_TIMEOUT, operation.signal, scope);
    if (!operation.isCurrent()) return;
    if (!settled) throw blocked(step, 'load-timeout');
    // `dataReady` - узел, приезжающий с данными, внутри которого живёт цель: есть
    // он - цели ждать нечего. Иначе три неприменимых шага вешали тур на 8 с (#2590).
    const ready = step.dataReady ? await waitForElement(step.dataReady, DATA_TARGET_TIMEOUT, operation.signal) : null;
    if (!operation.isCurrent()) return;
    const needsLongWait = revealed || attachmentChanged || !step.optional;
    const timeout = initial ? REVEAL_TARGET_TIMEOUT : (needsLongWait ? targetTimeoutFor(step) : (ready ? 300 : (step.waitsData ? DATA_TARGET_TIMEOUT : 700)));
    const el = await waitForElement(waitSelectorOf(step), timeout, operation.signal);
    if (!operation.isCurrent()) return;
    if (el) {
      // Подсвечиваем ровно то, что человек видит: длинная форма заявки остаётся
      // прокрученной от прошлого шага, и цель могла уехать за край экрана.
      const target = step.element === waitSelectorOf(step) ? el : findTourTarget(step.element);
      const prepared = await prepareTarget(target, step.scrollTo, operation);
      if (!operation.isCurrent()) return;
      if (!prepared || !isTourTargetVisible(prepared)) throw blocked(step, 'missing-target');
      if (step.dataOwner && !['ready', 'empty'].includes(readStepOwner(step.dataOwner, document, step.dataAvailability)?.state)) {
        throw blocked(step, 'owner-changed');
      }
      return true;
    }
    if (step.demo) return demoFallback;
    if (step.optional && !initial) return false;
    throw blocked(step, 'missing-target');
  };
}
