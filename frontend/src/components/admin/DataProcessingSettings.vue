<template>
  <div class="dp-settings">
          <h3 class="section-title">
            Обработка данных
          </h3>
          <p class="dp-desc">
            Документ открывается по ссылке «согласие» при подаче заявки и доступен по адресу
            <code>/data-processing</code>. PDF показывается прямо на странице, DOC, DOCX и XLSX —
            только для скачивания.
          </p>

          <p
            v-if="dpLoading"
            class="form-hint"
          >
            Загрузка...
          </p>

          <div
            v-else-if="dpMeta"
            class="dp-card"
          >
            <div class="dp-card__info">
              <span class="dp-card__name">{{ dpMeta.file_name }}</span>
              <span class="dp-card__meta">{{ dpExtLabel }} · загружен {{ dpUploadedLabel }}</span>
            </div>
            <div class="dp-card__actions">
              <a
                class="btn btn--ghost"
                href="/data-processing"
                target="_blank"
                rel="noopener"
              >Открыть</a>
              <button
                class="btn btn--ghost"
                :disabled="dpBusy"
                @click="downloadDp"
              >
                Скачать
              </button>
              <label
                class="btn btn--ghost"
                :class="{ 'btn--disabled': dpBusy }"
              >
                {{ dpUploading ? 'Загрузка...' : 'Заменить' }}
                <input
                  type="file"
                  accept=".pdf,.doc,.docx,.xlsx"
                  hidden
                  :disabled="dpBusy"
                  @change="onDpFileChange"
                >
              </label>
              <button
                class="btn btn--danger"
                :disabled="dpBusy"
                @click="deleteDp"
              >
                {{ dpDeleting ? 'Удаление...' : 'Удалить' }}
              </button>
            </div>
          </div>

          <div
            v-else
            class="dp-upload"
          >
            <label
              class="btn btn--primary"
              :class="{ 'btn--disabled': dpUploading }"
            >
              {{ dpUploading ? 'Загрузка...' : 'Загрузить документ' }}
              <input
                type="file"
                accept=".pdf,.doc,.docx,.xlsx"
                hidden
                :disabled="dpUploading"
                @change="onDpFileChange"
              >
            </label>
            <span class="form-hint">PDF, DOC, DOCX или XLSX.</span>
          </div>

          <h4 class="subsection-title pdc-title">
            Текст согласия при первом входе
          </h4>
          <p class="dp-desc">
            Этот текст показывается пользователю в окне при первом входе. Кнопку подтверждения
            он сможет нажать только прокрутив текст до конца. Текст загруженного документа
            переносится сюда сразу при загрузке: абзацы, списки и заголовки восстанавливаются
            по разметке файла, но таблицы не воспроизводятся и автонумерация Word теряется,
            поэтому результат нужно вычитать.
          </p>

          <p
            v-if="pdcLoading"
            class="form-hint"
          >
            Загрузка...
          </p>

          <template v-else>
            <div
              v-if="pdcRequired && !pdcHasText"
              class="pdc-warning"
              data-testid="pdc-empty-warning"
            >
              Запрос согласия включён, но текст пуст, поэтому согласие у пользователей не
              спрашивается. Задайте текст или выключите запрос.
            </div>

            <p
              v-if="pdcExtracting"
              class="form-hint"
              data-testid="pdc-extracting"
            >
              Переносим текст из документа...
            </p>

            <TextConstructor
              v-model="pdcText"
              class="pdc-editor"
              :disabled="pdcExtracting"
              :rows="12"
              placeholder="Текст согласия на обработку персональных данных"
            />

            <div class="pdc-actions">
              <button
                class="btn btn--primary"
                :disabled="pdcBusy"
                data-testid="pdc-save"
                @click="savePdcText"
              >
                {{ pdcSaving ? 'Сохранение...' : 'Сохранить текст' }}
              </button>
              <span
                v-if="pdcTextChanged"
                class="form-hint"
                data-testid="pdc-text-changed"
              >Текст изменён: при сохранении спросим, требовать ли согласие заново.</span>
              <span
                v-else
                class="form-hint"
              >Редакция {{ pdcVersion }}</span>
            </div>

            <div class="form-group">
              <label class="switch-label">
                <span class="switch-text">Запрашивать согласие при первом входе</span>
                <span
                  class="switch"
                  :class="{ 'switch--on': pdcRequired }"
                  role="switch"
                  :aria-checked="String(pdcRequired)"
                  tabindex="0"
                  data-testid="pdc-required-switch"
                  @click="togglePdcRequired"
                  @keydown.enter="togglePdcRequired"
                  @keydown.space.prevent="togglePdcRequired"
                >
                  <span class="switch__thumb" />
                </span>
              </label>
              <span
                class="form-hint"
                data-testid="pdc-required-hint"
              >{{ pdcRequiredHint }}</span>
            </div>

            <div class="pdc-actions">
              <button
                class="btn btn--ghost"
                :disabled="pdcBusy"
                data-testid="pdc-require-again"
                @click="requirePdcAgain"
              >
                {{ pdcRequiringAgain ? 'Обновление...' : 'Требовать согласие заново' }}
              </button>
              <span class="form-hint">
                Поднимает редакцию: все, кто согласился с прежней, подтвердят снова.
              </span>
            </div>

            <!-- Сбор согласий: сколько человек подтвердили текущую редакцию.
                 Считается той же меркой, что и гейт, поэтому число согласившихся
                 совпадает с числом тех, кого система пускает. -->
            <div
              v-if="pdcCollection"
              class="pdc-collection"
              data-testid="pdc-collection"
            >
              <div class="pdc-collection__head">
                <h5 class="pdc-collection__title">
                  Сбор согласий по редакции {{ pdcCollection.version }}
                </h5>
                <RefreshButton
                  :loading="pdcCollectionLoading"
                  data-testid="pdc-collection-refresh"
                  @refresh="fetchPdcCollection"
                />
              </div>

              <p
                v-if="!pdcCollection.active"
                class="form-hint"
                data-testid="pdc-collection-inactive"
              >
                Запрос согласия сейчас не действует, сбор не идёт. Подтвердить согласие
                пользователи смогут после включения.
              </p>

              <template v-else>
                <div
                  class="pdc-collection__bar"
                  role="progressbar"
                  aria-label="Доля подтвердивших согласие"
                  aria-valuemin="0"
                  aria-valuemax="100"
                  :aria-valuenow="pdcCollectionPercent"
                >
                  <div
                    class="pdc-collection__fill"
                    :style="{ transform: `scaleX(${pdcCollectionRatio})` }"
                  />
                </div>

                <p
                  class="pdc-collection__counts"
                  data-testid="pdc-collection-counts"
                >
                  Подтвердили {{ pdcCollection.accepted }} из {{ pdcCollection.total }}
                  ({{ pdcCollectionPercent }}%)
                </p>

                <template v-if="pdcCollection.pending_users.length">
                  <div class="pdc-pending">
                    <div class="pdc-pending__toolbar">
                      <h6 class="pdc-pending__title">
                        Ещё не подтвердили: {{ pdcCollection.pending }}
                      </h6>
                      <div class="pdc-pending__controls">
                        <input
                          v-model="pdcPendingQuery"
                          class="lk-input pdc-pending__search"
                          type="search"
                          placeholder="Поиск по ФИО, логину, организации"
                          data-testid="pdc-pending-search"
                        >
                        <BaseDropdown
                          v-model="pdcPendingOrg"
                          class="pdc-pending__filter"
                          :options="pdcPendingOrgOptions"
                          label-key="name"
                          value-key="id"
                          placeholder="Все организации"
                          searchable
                          data-testid="pdc-pending-org"
                        />
                        <button
                          class="btn btn--ghost"
                          :disabled="pdcExporting"
                          data-testid="pdc-collection-export"
                          @click="exportPdcPending"
                        >
                          {{ pdcExporting ? 'Готовим файл...' : 'Выгрузить' }}
                        </button>
                      </div>
                    </div>

                    <div class="pdc-pending__table rt-table">
                      <div class="pdc-pending__head rt-head-row">
                        <div class="pdc-pending__col pdc-pending__col--name">
                          ФИО
                        </div>
                        <div class="pdc-pending__col pdc-pending__col--login">
                          Логин
                        </div>
                        <div class="pdc-pending__col pdc-pending__col--org">
                          Организация
                        </div>
                      </div>
                      <div
                        class="pdc-pending__body"
                        data-testid="pdc-collection-pending"
                      >
                        <div
                          v-for="person in pdcPendingRows"
                          :key="person.id"
                          class="pdc-pending__row rt-row"
                        >
                          <div
                            class="pdc-pending__col pdc-pending__col--name"
                            data-label="ФИО"
                          >
                            {{ person.full_name || '-' }}
                          </div>
                          <div
                            class="pdc-pending__col pdc-pending__col--login"
                            data-label="Логин"
                          >
                            {{ loginLabel(person.username) }}
                          </div>
                          <div
                            class="pdc-pending__col pdc-pending__col--org"
                            data-label="Организация"
                          >
                            {{ person.organization || '-' }}
                          </div>
                        </div>
                      </div>
                    </div>

                    <p
                      v-if="!pdcPendingRows.length"
                      class="form-hint"
                      data-testid="pdc-pending-empty"
                    >
                      По запросу никого не нашлось.
                    </p>
                    <p
                      v-if="!pdcPendingComplete"
                      class="form-hint pdc-pending__warn"
                      data-testid="pdc-pending-partial"
                    >
                      Список загружен не полностью: показаны {{ pdcPendingSource.length }} из
                      {{ pdcCollection.pending }}, поиск и фильтр идут только по ним. Обновите
                      сводку или выгрузите список целиком.
                    </p>
                    <p
                      v-else-if="pdcPendingFiltered"
                      class="form-hint"
                      data-testid="pdc-pending-found"
                    >
                      Найдено {{ pdcPendingRows.length }} из {{ pdcCollection.pending }}.
                    </p>
                  </div>
                </template>
                <p
                  v-else
                  class="form-hint"
                >
                  Подтвердили все, кого закрывает запрос согласия.
                </p>
              </template>
            </div>
          </template>
  </div>
