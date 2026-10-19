import { openItemFromRoute, readOpenIdFromRoute } from './openQueryParam';

const pending = new WeakMap();
const routeKey = route => JSON.stringify([route?.path, route?.query]);

export async function openRegistrySearchLink(view, items, open, fetchPage, systemRead) {
  const id = readOpenIdFromRoute(view.$route);
  if (!id) return false;
  const key = routeKey(view.$route);
  if (pending.get(view) === key) return false;
  pending.set(view, key);
  try { return await openRegistrySearchResult({ router: view.$router, getRoute: () => view.$?.isUnmounted ? null : view.$route, items, open, fetchPage, systemRead }); }
  catch (error) { console.error('Не удалось открыть результат поиска', error); return false; }
  finally { if (pending.get(view) === key) pending.delete(view); }
}

// Поиск охватывает объединение доступных разделов; выбранная вкладка может быть уже.
// Догружаем карточку отдельно, не заменяя строки вкладки результатами этой загрузки.
export async function openRegistrySearchResult({ router, getRoute, items, open, fetchPage, systemRead = false }) {
  const route = getRoute(), id = readOpenIdFromRoute(route);
  if (!id) return false;
  const key = routeKey(route), current = () => routeKey(getRoute()) === key;
  if (openItemFromRoute({ router, route, items, open })) return true;
  const query = typeof route.query?.q === 'string' ? route.query.q.trim() : '';
  for (let page = 1; page <= 200; page += 1) {
    if (!current()) return false;
    const { items: rows, meta } = await fetchPage({ filter_type: systemRead ? 'all_system' : 'all', search_query: query, page, per_page: 100 });
    if (!current()) return false;
    if (openItemFromRoute({ router, route: getRoute(), items: rows, open })) return true;
    if (!rows.length || page * 100 >= (meta?.total || 0)) return false;
  }
  return false;
}
