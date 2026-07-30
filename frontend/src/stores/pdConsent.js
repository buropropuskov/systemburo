import { defineStore } from 'pinia';
import { ref } from 'vue';
import { getConsentGate, acceptConsent } from '@/api/pdConsent';

/**
 * Стор гейта согласия на обработку персональных данных (#1567).
 *
 * Единственный источник правды о том, нужно ли спрашивать согласие, - ответ
 * сервера (`GET /consents/gate`): он уже учитывает и выключатель запроса, и
 * пустой текст, и исключение супер-администратора. Фронт эти правила не
 * дублирует, иначе окно и серверный гейт разойдутся в том, кого закрывать.
 */
export const usePDConsentStore = defineStore('pdConsent', () => {
  // Ответ гейта получен. Пока false, окно не показываем: иначе на холодном
  // старте оно мигнёт до того, как выяснится, что согласие не требуется.
  const resolved = ref(false);
  const required = ref(false);
  const version = ref(0);
  // HTML согласия. В system_settings он лежит СЫРЫМ (TextConstructor отдаёт
  // нефильтрованный вывод редактора наружу через v-model), поэтому рендерить
  // его можно только через sanitizeHtml - см. PDConsentOverlay.
  const html = ref('');
  const docMeta = ref(null);

  // Один промис на все конкурентные вызовы: за состоянием ходят и App.created,
  // и успешный логин, и маркер отказа из ответа API - иначе на входе уходит
  // три одинаковых GET.
  let inflight = null;

  function applyState(state) {
    required.value = Boolean(state?.required);
    version.value = Number(state?.version) || 0;
    html.value = typeof state?.text === 'string' ? state.text : '';
    docMeta.value = state?.document || null;
    resolved.value = true;
  }

  /**
   * Запрашивает состояние гейта.
   * @param {boolean} [force] перечитать, даже если состояние уже известно
   * @returns {Promise<void>}
   */
  async function refresh(force = false) {
    if (inflight) return inflight;
    if (resolved.value && !force) return;
    inflight = (async () => {
      try {
        applyState(await getConsentGate());
      } catch {
        // Сеть или сервер недоступны: resolved НЕ поднимаем. Окно на догадке не
        // показываем (иначе первый же 502 запирает всех) и доступ на догадке не
        // открываем - его режет серверный гейт.
      } finally {
        inflight = null;
      }
    })();
    return inflight;
  }

  /**
   * Записывает согласие на текущую редакцию. Ошибку не глотает: показать её
   * пользователю должен вызывающий компонент, иначе клик по кнопке выглядит
   * проигнорированным.
   * @returns {Promise<void>}
   */
  async function accept() {
    applyState(await acceptConsent());
  }

  /**
   * Поднять флаг по маркеру отказа в ответе API (403 + `consent_required`).
   * Нужен, когда требуемую редакцию подняли в другой вкладке: об этом клиент
   * узнаёт из ответа очередного запроса раньше, чем сам сходит за состоянием
   * гейта, и без флага пользователь получил бы стену отказов вместо окна.
   * Текст перечитываем сразу - показанная редакция должна быть свежей.
   *
   * Вызывается из обработчика ответов в api/client.js.
   */
  function markRequiredFromResponse() {
    required.value = true;
    resolved.value = true;
    refresh(true);
  }

  /** Сброс при выходе: следующий пользователь на этом устройстве спросит своё. */
  function reset() {
    resolved.value = false;
    required.value = false;
    version.value = 0;
    html.value = '';
    docMeta.value = null;
  }

  return {
    resolved,
    required,
    version,
    html,
    docMeta,
    refresh,
    accept,
    markRequiredFromResponse,
    reset,
  };
});