</template>

<script>
import {
  getDataProcessingMeta,
  uploadDataProcessingDoc,
  deleteDataProcessingDoc,
  downloadDataProcessingDoc,
} from '@/api/dataProcessing';
import {
  getPDConsentSettings,
  savePDConsentText,
  setPDConsentRequired,
  requirePDConsentAgain,
  getPDConsentCollection,
} from '@/api/pdConsent';
import { extractDocumentHtml } from '@/utils/documentTextExtract';
import { stripHtml } from '@/utils/sanitize';
import { formatLogin } from '@/utils/formatName';
import { useDeletionsStore } from '@/stores/deletions';
import { useUiStore } from '@/stores/ui';
import TextConstructor from '@/components/TextConstructor.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';

/**
 * Раздел «Обработка данных» отдельной страницей (/admin/data-processing): документ
 * согласия, текст согласия при первом входе, выключатель запроса и ход сбора.
 *
 * Раньше жил вкладкой в настройках системы, но настройки - это параметры работы
 * (загрузки, пагинация, уведомления), а здесь целый механизм со своими данными о
 * людях. Соседство мешало и тем, и другим.
 */
export default {
  name: 'DataProcessingSettings',
  components: { TextConstructor, RefreshButton, BaseDropdown },
  data() {
    return {
      dpMeta: null,
      dpLoaded: false,
      dpLoading: false,
      dpUploading: false,
      dpDeleting: false,
      pdcText: '',
      // Текст, лежащий на сервере: с ним сверяем правку, чтобы спросить про
      // подтверждение заново только когда текст действительно менялся.
      pdcSavedText: '',
      pdcLoaded: false,
      pdcVersion: 1,
      pdcRequired: false,
      pdcLoading: false,
      pdcSaving: false,
      pdcExtracting: false,
      pdcRequiringAgain: false,
      pdcTogglingRequired: false,
      // Сводка сбора согласий: грузится вместе с разделом и после каждого действия,
      // которое меняет состав согласившихся (подъём редакции).
      pdcCollection: null,
      pdcCollectionLoading: false,
      pdcExporting: false,
      // Поиск и фильтр по списку не подтвердивших. Сам список приходит урезанным
      // (первые PendingListLimit), поэтому при первом же запросе догружаем полный:
      // искать по видимой полусотне значит врать администратору «никого не нашлось».
      pdcPendingQuery: '',
      pdcPendingOrg: null,
      pdcPendingFull: null,
      pdcPendingFullLoading: false,
    };
  },
  computed: {
    dpBusy() {
      // Перенос текста идёт следом за загрузкой и спрашивает подтверждение замены.
      // Пока висит вопрос, повторная загрузка запрещена: диалог подтверждения один
      // на приложение, и второй вызов оставил бы первый висеть без ответа.
      return this.dpUploading || this.dpDeleting || this.pdcExtracting;
    },

    dpExtLabel() {
      const ext = (this.dpMeta?.ext || '').replace('.', '').toUpperCase();
      return ext || 'Документ';
    },

    dpUploadedLabel() {
      if (!this.dpMeta?.uploaded_at) return '';
      const d = new Date(this.dpMeta.uploaded_at);
      if (Number.isNaN(d.getTime())) return '';
      return d.toLocaleDateString('ru-RU');
    },

    pdcRequiredHint() {
      return this.pdcRequired
        ? 'Включено: согласие при входе запрашивается.'
        : 'Выключено: согласие при входе не запрашивается.';
    },

    pdcCollectionRatio() {
      const c = this.pdcCollection;
      if (!c || !c.total) return 1;
      return Math.min(1, Math.max(0, c.accepted / c.total));
    },

    pdcCollectionPercent() {
      return Math.round(this.pdcCollectionRatio * 100);
    },

    pdcBusy() {
      return this.pdcSaving || this.pdcExtracting || this.pdcRequiringAgain || this.pdcTogglingRequired;
    },

    /** Список, по которому идут поиск и фильтр: полный, если он уже догружен. */
    pdcPendingSource() {
      return this.pdcPendingFull || this.pdcCollection?.pending_users || [];
    },

    pdcPendingOrgOptions() {
      const names = new Set();
      for (const person of this.pdcPendingSource) {
        if (person.organization) names.add(person.organization);
      }
      return [{ id: null, name: 'Все организации' }]
        .concat([...names].sort((a, b) => a.localeCompare(b, 'ru')).map((name) => ({ id: name, name })));
    },

    pdcPendingRows() {
      const variants = buildSearchVariants(this.pdcPendingQuery);
      return this.pdcPendingSource.filter((person) => {
        if (this.pdcPendingOrg && person.organization !== this.pdcPendingOrg) return false;
        if (!this.pdcPendingQuery.trim()) return true;
        return matchesSearch(
          `${person.full_name || ''} ${person.username || ''} ${person.organization || ''}`,
          variants,
        );
      });
    },

    /**
     * Список на руках целиком. Пока это не так, поиск отвечает только за
     * показанную часть, и молчать об этом нельзя: «никого не нашлось» тогда
     * означает «не нашлось среди показанных», а выглядит как факт.
     */
    pdcPendingComplete() {
      if (!this.pdcCollection) return true;
      return !this.pdcCollection.truncated || Boolean(this.pdcPendingFull);
    },

    /** Поиск или фильтр сузили список - об этом стоит сказать числом. */
    pdcPendingFiltered() {
      return this.pdcPendingRows.length !== this.pdcPendingSource.length;
    },

    pdcHasText() {
      const html = this.pdcText || '';
      if (/<img/i.test(html)) return true;
      return stripHtml(html) !== '';
    },

    // Сравниваем разметку целиком, а не видимый текст: смена выделения, врезка
    // картинки или переформатирование пунктов - тоже другая редакция согласия.
    // Лишний вопрос дёшев, пропущенная правка означала бы согласие, данное не
    // тому тексту, который человеку показывают.
    pdcTextChanged() {
      return (this.pdcText || '') !== (this.pdcSavedText || '');
    },
  },
  watch: {
    // Догрузка полного списка могла не удаться. Следующая же попытка поиска -
    // самый естественный момент попробовать снова: именно она от полноты зависит.
    pdcPendingQuery() {
      if (!this.pdcPendingComplete) this.loadPdcPendingFull();
    },
    pdcPendingOrg() {
      if (!this.pdcPendingComplete) this.loadPdcPendingFull();
    },
  },
  mounted() {
    this.fetchDataProcessingDoc();
    // Промис нужен переносу текста из документа: он не должен обогнать загрузку настроек.
    this.pdcReady = this.fetchPdConsentSettings();
  },
  methods: {
    async fetchDataProcessingDoc() {
      this.dpLoading = true;
      try {
        this.dpMeta = await getDataProcessingMeta();
        this.dpLoaded = true;
      } catch (error) {
        console.error('Ошибка загрузки документа согласия:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить документ', type: 'error' });
      } finally {
        this.dpLoading = false;
      }
    },

    async onDpFileChange(event) {
      const file = event.target.files?.[0];
      // Сбрасываем input, иначе повторный выбор того же файла не вызовет change.
      event.target.value = '';
      if (!file) return;
      this.dpUploading = true;
      let uploaded = null;
      try {
        this.dpMeta = await uploadDataProcessingDoc(file);
        this.dpLoaded = true;
        uploaded = this.dpMeta;
        useDeletionsStore().notify({ prefix: 'Документ ', bold: this.dpMeta.file_name, suffix: ' загружен' });
      } catch (error) {
        useDeletionsStore().notify({ prefix: error?.message || 'Ошибка загрузки документа', type: 'error' });
      } finally {
        this.dpUploading = false;
      }
      if (uploaded) await this.importTextFromDocument(file, uploaded.ext);
    },

    /**
     * Переносит текст только что загруженного документа в редактор. Читаем сам
     * выбранный файл, а не скачиваем его обратно: он уже в руках и содержимое то же.
     * @param {File} file
     * @param {string} ext расширение, каким его сохранил сервер
     */
    async importTextFromDocument(file, ext) {
      // Флаг поднимаем до вопроса о замене: он же гасит кнопки загрузки и
      // редактор, пока перенос не закончится.
      this.pdcExtracting = true;
      try {
        // Настройки согласия грузятся параллельно с разделом: их ответ перезаписал бы
        // перенесённый текст, поэтому сперва дожидаемся его.
        await this.pdcReady;
        if (this.pdcHasText) {
          const ok = await useUiStore().confirm({
            title: 'Текст согласия',
            message: 'Заменить текст согласия текстом из загруженного документа?'
              + ' Нынешний текст будет потерян.',
            confirmText: 'Заменить',
            danger: false,
          });
          if (!ok) return;
        }
        const html = await extractDocumentHtml(file, ext);
        if (!html) {
          useDeletionsStore().notify({
            prefix: ext === '.xlsx'
              ? 'В книге не нашлось текста: все ячейки пусты.'
              : 'В документе не нашлось текста. Возможно, это сканированные страницы.',
            type: 'warning',
          });
          return;
        }
        this.pdcText = html;
        useDeletionsStore().notify({
          prefix: 'Текст перенесён из ',
          bold: file.name,
          suffix: '. Проверьте разметку и сохраните.',
          type: 'info',
        });
      } catch (error) {
        useDeletionsStore().notify({
          prefix: error?.message || 'Не удалось перенести текст из документа',
          type: 'warning',
        });
      } finally {
        this.pdcExtracting = false;
      }
    },

    async downloadDp() {
      if (!this.dpMeta) return;
      try {
        await downloadDataProcessingDoc(this.dpMeta.file_name);
      } catch (error) {
        console.error('Ошибка скачивания документа согласия:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось скачать документ', type: 'error' });
      }
    },

    async deleteDp() {
      if (!this.dpMeta) return;
      const ok = await useUiStore().confirm({
        message: `Удалить документ «${this.dpMeta.file_name}»? Пользователи перестанут видеть его при подаче заявки.`,
      });
      if (!ok) return;
      const name = this.dpMeta.file_name;
      this.dpDeleting = true;
      try {
        await deleteDataProcessingDoc();
        this.dpMeta = null;
        useDeletionsStore().notify({ prefix: 'Документ ', bold: name, suffix: ' удалён' });
      } catch (error) {
        console.error('Ошибка удаления документа согласия:', error);
        useDeletionsStore().notify({ prefix: 'Ошибка удаления документа', type: 'error' });
      } finally {
        this.dpDeleting = false;
      }
    },

    applyPdConsentSettings(settings) {
      this.pdcText = settings?.text || '';
      this.pdcSavedText = this.pdcText;
      this.pdcVersion = settings?.version || 1;
      this.pdcRequired = Boolean(settings?.required);
    },

    async fetchPdConsentSettings() {
      this.pdcLoading = true;
      try {
        this.applyPdConsentSettings(await getPDConsentSettings());
        this.pdcLoaded = true;
        this.fetchPdcCollection();
      } catch (error) {
        useDeletionsStore().notify({
          prefix: error?.message || 'Не удалось загрузить текст согласия',
          type: 'error',
        });
      } finally {
        this.pdcLoading = false;
      }
    },

    async fetchPdcCollection() {
      this.pdcCollectionLoading = true;
      try {
        this.pdcCollection = await getPDConsentCollection();
        this.pdcPendingFull = null;
        if (this.pdcCollection?.truncated) this.loadPdcPendingFull();
      } catch (error) {
        console.error('Не удалось загрузить сводку по сбору согласий:', error);
        this.pdcCollection = null;
      } finally {
        this.pdcCollectionLoading = false;
      }
    },

    /**
     * Догружает полный список не подтвердивших. Нужен поиску и фильтру: искать по
     * показанной части значит отвечать «никого не нашлось» про человека, который в
     * списке есть. Грузим один раз на обновление сводки.
     */
    async loadPdcPendingFull() {
      if (this.pdcPendingFullLoading || this.pdcPendingFull) return;
      this.pdcPendingFullLoading = true;
      try {
        const full = await getPDConsentCollection({ full: true });
        this.pdcPendingFull = full?.pending_users || null;
      } catch (error) {
        // Молчать нельзя: подпись под таблицей скажет, что список неполный, а
        // следующая попытка поиска попробует догрузить его снова.
        console.error('Не удалось загрузить полный список не подтвердивших:', error);
      } finally {
        this.pdcPendingFullLoading = false;
      }
    },

    /** Логин с собачкой; прочерк, когда логина нет. */
    loginLabel(username) {
      return formatLogin(username) || '-';
    },

    async exportPdcPending() {
      if (!this.pdcCollection?.pending_users?.length) return;
      this.pdcExporting = true;
      try {
        // В файл идут ВСЕ, а не показанная часть: урезанная выгрузка тихо теряет людей.
        // Полный список обычно уже на руках - его тянет таблица ради поиска.
        // Идём на сервер только если догрузка не удалась.
        let rows = this.pdcPendingFull || this.pdcCollection.pending_users || [];
        if (this.pdcCollection.truncated && !this.pdcPendingFull) {
          const full = await getPDConsentCollection({ full: true });
          rows = full.pending_users || [];
        }
        const ExcelJS = (await import('exceljs')).default;
        const workbook = new ExcelJS.Workbook();
        const sheet = workbook.addWorksheet('Не подтвердили');

        const headerRow = sheet.addRow(['ФИО', 'Логин', 'Организация']);
        headerRow.height = 25;
        headerRow.eachCell((cell) => {
          cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF4F5BDF' } };
          cell.font = { name: 'Verdana', size: 11, bold: true, color: { argb: 'FFFFFFFF' } };
          cell.alignment = { vertical: 'middle', horizontal: 'center' };
        });
        rows.forEach((person) => {
          sheet.addRow([person.full_name, person.username, person.organization]);
        });
        sheet.columns.forEach((column) => { column.width = 32; });

        const buffer = await workbook.xlsx.writeBuffer();
        const blob = new Blob([buffer], {
          type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
        });
        const url = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = `Soglasie_ne_podtverdili_red${this.pdcCollection.version}.xlsx`;
        document.body.appendChild(link);
        link.click();
        link.remove();
        window.URL.revokeObjectURL(url);
      } catch (error) {
        useDeletionsStore().notify({
          prefix: error?.message || 'Не удалось выгрузить список',
          type: 'error',
        });
      } finally {
        this.pdcExporting = false;
      }
    },

    async savePdcText() {
      // Изменённый текст - это новая редакция согласия, и подтверждать её надо
      // заново. Решение за администратором: правка опечатки не стоит того, чтобы
      // поднимать окно у всех, а правка по существу стоит.
      let requireAgain = false;
      if (this.pdcTextChanged) {
        const answer = await useUiStore().confirm({
          title: 'Текст изменён',
          message: this.pdcRequired
            ? 'Запросить согласие заново у всех? Те, кто соглашался с прежней редакцией,'
              + ' подтвердят новую при следующем входе.'
            : 'Запросить согласие заново у всех? Сейчас запрос согласия выключен -'
              + ' подтверждать новую редакцию будут, когда вы его включите.',
          confirmText: 'Запросить заново',
          cancelText: 'Только сохранить',
          danger: false,
        });
        requireAgain = answer === true;
      }
      this.pdcSaving = true;
      try {
        this.applyPdConsentSettings(await savePDConsentText(this.pdcText, requireAgain));
        this.pdcLoaded = true;
        useDeletionsStore().notify({
          prefix: requireAgain
            ? `Текст сохранён, редакция ${this.pdcVersion}: согласие запросим заново`
            : 'Текст согласия сохранён',
        });
        if (requireAgain) this.fetchPdcCollection();
      } catch (error) {
        useDeletionsStore().notify({
          prefix: error?.message || 'Не удалось сохранить текст согласия',
          type: 'error',
        });
      } finally {
        this.pdcSaving = false;
      }
    },

    async togglePdcRequired() {
      if (this.pdcBusy) return;
      const next = !this.pdcRequired;
      if (next) {
        const ok = await useUiStore().confirm({
          title: 'Запрос согласия',
          message: 'Включить запрос согласия при первом входе? Снять его можно тем же тумблером.',
          confirmText: 'Включить',
          danger: false,
        });
        if (!ok) return;
      }
      this.pdcTogglingRequired = true;
      try {
        this.applyPdConsentSettings(await setPDConsentRequired(next));
        useDeletionsStore().notify({
          prefix: next ? 'Согласие запрашивается при входе' : 'Запрос согласия выключен',
        });
      } catch (error) {
        useDeletionsStore().notify({
          prefix: error?.message || 'Не удалось изменить настройку',
          type: 'error',
        });
      } finally {
        this.pdcTogglingRequired = false;
      }
    },

    async requirePdcAgain() {
      const ok = await useUiStore().confirm({
        title: 'Новая редакция',
        message: 'Требовать согласие заново? Все, кто соглашался с прежней редакцией текста,'
          + ' подтвердят его при следующем входе.',
        confirmText: 'Требовать заново',
        danger: false,
      });
      if (!ok) return;
      this.pdcRequiringAgain = true;
      try {
        this.applyPdConsentSettings(await requirePDConsentAgain());
        // Состав согласившихся только что обнулился - сводка обязана это показать.
        this.fetchPdcCollection();
        useDeletionsStore().notify({ prefix: `Редакция согласия поднята до ${this.pdcVersion}` });
      } catch (error) {
        useDeletionsStore().notify({
          prefix: error?.message || 'Не удалось запросить согласие заново',
          type: 'error',
        });
      } finally {
        this.pdcRequiringAgain = false;
      }
    },
  },
};
</script>

