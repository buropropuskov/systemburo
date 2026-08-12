import { defineStore } from 'pinia';
import { ref } from 'vue';

/**
 * Стор требования сменить пароль (#1911).
 *
 * Источник правды - сервер: гейт отбивает protected-запросы 403 с кодом
 * PASSWORD_CHANGE_REQUIRED, и об этом клиент узнаёт из ответа очередного запроса.
 * Своей копии флага фронт не держит и в отдельную ручку за ним не ходит: она
 * разошлась бы с гейтом ровно в тот момент, когда это дороже всего - после смены
 * пароля в соседней вкладке или после того, как срок пароля истёк прямо в работе.
 */
export const usePasswordChangeStore = defineStore('passwordChange', () => {
  const required = ref(false);

  /**
   * Поднять требование по маркеру отказа в ответе API.
   * Вызывается из обработчика ответов в api/client.js.
   */
  function markRequiredFromResponse() {
    required.value = true;
  }

  /** Сброс при выходе и после успешной смены: следующий вошедший спросит своё. */
  function reset() {
    required.value = false;
  }

  return { required, markRequiredFromResponse, reset };
});
