<template>
  <!-- Закрытие проигрывается здесь, а не у родителя: он монтирует панель по v-if, и
       без своей transition она пропадала мгновенно, тогда как окна проекта уезжают
       плавно. Родителю о закрытии сообщаем после leave - тогда unmount не обрывает
       анимацию на середине. -->
  <transition name="detail-close">
    <!-- Внешний контейнер для модального окна -->
    <div
      v-if="visible"
      class="application-detail-overlay"
      @click.self="closeApplicationDetail"
    >
    <!-- Модальное окно пересылки -->
    <ForwardModal
      :show="showForwardModal"
      :all-users="allUsers"
      :responsible-users="responsibleUsers"
      :existing-approvers="approvers"
      :existing-viewers="viewers"
      :attachments="attachments"
      :reader-only="isForwardReaderOnly"
      :is-sending="isForwarding"
      @close="closeForwardModal"
      @send="sendForwardRequest"
    />

    <!-- Получатели заявки (#1952) -->
    <ApplicationParticipantsModal
      :show="showParticipantsModal"
      :application-id="Number(applicationData.id)"
      @close="showParticipantsModal = false"
      @select="openParticipantFromList"
    />

    <!-- Карточка участника (#1952). Одна на оба входа - строку списка получателей
         и согласующего в блоке согласования: два экземпляра разошлись бы тем,
         кто открыт и поверх чего лежит. -->
    <ApplicationParticipantCard
      :show="showParticipantCard"
      :participant="selectedParticipant"
      :loading="participantCardLoading"
      :error="participantCardError"
      @close="showParticipantCard = false"
    />

    <!-- Дополнение поданной заявки (#1685) -->
    <SupplementModal
      :show="showSupplementModal"
      :application="applicationData"
      :attachments="attachments"
      :all-unloading-places="allUnloadingPlaces"
      :license-plate-formats="licensePlateFormats"
      :all-tables="allTables"
      @close="showSupplementModal = false"
      @submitted="onSupplementSubmitted"
    />

    <div
      class="application-detail"
      data-testid="ob-detail-card"
      :class="{ 'is-dragging': sheetDragging }"
      :style="sheetOffset ? { transform: `translateY(${sheetOffset}px)` } : null"
      @touchstart="onSheetTouchStart"
      @touchmove="onSheetTouchMove"
      @touchend="onSheetTouchEnd"
    >
      <!-- Ползунок bottom-sheet (виден только на мобилке), свайп вниз закрывает -->
      <div
        class="sheet-handle"
        aria-hidden="true"
      />
      <!-- Заголовок и кнопки -->
      <div
        class="detail-header"
        data-testid="ob-detail-header"
      >
        <div class="detail-header-left">
          <div class="detail-title-row">
            <h3 class="detail-title">
              Заявка <CopyableNumber data-testid="app-detail-number" :value="applicationData.application_number" />
            </h3>
            <div class="detail-datetime">
              {{ formatDateTime(applicationData.sending_datetime) }}
              <span class="weekday">{{ weekdayName(applicationData.sending_datetime) }}</span>
            </div>
            <!-- Кнопка пересылки (рядом с датой): fade при появлении/скрытии -->
            <transition name="fade">
              <button
                v-if="mode === 'center' && canForwardApplication && can('action.forward.application')"
                class="forward-btn"
                data-testid="app-detail-button-forward"
                :disabled="updatingConfirmation || processingApplication"
                @click="forwardApplication"
              >
                <span
                  v-if="updatingConfirmation || processingApplication"
                  class="button-loading"
                />
                <template v-else>
                  <span class="forward-btn__text">Переслать</span>
                  <!-- На мобилке кнопка сжимается до иконки (текст скрыт @768) -->
                  <svg
                    class="forward-btn__icon"
                    width="18"
                    height="18"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    aria-hidden="true"
                  >
                    <polyline points="15 17 20 12 15 7" />
                    <path d="M4 18v-2a4 4 0 0 1 4-4h12" />
                  </svg>
                </template>
              </button>
            </transition>
            <!-- Скачать бланк: на мобилке кнопку убрали из строки списка (W3.8),
                 отсюда родитель (Центр/Кабинет) открывает выбор бланков. Только @768. -->
            <button
              v-if="canDownloadBlank || tourOnlyActions"
              class="detail-download-btn"
              :class="{ 'is-tour-stub': !canDownloadBlank }"
              :disabled="!canDownloadBlank"
              data-testid="app-detail-button-download"
              title="Скачать бланк"
              @click="$emit('download', applicationData)"
            >
              <svg
                width="17"
                height="17"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
                aria-hidden="true"
              >
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                <polyline points="7 10 12 15 17 10" />
                <line
                  x1="12"
                  y1="15"
                  x2="12"
                  y2="3"
                />
              </svg>
              <span class="detail-download-btn__text">Скачать</span>
            </button>
            <!-- Получатели (#1952): кто видит заявку и кто по ней голосует. Своего
                 гейта у кнопки нет - метод отдаёт список тому, кому видна сама
                 заявка, а она уже открыта. -->
            <!-- aria-label дублирует подпись: на мобилке и в сжатом виде (см. ниже)
                 текст скрыт, и без него кнопка остаётся безымянным кружком для
                 скринридера. title - та же подпись хинтом при наведении: без
                 текста кружок ничего не объясняет визуально. -->
            <button
              class="participants-btn"
              data-testid="app-detail-button-participants"
              aria-label="Получатели"
              title="Получатели"
              @click="showParticipantsModal = true"
            >
              <svg
                class="participants-btn__icon"
                width="17"
                height="17"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
                aria-hidden="true"
              >
                <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
                <circle
                  cx="9"
                  cy="7"
                  r="4"
                />
                <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
                <path d="M16 3.13a4 4 0 0 1 0 7.75" />
              </svg>
              <span class="participants-btn__text">Получатели</span>
            </button>
          </div>
        </div>
        <div class="detail-header-right">
          <!-- Бейдж и action-bar в своей обёртке: у неё свой flex-wrap, поэтому
               перенос их содержимого не выталкивает крестик на отдельную строку
               (#1685 -> регрессия шапки, замерено в браузере). -->
          <div class="detail-header-actions">
            <!-- Второй бейдж: идёт повторный круг по дополнению (#1685). Рядом с бейджем
                 согласования, а не вместо него - статус заявки остаётся «Согласовано»,
                 потому что от него зависит допуск уже выданных пропусков. -->
            <transition name="fade">
              <Badge
                v-if="openSupplementBadge"
                class="supplement-round-badge"
                :variant="openSupplementBadge.variant"
                size="md"
                dot
                :title="openSupplementBadge.hint"
                data-testid="app-detail-supplement-round-badge"
              >
                {{ openSupplementBadge.text }}
              </Badge>
            </transition>
            <ApplicationActionBar
              v-if="mode !== 'center' || can('action.approve.application')"
              :application="applicationData"
              :current-user-id="currentUserId"
              :responsible-users="responsibleUsers"
              :approvers="approvers"
              :is-approver="isApprover"
              :mode="mode"
              :processing="processingApplication"
              :updating-confirmation="updatingConfirmation"
              :action-comment="actionComment"
              :has-unoverridden-blacklist-flags="!!applicationData.has_unoverridden_blacklist_flags"
              :ready="actionsReady"
              :supplements="supplements"
              @action-completed="handleActionCompleted"
              @processing-change="processingApplication = $event"
              @updating-confirmation-change="updatingConfirmation = $event"
              @comment-clear="clearCommentFromLocalStorage"
            >
              <template #user-actions>
                <transition name="fade">
                  <button
                    v-if="canSupplementApplication || tourOnlyActions"
                    class="supplement-btn"
                    :class="{ 'is-tour-stub': !canSupplementApplication }"
                    :disabled="!canSupplementApplication"
                    data-testid="app-detail-button-supplement"
                    @click="showSupplementModal = true"
                  >
                    Дополнить
                  </button>
                </transition>
                <BaseDropdown
                  class="duplicate-dropdown"
                  data-testid="ob-detail-duplicate"
                  :options="duplicatePresets"
                  :model-value="null"
                  label-key="label"
                  value-key="key"
                  placeholder="Продублировать"
                  @update:model-value="handleDuplicatePreset"
                />
                <transition name="fade">
                  <button
                    v-if="canWithdraw || tourOnlyActions"
                    class="withdraw-btn"
                    :class="{ 'is-tour-stub': !canWithdraw }"
                    :disabled="!canWithdraw"
                    data-testid="ob-detail-revoke"
                    @click="withdrawApplication"
                  >
                    Отозвать
                  </button>
                </transition>
              </template>
            </ApplicationActionBar>
          </div>

          <button
            class="close-detail-btn"
            @click="close"
          >
            ×
          </button>
        </div>
      </div>

      <div
        ref="sheetScroll"
        class="detail-content"
      >
        <!-- Левая колонка - вложения -->
        <div
          class="detail-left-column"
          :class="{ collapsed: isLeftColumnCollapsed }"
        >
          <!-- Обёртка-якорь для кросс-колоночного order на мобилке (@768). -->
          <div class="detail-order-picker">
            <ApplicationAttachments
              :application-id="applicationData.id"
              :attachments="attachments"
              :collapsed="isLeftColumnCollapsed"
              @attachment-selected="selectAttachment"
              @toggle-collapse="toggleLeftColumn"
            />
          </div>
        </div>

        <!-- Центральная колонка - детали -->
        <div class="detail-main-column">
          <!-- Сообщение заявки -->
          <div class="message-section">
            <div class="message-section-header">
              <h4>Сообщение к заявке {{ applicationData.application_number }}</h4>
            </div>
            <ApplicationFiles
              :application-id="Number(applicationData.id)"
              :can-remove="canRemoveFiles"
            />
            <template v-if="hasMessage">
              <!-- Тап по превью открывает полное сообщение в окне (кнопка "Открыть в
                   окне" убрана, W3.10); аффорданс - хинт-строка под превью. -->
              <div
                ref="messagePreview"
                class="message-content text-constructor-content message-preview"
                :class="{ 'is-clamped': messageClamped }"
                @click="openMessageFromPreview"
                v-html="sanitizedMessage"
              />
              <button
                type="button"
                class="message-open-hint"
                @click="showMessageModal = true"
              >
                <svg
                  width="14"
                  height="14"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  aria-hidden="true"
                >
                  <polyline points="15 3 21 3 21 9" />
                  <polyline points="9 21 3 21 3 15" />
                  <line
                    x1="21"
                    y1="3"
                    x2="14"
                    y2="10"
                  />
                  <line
                    x1="3"
                    y1="21"
                    x2="10"
                    y2="14"
                  />
                </svg>
                Нажмите, чтобы открыть полностью
              </button>
            </template>
            <div
              v-else
              class="message-content message-empty"
            >
              Сообщение отсутствует
            </div>
          </div>

          <ApplicationMessageModal
            :show="showMessageModal"
            :message="applicationData.message || ''"
            :application-number="applicationData.application_number"
            @close="showMessageModal = false"
          />

          <!-- Заметка бюро: рабочий стикер принимающих о том, почему заявка не
               сделана. Гейт здесь для удобства - текст заметки бэк отдаёт в детали
               заявки только принимающему, у остальных его в applicationData нет. -->
          <ApplicationBureauNote
            v-if="isApprover"
            :application-id="Number(applicationData.id)"
            :note="applicationData.bureau_note || null"
            @update="onBureauNoteUpdate"
          />

          <!-- Сообщения при пересылке (#967), видны всем получателям -->
          <div class="detail-order-forward">
            <ForwardMessages
              ref="forwardMessagesComponent"
              :application-id="applicationData.id"
            />
          </div>

          <!-- Обсуждение заявки (#973): тема + тред ответов -->
          <div class="detail-order-questions">
            <ApplicationQuestions
              ref="questionsComponent"
              :application-id="applicationData.id"
              :attachments="attachments"
              :current-user-id="currentUserId"
              :current-user-name="currentUserName"
              :initiator-user-id="applicationData.sender_user_id"
              :can-ask="canAskQuestion"
              @all-questions-read="$emit('questions-read', $event)"
            />
          </div>

          <!-- Детали выбранного вложения -->
          <div
            v-if="selectedAttachment"
            class="detail-order-selected-attachment"
          >
            <ApplicationAttachmentDetail
              :attachment="selectedAttachment"
              :cars="attachmentCars"
              :employees="attachmentEmployees"
              :items="attachmentItems"
              :loading="loadingAttachmentDetails"
              :can-override="canOverrideBlacklist"
              :can-remove="canRemoveElements"
              :can-assign="canAssignPlaces"
              :application-id="applicationData.id"
              @open-vehicle="openVehicleModal"
              @open-employee="openEmployeeModal"
              @override-element="openOverrideModal"
              @remove-element="openRemovalModal"
              @assignments-changed="loadAttachmentDetails(selectedAttachment.id)"
            />
          </div>
        </div>

        <!-- Правая колонка - информация о заявке и согласовании -->
        <div class="detail-right-column">
          <!-- Основная информация -->
          <div class="basic-info-section">
            <h4>Основная информация</h4>
            <div class="info-grid">
              <div class="info-row">
                <span class="info-label">Организация / Отдел:</span>
                <span class="info-value">{{ applicationData.organization_name }}</span>
              </div>
              <div
                v-if="applicationData.company_name"
                class="info-row"
              >
                <span class="info-label">Компания:</span>
                <span class="info-value">{{ applicationData.company_name }}</span>
              </div>
              <div class="info-row">
                <span class="info-label">Отправитель:</span>
                <span class="info-value sender-value">
                  <span>{{ applicationData.sender_full_name || applicationData.sender_name }}</span>
                  <Badge
                    v-if="applicationData.sender_is_important"
                    variant="info"
                    size="sm"
                    class="sender-important-tag"
                  >
                    <svg
                      width="12"
                      height="12"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                    ><polygon points="12 2 15 8.6 22 9.3 16.8 14 18.3 21 12 17.3 5.7 21 7.2 14 2 9.3 9 8.6" /></svg>
                    Важный
                  </Badge>
                </span>
              </div>
            </div>

            <!-- Разбор наименования, заведённого подачей (#1437): плашка видна только
                 тому, у кого есть право разбора, и только пока запись на проверке. -->
            <DirectoryModeration
              v-for="entry in pendingDirectoryEntries"
              :key="entry.kind"
              :kind="entry.kind"
              :entry-id="entry.id"
              :entry-name="entry.name"
              @resolved="onDirectoryResolved"
            />
          </div>

          <!-- Блок статуса заявки (для принятых/отказанных/завершенных/отозванных).
               Отозвана держим здесь же: после отзыва инфо о принятии не должно пропадать.
               fade при появлении секции (смена статуса). Карточка с box-shadow, поэтому
               НЕ grid-collapse (overflow-контейнер обрезал бы тень) - opacity/translateY. -->
          <transition name="fade">
            <div
              v-if="hasStatusSection"
              class="application-status-section"
              data-testid="ob-detail-status-section"
            >
              <div class="status-header">
                <h4>Статус заявки</h4>
                <span
                  class="status-mini-badge"
                  :class="getStatusBadgeClass(applicationData.status)"
                >
                  {{ applicationData.status }}
                </span>
              </div>

              <!-- Инфо о статусе: cross-fade при смене статуса (out-in по :key=status) -->
              <transition
                name="fade"
                mode="out-in"
              >
                <div
                  :key="applicationData.status"
                  class="status-info-swap"
                >
                  <!-- Для статусов В работе и Отказано -->
                  <div
                    v-if="applicationData.status === 'В работе' || applicationData.status === 'Отказано'"
                    class="status-info"
                  >
                    <div
                      v-if="applicationData.responsible_user_id"
                      class="status-info-row"
                    >
                      <span class="status-info-label">{{ applicationData.status === 'В работе' ? 'Принял(-а):' : 'Отказал(а):' }}</span>
                      <span class="status-info-value">{{ applicationData.responsible_name || 'Не указан' }}</span>
                    </div>
                    <div
                      v-if="applicationData.confirmation_datetime"
                      class="status-info-row"
                    >
                      <span class="status-info-label">Время:</span>
                      <span class="status-info-value">{{ formatDateTime(applicationData.confirmation_datetime) }}</span>
                    </div>
                    <div class="status-info-row comment-row">
                      <span class="status-info-label">Комментарий:</span>
                      <div class="status-info-value comment-text">
                        {{ applicationData.responsible_comment || 'Комментария нет' }}
                      </div>
                    </div>
                  </div>

                  <!-- Для статуса Завершено (показываем и принятие, и завершение) -->
                  <div
                    v-else-if="applicationData.status === 'Завершено'"
                    class="status-info"
                  >
                    <!-- Информация о принятии -->
                    <div
                      v-if="applicationData.responsible_name"
                      class="status-info-row"
                    >
                      <span class="status-info-label">Принял(-а):</span>
                      <span class="status-info-value">{{ applicationData.responsible_name }}</span>
                    </div>
                    <div
                      v-if="applicationData.confirmation_datetime"
                      class="status-info-row"
                    >
                      <span class="status-info-label">Время принятия:</span>
                      <span class="status-info-value">{{ formatDateTime(applicationData.confirmation_datetime) }}</span>
                    </div>
                    <!-- Информация о завершении -->
                    <div
                      v-if="applicationData.completed_by_name"
                      class="status-info-row"
                    >
                      <span class="status-info-label">Завершил(-а):</span>
                      <span class="status-info-value">{{ applicationData.completed_by_name }}</span>
                    </div>
                    <div
                      v-if="applicationData.completed_at"
                      class="status-info-row"
                    >
                      <span class="status-info-label">Время завершения:</span>
                      <span class="status-info-value">{{ formatDateTime(applicationData.completed_at) }}</span>
                    </div>
                    <!-- Комментарий к завершению (или общий) -->
                    <div class="status-info-row comment-row">
                      <span class="status-info-label">Комментарий:</span>
                      <div class="status-info-value comment-text">
                        {{ applicationData.completion_comment || 'Комментария нет' }}
                      </div>
                    </div>
                  </div>

                  <!-- Для статуса Отозвана: если заявку успели принять до отзыва - показываем,
                 кто принял, чтобы информация не пропадала после отзыва. -->
                  <div
                    v-else-if="applicationData.status === 'Отозвана' && applicationData.responsible_user_id"
                    class="status-info"
                  >
                    <div class="status-info-row">
                      <span class="status-info-label">Принял(-а):</span>
                      <span class="status-info-value">{{ applicationData.responsible_name || 'Не указан' }}</span>
                    </div>
                    <div
                      v-if="applicationData.confirmation_datetime"
                      class="status-info-row"
                    >
                      <span class="status-info-label">Время принятия:</span>
                      <span class="status-info-value">{{ formatDateTime(applicationData.confirmation_datetime) }}</span>
                    </div>
                    <div
                      v-if="applicationData.responsible_comment"
                      class="status-info-row comment-row"
                    >
                      <span class="status-info-label">Комментарий:</span>
                      <div class="status-info-value comment-text">
                        {{ applicationData.responsible_comment }}
                      </div>
                    </div>
                  </div>
                </div>
              </transition>
            </div>
          </transition>

          <!-- Поле для комментария: fade + показ по showCommentSection. Карточка с
               box-shadow, поэтому opacity/translateY-переход, без overflow-контейнера. -->
          <transition name="fade">
            <div
              v-if="showCommentSection"
              class="comment-action-section"
            >
              <h4>Комментарий</h4>
              <textarea
                v-model="actionComment"
                class="comment-action-textarea"
                placeholder="Вы можете написать здесь комментарий (необязательно)"
                rows="3"
                @input="saveCommentToLocalStorage"
              />
            </div>
          </transition>

          <!-- Компонент согласования (без информации о принявшем). Обёртка нужна
               для order на мобилке: держим согласование в блоке "комментарий/действие".
               Она же - якорь тура: сама .detail-right-column на <768 уходит в
               display:contents (нулевой box), подсветить её нельзя. -->
          <div
            class="detail-order-confirmation"
            data-testid="ob-detail-status"
          >
            <ApplicationConfirmation
              ref="confirmationComponent"
              :application="applicationData"
              :responsible-users="responsibleUsers"
              :current-user-id="currentUserId"
              :updating-confirmation="updatingConfirmation"
              @select-user="openParticipantByUser"
            />
          </div>

          <!-- Раунды дополнения (#1685): показываем только когда они у заявки есть -
               у подавляющего большинства заявок дополнений нет, и пустая карточка
               в колонке была бы шумом. Обёртка нужна для order на мобилке; v-if на
               ней же, а не на панели - пустой div в промоутнутой ленте забирал бы
               свой gap. -->
          <div
            v-if="hasSupplements"
            class="detail-order-supplement"
          >
            <SupplementPanel
              :supplements="supplements"
              :current-user-id="currentUserId"
              :loading="supplementsLoading"
              :error="supplementsError"
            />
          </div>

          <div
            v-if="can('center.application_history')"
            class="history-button-section"
          >
            <ApplicationHistory
              ref="historyComponent"
              :application-id="applicationData.id"
              :application-number="applicationData.application_number"
              :current-user-id="currentUserId"
              :current-user-name="currentUserName"
              :application-organization="applicationData.organization_name"
              @application-updated="handleApplicationUpdate"
            />
          </div>
        </div>
      </div>
    </div>
    <VehicleDetailsModal
      :show="showVehicleModal"
      :vehicle="selectedVehicle"
      :all-unloading-places="allUnloadingPlaces"
      :all-tables="allTables"
      :license-plate-formats="licensePlateFormats"
      :current-user-id="currentUserId"
      :current-user-name="currentUserName"
      :show-car-features="true"
      :source="'application'"
      :can-override="canOverrideBlacklist"
      :can-cancel-override="canManageBlacklistOverride"
      @close="showVehicleModal = false"
      @override="onCardOverride('vehicle')"
      @cancel-override="onCardCancelOverride('vehicle')"
    />

    <EmployeeDetailsModal
      :show="showEmployeeModal"
      :employee="selectedEmployee"
      :all-tables="allTables"
      :current-user-id="currentUserId"
      :current-user-name="currentUserName"
      :source="'application'"
      :can-override="canOverrideBlacklist"
      :can-cancel-override="canManageBlacklistOverride"
      @close="showEmployeeModal = false"
      @override="onCardOverride('employee')"
      @cancel-override="onCardCancelOverride('employee')"
    />

    <ElementRemovalModal
      :show="showRemovalModal"
      :label="removalLabel"
      :submitting="removalSubmitting"
      @confirm="confirmRemoval"
      @close="showRemovalModal = false"
    />

    <BlacklistOverrideModal
      :show="showOverrideModal"
      :flag="overrideFlag"
      :submitting="overrideSubmitting"
      @confirm="confirmOverride"
      @close="showOverrideModal = false"
    />
    </div>
  </transition>
</template>

<script>
import { apiRequest } from '@/api/client'
import { markAsRead, getApplicationSupplements, getApplicationParticipants } from '@/api/applications'
import { useDeletionsStore } from '@/stores/deletions'
import { useUiStore } from '@/stores/ui'
import { SUPPLEMENT_APPROVED } from '@/utils/supplementStatuses'
import { usePermissionsStore } from '@/stores/permissions'
import { useAuthStore } from '@/stores/auth'
import ApplicationAttachments from './ApplicationAttachments.vue'
import ApplicationFiles from './ApplicationFiles.vue'
import ApplicationConfirmation from './ApplicationConfirmation.vue'
import ApplicationHistory from './ApplicationHistory.vue'
import ForwardModal from './ForwardModal.vue'
import ForwardMessages from './ForwardMessages.vue'
import ApplicationBureauNote from './ApplicationBureauNote.vue'
import ApplicationQuestions from './ApplicationQuestions.vue'
import ApplicationActionBar from './ApplicationActionBar.vue'
import ApplicationAttachmentDetail from './ApplicationAttachmentDetail.vue'
import BlacklistOverrideModal from './BlacklistOverrideModal.vue'
import ElementRemovalModal from './ElementRemovalModal.vue'
import VehicleDetailsModal from '../CreateApplication/VehicleDetailsModal.vue'
import EmployeeDetailsModal from '../CreateApplication/EmployeeDetailsModal.vue'
import Badge from '@/components/ui/Badge.vue'
import BaseDropdown from '@/components/ui/BaseDropdown.vue'
import CopyableNumber from '@/components/ui/CopyableNumber.vue'
import { sanitizeHtml } from '@/utils/sanitize'
import { weekdayName } from '@/utils/datetime'
import { setModalOpen, releaseModal, isTopModal, isEscapeHandled, markEscapeHandled } from '@/utils/modalStack'
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock'

/** Слой панели заявки - то же значение, что у неё в стилях (.application-detail). */
const DETAIL_STACK_LAYER = 10002

/**
 * Длительность ухода панели. Совпадает с transition в стилях; о закрытии сообщаем
 * родителю по её истечении, а не по `after-leave`: хук перехода не отрабатывает там,
 * где браузерных переходов нет (jsdom), и панель осталась бы висеть.
 */
const DETAIL_CLOSE_MS = 200
import ApplicationMessageModal from './ApplicationMessageModal.vue'
import ApplicationParticipantsModal from './ApplicationParticipantsModal.vue'
import ApplicationParticipantCard from './ApplicationParticipantCard.vue'
import DirectoryModeration from '@/components/directory/DirectoryModeration.vue'
import eventStream from '@/services/eventStream'
import { ref } from 'vue'
import { useSwipeDismiss } from '@/composables/useSwipeDismiss'
import SupplementModal from '../CreateApplication/SupplementModal.vue'
import SupplementPanel from './SupplementPanel.vue'
import { removeApplicationElements } from '@/api/applicationAssignments'

// Статусы, в которых заявку ещё можно дополнить (#1685). Зеркало
// services.supplementAllowedStatuses - остальные бэк отклоняет с 409.
const SUPPLEMENT_ALLOWED_STATUSES = ['Непрочитано', 'В обработке', 'В работе']

export default {
    name: 'ApplicationDetail',
    components: {
        ApplicationAttachments,
        ApplicationFiles,
        ApplicationBureauNote,
        ApplicationConfirmation,
        ApplicationHistory,
        ForwardModal,
        ForwardMessages,
        ApplicationQuestions,
        ApplicationActionBar,
        ApplicationAttachmentDetail,
        BlacklistOverrideModal,
        ElementRemovalModal,
        VehicleDetailsModal,
        EmployeeDetailsModal,
        Badge,
        BaseDropdown,
        CopyableNumber,
        ApplicationMessageModal,
        ApplicationParticipantsModal,
        ApplicationParticipantCard,
        DirectoryModeration,
        SupplementModal,
        SupplementPanel
    },
    props: {
        application: {
            type: Object,
            required: true
        },
        currentUserId: {
            type: Number,
            default: null
        },
        currentUserName: {
            type: String,
            default: ''
        },
        mode: {
            type: String,
            default: 'center'
        }
    },
    emits: ['close', 'confirmation-updated', 'duplicate', 'withdraw', 'application-updated', 'update-application', 'application-changed', 'questions-read', 'download'],
    setup(props, { emit }) {
        const permissionsStore = usePermissionsStore();
        // Bottom-sheet на мобилке (#1097 W3.9): свайп вниз за ползунок/шапку закрывает.
        // sheetScroll - реальный скролл-контейнер (@1024 .detail-content), чтобы свайп
        // внутри прокрученного контента был обычным скроллом, а не закрытием.
        // handleSelector включает .detail-header (фиксированную шапку) - свайп из неё
        // закрывает ДАЖЕ при прокрученном контенте, не заставляя листать наверх (#1097 r2).
        const sheetScroll = ref(null);
        const swipe = useSwipeDismiss(() => emit('close'), {
            getScrollTop: () => sheetScroll.value?.scrollTop ?? 0,
            handleSelector: '.sheet-handle, .detail-header',
        });
        return {
            permissionsStore,
            sheetScroll,
            sheetOffset: swipe.offset,
            sheetDragging: swipe.isDragging,
            onSheetTouchStart: swipe.onTouchStart,
            onSheetTouchMove: swipe.onTouchMove,
            onSheetTouchEnd: swipe.onTouchEnd,
            dismissSheet: swipe.dismiss,
        };
    },
    data() {
        return {
            // Панель открыта. Гасим флаг при закрытии - это запускает leave, а
            // родителю о закрытии сообщаем уже после него.
            visible: true,
            closeTimer: null,
            applicationData: { ...this.application },
            eventStreamOff: null,
            eventStreamAppId: null,
            loadDetailSeq: 0,
            showMessageModal: false,
            messageClamped: false,
            attachments: [],
            selectedAttachment: null,
            attachmentCars: [],
            attachmentEmployees: [],
            attachmentItems: [],
            responsibleUsers: [],
            viewers: [],              // читатели
            updatingConfirmation: false,
            processingApplication: false,
            loadingAttachmentDetails: false,
            actionsReady: false,
            // Пресеты срока для дропдауна "Продублировать": проставляют дату действия
            // во все вложения дубля. 'other' - дублирует без даты (пользователь задаёт сам).
            duplicatePresets: [
                { key: 'nextMonth', label: 'На следующий месяц' },
                { key: 'tomorrow', label: 'На завтра' },
                { key: 'other', label: 'Другой срок' }
            ],
            isLeftColumnCollapsed: false,
            showSupplementModal: false,
            // Раунды дополнения заявки (#1685): свой список, свой seq-токен и своя
            // ошибка - карточка заявки не должна падать из-за недоступности раундов.
            supplements: [],
            supplementsLoading: false,
            supplementsError: '',
            supplementsSeq: 0,
            showForwardModal: false,
            showParticipantsModal: false,
            // Карточка участника (#1952). Список участников тянем ЛЕНИВО и держим до
            // следующего обновления детали: из блока согласования контактов и ролей
            // нет вовсе, а платить запросом за каждое открытие карточки не за что.
            // Тот, кто карточку ни разу не открыл, за неё и не платит.
            showParticipantCard: false,
            selectedParticipant: null,
            participantCardLoading: false,
            participantCardError: '',
            // Кого открывают прямо сейчас: пока летит запрос, человек успевает
            // кликнуть соседа, и ответ на прошлый клик не должен подменить карточку.
            participantCardUserId: null,
            participants: [],
            participantsLoadedFor: null,
            participantsInflight: null,
            // Поколение списка: деталь перечитывают по live-сигналу, и ответ,
            // стартовавший до этого, не должен осесть в памяти как свежий.
            participantsGeneration: 0,
            isForwarding: false,
            allUsers: [],
            approvers: [],
            isApproverSelf: false,
            actionComment: '',
            lastUserComment: '',
            storageKey: '',
            allUnloadingPlaces: [],
            licensePlateFormats: [],
            allTables: [],
            showVehicleModal: false,
            showEmployeeModal: false,
            selectedVehicle: null,
            selectedEmployee: null,
            showOverrideModal: false,
            overrideFlag: null,
            overrideLabel: '',
            overrideSubmitting: false,
            showRemovalModal: false,
            removalLabel: '',
            removalElementId: null,
            removalSubmitting: false
        }
    },
    computed: {
        /**
         * Идёт обучение. Тогда действия над заявкой показываем все, даже те, что
         * этой заявке сейчас не положены - иначе тур рассказывает про кнопку,
         * которой на экране нет, и человек ищет её глазами. Показанные «лишними»
         * кнопки неактивны (см. tourOnly в шаблоне).
         */
        tourOnlyActions() {
            return useUiStore().tourActive;
        },

        /** Супер-администратор: доступ к заявке и пересылка ему открыты безусловно. */
        isSuperAdmin() {
            return useAuthStore().isSuperAdmin;
        },

        /** Бланки к заявке настроены и их выгрузка этому режиму/праву доступна. */
        canDownloadBlank() {
            return Boolean(this.applicationData.has_blank_template)
                && (this.mode !== 'center' || this.can('action.export.applications'));
        },

        hasMessage() {
            const m = this.applicationData?.message;
            if (!m) return false;
            return m.includes('<img') || m.replace(/<[^>]*>/g, '').trim().length > 0;
        },

        sanitizedMessage() {
            return this.applicationData?.message ? sanitizeHtml(this.applicationData.message) : '';
        },

        isResponsibleUser() {
            if (!this.currentUserId || !this.responsibleUsers.length) return false;
            return this.responsibleUsers.some(user => user.id === this.currentUserId);
        },

        /**
         * Пропустить помеченный элемент вправе и согласующий, и принимающий - кто
         * первым дошёл до заявки. Отменять подтверждение оба могли и раньше.
         */
        canOverrideBlacklist() {
            return this.isResponsibleUser || this.isApprover;
        },

        /**
         * Состав поданной заявки правит только принимающий, и только пока заявка не
         * закрыта: белый список статусов повторяет серверный гард.
         */
        canRemoveElements() {
            if (!this.isApprover) return false;
            const status = this.applicationData && this.applicationData.status;
            return ['Непрочитано', 'В обработке', 'В работе'].includes(status);
        },

        isApprover() {
            if (this.isApproverSelf) return true;
            // Состав приходит только администратору - для него это тот же ответ, просто
            // из уже загруженных данных.
            if (!this.currentUserId || !this.approvers.length) return false;
            return this.approvers.some(approver => approver.user_id === this.currentUserId);
        },

        /**
         * Доназначать посты и места может принимающий, пока заявка не закрыта.
         * Набор статусов - зеркало серверного гейта (#1393): белый список, а не
         * «всё кроме терминальных», иначе новый терминальный статус молча пройдёт.
         */
        canAssignPlaces() {
            const editable = ['Непрочитано', 'В обработке', 'В работе'];
            return this.isApprover && editable.includes(this.applicationData?.status);
        },

        isViewer() {
            if (!this.currentUserId || !this.viewers.length) return false;
            return this.viewers.some(viewer => viewer.user_id === this.currentUserId);
        },

        /**
         * Наименования заявки, заведённые самой подачей и ждущие разбора (#1437).
         * Гейт - право разбора: заявителю показывать нечего, действия ему всё равно
         * закрыты серверным middleware.
         *
         * organization_name у заявки без организации содержит имя компании
         * (COALESCE на бэке), поэтому имя компании берём из company_name.
         */
        pendingDirectoryEntries() {
            if (!this.can('application.organization.moderate')) return [];
            const a = this.applicationData;
            if (!a) return [];
            const entries = [];
            if (a.organization_id && a.organization_moderation_status === 'pending') {
                entries.push({ kind: 'organization', id: a.organization_id, name: a.organization_name || '' });
            }
            if (a.company_id && a.company_moderation_status === 'pending') {
                entries.push({ kind: 'company', id: a.company_id, name: a.company_name || '' });
            }
            return entries;
        },

        canAskQuestion() {
            // Начать обсуждение может любой с доступом к заявке, ВКЛЮЧАЯ инициатора (#973
            // followup): инициатор обсуждает свою же заявку. Реальный гейт - на бэке.
            const a = this.applicationData;
            if (!a) return false;
            return this.isResponsibleUser || this.isApprover || this.isViewer ||
                a.sender_user_id === this.currentUserId;
        },

        /**
         * Убрать приложенный файл (#1721) может носитель права администрирования:
         * состав заявки после подачи неизменен, а удаление нужно, чтобы вычистить
         * приложенное вопреки подписи поля. Зеркалит гейт роута (page.admin) - по
         * одному лишь признаку супер-администратора крестик не видел обычный
         * администратор, у которого это право есть.
         */
        canRemoveFiles() {
            return this.can('page.admin');
        },

        // Отозвать свою заявку может только отправитель и только пока она не в
        // терминальном (закрытом) статусе - зеркалит BE-гейт WithdrawApplication (#951).
        canWithdraw() {
            const a = this.applicationData;
            if (!a || a.sender_user_id !== this.currentUserId) return false;
            return !['Завершено', 'Не согласовано', 'Отказано', 'Отозвана'].includes(a.status);
        },

        /**
         * Дополнить заявку (#1685) может её автор, пока заявка не закрыта и по ней нет
         * незакрытого раунда дополнения. Список статусов - БЕЛЫЙ и повторяет
         * supplementAllowedStatuses бэка: чёрный перечень терминальных пропустил бы
         * «Согласование»/«Не согласовано», на которых сервер отвечает 409.
         *
         * open_supplement приезжает отдельным срезом; пока поля в ответе нет, считаем,
         * что открытого раунда нет - иначе кнопка не появилась бы вовсе.
         */
        canSupplementApplication() {
            const a = this.applicationData;
            if (!a || a.sender_user_id !== this.currentUserId) return false;
            if (!this.can('action.supplement.application')) return false;
            if (a.open_supplement) return false;
            return SUPPLEMENT_ALLOWED_STATUSES.includes(a.status);
        },

        /**
         * Есть ли у заявки раунды дополнения (#1685). Признак берём из детали
         * (supplements_count/open_supplement), а не из длины загруженного списка:
         * пока список едет, панель обязана уже стоять со своим лоадером, иначе она
         * выскочит рывком, а на ошибке загрузки не появится вовсе.
         */
        hasSupplements() {
            const a = this.applicationData;
            if (!a) return false;
            return Number(a.supplements_count) > 0 || !!a.open_supplement || this.supplements.length > 0;
        },

        /**
         * Плашка «идёт повторный круг» для шапки (#1685).
         *
         * Бейдж согласования остаётся на месте: заявка как была «Согласовано», так и
         * остаётся - откат её статуса снял бы с проходной уже выданные пропуска. Про
         * добавку сообщает вторая плашка, новых значений confirmation не заводим.
         *
         * @returns {{ variant: string, text: string, hint: string }|null}
         */
        openSupplementBadge() {
            const open = this.applicationData && this.applicationData.open_supplement;
            if (!open) return null;

            const title = open.number ? `Дополнение №${open.number}` : 'Дополнение';
            const awaitingAccept = open.status === SUPPLEMENT_APPROVED;
            return {
                variant: awaitingAccept ? 'info' : 'warning',
                text: awaitingAccept ? `+ ${title} ждёт принятия` : `+ ${title} на согласовании`,
                hint: awaitingAccept
                    ? `${title} согласовано и ждёт решения принимающего. Статус самой заявки не менялся.`
                    : `${title} ждёт голосов согласующих. Статус самой заявки не менялся.`
            };
        },

        // Отменить подтверждение пропуска может ответственный по заявке ИЛИ принимающий -
        // зеркалит право DELETE /blacklist-overrides на бэке (шире, чем создание override).
        canManageBlacklistOverride() {
            return this.isResponsibleUser || this.isApprover;
        },

        /**
         * Доступ к заявке: зеркало CanAccessApplication на бэке. Супер-админ и
         * принимающий (оператор бюро) видят любую заявку, остальные - свою по роли
         * на ней: отправитель, ответственный, согласующий (он же строка в
         * responsible_users) и читатель.
         */
        hasApplicationAccess() {
            const a = this.applicationData;
            if (!a) return false;
            if (this.isSuperAdmin) return true;
            return this.isApprover || this.isResponsibleUser || this.isViewer ||
                a.sender_user_id === this.currentUserId;
        },

        /**
         * Переслать заявку вправе любой, у кого есть к ней доступ (#1948): гейт
         * пересылки на бэке = гейт доступа. Прежнее «только ответственный» осталось от
         * #680 и отсекало отправителя, принимающего и читателя, хотя сервер их пускает.
         * Отозванную заявку сервер отбивает checkNotWithdrawn - действий по ней нет.
         */
        canForwardApplication() {
            return this.hasApplicationAccess && this.applicationData.status !== 'Отозвана';
        },

        /**
         * Заявка доступна только на просмотр: тогда и переслать её можно лишь на
         * просмотр - назначение согласующего или ответственного сервер отбивает 403.
         * Зеркало forwardAuthority.readerOnly: супер-админ и принимающий проходят
         * раньше проверки роли на заявке, дальше решают отправитель/ответственный.
         */
        isForwardReaderOnly() {
            if (this.isSuperAdmin || this.isApprover) return false;
            if (this.applicationData?.sender_user_id === this.currentUserId) return false;
            return !this.isResponsibleUser;
        },

        hasUserVoted() {
            if (!this.currentUserId || !this.responsibleUsers.length) return false;
            const currentUser = this.responsibleUsers.find(user => user.id === this.currentUserId);
            return currentUser && currentUser.approval_status !== 'pending';
        },

        isApproverActionDone() {
            if (!this.isApprover || this.isResponsibleUser) return false;
            return this.applicationData.status === 'В работе' || this.applicationData.status === 'Отказано' || this.applicationData.status === 'Завершено';
        },

        userVoteStatus() {
            if (!this.currentUserId || !this.responsibleUsers.length) return null;
            const currentUser = this.responsibleUsers.find(user => user.id === this.currentUserId);
            
            if (!currentUser) return null;
            
            if (currentUser.approval_status === 'approved') {
                return {
                    text: 'Вы согласовали',
                    class: 'vote-approved'
                };
            } else if (currentUser.approval_status === 'rejected') {
                return {
                    text: 'Вы отказали',
                    class: 'vote-rejected'
                };
            }
            
            return null;
        },

        canLeaveComment() {
            if (this.processingApplication) return false;
            // Терминальные (закрытые) статусы: поток согласования завершён, действия и
            // комментарий недоступны - список зеркалит canWithdraw/BE-гейт (#951). Раньше
            // проверялась только «Отозвана», из-за чего на «Завершено» ответственный, ещё
            // не голосовавший, проходил ветку ниже (return !hasUserVoted = true) и видел
            // поле комментария на завершённой заявке (баг keeq0, #1097).
            if (['Завершено', 'Не согласовано', 'Отказано', 'Отозвана'].includes(this.applicationData.status)) return false;

            if (this.isApprover && !this.isResponsibleUser) {
                return !this.isApproverActionDone;
            }
            
            if (this.isResponsibleUser) {
                return !this.hasUserVoted;
            }
            
            return false;
        },

        hasStatusSection() {
            return ['В работе', 'Отказано', 'Завершено', 'Отозвана'].includes(this.applicationData.status);
        },

        // Условие показа секции комментария (зеркалит v-if шаблона) - вынесено в computed,
        // чтобы обёртка-<transition> и сама секция гейтились одним источником правды.
        showCommentSection() {
            return this.canLeaveComment && !this.hasUserVoted && !this.isApproverActionDone &&
                (this.mode !== 'center' || this.can('action.approve.application'));
        },

    },
    watch: {
        application: {
            immediate: true,
            handler(newApplication, oldApplication) {
                if (newApplication && newApplication.id) {
                    this.applicationData = { ...newApplication };
                    this.showMessageModal = false;
                    this.storageKey = `app_comment_${this.currentUserId}_${newApplication.id}`;
                    this.loadCommentFromLocalStorage();
                    this.loadApplicationDetails(newApplication);
                    if (!oldApplication || oldApplication.id !== newApplication.id) {
                        markAsRead(newApplication.id).catch(() => {});
                        this.subscribeToApplication(newApplication.id);
                    }
                }
            },
            deep: true
        },
        sanitizedMessage() {
            this.$nextTick(() => this.updateMessageClamp());
        }
    },
    mounted() {
        this.loadCommonData();
        this.$nextTick(() => this.updateMessageClamp());
        // Real-time (#840 V4): подписка на изменения открытой заявки (сам scope
        // ставится в watch application по её id). connect - refcount'ный.
        eventStream.connect();
        // Панель заявки закрывается по Escape наравне с окнами. В общей стопке она
        // стоит своим слоем: пока поверх открыто окно (получатели, карточка участника,
        // пересылка), Escape закрывает его, а до панели доходит, когда она верхняя.
        setModalOpen(this, true, DETAIL_STACK_LAYER);
        document.addEventListener('keydown', this.handleDetailEscape);
        // Панель заявки - полноэкранное окно на мобилке (bottom-sheet), и фон под
        // ней (Центр/кабинет) должен стоять на месте, как под любым другим окном
        // проекта - через общий замок (владелец по стопке, не голое присвоение).
        setBodyScrollLock(this, true);
    },
    beforeUnmount() {
        if (this.eventStreamOff) {
            this.eventStreamOff();
            this.eventStreamOff = null;
        }
        eventStream.disconnect();
        document.removeEventListener('keydown', this.handleDetailEscape);
        releaseModal(this);
        releaseBodyScrollLock(this);
        if (this.closeTimer) clearTimeout(this.closeTimer);
    },
    methods: {
        can(key) {
            return this.permissionsStore.hasPermission(key);
        },

        /**
         * Escape закрывает панель заявки, если поверх неё ничего не открыто.
         * @param {KeyboardEvent} e
         */
        handleDetailEscape(e) {
            if (e.key !== 'Escape') return;
            // Окно поверх панели забирает нажатие себе - и по стопке, и по пометке на
            // событии: снятие со стопки происходит следующим тиком, а слушатели одного
            // нажатия идут подряд, поэтому одной стопки мало.
            if (isEscapeHandled(e)) return;
            if (!isTopModal(this)) return;
            markEscapeHandled(e);
            this.close();
        },

        /**
         * Запись справочника разобрана (#1437): плашка гаснет сразу, наименование в
         * шапке заявки становится итоговым. Тот же объект уходит наверх - список Центра
         * держит свою копию заявки и без эмита показывал бы старое наименование.
         *
         * При привязке id меняется на целевую запись: ссылки заявки бэк уже перевёл.
         */
        onDirectoryResolved({ kind, id, name }) {
            const patch = kind === 'company'
                ? { company_id: id ?? this.applicationData.company_id, company_name: name, company_moderation_status: 'approved' }
                : { organization_id: id ?? this.applicationData.organization_id, organization_name: name, organization_moderation_status: 'approved' };
            // Заявка без организации показывает в шапке имя компании (COALESCE на бэке).
            if (kind === 'company' && !this.applicationData.organization_id) {
                patch.organization_name = name;
            }
            this.applicationData = { ...this.applicationData, ...patch };
            this.$emit('application-changed', this.applicationData);
        },
        // Подписка на real-time обновления конкретной заявки (#840 V4): снимает
        // старую подписку, подписывается на scope application:<id>.
        subscribeToApplication(appId) {
            if (this.eventStreamOff) {
                this.eventStreamOff();
                this.eventStreamOff = null;
            }
            if (!appId) return;
            this.eventStreamAppId = appId;
            this.eventStreamOff = eventStream.subscribe(`application:${appId}`, () => this.refreshLiveDetail());
        },
        // Тихий рефетч открытой детали по сигналу application.updated: статус/
        // согласующие/вложения (loadApplicationDetails) + история + вопросы. Обновляет
        // только текущую заявку - участник видит изменения без F5.
        refreshLiveDetail() {
            if (!this.applicationData || !this.applicationData.id) return;
            // preserveSelection: фоновый рефетч от чужого действия не должен сбрасывать
            // выбранное вложение и мигать кнопками действий (это не наше действие).
            this.loadApplicationDetails(this.applicationData, { preserveSelection: true });
            if (this.$refs.historyComponent && this.$refs.historyComponent.loadHistory) {
                this.$refs.historyComponent.loadHistory();
            }
            if (this.$refs.questionsComponent && this.$refs.questionsComponent.load) {
                this.$refs.questionsComponent.load();
            }
            if (this.$refs.forwardMessagesComponent && this.$refs.forwardMessagesComponent.load) {
                this.$refs.forwardMessagesComponent.load();
            }
        },
        updateMessageClamp() {
            const el = this.$refs.messagePreview;
            this.messageClamped = !!el && el.scrollHeight > el.clientHeight + 2;
        },

        handleActionCompleted({ success, message, type }) {
            const resolvedType = type || (success ? 'success' : 'error');
            // ActionBar шлёт ошибку как "Ошибка: ...", а карточка тоста уже даёт заголовок
            // "Ошибка" - снимаем префикс, чтобы не дублировать.
            const text = resolvedType === 'error' ? String(message ?? '').replace(/^Ошибка:\s*/, '') : message;
            useDeletionsStore().notify({ bold: text, type: resolvedType });
            if (success) {
                // loadApplicationDetails тянет за собой и раунды дополнения: решение по
                // раунду меняет и его статус, и состав вложения (#1685).
                this.loadApplicationDetails(this.applicationData);
                if (this.$refs.historyComponent) {
                    this.$refs.historyComponent.loadHistory();
                }
                this.$emit('application-changed', this.applicationData);
            }
        },

        /**
         * Раунды дополнения заявки (#1685). Зовётся из loadApplicationDetails, то есть
         * на всех путях обновления карточки: открытие, собственное действие, тихий
         * рефетч по application.updated (SSE) и подача нового дополнения - чужое решение
         * долетает без F5.
         *
         * Свой seq-токен: SSE-сигнал и собственное действие могут идти подряд, и ответ
         * более раннего запроса не должен затирать более свежий (#632/#840).
         */
        async loadSupplements() {
            const a = this.applicationData;
            if (!a || !a.id) return;
            // Заявок без дополнений подавляющее большинство - лишний запрос на каждое
            // открытие карточки не делаем.
            if (!Number(a.supplements_count) && !a.open_supplement) {
                this.supplements = [];
                this.supplementsError = '';
                return;
            }

            const seq = ++this.supplementsSeq;
            this.supplementsLoading = true;
            try {
                const rounds = await getApplicationSupplements(a.id);
                if (seq !== this.supplementsSeq) return;
                this.supplements = Array.isArray(rounds) ? rounds : [];
                this.supplementsError = '';
            } catch (error) {
                if (seq !== this.supplementsSeq) return;
                // Текст ошибки показывает сама панель: тост на фоновом рефетче по SSE
                // всплывал бы при каждом сигнале, а карточка заявки остаётся рабочей.
                this.supplementsError = error.message || 'Не удалось загрузить дополнения заявки';
            } finally {
                if (seq === this.supplementsSeq) this.supplementsLoading = false;
            }
        },

        /**
         * Карточка участника по строке списка получателей (#1952). Запись уже
         * загружена окном - открытие бесплатно.
         * @param {object} participant
         */
        openParticipantFromList(participant) {
            this.participantCardUserId = Number(participant?.user_id) || null;
            this.participantCardLoading = false;
            this.participantCardError = '';
            this.selectedParticipant = participant;
            this.showParticipantCard = true;
        },

        /**
         * Карточка участника по клику в блоке «Ответственные за согласование»
         * (#1952). У блока есть только ФИО, должность и голос - контакты, место
         * работы и остальные роли лежат в ответе про участников, поэтому его и
         * подтягиваем: один раз на заявку, дальше из памяти.
         * @param {{id: number}} user строка responsible_users
         */
        async openParticipantByUser(user) {
            const userId = Number(user?.id);
            this.participantCardUserId = userId;
            this.selectedParticipant = null;
            this.participantCardError = '';
            this.participantCardLoading = true;
            this.showParticipantCard = true;
            try {
                const list = await this.ensureParticipants();
                if (this.participantCardUserId !== userId) return;
                const found = (list || []).find(p => Number(p.user_id) === userId);
                if (found) {
                    this.selectedParticipant = found;
                } else {
                    this.participantCardError = 'Не нашли этого человека среди получателей заявки.';
                }
            } catch (error) {
                if (this.participantCardUserId !== userId) return;
                this.participantCardError = error.message || 'Не удалось загрузить данные участника';
            } finally {
                if (this.participantCardUserId === userId) this.participantCardLoading = false;
            }
        },

        /**
         * Список участников заявки: из памяти, если он уже загружен для этой заявки,
         * иначе одним запросом. Пока запрос летит, второй клик присоединяется к нему -
         * дедуп безопасен, потому что заявка у обоих кликов одна и та же и проверяется
         * при записи: ответ по чужой заявке (успели переключить) не сохраняем.
         * @returns {Promise<Array>}
         */
        ensureParticipants() {
            const id = Number(this.applicationData.id);
            if (!id) return Promise.resolve([]);
            if (this.participantsLoadedFor === id) return Promise.resolve(this.participants);
            if (this.participantsInflight && this.participantsInflight.id === id) {
                return this.participantsInflight.promise;
            }

            const generation = this.participantsGeneration;
            const promise = getApplicationParticipants(id)
                .then((list) => {
                    const fresh = Array.isArray(list) ? list : [];
                    // Пока ответ летел, деталь перечитали или заявку сменили: показать
                    // его тому, кто кликнул, ещё можно, а запоминать уже нельзя -
                    // следующий клик обязан спросить заново.
                    if (generation === this.participantsGeneration && Number(this.applicationData.id) === id) {
                        this.participants = fresh;
                        this.participantsLoadedFor = id;
                    }
                    return fresh;
                })
                .finally(() => {
                    // Сверяем по самому промису: после сброса кэша в поле уже может
                    // лежать запрос следующего клика по той же заявке.
                    if (this.participantsInflight && this.participantsInflight.promise === promise) {
                        this.participantsInflight = null;
                    }
                });
            this.participantsInflight = { id, promise };
            return promise;
        },

        /**
         * Сбросить память об участниках: заявку сменили или её деталь перечитали.
         * Голоса согласующих меняются вместе с деталью, и карточка обязана
         * показывать то же, что блок согласования за ней.
         */
        resetParticipantsCache() {
            this.participantsGeneration += 1;
            this.participants = [];
            this.participantsLoadedFor = null;
            this.participantsInflight = null;
        },

        /**
         * Дополнение принято (#1685): перечитываем карточку, чтобы новые строки появились
         * в составе вложения, а признак открытого раунда (open_supplement) обновился и
         * погасил кнопку. Ленту истории двигаем тем же заходом - раунд пишется в неё.
         */
        onSupplementSubmitted() {
            this.showSupplementModal = false;
            this.loadApplicationDetails(this.applicationData, { preserveSelection: true });
            if (this.$refs.historyComponent && this.$refs.historyComponent.loadHistory) {
                this.$refs.historyComponent.loadHistory();
            }
            this.$emit('application-changed', this.applicationData);
        },

        getStatusBadgeClass(status) {
            const classes = {
                'В работе': 'status-mini-work',
                'Отказано': 'status-mini-rejected',
                'Завершено': 'status-mini-completed',
                'Отозвана': 'status-mini-rejected'
            };
            return classes[status] || '';
        },

        saveCommentToLocalStorage() {
            if (this.storageKey && this.currentUserId) {
                localStorage.setItem(this.storageKey, this.actionComment);
            }
        },

        loadCommentFromLocalStorage() {
            if (this.storageKey && this.currentUserId) {
                const savedComment = localStorage.getItem(this.storageKey);
                if (savedComment) {
                    this.actionComment = savedComment;
                    this.lastUserComment = savedComment;
                }
            }
        },

        clearCommentFromLocalStorage() {
            if (this.storageKey) {
                localStorage.removeItem(this.storageKey);
            }
        },

        // Заметка сохранена или снята: кладём ответ метода в карточку, не перечитывая
        // деталь. Полный рефетч сбросил бы выбранное вложение и мигнул кнопками ради
        // одного поля, которое сервер только что вернул.
        onBureauNoteUpdate(note) {
            this.applicationData = { ...this.applicationData, bureau_note: note };
        },

        async loadApplicationDetails(application, { preserveSelection = false } = {}) {
            const seq = ++this.loadDetailSeq;
            // На фоновом live-рефетче (не наше действие) кнопки не гасим - иначе мигание.
            if (!preserveSelection) this.actionsReady = false;
            try {
                const [appResponse, attachmentsResponse, viewersResponse] = await Promise.all([
                    apiRequest(`/applications/${application.id}/details`, {
                        method: "GET",
                    }),
                    apiRequest(`/applications/${application.id}/attachments`, {
                        method: "GET",
                    }),
                    apiRequest(`/applications/${application.id}/viewers`, {
                        method: "GET",
                    })
                ]);

                if (appResponse.ok) {
                    const appData = await appResponse.json();
                    // seq-guard (#632/#840): при подряд идущих live-сигналах устаревший
                    // ответ не должен затирать данные более свежего рефетча.
                    if (seq !== this.loadDetailSeq) return;

                    this.applicationData = {
                        ...this.applicationData,
                        ...appData
                    };

                    // Деталь перечитана - вместе с ней могли смениться голоса
                    // согласующих, поэтому карточка участника берёт список заново
                    // при следующем открытии (#1952).
                    this.resetParticipantsCache();

                    // Раунды дополнения (#1685) - отдельной ручкой, признак их наличия
                    // приезжает только что вместе с деталью. Без await: у панели свой
                    // лоадер, а кнопки действий заявки её ждать не должны.
                    this.loadSupplements();

                    if (appData.responsible_users) {
                        this.responsibleUsers = appData.responsible_users.map(user => ({
                            ...user,
                            approval_status: user.approval_status || 'pending'
                        }));
                        
                        if (this.currentUserId) {
                            const currentUser = this.responsibleUsers.find(u => u.id === this.currentUserId);
                            if (currentUser && currentUser.approval_comment) {
                                this.actionComment = currentUser.approval_comment;
                                this.lastUserComment = currentUser.approval_comment;
                                this.saveCommentToLocalStorage();
                            } else {
                                this.loadCommentFromLocalStorage();
                            }
                        }
                    }
                    
                    if (this.$refs.confirmationComponent) {
                        this.$refs.confirmationComponent.$forceUpdate();
                    }
                }

                if (attachmentsResponse.ok) {
                    // При live-рефетче сохраняем выбранное вложение по id (не сбрасываем
                    // на первое) - иначе фоновое обновление перекинет чужую вкладку.
                    const prevSelectedId = preserveSelection && this.selectedAttachment ? this.selectedAttachment.id : null;
                    const newAttachments = await attachmentsResponse.json();
                    if (seq !== this.loadDetailSeq) return;
                    this.attachments = newAttachments;
                    if (this.attachments.length > 0) {
                        const keep = prevSelectedId ? this.attachments.find(a => a.id === prevSelectedId) : null;
                        this.selectedAttachment = keep || this.attachments[0];
                        await this.loadAttachmentDetails(this.selectedAttachment.id);
                    }
                }

                if (viewersResponse.ok) {
                    const newViewers = await viewersResponse.json();
                    if (seq !== this.loadDetailSeq) return;
                    this.viewers = newViewers;
                }

                // Получатели и состав принимающих нужны окну пересылки, а оно живёт
                // только в "Центре заявок". В личном кабинете кнопки пересылки нет,
                // поэтому и запросов не делаем.
                if (this.mode === 'center') {
                    await this.fetchForwardRecipients();
                    await this.fetchApprovers();
                }

                // А вот ответ про себя нужен в любом режиме и любому пользователю:
                // от него зависят кнопки принимающего, в том числе решение по дополнению.
                await this.fetchIsApprover();

            } catch (error) {
                console.error("Ошибка при загрузке деталей заявки:", error);
            } finally {
                // П.46: кнопки действий показываем только после загрузки ролей/согласующих,
                // иначе мигают не те кнопки пока responsibleUsers/approvers пустые.
                this.actionsReady = true;
            }
        },

        /**
         * Получатели для окна пересылки - из двух источников, по праву на список
         * пользователей.
         *
         * Узкий круг кандидатов (коллеги по организации и компании плюс руководители) -
         * это ограничение бэка для рядового участника заявки, а не общее правило:
         * forwardAuthority не сужает получателей ни супер-админу, ни принимающему -
         * маршрутизация заявок по чужим организациям и есть работа оператора бюро.
         * Поэтому носителю page.admin.users оставляем полный /users/all, как было, а
         * остальным даём неадминских кандидатов: на /users/all они получали 403 и
         * пустой выбор в окне.
         *
         * silent403 на обеих ветках: окно деградирует до пустого списка молча - тост
         * "Недостаточно прав" здесь лишний, запроса пользователь не делал.
         */
        async fetchForwardRecipients() {
            const path = this.can('page.admin.users') ? "/users/all" : "/users/recipient-candidates";
            try {
                const response = await apiRequest(path, { silent403: true });
                if (response.ok) {
                    this.allUsers = (await response.json()) || [];
                }
            } catch (error) {
                console.error("Error fetching forward recipients:", error);
            }
        },

        async fetchApprovers() {
            // Полный состав принимающих нужен только окну пересылки (исключить их из
            // адресатов) и доступен администратору. Обычный пользователь получает 403,
            // и это нормально - на его собственные кнопки состав не влияет.
            try {
                const response = await apiRequest("/application-approvers", { silent403: true });
                if (response.ok) {
                    this.approvers = await response.json();
                }
            } catch (error) {
                console.error("Error fetching approvers:", error);
            }
        },

        /**
         * Ответ на вопрос «я принимающий?» - отдельным запросом про себя.
         *
         * Раньше это выводили из полного состава принимающих, но он под правом
         * администратора: принимающий без этого права получал пустой список и не видел
         * НИ ОДНОЙ своей кнопки - ни «Принять в работу», ни решения по дополнению.
         * Ошибки при этом не было нигде, 403 гасится молча (#1685).
         */
        async fetchIsApprover() {
            try {
                const response = await apiRequest("/application-approvers/me");
                if (response.ok) {
                    const body = await response.json();
                    this.isApproverSelf = !!(body && body.is_approver);
                }
            } catch (error) {
                console.error("Error checking approver role:", error);
            }
        },

        async loadAttachmentDetails(attachmentId) {
            if (!attachmentId) return;

            this.loadingAttachmentDetails = true;
            try {
                this.attachmentCars = [];
                this.attachmentEmployees = [];
                this.attachmentItems = [];

                const attachment = this.attachments.find(a => a.id === attachmentId);
                if (!attachment) return;

                switch (attachment.attachment_type) {
                    case 'cars': {
                        const carsResponse = await apiRequest(`/attachments/${attachmentId}/cars`, {
                            method: "GET",
                        });
                        if (carsResponse.ok) {
                            this.attachmentCars = await carsResponse.json();
                        }
                        break;
                    }
                    
                    case 'people': {
                        const employeesResponse = await apiRequest(`/attachments/${attachmentId}/employees`, {
                            method: "GET",
                        });
                        if (employeesResponse.ok) {
                            this.attachmentEmployees = await employeesResponse.json();
                        }
                        break;
                    }
                    
                    case 'items': {
                        const itemsResponse = await apiRequest(`/attachments/${attachmentId}/items`, {
                            method: "GET",
                        });
                        if (itemsResponse.ok) {
                            this.attachmentItems = await itemsResponse.json();
                        }
                        break;
                    }
                }
            } catch (error) {
                console.error("Ошибка при загрузке деталей вложения:", error);
            } finally {
                this.loadingAttachmentDetails = false;
            }
        },

        selectAttachment(attachment) {
            this.selectedAttachment = attachment;
            this.loadAttachmentDetails(attachment.id);
        },

        handleApplicationUpdate() {
            this.loadApplicationDetails(this.applicationData);
        },

        forwardApplication() {
            this.showForwardModal = true;
        },

        closeForwardModal() {
            this.showForwardModal = false;
        },

        async sendForwardRequest({ users = [], attachment_ids = [], message = '' } = {}) {
            if (users.length === 0) return;

            this.isForwarding = true;
            try {
                const usersToSend = users.map(user => ({
                    user_id: user.user_id,
                    required_approval: user.required_approval || false,
                    can_view: user.can_view !== undefined ? user.can_view : !user.required_approval
                }));

                const response = await apiRequest(`/applications/${this.applicationData.id}/forward`, {
                    method: "POST",
                    body: JSON.stringify({
                        users: usersToSend,
                        attachment_ids,
                        message
                    })
                });

                if (response.ok) {
                    useDeletionsStore().notify({ bold: 'Заявка переслана', type: 'success' });
                    this.closeForwardModal();

                    await this.loadApplicationDetails(this.applicationData);

                    if (this.$refs.historyComponent) {
                        this.$refs.historyComponent.loadHistory();
                    }

                    if (this.$refs.forwardMessagesComponent) {
                        this.$refs.forwardMessagesComponent.load();
                    }

                    this.$emit('application-changed', this.applicationData);

                } else {
                    const errorText = await response.text();
                    useDeletionsStore().notify({ prefix: 'Не удалось переслать: ', bold: errorText || 'ошибка', type: 'error' });
                }
            } catch (error) {
                console.error("Ошибка при пересылке заявки:", error);
                useDeletionsStore().notify({ prefix: 'Не удалось переслать: ', bold: 'ошибка сети', type: 'error' });
            } finally {
                this.isForwarding = false;
            }
        },

        async duplicateApplication(preset = 'other') {
            try {
                const fetchResults = await Promise.all(
                    this.attachments.map(attachment => {
                        const endpoint = {
                            cars: `/attachments/${attachment.id}/cars`,
                            people: `/attachments/${attachment.id}/employees`,
                            items: `/attachments/${attachment.id}/items`,
                        }[attachment.attachment_type];

                        if (!endpoint) return Promise.resolve({ attachment, data: [] });

                        return apiRequest(endpoint, { method: 'GET' })
                            .then(r => r.ok ? r.json() : [])
                            .then(data => ({ attachment, data }));
                    })
                );

                // Шаблоны вложений нужны для поля title (категория): BlankSelector
                // раскладывает бланки по категориям именно по attachment.title, и без
                // него восстановленный черновик не отображается (0/0 в категориях).
                const templatesResp = await apiRequest('/attachments', { method: 'GET' });
                const templates = templatesResp.ok ? await templatesResp.json() : [];

                const newAttachments = [];
                const vehiclesByAttachment = {};
                const employeesByAttachment = {};
                const itemsByAttachment = {};
                const attachmentDatesByAttachment = {};
                // Дата действия по выбранному пресету - одна на все вложения ('other' -> null, без даты).
                const presetDate = this.buildPresetDate(preset);

                for (const { attachment, data } of fetchResults) {
                    // local_id как в BlankSelector.addAttachment — числовой ключ без id существующего вложения
                    const localId = Date.now() + Math.random();
                    const template = templates.find(t => t.id === attachment.unique_attachment_id)
                        || templates.find(t => t.attachment_type === attachment.attachment_type);

                    newAttachments.push({
                        id: template ? template.id : attachment.unique_attachment_id,
                        local_id: localId,
                        template_id: template ? template.id : attachment.unique_attachment_id,
                        title: template ? template.title : null,
                        name: template ? template.name : attachment.attachment_name,
                        display_name: attachment.attachment_display_name || (template && template.display_name),
                        attachment_type: attachment.attachment_type,
                        instruction: template ? template.instruction : null,
                        created_at: new Date().toISOString(),
                        is_active: true,
                    });

                    // Пресет проставляет дату действия во все вложения; время копируем из
                    // исходного вложения (обрезаем секунды: "12:00:00" -> "12:00").
                    if (presetDate) {
                        attachmentDatesByAttachment[localId] = {
                            ...presetDate,
                            startTime: (attachment.entry_time_from || '').slice(0, 5),
                            endTime: (attachment.entry_time_to || '').slice(0, 5),
                            roofAccess: false,
                            freeParking: false,
                            errors: {},
                        };
                    }

                    if (attachment.attachment_type === 'cars') {
                        vehiclesByAttachment[localId] = data.map((car, idx) => ({
                            id: idx + 1,
                            plateNumber: car.car_number,
                            mark: car.car_brand,
                            markId: null,
                            markName: car.car_brand || null,
                            unloadingPlace: car.unload_place || '',
                            unloadPlaces: car.unload_places || [],
                            passage_tables: car.target_tables ? car.target_tables.map(t => t.id) : [],
                            formatId: null,
                            isExisting: false,
                        }));
                    } else if (attachment.attachment_type === 'people') {
                        employeesByAttachment[localId] = data.map((emp, idx) => ({
                            id: idx + 1,
                            lastName: emp.last_name,
                            firstName: emp.first_name,
                            middleName: emp.middle_name || '',
                            position: emp.position || '',
                            citizenshipId: emp.citizenship_id || null,
                            citizenshipName: emp.citizenship_name || '',
                            passportSeriesNumber: emp.passport_series_number || '',
                            patentNumber: emp.patent_number || null,
                            otherPermission: emp.other_permission || null,
                            targetTables: emp.target_tables || [],
                            passageTables: '',
                            isExisting: false,
                        }));
                    } else if (attachment.attachment_type === 'items') {
                        itemsByAttachment[localId] = data.map((item, idx) => ({
                            id: idx + 1,
                            itemName: item.name,
                            quantity: item.count,
                        }));
                    }
                }

                const draftState = {
                    message: this.applicationData.message || '',
                    attachments: newAttachments,
                    vehiclesByAttachment,
                    employeesByAttachment,
                    itemsByAttachment,
                    // Пресет проставил дату действия ('other' -> пусто, пользователь задаёт сам).
                    attachmentDatesByAttachment,
                    customFieldsByAttachment: {},
                    consentGiven: false,
                    vehicleIdCounter: 1,
                    employeeIdCounter: 1,
                    itemIdCounter: 1,
                };

                // Пишем во временный ключ, а НЕ в draftApplicationState: на странице
                // оформления может быть уже начатый черновик - CreateApplication сам решит
                // (заменить/объединить/отмена), забирать ли этот дубль (#952).
                // ownerId - чтобы дубль не достался тому, кто войдёт в этом браузере
                // следующим: хранилище одно на устройство, а учётные записи сменяются.
                localStorage.setItem('pendingDuplicateState', JSON.stringify({
                    ...draftState,
                    ownerId: useAuthStore().userPayload?.user_id ?? null,
                }));
                this.$emit('duplicate');
            } catch (error) {
                console.error('Ошибка при дублировании заявки:', error);
                useDeletionsStore().notify({ prefix: 'Не удалось продублировать заявку: ', bold: 'ошибка', type: 'error' });
            }
        },

        // Дропдаун "Продублировать": ключ пресета -> дублирование с проставленной датой.
        handleDuplicatePreset(preset) {
            this.duplicateApplication(preset);
        },

        // Дата действия для пресета срока (формат дд.мм.гггг, как ждёт форма заявки).
        // tomorrow - один день (завтра); nextMonth - весь следующий календарный месяц
        // (как пресет nextMonth в DateFilter); other/иное - null (дублируем без даты).
        buildPresetDate(preset) {
            const pad = n => String(n).padStart(2, '0');
            const fmt = d => `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()}`;
            const now = new Date();
            if (preset === 'tomorrow') {
                const t = new Date(now);
                t.setDate(now.getDate() + 1);
                return { isOneDay: true, singleDate: fmt(t), startDate: '', endDate: '' };
            }
            if (preset === 'nextMonth') {
                const start = new Date(now.getFullYear(), now.getMonth() + 1, 1);
                const end = new Date(now.getFullYear(), now.getMonth() + 2, 0);
                return { isOneDay: false, singleDate: '', startDate: fmt(start), endDate: fmt(end) };
            }
            return null;
        },

        async withdrawApplication() {
            const ok = await useUiStore().confirm({
                title: 'Отозвать заявку?',
                message: 'При отзыве все машины, люди и вложения в заявке станут неактивны, и заявка перестанет действовать - охрана не пропустит. Вернуть заявку в работу нельзя; можно только продублировать её для повторного согласования.',
                confirmText: 'Отозвать',
                cancelText: 'Отмена',
                danger: true,
            });
            if (!ok) return;

            try {
                const response = await apiRequest(`/applications/${this.applicationData.id}/withdraw`, { method: 'POST' });
                if (response.ok) {
                    useDeletionsStore().notify({ bold: 'Заявка отозвана', type: 'success' });
                    this.$emit('withdraw');
                    this.close();
                } else {
                    const data = await response.json().catch(() => ({}));
                    useDeletionsStore().notify({ prefix: 'Не удалось отозвать заявку: ', bold: data.message || 'ошибка', type: 'error' });
                }
            } catch {
                useDeletionsStore().notify({ prefix: 'Не удалось отозвать заявку: ', bold: 'ошибка сети', type: 'error' });
            }
        },

        formatDateTime(dateTimeString) {
            if (!dateTimeString) return '';
            const date = new Date(dateTimeString);
            return date.toLocaleString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric',
                hour: '2-digit',
                minute: '2-digit'
            });
        },

        weekdayName,

        getUserDisplayName(user) {
            const names = [user.last_name, user.first_name, user.middle_name].filter(Boolean);
            return names.length > 0 ? names.join(' ') : user.username;
        },

        toggleLeftColumn() {
            this.isLeftColumnCollapsed = !this.isLeftColumnCollapsed;
        },

        closeApplicationDetail() {
            this.close();
        },

        close() {
            // На мобилке лист уезжает вниз тем же путём, что и свайп: dismissSheet ведёт
            // жест до конца и сам эмитит close. На десктопе гасим флаг - закрытие
            // проигрывает transition панели, а родителю сообщаем после leave.
            // matchMedia есть не везде (в jsdom его нет вовсе), а закрытие обязано
            // работать всегда: без проверки метод падал молча, и панель не закрывалась.
            const isSheet = typeof window !== 'undefined'
                && typeof window.matchMedia === 'function'
                && window.matchMedia('(max-width: 768px)').matches;
            if (isSheet) {
                this.dismissSheet();
                return;
            }
            if (this.closeTimer) return;
            this.visible = false;
            this.closeTimer = setTimeout(() => {
                this.closeTimer = null;
                this.$emit('close');
            }, DETAIL_CLOSE_MS);
        },

        /**
         * Открыть сообщение в окне по тапу на превью. Клик по ссылке внутри сообщения
         * не перехватываем - иначе переход по ссылке и модалка сработали бы разом.
         */
        openMessageFromPreview(event) {
            if (event?.target?.closest?.('a')) return;
            this.showMessageModal = true;
        },

        async loadCommonData() {
            try {
                const [placesRes, formatsRes, tablesRes] = await Promise.all([
                    apiRequest("/unload-places", {}),
                    apiRequest("/license-plate-formats", {}),
                    apiRequest("/system-tables", {})
                ]);

                if (placesRes.ok) {
                    this.allUnloadingPlaces = await placesRes.json();
                }
                if (formatsRes.ok) {
                    this.licensePlateFormats = await formatsRes.json();
                }
                if (tablesRes.ok) {
                    this.allTables = await tablesRes.json();
                }
            } catch (error) {
                console.error("Ошибка при загрузке общих данных:", error);
            }
        },

        openVehicleModal(car) {
            this.selectedVehicle = {
                id: car.id,
                plateNumber: car.car_number,
                mark: car.car_brand,
                formatId: car.formatId || null,
                organization: car.organization || null,
                organizationId: car.organization_id || null,
                company: car.company || null,
                companyId: car.company_id || null,
                isExisting: true,
                unloadPlaces: car.unload_places ? car.unload_places.map(p => p.id) : [],
                target_tables: car.target_tables ? car.target_tables.map(t => t.id) : [],
                entry_date_to: car.entry_date_to || null,
                entry_time_from: car.entry_time_from || null,
                entry_time_to: car.entry_time_to || null,
                applicationId: this.applicationData.id,
                territory_status: 0,
                entry_checked: false,
                exit_checked: false,
                blacklist_similar: car.blacklist_similar || null
            };
            this.showVehicleModal = true;
        },

        openEmployeeModal(employee) {
            this.selectedEmployee = {
                id: employee.id,
                last_name: employee.last_name,
                first_name: employee.first_name,
                middle_name: employee.middle_name,
                position: employee.position,
                citizenshipName: employee.citizenship_name,
                passport_series_number: employee.passport_series_number,
                patent_number: employee.patent_number,
                other_permission: employee.other_permission,
                organization: employee.organization || null,
                organizationId: employee.organization_id || null,
                company: employee.company || null,
                companyId: employee.company_id || null,
                entry_date_to: employee.entry_date_to || null,
                pass_time: employee.pass_time || null,
                target_tables: employee.target_tables ? employee.target_tables.map(t => t.id) : [],
                applicationId: this.applicationData.id,
                territory_status: 0,
                blacklist_similar: employee.blacklist_similar || null
            };
            this.showEmployeeModal = true;
        },

        openRemovalModal({ label, id }) {
            this.removalLabel = label || '';
            this.removalElementId = id || null;
            this.showRemovalModal = true;
        },

        async confirmRemoval(reason) {
            if (!this.removalElementId || !this.selectedAttachment) return;
            const elementType = this.selectedAttachment.attachment_type === 'people' ? 'people' : 'cars';
            this.removalSubmitting = true;
            try {
                await removeApplicationElements(this.applicationData.id, {
                    elementType,
                    elementIds: [this.removalElementId],
                    reason
                });
                useDeletionsStore().notify({
                    prefix: 'Убрано из заявки: ',
                    bold: this.removalLabel || 'элемент',
                    type: 'success'
                });
                this.showRemovalModal = false;
                await Promise.all([
                    this.loadAttachmentDetails(this.selectedAttachment.id),
                    this.refreshApplicationGate()
                ]);
                this.$emit('application-changed', this.applicationData);
            } catch (error) {
                useDeletionsStore().notify({
                    prefix: 'Не удалось убрать из заявки: ',
                    bold: error.message || 'ошибка',
                    type: 'error'
                });
            } finally {
                this.removalSubmitting = false;
            }
        },

        openOverrideModal({ label, flag }) {
            this.overrideLabel = label || '';
            this.overrideFlag = flag || null;
            this.showOverrideModal = true;
        },

        async confirmOverride(comment) {
            if (!this.overrideFlag) return;
            this.overrideSubmitting = true;
            try {
                const response = await apiRequest(`/applications/${this.applicationData.id}/blacklist-overrides`, {
                    method: 'POST',
                    body: JSON.stringify({ flag_id: this.overrideFlag.flag_id, comment })
                });
                if (response.ok) {
                    useDeletionsStore().notify({
                        prefix: 'Пропуск подтверждён: ',
                        bold: this.overrideLabel || 'элемент',
                        type: 'success'
                    });
                    this.showOverrideModal = false;
                    await Promise.all([
                        this.selectedAttachment ? this.loadAttachmentDetails(this.selectedAttachment.id) : Promise.resolve(),
                        this.refreshApplicationGate()
                    ]);
                    this.syncSelectedDetailFlags();
                    this.$emit('application-changed', this.applicationData);
                } else if (!response.ok) {
                    const data = await response.json();
                    useDeletionsStore().notify({
                        prefix: 'Не удалось подтвердить пропуск: ',
                        bold: data.message || 'ошибка',
                        type: 'error'
                    });
                }
            } catch (error) {
                console.error('Ошибка при подтверждении пропуска:', error);
                useDeletionsStore().notify({
                    prefix: 'Не удалось подтвердить пропуск: ',
                    bold: 'ошибка сети',
                    type: 'error'
                });
            } finally {
                this.overrideSubmitting = false;
            }
        },

        // После override обновляем только заявочные поля (в т.ч. гейт
        // has_unoverridden_blacklist_flags), не сбрасывая выбранное вложение.
        async refreshApplicationGate() {
            try {
                const response = await apiRequest(`/applications/${this.applicationData.id}/details`, { method: 'GET' });
                if (response.ok) {
                    const appData = await response.json();
                    this.applicationData = { ...this.applicationData, ...appData };
                    return;
                }
            } catch (error) {
                console.error('Ошибка при обновлении состояния заявки:', error);
            }
            // Override прошёл, но детали не перечитались - кнопка согласования может
            // остаться заблокированной. Сообщаем, чтобы пользователь обновил вручную.
            useDeletionsStore().notify({
                prefix: 'Пропуск подтверждён, ',
                bold: 'обновите страницу для согласования',
                type: 'error'
            });
        },

        // Открытая карточка детали держит свою копию flag - после override/отмены
        // переносим в неё свежий blacklist_similar из перечитанного вложения, чтобы блок
        // "Подозрение на обход ЧС" сразу показал актуальный статус без закрытия карточки.
        syncSelectedDetailFlags() {
            if (this.showVehicleModal && this.selectedVehicle) {
                const fresh = this.attachmentCars.find(c => c.id === this.selectedVehicle.id);
                if (fresh) this.selectedVehicle.blacklist_similar = fresh.blacklist_similar || null;
            }
            if (this.showEmployeeModal && this.selectedEmployee) {
                const fresh = this.attachmentEmployees.find(emp => emp.id === this.selectedEmployee.id);
                if (fresh) this.selectedEmployee.blacklist_similar = fresh.blacklist_similar || null;
            }
        },

        // "Всё равно пропустить" из карточки детали - переиспользуем POST-флоу с причиной.
        onCardOverride(kind) {
            const entity = kind === 'vehicle' ? this.selectedVehicle : this.selectedEmployee;
            if (!entity || !entity.blacklist_similar) return;
            const label = kind === 'vehicle'
                ? (entity.plateNumber || 'Т/С')
                : [entity.last_name, entity.first_name, entity.middle_name].filter(Boolean).join(' ').trim() || 'Сотрудник';
            this.openOverrideModal({ label, flag: entity.blacklist_similar });
        },

        // "Отменить" подтверждение пропуска из карточки - подтверждение БЕЗ причины, затем DELETE.
        async onCardCancelOverride(kind) {
            const entity = kind === 'vehicle' ? this.selectedVehicle : this.selectedEmployee;
            const flag = entity && entity.blacklist_similar;
            if (!flag || !flag.flag_id) return;
            const label = kind === 'vehicle'
                ? (entity.plateNumber || 'Т/С')
                : [entity.last_name, entity.first_name, entity.middle_name].filter(Boolean).join(' ').trim() || 'Сотрудник';

            const ok = await useUiStore().confirm({
                title: 'Снять подтверждение пропуска?',
                message: 'Подтверждение пропуска будет снято, и согласование заявки снова заблокируется по этому элементу.',
                confirmText: 'Снять',
                cancelText: 'Отмена',
                danger: true
            });
            if (!ok) return;

            try {
                const response = await apiRequest(
                    `/applications/${this.applicationData.id}/blacklist-overrides?flag_id=${flag.flag_id}`,
                    { method: 'DELETE' }
                );
                if (response.ok) {
                    useDeletionsStore().notify({
                        prefix: 'Подтверждение пропуска снято: ',
                        bold: label,
                        type: 'success'
                    });
                    await Promise.all([
                        this.selectedAttachment ? this.loadAttachmentDetails(this.selectedAttachment.id) : Promise.resolve(),
                        this.refreshApplicationGate()
                    ]);
                    this.syncSelectedDetailFlags();
                    this.$emit('application-changed', this.applicationData);
                } else if (!response.ok) {
                    const data = await response.json();
                    useDeletionsStore().notify({
                        prefix: 'Не удалось снять подтверждение: ',
                        bold: data.message || 'ошибка',
                        type: 'error'
                    });
                }
            } catch (error) {
                console.error('Ошибка при отмене подтверждения пропуска:', error);
                useDeletionsStore().notify({
                    prefix: 'Не удалось снять подтверждение: ',
                    bold: 'ошибка сети',
                    type: 'error'
                });
            }
        }
    }
}
</script>

<style scoped>
/* Стили остаются без изменений, как в вашем коде */
.application-status-section {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    padding: 15px;
    margin-bottom: 10px;
    box-shadow: 0 2px 12px var(--shadow-drop);
    /* Зазор держит gap, а не margin-bottom заголовка: при отозванной заявке и без
       принявшего блок под заголовком пуст, и margin оставался пустотой снизу (#1587).
       Пустую обёртку убираем из раскладки - скрытый элемент gap не считает. */
    display: flex;
    flex-direction: column;
    gap: 15px;
}

.application-status-section > *:empty {
    display: none;
}

.status-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.status-header h4 {
    font-size: 18px;
    color: var(--accent-text);
    font-weight: 700;
    margin: 0;
}

.status-mini-badge {
    padding: 4px 12px;
    border-radius: 20px;
    font-size: 12px;
    font-weight: 500;
    display: inline-block;
    border: 1px solid;
}

.status-mini-work {
    background-color: color-mix(in srgb, var(--accent) 10%, var(--surface));
    color: var(--accent-text);
    border-color: rgba(79, 91, 223, 0.3);
}

.status-mini-rejected {
    background-color: color-mix(in srgb, var(--danger) 10%, var(--surface));
    color: var(--danger-text);
    border-color: rgba(220, 38, 38, 0.3);
}

.status-mini-completed {
    background-color: color-mix(in srgb, var(--success) 10%, var(--surface));
    color: var(--success-text);
    border-color: color-mix(in srgb, var(--success) 45%, transparent);
}

.status-info {
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.status-info-row {
    display: flex;
    justify-content: space-between;
    padding: 4px 0;
}

.status-info-row.comment-row {
    align-items: flex-start;
    flex-direction: column;
    gap: 5px;
}

.status-info-label {
    color: var(--text-muted);
    font-size: 14px;
    font-weight: 400;
    min-width: 120px;
}

.status-info-value {
    color: var(--text);
    font-size: 15px;
    font-weight: 400;
    text-align: end;
    flex: 1;
}

.status-info-value.comment-text {
    font-weight: 400;
    white-space: pre-wrap;
    word-break: break-word;
    line-height: 1.5;
    font-size: 13px;
    text-align: start;
    color: var(--text);
    
}

.comment-action-section {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    padding: 15px;
    margin-bottom: 10px;
    box-shadow: 0 2px 12px var(--shadow-drop);
}

.comment-action-section h4 {
    font-size: 18px;
    color: var(--accent-text);
    font-weight: 700;
    margin-bottom: 10px;
}

.comment-action-textarea {
    width: 100%;
    padding: 12px;
    border: 1px solid var(--border);
    border-radius: 15px;
    font-size: 14px;
    font-family: inherit;
    resize: none;
    transition: all 0.2s ease;
    background-color: var(--surface);
}

.comment-action-textarea:focus {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.1);
}

/* Закрытие панели: затемнение гаснет, карточка чуть уходит вниз и тает. Только
   opacity и transform - как у остальных окон проекта. На мобилке лист уезжает вниз
   целиком, повторяя жест свайпа. */
.detail-close-leave-active {
    transition: opacity 0.2s ease;
}

.detail-close-leave-active .application-detail {
    transition: transform 0.2s ease, opacity 0.2s ease;
}

.detail-close-leave-to {
    opacity: 0;
}

.detail-close-leave-to .application-detail {
    opacity: 0;
    transform: translateY(8px);
}

@media (max-width: 768px) {
    .detail-close-leave-to .application-detail {
        transform: translateY(100%);
        opacity: 1;
    }
}

@media (prefers-reduced-motion: reduce) {
    .detail-close-leave-active,
    .detail-close-leave-active .application-detail {
        transition: none;
    }
}

/* Остальные стили остаются без изменений */
.application-detail-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: var(--overlay);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 10002;
    backdrop-filter: blur(0.1px);
    -webkit-backdrop-filter: blur(0.1px);
    animation: fadeIn 0.2s ease-out;
}

@keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
}

/* Плавные переходы блоков детали при смене статуса (эталон ApplicationHistory.vue).
   opacity+translateY, чтобы не резать box-shadow карточек overflow-контейнером. */
.fade-enter-active,
.fade-leave-active {
    transition: opacity 0.2s ease, transform 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
    opacity: 0;
    transform: translateY(-10px);
}

.application-detail {
    background: var(--surface);
    border-radius: 30px;
    width: 1600px;
    max-width: 95%;
    /* zoom-safe (#1097): 90vh под корневым zoom (viewportScale) считается от
       НЕзумленной высоты -> модалка выше зумленного вьюпорта (900px при 2560),
       верх/низ уезжают за экран + пустота снизу. --app-vh нормирован на zoom. */
    height: calc(var(--app-vh, 1vh) * 90);
    display: flex;
    flex-direction: column;
    box-shadow: 0 4px 20px var(--shadow-drop);
    overflow: hidden;
}

.detail-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 15px 20px;
    border-bottom: 1px solid var(--border);
    background: var(--surface-sunken);
    min-height: 40px;
}

.detail-header-left {
    display: flex;
    flex-direction: column;
    gap: 5px;
    flex: 1;
}

.detail-title-row {
    display: flex;
    align-items: center;
    gap: 20px;
    flex-wrap: wrap;
    /* Контейнерный запрос ниже мерит именно этот ряд, а не вьюпорт: ширина
       заголовка "плавает" от длины номера заявки и от соседней правой колонки
       (бейдж дополнения/панель действий), поэтому фиксированный breakpoint по
       окну то срабатывал бы рано, то поздно. */
    container-type: inline-size;
}

.detail-title {
    font-size: 20px;
    font-weight: 700;
    color: var(--text);
    margin: 0;
    line-height: 1.2;
}

.detail-datetime {
    font-size: 15px;
    color: var(--text-muted);
    line-height: 1.2;
    font-weight: 500;
    display: flex;
    align-items: center;
    gap: 5px;
}

.weekday {
    font-size: 15px;
    color: var(--text-muted);
}

.forward-btn {
    padding: 6px 24px;
    border: none;
    border-radius: 50px;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
    min-width: 120px;
    border: 1px solid var(--border);
    background: var(--accent);
    color: var(--accent-contrast);
    margin-left: 10px;
}

.forward-btn:hover:not(:disabled) {
    background: var(--accent-hover);
}

/* Получатели (#1952) - вторичное действие рядом с "Переслать": та же пилюля и та
   же высота, но контурная, чтобы не спорить с основным действием шапки.
   Отрицательный отступ подтягивает её к "Переслать": общий gap ряда (20px, тот же,
   что между заголовком и датой) для двух смежных действий читался слишком разреженно
   (владелец: "отступ между кнопками слишком большой"). */
.participants-btn {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 6px 18px;
    border-radius: 50px;
    border: 1px solid var(--accent);
    background: var(--surface);
    color: var(--accent-text);
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    margin-left: -10px;
    transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease, margin 0.15s ease;
}

.participants-btn:hover {
    background: var(--accent-tint);
}

.participants-btn__icon {
    flex-shrink: 0;
}

/* Ряд заголовка тесен уже на десктопе (не только на мобилке): полная пилюля
   "Получатели" - самый широкий необязательный элемент ряда, и когда соседняя
   правая колонка шапки разрастается (бейдж дополнения + панель действий),
   именно она первой не помещается и переносится одна, оторванно от даты и
   "Переслать" (замерено в браузере - разрыв на 1440/1100 при 1920 и 1280 в
   порядке). Сжимаем её в такой же кружок с иконкой, что и на мобилке (см.
   @media 768px ниже), но по контейнеру самого ряда - тогда сжатие срабатывает
   ровно там, где не хватает места, а не по случайной ширине окна. */
@container (max-width: 1040px) {
    .participants-btn {
        width: 30px;
        height: 30px;
        min-width: 30px;
        padding: 0;
        gap: 0;
        /* Тут "Получатели" уже кружок с иконкой, а не пилюля с подписью - полные -10px
           пилюльного режима смотрелись слишком тесно рядом с "Переслать". Было -6px
           (гэп 14px) - владелец попросил ещё плотнее и на 1440, и на 1920 (ряд шапки
           там сжат до кружка тем же контейнерным запросом, ширина ряда своя, не от
           вьюпорта). */
        margin-left: -12px;
        border-radius: 50%;
        justify-content: center;
    }

    .participants-btn__text {
        display: none;
    }
}

.detail-header-right {
    display: flex;
    align-items: center;
    gap: 15px;
    /* Правая часть шапки - сама элемент flex-раскладки, а такой по умолчанию не
       сжимается уже содержимого (min-width: auto) и вылезает за карточку вместо
       переноса. С появлением ряда действий по дополнению (#1685) содержимого стало
       больше, и на промежуточных ширинах, около 780, кнопка уезжала за край окна:
       карточка кончалась на 761, а кнопка шла до 788. Замерено в браузере. */
    min-width: 0;
    /* nowrap - перенос содержимого отдан вложенной .detail-header-actions (см.
       ниже); если бы переносился этот уровень целиком, крестик мог уйти на
       отдельную СВОЮ строку и всё равно уехать вниз - именно так и было, замерено
       на ширине 900: closeTop 159.6 при cardTop 45. */
    flex-wrap: nowrap;
    justify-content: flex-end;
    /* .detail-header центрирует своих детей по высоте (align-items: center) - без
       этого правая часть, чья высота гуляет от переноса ряда дополнения (#1685) и
       собственного flex-wrap, всплывала бы вверх-вниз вместе с левой колонкой.
       Прижимаем к верху шапки: замерено, closeTop гулял 93.8-201.8px на разных
       ширинах при неизменном cardTop=45. */
    align-self: flex-start;
}

.detail-header-actions {
    display: flex;
    align-items: center;
    gap: 15px;
    row-gap: 8px;
    flex-wrap: wrap;
    justify-content: flex-end;
    /* Сжимается и переносится сама - крестику-соседу больше некуда уезжать: он не
       участвует в этом переносе. БЕЗ flex-grow (было flex: 1 1 auto) - растущая
       обёртка занимала всю ширину шапки до крестика, и на переносе строки бейдж/
       кнопки позиционировались justify-content'ом ОТНОСИТЕЛЬНО ЭТОЙ ШИРОКОЙ рамки,
       а не своего содержимого - на 390 бейдж повисал с 98px пустоты слева
       (владелец: "зачем отцентровал"). Без роста блок хугает контент, как и до
       обёртки. */
    flex-shrink: 1;
    min-width: 0;
}

.supplement-round-badge {
    flex-shrink: 0;
}

.forward-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

.button-loading {
    display: inline-block;
    width: 16px;
    height: 16px;
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-radius: 50%;
    border-top-color: var(--surface);
    animation: spin 0.8s linear infinite;
}

@keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
}

.close-detail-btn {
    background: none;
    border: none;
    font-size: 24px;
    color: var(--text-muted);
    cursor: pointer;
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    transition: all 0.2s ease;
    /* Ряд дополнения (#1685) сделал ApplicationActionBar двухрядным - крестик
       без своего align-self центрировался по высоте самого высокого соседа и
       уезжал вниз на 30-100px в зависимости от ширины окна. Прижимаем к верху
       шапки независимо от высоты соседей. Замерено в браузере. */
    align-self: flex-start;
    flex-shrink: 0;
}

.close-detail-btn:hover {
    background: var(--border);
    color: var(--text);
}

.detail-content {
    display: flex;
    flex: 1;
    overflow: hidden;
}

.detail-left-column {
    /* 225, а не 240: 15px отданы таблице состава - в ней не помещалось
       «Дебаркадер №1» в колонке мест разгрузки. */
    width: 225px;
    border-right: 1px solid var(--border);
    overflow-y: auto;
    background: var(--surface-sunken);
    padding: 15px;
    transition: width 0.3s ease;
}

.detail-left-column.collapsed {
    width: 85px;
    padding: 10px;
}

.detail-main-column {
    flex: 1;
    padding: 15px;
    /* Подложка, как у боковых колонок: без неё секции ложатся прямо на модалку,
       совпадают с ней цветом и читаются одним полотном, тогда как слева и справа
       такие же секции выглядят приподнятыми (#1581). */
    background: var(--surface-sunken);
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 10px;
}

/* Блоки колонки держат свою высоту, а колонка скроллится. Без этого flex-column
   сжимает дочерние блоки (у части overflow:hidden) и контент режется вместо скролла. */
/* Обёртки веток и вопросов рендерятся всегда, а их содержимое - по данным: пустой
   список пересылок оставлял div высотой 0, и gap колонки считал его за блок, давая
   лишний зазор между сообщением и вопросами (#1587). */
.detail-main-column > *:empty {
    display: none;
}

.detail-main-column > * {
    flex-shrink: 0;
}

.detail-right-column {
    width: 360px;
    border-left: 1px solid var(--border);
    overflow-y: auto;
    padding: 15px;
    background: var(--surface-sunken);
}

.message-section {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    padding: 15px;
    box-shadow: 0 2px 12px var(--shadow-drop);
    overflow: hidden;
}

.message-section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    margin: -15px -15px 12px;
    padding: 12px 15px;
    border-bottom: 1px solid var(--border);
}

.message-section h4 {
    margin: 0;
    font-size: 14px;
    color: var(--text-muted);
    font-weight: 400;
}

/* Хинт-аффорданс: по тапу на превью открывается полное сообщение в окне (W3.10). */
.message-open-hint {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    margin-top: 8px;
    padding: 0;
    border: none;
    background: none;
    color: var(--text-muted);
    font-size: 13px;
    cursor: pointer;
    transition: color 0.15s ease;
}

.message-open-hint:hover {
    color: var(--accent-text);
}

.message-content {
    font-size: 15px;
    line-height: 150%;
    color: var(--text);
}

.message-empty {
    white-space: pre-wrap;
    color: var(--text-muted);
}

.message-preview {
    position: relative;
    max-height: 150px;
    overflow: hidden;
    word-break: break-word;
    cursor: pointer;
}

.message-preview.is-clamped::after {
    content: "";
    position: absolute;
    left: 0;
    right: 0;
    bottom: 0;
    height: 46px;
    background: linear-gradient(to bottom, rgba(255, 255, 255, 0), var(--surface));
    pointer-events: none;
}

.message-preview :deep(img) {
    max-width: 100%;
    height: auto;
}

.message-preview :deep(.constructor-image.img-align-left) { float: left; margin: 0 14px 10px 0; }
.message-preview :deep(.constructor-image.img-align-right) { float: right; margin: 0 0 10px 14px; }
.message-preview :deep(.constructor-image.img-align-center) { display: block; margin: 10px auto; float: none; }

.message-preview :deep(p) { margin: 4px 0; }

.message-preview :deep(ul),
.message-preview :deep(ol) {
    padding-left: 22px;
    margin: 6px 0;
}

.message-preview :deep(strong) { font-weight: 700; }
.message-preview :deep(em) { font-style: italic; }
.message-preview :deep(u) { text-decoration: underline; }

.message-preview :deep(h1),
.message-preview :deep(.heading-h1) { font-size: 20px; font-weight: 700; margin: 6px 0; }

.message-preview :deep(h2),
.message-preview :deep(.heading-h2) { font-size: 17px; font-weight: 600; margin: 6px 0; }

.message-preview :deep(.black-text) { color: #000 !important; }
.message-preview :deep(.red-text) { color: #FF0000 !important; }
.message-preview :deep(.green-text) { color: #079D1D !important; }
.message-preview :deep(.blue-text) { color: var(--accent-text) !important; }

.message-preview :deep(.font-size-10) { font-size: 10px !important; }
.message-preview :deep(.font-size-12) { font-size: 12px !important; }
.message-preview :deep(.font-size-14) { font-size: 14px !important; }
.message-preview :deep(.font-size-16) { font-size: 16px !important; }
.message-preview :deep(.font-size-18) { font-size: 18px !important; }
.message-preview :deep(.font-size-20) { font-size: 20px !important; }

.message-preview :deep(.font-weight-300) { font-weight: 300 !important; }
.message-preview :deep(.font-weight-400) { font-weight: 400 !important; }
.message-preview :deep(.font-weight-500) { font-weight: 500 !important; }
.message-preview :deep(.font-weight-600) { font-weight: 600 !important; }
.message-preview :deep(.font-weight-900) { font-weight: 900 !important; }

.message-preview :deep(.text-align-left) { text-align: left !important; }
.message-preview :deep(.text-align-center) { text-align: center !important; }
.message-preview :deep(.text-align-right) { text-align: right !important; }

.basic-info-section {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    padding: 15px;
    margin-bottom: 10px;
    box-shadow: 0 2px 12px var(--shadow-drop);
}

.basic-info-section h4 {
    font-size: 18px;
    color: var(--accent-text);
    font-weight: 700;
    margin-bottom: 15px;
}

.info-grid {
    display: flex;
    flex-direction: column;
    gap: 15px;
}

.info-row {
    display: flex;
    flex-direction: column;
    gap: 0px;
}

.info-label {
    color: var(--text-muted);
    font-size: 14px;
    font-weight: 400;
    min-width: 140px;
    text-align: left;
}

.info-value {
    color: var(--text);
    font-size: 15px;
    text-align: left;
    flex: 1;
    font-weight: 400;
}

/* Имя отправителя и тег "Важный" в одну строку: тег - пилюль рядом с именем (как теги
   Крыша/Парковка), а не отдельной строкой под ним. */
.sender-value {
    display: inline-flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
}

.sender-important-tag {
    flex-shrink: 0;
}

/* Дропдаун "Продублировать": перекрашиваем триггер BaseDropdown в синюю primary-кнопку
   (как была прежняя .duplicate-btn), меню остаётся штатным белым. */
.duplicate-dropdown {
    min-width: 160px;
}

/* Меню шире кнопки, чтобы длинные пункты ("На следующий месяц") не обрезались. */
.duplicate-dropdown :deep(.base-dropdown__menu) {
    width: max-content;
    min-width: 100%;
}

.duplicate-dropdown :deep(.base-dropdown__button) {
    min-height: 34px;
    justify-content: center;
    gap: 8px;
    border-color: var(--accent);
    background: var(--accent);
}

.duplicate-dropdown :deep(.base-dropdown__button:hover:not(:disabled)) {
    border-color: var(--accent-hover);
    background: var(--accent-hover);
}

.duplicate-dropdown :deep(.base-dropdown__text),
.duplicate-dropdown :deep(.base-dropdown__text--placeholder) {
    color: var(--accent-contrast);
    font-weight: 600;
}

.duplicate-dropdown :deep(.base-dropdown__arrow) {
    color: var(--accent-contrast);
}

/* "Дополнить" (#1685) стоит в ряду автора рядом с "Продублировать": то же тело пилюли,
   но secondary-заливка - основное действие в ряду по-прежнему дублирование. */
.supplement-btn {
    padding: 6px 24px;
    border: 1px solid var(--accent);
    border-radius: 50px;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: background-color 0.2s ease, color 0.2s ease;
    min-width: 140px;
    background: var(--surface);
    color: var(--accent-text);
}

.supplement-btn:hover {
    background: var(--accent-tint);
}

.withdraw-btn {
    padding: 6px 24px;
    border: 1px solid var(--border);
    border-radius: 50px;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
    min-width: 140px;
    background: var(--danger);
    color: var(--fill-text);
}

.withdraw-btn:hover {
    background: color-mix(in srgb, var(--danger) 85%, var(--text));
}

/*
 * Кнопка, показанная только ради обучения: этой заявке действие не положено, но
 * тур про него рассказывает. Приглушаем и гасим наведение, чтобы её не приняли
 * за рабочую.
 */
.is-tour-stub {
    opacity: 0.5;
    cursor: default;
    pointer-events: none;
}

.revoke-btn, .restore-btn {
    padding: 6px 24px;
    border: none;
    border-radius: 50px;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
    min-width: 140px;
    border: 1px solid var(--border);
    background: var(--warning);
    color: var(--fill-text);
}

.revoke-btn:hover:not(:disabled) {
    background: var(--warning);
}

.restore-btn {
    background: var(--success);
}

.restore-btn:hover:not(:disabled) {
    background: color-mix(in srgb, var(--success) 85%, var(--text));
}

.revoke-btn:disabled,
.restore-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

.history-button-section {
    margin: 10px 0;
    display: flex;
    justify-content: flex-end;
}

/* Ползунок bottom-sheet и иконка-вариант кнопки пересылки - только на мобилке (@768). */
.sheet-handle {
    display: none;
}

.forward-btn__icon {
    display: none;
}

/* Кнопка "Скачать" в детали - только на мобилке (@768); на десктопе бланк качают
   из строки списка (W3.8), там кнопка остаётся. */
.detail-download-btn {
    display: none;
}

/* Адаптив (#1097 S6): 3 фикс-колонки не помещаются на планшете/мобиле - стекаем их
   вертикально (вложения -> детали -> статус/согласование), скролл держит весь
   .detail-content целиком вместо трёх независимых внутренних скроллов. */
@media (max-width: 1024px) {
    .detail-content {
        flex-direction: column;
        overflow-y: auto;
        overflow-x: hidden;
    }

    .detail-left-column,
    .detail-right-column {
        width: 100%;
        overflow-y: visible;
    }

    .detail-left-column {
        border-right: none;
        border-bottom: 1px solid var(--border);
    }

    .detail-left-column.collapsed {
        width: 100%;
        padding: 15px;
    }

    .detail-right-column {
        border-left: none;
        border-top: 1px solid var(--border);
    }

    .detail-main-column {
        flex: none;
        overflow-y: visible;
    }
}

@media (max-width: 768px) {
    /* Bottom-sheet: оверлей прижимает лист к низу, лист во всю ширину выезжает
       снизу; свайп вниз за ползунок закрывает (useSwipeDismiss, #1097 W3.9). */
    .application-detail-overlay {
        align-items: flex-end;
    }

    .application-detail {
        position: relative;
        width: 100%;
        max-width: 100%;
        height: 92dvh;
        max-height: 92dvh;
        border-radius: 16px 16px 0 0;
        /* Выезд снизу при появлении + snap-back после свайпа. */
        animation: detailSlideUp 0.3s ease-out;
        transition: transform 0.3s ease;
    }

    /* Пока тянем пальцем - без анимации (лист следует за пальцем 1:1). */
    .application-detail.is-dragging {
        transition: none;
    }

    .sheet-handle {
        display: block;
        width: 40px;
        height: 4px;
        margin: 10px auto 2px;
        border-radius: 2px;
        background: var(--border);
        flex-shrink: 0;
    }

    /* Заголовок с датой/кнопкой пересылки/панелью действий на узком экране не
       помещается в одну строку - переносим правую группу под левую. Верхний
       отступ увеличен, чтобы заголовок не наезжал на ползунок/крестик. */
    .detail-header {
        /* Колонка: блок заголовка (тайтл/дата/кнопки) над статусом-действиями.
           Было flex-wrap:wrap (ряд) - при длинном номере/статусе "Завершено" правый
           блок (статус) улетал в непредсказуемое место, а тайтл жался (detail 2/3). */
        flex-direction: column;
        align-items: stretch;
        row-gap: 10px;
        padding: 18px 15px 12px;
        /* На мобилке колонки промоутнуты (display:contents) и теряют серый фон,
           а шапка оставалась #fafafa - единственный серый блок на белом листе.
           Приводим к белому, чтобы лист был однотонным. */
        background: var(--surface);
    }

    .detail-header-left {
        flex: none;
    }

    /* Тайтл, дата и кнопки-иконки в одну компактную строку (было 20px - слишком
       разрежено на узком экране). */
    .detail-title-row {
        gap: 8px;
    }

    /* Заголовок на своей строке (полная ширина) - иначе длинный "Заявка № ..." +
       дата + кнопки жмутся в один flex-ряд и хаотично переносятся; дата и кнопки
       уходят на строку ниже под заголовком. Шрифт 17px чтобы номер влезал в строку. */
    .detail-title {
        flex: 1 1 100%;
        font-size: 17px;
    }

    /* Крестик - в правый верхний угол листа поверх шапки (не в потоке). */
    .close-detail-btn {
        position: absolute;
        top: 12px;
        right: 12px;
        z-index: 2;
    }

    .detail-header-right {
        flex-wrap: wrap;
        justify-content: flex-start;
        row-gap: 8px;
    }

    /* Крестик на мобилке вне потока (position: absolute выше), поэтому здесь
       единственный видимый ребёнок .detail-header-right - .detail-header-actions.
       Её собственный justify-content: flex-end (для desktop, чтобы упираться в
       крестик) на мобилке разворачивал бейдж и кнопки к ПРАВОМУ краю шапки - как
       было до обёртки, здесь нужен flex-start (прижим к заголовку, слева). */
    .detail-header-actions {
        justify-content: flex-start;
    }

    /* Ряд действий заявки (в т.ч. решение по раунду дополнения) - на своей
       строке, ПОД бейджем "+ Дополнение №N на согласовании": .detail-header-actions
       на мобилке остаётся flex-рядом с переносом, и без явного flex-basis панель
       вставала бы рядом с бейджем, а не под ним (владелец: "размести кнопки под
       шапкой"). На десктопе это правило снято намеренно - там всё должно стоять
       в одну строку с бейджем, а не переноситься под него (владелец: "всё в
       одну строку"). */
    .detail-header-actions :deep(.action-bar-root) {
        flex-basis: 100%;
    }

    /* Ряд автора вырос до трёх кнопок (#1685): "Дополнить" + "Продублировать" +
       "Отозвать" в nowrap не влезают в 390 (378px против 366 доступных). Разрешаем
       перенос именно этому ряду - nowrap в ActionBar ставился ради пары
       "Согласовать"/"Отказать" и её не трогаем - и снимаем минимальные ширины,
       чтобы пилюли шли по содержимому. */
    .detail-header-right :deep(.view-buttons) {
        flex-wrap: wrap;
        row-gap: 8px;
    }

    .supplement-btn,
    .withdraw-btn {
        min-width: auto;
        padding: 6px 14px;
        white-space: nowrap;
    }

    .duplicate-dropdown {
        min-width: auto;
    }

    /* Кнопки пересылки и скачивания на мобилке - единый outline-стиль: белый круг,
       синий border, синяя иконка, одинаковый размер (Скачать/Переслать/Экспорт). */
    .forward-btn {
        width: 30px;
        height: 30px;
        min-width: 30px;
        padding: 0;
        margin-left: 0;
        border-radius: 50%;
        border: 1px solid var(--accent);
        background: var(--surface);
        color: var(--accent-text);
        display: inline-flex;
        align-items: center;
        justify-content: center;
    }

    .forward-btn:hover:not(:disabled) {
        background: var(--accent-tint);
    }

    .forward-btn__text {
        display: none;
    }

    .forward-btn__icon {
        display: inline-block;
    }

    /* "Получатели" сворачивается в такой же круг: ряд заголовка на 390 несёт дату
       и кнопки-иконки, и пилюля с подписью выдавила бы их на лишнюю строку.
       margin-left сбрасываем: desktop-подтяжка к "Переслать" (-10px) здесь лишняя -
       строка и так сжата своим gap:8px (см. .detail-title-row выше), а с отрицательным
       отступом поверх круги наложились бы друг на друга. */
    .participants-btn {
        width: 30px;
        height: 30px;
        min-width: 30px;
        padding: 0;
        gap: 0;
        margin-left: 0;
        border-radius: 50%;
        justify-content: center;
    }

    .participants-btn__text {
        display: none;
    }


    /* W3.10: кросс-колоночный порядок секций детали на мобилке.
       Колонки промоутим через display:contents - их box исчезает (padding/border/gap
       уходят с колонок), а дочерние секции становятся flex-элементами общего
       .detail-content, где order работает КРОСС-колоночно. sheetScroll (свайп W3.9)
       остаётся на .detail-content: contents только на КОЛОНКАХ, не на самом контейнере.
       Порядок: сообщение(1) -> форвард(2) -> вопросы(3) -> пикер вложений(4) ->
       выбранное вложение(5) -> комментарий+согласование(6) -> дополнения(7) ->
       инфо(8) -> статус(9) -> история(10). */
    .detail-content {
        gap: 10px;
        padding: 12px;
    }

    .detail-left-column,
    .detail-main-column,
    .detail-right-column {
        display: contents;
    }

    .message-section { order: 1; }
    .detail-order-forward { order: 2; }
    .detail-order-questions { order: 3; }
    .detail-order-picker { order: 4; }
    .detail-order-selected-attachment { order: 5; }

    /* display:contents убирает box левой колонки (background/border/padding), поэтому
       секция пикера вложений рендерилась бы "голой" рядом с карточными соседями -
       возвращаем ей карточный стиль (как у .message-section и др.). */
    .detail-order-picker {
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: 20px;
        padding: 15px;
        box-shadow: 0 2px 12px var(--shadow-drop);
    }
    /* Комментарий - к кнопкам действия (Принять/Отказать в шапке): поднимаем его
       в самый верх контента, сразу под шапку, чтобы поле не было оторвано от
       действия внизу списка. Показывается только когда есть что комментировать. */
    .comment-action-section { order: 0; }
    .detail-order-confirmation { order: 6; }
    /* Дополнения - под согласование (раунд и есть повторный круг согласования) и
       заведомо ниже сообщения, действий и вложений. Без своего order панель шла с
       нулевым (дефолт) и вставала в самый верх ленты, до сообщения. */
    .detail-order-supplement { order: 7; }
    .basic-info-section { order: 8; }
    .application-status-section { order: 9; }
    .history-button-section { order: 10; }

    /* Промоутнутые секции не сжимаем - иначе flex-column режет высокий контент
       (пикер/вложение) вместо скролла .detail-content (ср. .detail-main-column > *). */
    .detail-order-picker,
    .detail-order-forward,
    .detail-order-questions,
    .detail-order-selected-attachment,
    .detail-order-confirmation,
    .detail-order-supplement,
    .message-section,
    .basic-info-section,
    .application-status-section,
    .comment-action-section,
    .history-button-section {
        flex-shrink: 0;
    }
}

@keyframes detailSlideUp {
    from { transform: translateY(100%); }
    to { transform: translateY(0); }
}

/* Кнопка "Скачать бланк" в шапке детали (W3.8) - иконка + текст, вторичный стиль.
   Брейкпоинт 767.98 (не 768) совпадает со скрытием кнопки в строке списка - иначе
   ровно на 768px "Скачать" была бы видна и в строке, и в детали одновременно. */
@media (max-width: 767.98px) {
    /* Скачать - только иконка в таком же outline-круге, что и Переслать (единый
       стиль кнопок-иконок в шапке детали). Текст скрыт. */
    .detail-download-btn {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        gap: 0;
        margin-left: 0;
        width: 30px;
        height: 30px;
        min-width: 30px;
        padding: 0;
        border: 1px solid var(--accent);
        border-radius: 50%;
        background: var(--surface);
        color: var(--accent-text);
        cursor: pointer;
        transition: background 0.2s ease, color 0.2s ease;
    }

    .detail-download-btn__text {
        display: none;
    }

    .detail-download-btn:hover {
        background: var(--accent-tint);
    }
}
</style>