<style scoped>
/* Мобильный селектор раздела вместо тесной полосы табов (#1208). */
.admin-settings__section-select {
  display: none;
}

.sidebar-item {
  padding: 10px 16px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s ease;
  border-left: 3px solid transparent;
  font-family: 'Montserrat', sans-serif;
}

.sidebar-item:hover {
  background: var(--surface-2);
}

.sidebar-item--active {
  background: var(--accent-tint);
  color: var(--accent-text);
  border-left-color: var(--accent-text);
  font-weight: 600;
}

.sidebar-item + .sidebar-item {
  border-top: 1px solid var(--border);
}

.admin-settings__content {
  flex: 1;
  min-width: 0;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  padding: 16px 20px;
}

.settings-section {
  animation: fadeIn 0.2s ease;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: translateY(0); }
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
  margin: 0 0 16px 0;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border);
}

.form-group {
  margin-bottom: 16px;
}

.form-label {
  display: block;
  font-size: 12px;
  font-weight: 500;
  color: var(--text);
  margin-bottom: 6px;
  font-family: 'Montserrat', sans-serif;
}

.form-input {
  width: 140px;
  padding: 6px 10px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  font-size: 13px;
  font-family: 'Montserrat', sans-serif;
  color: var(--text);
  transition: border-color 0.2s;
  outline: none;
}

