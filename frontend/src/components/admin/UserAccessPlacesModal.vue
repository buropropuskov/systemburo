<template>
  <Teleport to="body">
    <div
      class="modal-overlay"
      :class="{ 'modal-leaving': leaving }"
      data-testid="user-access-places-modal"
      @mousedown="onOverlayMousedown"
      @mouseup="onOverlayMouseup"
    >
      <div
        class="access-places-modal"
        @mousedown.stop
      >
        <header class="access-places-modal__header">
          <h3>Места доступа «{{ user?.username }}»</h3>
          <button
            class="close-btn"
            aria-label="Закрыть"
            @click="close"
          >
            ×
          </button>
        </header>

        <div class="access-places-modal__body">
          <p
            v-if="loading"
            class="access-places-modal__loading"
          >
            Загрузка...
          </p>

          <template v-else>
            <section class="access-block">
              <label class="block-label">Места разгрузки</label>
              <div class="places-container">
                <div
                  v-if="unloadPlaces.length"
                  class="places-grid"
                >
                  <div
                    v-for="place in unloadPlaces"
                    :key="place.id"
                    class="place-item"
                    :class="{ selected: selectedUnloadPlaceIds.includes(place.id) }"
                    @click="toggleUnloadPlace(place.id)"
                  >
                    {{ place.name }}
                  </div>
                </div>
                <div
                  v-else
                  class="no-places-message"
                >
                  <p>Нет доступных мест разгрузки</p>
                </div>
              </div>
            </section>

            <section class="access-block">
              <label class="block-label">Места прохода</label>
              <div class="places-container">
                <div
                  v-if="tables.length"
                  class="places-grid"
                >
                  <div
                    v-for="table in tables"
                    :key="table.id"
                    class="place-item"
                    :class="{ selected: selectedTableIds.includes(table.id) }"
                    @click="toggleTable(table.id)"
                  >
                    {{ tableName(table) }}
                  </div>
                </div>
                <div
                  v-else
                  class="no-places-message"
                >
                  <p>Нет доступных мест прохода</p>
                </div>
              </div>
            </section>
          </template>
        </div>

        <footer class="access-places-modal__footer">
          <button
            class="lk-button lk-button--ghost"
            :disabled="saving"
            @click="close"
          >
            Отмена
          </button>
          <button
            class="lk-button lk-button--primary"
            data-testid="access-places-save"
            :disabled="saving || loading || !isDirty"
            @click="save"
          >
            {{ saving ? 'Сохранение...' : 'Сохранить' }}
          </button>
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<script>
import { useOverlayClose } from '@/composables/useOverlayClose';
import { useDeletionsStore } from '@/stores/deletions';
import { registerDirtyTracker } from '@/utils/dirtyTracker';
import { apiRequest } from '@/api/client';
import {
  getUserUnloadPlaces,
  setUserUnloadPlaces,
  getUserTables,
  setUserTables,
} from '@/api/users';

/**
 * Модалка настройки мест доступа охранника ЧОП (#706): какие места разгрузки и
 * места прохода видит пользователь во вкладке «Доступные мне». Заменяет привязку
 * целиком (delete-all-then-recreate на бэке), не добавляет.
 */
