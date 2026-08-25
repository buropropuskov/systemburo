import { openItemFromRoute } from '@/utils/openQueryParam';
import { readSearchFromRoute } from '@/utils/searchQueryParam';

/**
 * Открытие записи по ссылке из сквозного поиска для экранов на Options API.
 *
 * Паттерн одинаков у восьми экранов: раскрыть запись после загрузки списка и ещё раз
 * при смене адреса - из поиска можно уйти на страницу, где уже стоишь, и тогда
 * компонент не монтируется заново, а список не перезагружается.
 *
 * Примесь, а не composable: экраны написаны на Options API без `setup()`, и ради
 * связки пришлось бы заводить его в каждом - вышло бы больше кода, чем логики.
 *
 * Экран сам вызывает `openFromSearchLink()` там, где список приехал.
 *
 * @param {(vm: object) => object[]} itemsOf откуда брать список
 * @param {string} openMethod имя метода экрана, раскрывающего запись
 * @param {(row: object) => (number|string|undefined)} [idOf] если id лежит внутри обёртки
 * @param {string} [searchField] поле экрана со строкой поиска - примесь заполнит его из
 *        адреса, чтобы список сузился тем же запросом, по которому запись нашлась
 * @returns {object} примесь с методом `openFromSearchLink` и наблюдателем за адресом
 */
export function openFromSearchLink(itemsOf, openMethod, idOf, searchField) {
  return {
    data() {
      return searchField ? { [searchField]: readSearchFromRoute(this.$route) } : {};
    },
    watch: {
      '$route.query.open'(value) { if (value) this.openFromSearchLink(); },
    },
    methods: {
      openFromSearchLink() {
        openItemFromRoute({
          router: this.$router, route: this.$route,
          items: itemsOf(this), open: this[openMethod], idOf,
        });
      },
    },
  };
}