.form-input:focus {
  border-color: var(--accent);
}

.form-input:disabled {
  background: var(--surface-2);
  color: var(--text-muted);
  cursor: not-allowed;
}

.form-hint {
  display: block;
  font-size: 11px;
  color: var(--text-muted);
  margin-top: 4px;
}

.section-intro {
  margin: 0 0 16px;
  font-size: 13px;
}

.subsection-title {
  margin: 0 0 8px;
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
}

.bureau-schedule {
  margin-top: 24px;
  padding-top: 20px;
  border-top: 1px solid var(--border);
}

.form-range {
  width: 100%;
  max-width: 300px;
  margin-top: 4px;
  accent-color: var(--accent-text);
  cursor: pointer;
}

.range-labels {
  display: flex;
  justify-content: space-between;
  max-width: 300px;
  font-size: 11px;
  color: var(--text-muted);
  margin-top: 2px;
}

.checkbox-group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  padding: 4px 10px;
  border: 1px solid var(--border);
  border-radius: 50px;
  transition: all 0.2s;
  font-size: 12px;
}

.checkbox-label:hover {
  border-color: var(--accent);
  background: var(--accent-tint);
}

.checkbox-label input[type="checkbox"] {
  accent-color: var(--accent-text);
  cursor: pointer;
}

