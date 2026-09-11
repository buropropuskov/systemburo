import { defineStore } from 'pinia';

/**
 * Окно отмены ошибочной отметки прохода (#2437).
 *
 * Стор, а не модалка внутри таблицы: отметку ставят три компонента, и у всех трёх
 * блоки шаблона давно за порогом размера. Одно окно на приложение (по образцу
 * ConfirmDialog) держит и разметку, и поведение в одном месте, а таблицам остаётся
 * один вызов.
 */
export const usePassageRevertStore = defineStore('passageRevert', {
  state: () => ({
    /**
     * Открытый запрос на отмену либо null.
     * {kind, id, direction, tableId, subject, onDone}
     */
    request: null,
  }),
  actions: {
    /**
     * Открывает окно причины.
     *
     * @param {{kind: 'employees'|'cars', id: number, direction: 'entry'|'exit', tableId: number, subject: string, onDone: Function}} request
     */
    ask(request) {
      this.request = request;
    },
    close() {
      this.request = null;
    },
  },
});