export default {
  name: 'UserAccessPlacesModal',
  props: {
    user: { type: Object, default: null },
  },
  emits: ['close', 'updated'],
  setup() {
    const overlay = { close: () => {} };
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => overlay.close());
    return { onOverlayMousedown, onOverlayMouseup, overlay };
  },
  data() {
    return {
      leaving: false,
      loading: true,
      saving: false,
      unloadPlaces: [],
      tables: [],
      selectedUnloadPlaceIds: [],
      selectedTableIds: [],
      originalUnloadPlaceIds: [],
      originalTableIds: [],
    };
  },
  computed: {
    isDirty() {
      return (
        !this.sameSet(this.selectedUnloadPlaceIds, this.originalUnloadPlaceIds) ||
        !this.sameSet(this.selectedTableIds, this.originalTableIds)
      );
    },
  },
  created() {
    this.overlay.close = () => this.close();
  },
  async mounted() {
    document.addEventListener('keydown', this.onKeydown);
    this._stopGuard = registerDirtyTracker({
      isDirty: () => this.isDirty,
      getChanges: () => ['Изменены места доступа охранника'],
      save: async () => { await this.save(); },
    });
    await this.load();
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.onKeydown);
    this._stopGuard?.();
  },
  methods: {
    tableName(table) {
      return table.display_name || table.name || 'Без названия';
    },

    sameSet(a, b) {
      if (a.length !== b.length) return false;
      const sortedA = [...a].sort((x, y) => x - y);
      const sortedB = [...b].sort((x, y) => x - y);
      return sortedA.every((v, i) => v === sortedB[i]);
    },

    async load() {
      this.loading = true;
      const username = this.user?.username;
      try {
        const [allPlaces, allTables, userPlaces, userTables] = await Promise.all([
          this.fetchAllUnloadPlaces(),
          this.fetchAllTables(),
          getUserUnloadPlaces(username),
          getUserTables(username),
        ]);
        this.unloadPlaces = allPlaces;
        this.tables = allTables;
        // При ошибочном ответе GET .json() отдаёт { message } (truthy объект, не массив) -
        // Array.isArray отсекает его, иначе .map упал бы TypeError'ом.
        this.selectedUnloadPlaceIds = Array.isArray(userPlaces) ? userPlaces.map(p => p.id) : [];
        this.selectedTableIds = Array.isArray(userTables) ? userTables.map(t => t.id) : [];
        this.originalUnloadPlaceIds = [...this.selectedUnloadPlaceIds];
        this.originalTableIds = [...this.selectedTableIds];
      } catch (error) {
        console.error('Не удалось загрузить места доступа:', error);
        useDeletionsStore().notify({
          prefix: 'Не удалось загрузить места доступа: ',
          bold: 'ошибка сети',
          type: 'error',
        });
      } finally {
        this.loading = false;
      }
    },

    async fetchAllUnloadPlaces() {
      const response = await apiRequest('/unload-places');
      if (!response.ok) return [];
      return response.json();
    },

    async fetchAllTables() {
      const response = await apiRequest('/system-tables');
      if (!response.ok) return [];
      const data = await response.json();
      // /system-tables оборачивает каждый элемент в { table: {...} }; разворачиваем,
      // чтобы дальше читать плоско (как NavMenu) и совпасть с id из /users/:u/tables.
      return (data || [])
        .map(t => t.table || t)
        .filter(t => t.table_type !== 'cars');
    },

    toggleUnloadPlace(id) {
      const i = this.selectedUnloadPlaceIds.indexOf(id);
      if (i > -1) this.selectedUnloadPlaceIds.splice(i, 1);
      else this.selectedUnloadPlaceIds.push(id);
    },

    toggleTable(id) {
      const i = this.selectedTableIds.indexOf(id);
      if (i > -1) this.selectedTableIds.splice(i, 1);
      else this.selectedTableIds.push(id);
    },

    async save() {
      if (this.saving || !this.user?.username) return;
      this.saving = true;
      const username = this.user.username;
      try {
        const [placesRes, tablesRes] = await Promise.all([
          setUserUnloadPlaces(username, this.selectedUnloadPlaceIds),
          setUserTables(username, this.selectedTableIds),
        ]);
        // PUT идемпотентны (delete-all-then-recreate), поэтому фиксируем original
        // по каждому набору отдельно: при частичном сбое dirty остаётся только у упавшего.
        if (placesRes.ok) this.originalUnloadPlaceIds = [...this.selectedUnloadPlaceIds];
        if (tablesRes.ok) this.originalTableIds = [...this.selectedTableIds];

        if (placesRes.ok && tablesRes.ok) {
          useDeletionsStore().notify({
            prefix: 'Места доступа сохранены для ',
            bold: username,
            type: 'success',
          });
          this.$emit('updated');
          this.close();
          return;
        }

        const failed = [
          !placesRes.ok && 'места разгрузки',
          !tablesRes.ok && 'места прохода',
        ].filter(Boolean).join(' и ');
        useDeletionsStore().notify({
          prefix: 'Не удалось сохранить ',
          bold: failed,
          type: 'error',
        });
      } catch (error) {
        console.error('Не удалось сохранить места доступа:', error);
        useDeletionsStore().notify({
          prefix: 'Не удалось сохранить места доступа: ',
          bold: 'ошибка сервера',
          type: 'error',
        });
      } finally {
        this.saving = false;
      }
    },

    onKeydown(e) {
      if (e.key === 'Escape') this.close();
    },

    close() {
      if (this.leaving) return;
      this.leaving = true;
      setTimeout(() => this.$emit('close'), 250);
    },
  },
};
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 12000;
  padding: 20px;
  animation: fadeIn 0.2s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.modal-overlay.modal-leaving {
  animation: fadeOut 0.25s ease-in forwards;
}

.modal-overlay.modal-leaving .access-places-modal {
  animation: slideDown 0.25s ease-in forwards;
}

@keyframes fadeOut {
  from { opacity: 1; }
  to { opacity: 0; }
}

@keyframes slideDown {
  from { transform: translateY(0); opacity: 1; }
  to { transform: translateY(20px); opacity: 0; }
}

.access-places-modal {
  background: var(--surface);
  border-radius: 45px;
  width: 100%;
  max-width: 640px;
  max-height: calc(var(--app-vh, 1vh) * 88);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 10px 30px var(--shadow-drop);
  animation: slideUp 0.2s ease-out;
}

@keyframes slideUp {
  from { transform: translateY(20px); opacity: 0; }
  to { transform: translateY(0); opacity: 1; }
}

.access-places-modal__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 24px;
  border-bottom: 1px solid var(--color-border);
}

.access-places-modal__header h3 {
  margin: 0;
  font-size: 1.1em;
  font-weight: 600;
  color: var(--text);
}

.close-btn {
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  line-height: 1;
  color: var(--text-muted);
  background: none;
  border: none;
  cursor: pointer;
  border-radius: 50%;
  transition: all 0.2s;
}

.close-btn:hover {
  color: var(--text);
  background: var(--surface-2);
}

.access-places-modal__body {
  padding: 20px 24px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.access-places-modal__loading {
  margin: 0;
  text-align: center;
  color: var(--color-text-muted);
}

.access-block {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.block-label {
  font-size: 0.85em;
  color: var(--text-muted);
  font-weight: 400;
}

.places-container {
  width: fit-content;
  background: var(--surface);
  border-radius: 15px;
  border: 1px solid var(--border);
  padding: 12px;
}

.places-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 5px;
  row-gap: 5px;
}

.place-item {
  height: 30px;
  border-radius: 50px;
  background: var(--surface-2);
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  width: 140px;
  min-width: 80px;
  padding: 0 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.2s ease;
  white-space: nowrap;
  user-select: none;
  text-align: center;
}

.place-item:hover {
  background: var(--row-hover);
}

.place-item.selected {
  background: var(--accent);
  color: var(--accent-contrast);
}

.no-places-message {
  text-align: center;
  padding: 20px;
  color: var(--text-muted);
  font-style: italic;
}

.access-places-modal__footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 24px;
  border-top: 1px solid var(--color-border);
}

@media (max-width: 768px) {
  .places-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