.checkbox-text {
  font-family: 'Montserrat', sans-serif;
  font-size: 12px;
  color: var(--text);
  white-space: nowrap;
}

/* Switch */
.switch-label {
  display: flex;
  align-items: center;
  gap: 12px;
  cursor: pointer;
}

.switch-text {
  font-size: 12px;
  font-weight: 500;
  color: var(--text);
  font-family: 'Montserrat', sans-serif;
}

.switch {
  position: relative;
  width: 40px;
  height: 22px;
  background: var(--border);
  border-radius: 11px;
  transition: background 0.2s ease;
  flex-shrink: 0;
  outline: none;
}

.switch:focus-visible {
  box-shadow: 0 0 0 2px rgba(79, 91, 223, 0.3);
}

.switch--on {
  background: var(--accent);
}

.switch__thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 18px;
  height: 18px;
  background: var(--surface);
  border-radius: 50%;
  transition: transform 0.2s ease;
}

.switch--on .switch__thumb {
  transform: translateX(18px);
}

/* Кнопки */
.btn {
  padding: 6px 20px;
  border: 1px solid transparent;
  border-radius: 50px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  font-family: 'Montserrat', sans-serif;
  transition: all 0.2s;
  height: 32px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 100px;
}

.btn--primary {
  background: var(--accent);
  color: var(--accent-contrast);
  border-color: var(--accent);
}

