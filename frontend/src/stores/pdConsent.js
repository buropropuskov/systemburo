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
  // Когда появилась действующая редакция: показывается человеку рядом с номером.
  const versionAt = ref('');
  // HTML согласия. В system_settings он лежит СЫРЫМ (TextConstructor отдаёт
  // нефильтрованный вывод редактора наружу через v-model), поэтому рендерить
  // его можно только через sanitizeHtml - см. PDConsentOverlay.
  const html = ref('');
  const docMeta = ref(null);

  // Один промис на все конкурентные вызовы: за состоянием ходят и App.created,
  // и успешный логин, и маркер отказа из ответа API - иначе на входе уходит
  // три одинаковых GET.
  let inflight = null;
  // Поколение состояния. Ответ, начатый до смены поколения, относится к прошлой
  // сессии (вошёл другой пользователь, вышли, только что записали согласие) и
  // применять его нельзя: он вернул бы чужую редакцию текста или снятое окно.
  let generation = 0;

  /** Пометить летящие запросы устаревшими и освободить место новому. */
  function invalidateInflight() {
    generation += 1;
    inflight = null;
  }

  function applyState(state) {
    required.value = Boolean(state?.required);
    version.value = Number(state?.version) || 0;
    versionAt.value = typeof state?.version_at === 'string' ? state.version_at : '';
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
    // force идёт на сервер даже поверх летящего запроса: он и означает «состояние
    // сменилось». Переиспользовать чужой промис тут нельзя - именно так ответ на
    // запрос предыдущего пользователя дописался бы в сессию следующего.
    if (force) invalidateInflight();
    else if (inflight) return inflight;
    else if (resolved.value) return;

    const startedAt = generation;
    const request = (async () => {
      try {
        const state = await getConsentGate();
        if (startedAt !== generation) return;
        applyState(state);
      } catch {
        // Сеть или сервер недоступны: resolved НЕ поднимаем. Окно на догадке не
        // показываем (иначе первый же 502 запирает всех) и доступ на догадке не
        // открываем - его режет серверный гейт.
      } finally {
        if (inflight === request) inflight = null;
      }
    })();
    inflight = request;
    return request;
  }

  /**
   * Записывает согласие на текущую редакцию. Ошибку не глотает: показать её
   * пользователю должен вызывающий компонент, иначе клик по кнопке выглядит
   * проигнорированным.
   * @returns {Promise<void>}
   */
  async function accept() {
    const state = await acceptConsent();
    // Летящий запрос состояния стартовал до записи согласия и вернёт required:
    // true - без сброса поколения он вернул бы окно уже согласившемуся.
    invalidateInflight();
    applyState(state);
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
    // Ответ, летящий с прошлой сессии, иначе допишет состояние вышедшего юзера.
    invalidateInflight();
    resolved.value = false;
    required.value = false;
    version.value = 0;
    versionAt.value = '';
    html.value = '';
    docMeta.value = null;
  }

  return {
    resolved,
    required,
    version,
    versionAt,
    html,
    docMeta,
    refresh,
    accept,
    markRequiredFromResponse,
    reset,
  };
});