.btn--primary:hover:not(:disabled) {
  background: var(--accent-hover);
}

.btn--primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn--ghost {
  background: var(--surface);
  color: var(--accent-text);
  border-color: var(--accent);
  text-decoration: none;
}

.btn--ghost:hover:not(:disabled):not(.btn--disabled) {
  background: var(--accent-tint);
}

.btn--danger {
  background: var(--surface);
  color: var(--danger-text);
  border-color: var(--danger);
}

.btn--danger:hover:not(:disabled) {
  background: var(--danger-bg);
}

.btn--danger:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn--disabled {
  opacity: 0.5;
  cursor: not-allowed;
  pointer-events: none;
}

/* Обработка данных */
.dp-desc {
  font-size: 12px;
  color: var(--text);
  line-height: 1.5;
  margin: 0 0 16px 0;
}

.dp-desc code {
  background: var(--accent-tint);
  color: var(--accent-text);
  padding: 1px 6px;
  border-radius: var(--radius-sm);
  font-size: 11px;
}

.dp-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  padding: 14px 16px;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
}

.dp-card__info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.dp-card__name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
  word-break: break-word;
}

.dp-card__meta {
  font-size: 11px;
  color: var(--text-muted);
}

.dp-card__actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.dp-upload {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.dp-upload .form-hint {
  margin-top: 0;
}

/* Текст согласия при первом входе */
.pdc-title {
  margin-top: 28px;
  padding-top: 20px;
  border-top: 1px solid var(--border);
}

.pdc-warning {
  margin-bottom: 16px;
  padding: 12px 16px;
  border: 1px solid var(--danger);
  border-radius: var(--radius-md);
  background: var(--danger-bg);
  color: var(--danger-text);
  font-size: 13px;
  line-height: 1.5;
}

.pdc-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin: 12px 0;
}

.pdc-actions .form-hint {
  margin-top: 0;
}

.pdc-editor {
  margin-bottom: 4px;
}

.pdc-collection {
  margin-top: 20px;
  padding: 16px 18px;
  border: 1px solid var(--color-border, var(--border));
  border-radius: var(--radius-md, 15px);
  background: var(--surface);
}

.pdc-collection__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.pdc-collection__title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text, var(--text));
}

/* Дорожка приглушена намеренно: на акцентном оттенке пустая полоса читалась как
   заполненная, и «0 из 18» выглядело как «собрано всё» (замер на стенде). */
.pdc-collection__bar {
  height: 6px;
  border-radius: 999px;
  overflow: hidden;
  background: color-mix(in srgb, var(--color-text, var(--text)) 12%, transparent);
}

.pdc-collection__fill {
  height: 100%;
  background: var(--accent);
  transform: scaleX(0);
  transform-origin: left center;
  transition: transform 0.2s ease-out;
}

.pdc-collection__counts {
  margin: 8px 0 12px;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text, var(--text));
}

/* Не подтвердившие - обычная таблица проекта: шапка с поиском и фильтром,
   строки с подписями колонок, на узком экране rt-* разворачивает их в карточки. */
.pdc-pending {
  margin-top: 14px;
}

.pdc-pending__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 10px;
}

.pdc-pending__title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text, var(--text));
}

.pdc-pending__controls {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.pdc-pending__search {
  width: 260px;
  max-width: 100%;
}

.pdc-pending__filter {
  min-width: 200px;
}

.pdc-pending__warn {
  color: var(--warning-text);
}

.pdc-pending__table {
  border: 1px solid var(--color-border, var(--border));
  border-radius: var(--radius-md, 15px);
  overflow: hidden;
}

.pdc-pending__head,
.pdc-pending__row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 9px 14px;
}

.pdc-pending__head {
  background: var(--color-bg-muted, color-mix(in srgb, var(--text) 4%, var(--surface)));
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  color: var(--color-text-muted, var(--text-muted));
}

.pdc-pending__body {
  max-height: 320px;
  overflow-y: auto;
}

.pdc-pending__row {
  border-top: 1px solid var(--color-border, var(--border));
  font-size: 13px;
  color: var(--color-text, var(--text));
}

.pdc-pending__col {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pdc-pending__col--name {
  flex: 1 1 40%;
}

.pdc-pending__col--login {
  flex: 0 0 26%;
  color: var(--color-text-muted, var(--text-muted));
}

.pdc-pending__col--org {
  flex: 1 1 34%;
  color: var(--color-text-muted, var(--text-muted));
}

@media (max-width: 768px) {
  .pdc-pending__search,
  .pdc-pending__filter {
    width: 100%;
  }

  .pdc-pending__controls {
    width: 100%;
  }
}

</style>
