<template>
  <!-- Экран входа лежит на фирменном синем, который темы не знает: в тёмной
       палитре заголовок и карточка контактов на нём пропадали. Поэтому вход -
       «светлый остров»: локальный data-theme перекрывает выбранную тему только
       внутри этого экрана (см. assets/tokens.css). -->
  <div
    class="login"
    data-theme="light"
    @mousemove="handleMouseMove"
  >
    <div class="login-background">
      <div
        v-for="(orb, i) in backgroundOrbs"
        :key="i"
        class="floating-shape"
        :class="`floating-shape--${orb.path}`"
        :style="orbStyle(orb)"
      />
    </div>

    <div
      class="login-pattern"
      :style="parallaxStyle"
    />

    <!-- Пейзаж вместо снимка: три плана силуэтов с воздушной перспективой -
         дальний светлее и мягче, ближний темнее и крупнее. Плоская заливка
         глубины не давала, а именно её давало прежнее фото. Растягивается по
         ширине (preserveAspectRatio none): формы плавные, искажение не читается,
         зато экран заполнен и на телефоне. -->
    <svg
      class="login-scene"
      viewBox="0 0 1440 900"
      preserveAspectRatio="none"
      aria-hidden="true"
      focusable="false"
    >
      <defs>
        <linearGradient
          id="lsSky"
          x1="0"
          y1="0"
          x2="0"
          y2="1"
        >
          <stop
            offset="0%"
            stop-color="#1B2278"
            stop-opacity="0.55"
          />
          <stop
            offset="45%"
            stop-color="#3A45C9"
            stop-opacity="0.15"
          />
          <stop
            offset="100%"
            stop-color="#8FA0FF"
            stop-opacity="0.18"
          />
        </linearGradient>
        <!-- Заливки планов непрозрачные, прозрачность живёт на группе: иначе холм
             не перекрывает стволы своих деревьев и они просвечивают до низа. -->
        <linearGradient
          id="lsFar"
          x1="0"
          y1="0"
          x2="0"
          y2="1"
        >
          <stop
            offset="0%"
            stop-color="#D3DAFF"
          />
          <stop
            offset="100%"
            stop-color="#AFBCFF"
          />
        </linearGradient>
        <linearGradient
          id="lsMid"
          x1="0"
          y1="0"
          x2="0"
          y2="1"
        >
          <stop
            offset="0%"
            stop-color="#39439F"
          />
          <stop
            offset="100%"
            stop-color="#232B79"
          />
        </linearGradient>
        <linearGradient
          id="lsNear"
          x1="0"
          y1="0"
          x2="0"
          y2="1"
        >
          <stop
            offset="0%"
            stop-color="#1A2166"
          />
          <stop
            offset="100%"
            stop-color="#0F1443"
          />
        </linearGradient>
        <radialGradient
          id="lsSun"
          cx="50%"
          cy="50%"
          r="50%"
        >
          <stop
            offset="0%"
            stop-color="#FFE6B8"
            stop-opacity="0.42"
          />
          <stop
            offset="60%"
            stop-color="#FFC98A"
            stop-opacity="0.14"
          />
          <stop
            offset="100%"
            stop-color="#FFC98A"
            stop-opacity="0"
          />
        </radialGradient>
      </defs>

      <rect
        x="0"
        y="0"
        width="1440"
        height="900"
        fill="url(#lsSky)"
      />
      <!-- Тёплое свечение у горизонта: единственный тёплый тон на экране, без
           него синий остаётся однородным на всю высоту. -->
      <ellipse
        cx="1000"
        cy="566"
        rx="620"
        ry="215"
        fill="url(#lsSun)"
      />

      <!-- Дальний план: пологие холмы у горизонта, почти растворены в дымке -->
      <path
        d="M0 610C180 566 320 592 470 604s280-52 430-44 250 60 380 42 160-26 160-26V900H0Z"
        fill="url(#lsFar)"
        opacity="0.13"
      />
      <!-- Средний план: аллея округлых крон вдоль гребня холма -->
      <g
        fill="url(#lsMid)"
        opacity="0.4"
      >
        <!-- Ствол уводится заведомо ниже гребня: холм рисуется поверх и обрезает
             его, иначе дерево висит на палочке над склоном. -->
        <g
          v-for="(tree, i) in midTrees"
          :key="`m${i}`"
        >
          <rect
            :x="tree.x - 2"
            :y="tree.y - 6"
            width="4"
            :height="tree.r + 120"
          />
          <ellipse
            :cx="tree.x"
            :cy="tree.y - tree.r * 0.6"
            :rx="tree.r"
            :ry="tree.r * 1.05"
          />
        </g>
        <path d="M0 700C150 660 260 686 420 674s300-58 470-40 260 62 400 44 150-22 150-22V900H0Z" />
      </g>
      <!-- Ближний план: крупные кроны на тёмном силуэте -->
      <g
        fill="url(#lsNear)"
        opacity="0.68"
      >
        <g
          v-for="(tree, i) in nearTrees"
          :key="`n${i}`"
        >
          <rect
            :x="tree.x - 4"
            :y="tree.y - 10"
            width="8"
            :height="tree.r + 160"
          />
          <ellipse
            :cx="tree.x"
            :cy="tree.y - tree.r * 0.65"
            :rx="tree.r"
            :ry="tree.r * 1.1"
          />
        </g>
        <path d="M0 800C170 764 300 790 480 780s330-56 520-34 270 56 440 30V900H0Z" />
      </g>

      <!-- Огни вдоль кромки: тёплые точки оживляют силуэт и дают глазу, за что
           зацепиться. Мерцание разнесено по фазам, чтобы не пульсировали хором. -->
      <g
        v-for="(light, i) in parkLights"
        :key="`l${i}`"
      >
        <circle
          class="login-scene__glow"
          :cx="light.x"
          :cy="light.y"
          :r="light.r * 9"
          fill="#FFD9A0"
          :style="{ animationDuration: `${light.dur}s`, animationDelay: `${light.delay}s` }"
        />
        <circle
          class="login-scene__light"
          :cx="light.x"
          :cy="light.y"
          :r="light.r"
          fill="#FFF0D2"
          :style="{ animationDuration: `${light.dur}s`, animationDelay: `${light.delay}s` }"
        />
      </g>
    </svg>

    <!-- Линии-траектории в небе: спокойный ритм поверх сетки. -->
    <svg
      class="login-lines"
      viewBox="0 0 1440 900"
      preserveAspectRatio="none"
      aria-hidden="true"
      focusable="false"
    >
      <g
        v-for="(line, i) in backgroundLines"
        :key="i"
        class="login-lines__group"
        :style="{ animationDuration: `${line.dur}s`, animationDelay: `${line.delay}s` }"
      >
        <path
          :d="line.d"
          fill="none"
          :stroke="`rgba(255, 255, 255, ${line.alpha})`"
          :stroke-width="line.w"
          stroke-linecap="round"
        />
      </g>
    </svg>
        
    <div class="login__container">
      <div class="login__active">
        <div class="login__header">
          <h1
            class="login__title"
            :style="titleStyle"
          >
            Войдите в аккаунт
          </h1>
          <h3
            class="login__subtitle"
            :style="subtitleStyle"
          >
            для продолжения
          </h3>
        </div>
        <form
          class="login__form"
          data-testid="login-form"
          @submit.prevent="handleSubmit"
        >
          <div class="inputs">
            <div
              class="login__input"
              :style="input1Style"
            >
              <img
                src="@/assets/icons/login.png"
                alt=""
                class="input__icon"
              >
              <input
                v-model="formData.username"
                class="input"
                type="text"
                data-testid="login-input-username"
                autocomplete="off"
                autocorrect="off"
                autocapitalize="off"
                spellcheck="false"
                placeholder="Логин"
                aria-label="Имя пользователя"
                @keyup.enter="handleSubmit"
              >
            </div>
            <div
              class="login__input"
              :style="input2Style"
            >
              <img
                src="@/assets/icons/password.png"
                alt=""
                class="input__icon"
              >
              <input
                v-model="formData.password"
                class="input"
                type="password"
                data-testid="login-input-password"
                autocomplete="new-password"
                autocorrect="off"
                autocapitalize="off"
                spellcheck="false"
                placeholder="Пароль"
                aria-label="Пароль"
                @keyup.enter="handleSubmit"
              >
            </div>
          </div>
                    
          <a
            href="#"
            class="remember-password"
            :style="linkStyle"
            @click.prevent="showPasswordRecovery = true"
          >Забыли пароль?</a>
                    
          <div
            class="login__footer"
            :style="footerStyle"
          >
            <div class="error-container">
              <transition name="fade">
                <div
                  v-if="showError"
                  class="error-message"
                  data-testid="login-error-message"
                >
                  {{ displayError }}
                </div>
              </transition>
            </div>
                        
            <div class="footer__button">
              <button
                class="login__button"
                data-testid="login-button-submit"
                :class="{'loading': isLoading, 'success': isSuccess, 'cooling': isCoolingDown}"
                :disabled="isLoading || isSuccess || isCoolingDown"
              >
                <p class="button__text">
                  {{ getButtonText }}
                </p>
                <img
                  v-if="!isLoading && !isSuccess"
                  src="@/assets/icons/key-blue.png"
                  alt=""
                  class="input__icon"
                >
                <div
                  v-if="isLoading"
                  class="spinner"
                />
              </button>
              <div
                class="custom-lock"
                :class="{'error': hasError, 'shaking': isShaking, 'success': isSuccess}"
              >
                <div class="lock-arc" />
                <div class="lock-body" />
              </div>
            </div>
          </div>
        </form>
      </div>
      <div
        class="login__info"
        :style="infoStyle"
      >
        <div class="info-notifications">
          <transition
            name="notification"
            mode="out-in"
          >
            <div
              v-if="showNotification"
              :key="notificationText"
              class="notification"
              data-testid="login-copy-notification"
            >
              {{ notificationText }}
            </div>
          </transition>
        </div>

        <h2 class="info__title">
          Добро пожаловать!
        </h2>
        <p class="info__text">
          Для продолжения, необходимо войти в аккаунт,
          используя выданные данные.
        </p>
        <h3 class="info__title help">
          Помощь и поддержка
        </h3>
        <p class="info__text">
          Обратитесь к нам, чтобы получить учётную запись, восстановить доступ или решить другие проблемы:
        </p>
        <div class="info__contacts">
          <div class="contact">
            <img
              src="@/assets/icons/email-blue.png"
              class="contact__icon"
              alt=""
            >
            <p
              class="contact__text contact__text--clickable"
              @click="copyEmail"
            >
              {{ bureauEmail }}
            </p>
          </div>
          <div class="contact">
            <img
              src="@/assets/icons/phone-blue.png"
              class="contact__icon"
              alt=""
            >
            <p
              class="contact__text contact__text--clickable"
              @click="copyPhone"
            >
              {{ bureauPhone }}
            </p>
          </div>
          <p class="time">
            ПН-ПТ: <strong>08:00 - 22:00</strong> СБ-ВС и ПРАЗДНИКИ: <strong>08:00 - 20:00</strong>
          </p>
        </div>
      </div>
    </div>

    <PasswordRecoveryModal
      :show="showPasswordRecovery"
      @close="showPasswordRecovery = false"
    />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { useContactsStore } from '@/stores/contacts'
import { resolveLoginRedirect } from '@/utils/postLoginRedirect'
import PasswordRecoveryModal from '@/components/PasswordRecoveryModal.vue'

// Фолбэк-контакты Бюро, если в настройках системы они ещё не заданы.
const FALLBACK_BUREAU_EMAIL = 'buropropuskov@dreamisland.ru'
const FALLBACK_BUREAU_PHONE = '+7 (910) 083 00-55'
export default {
    components: { PasswordRecoveryModal },
    emits: ['login-success'],
    data() {
        return {
            formData: {
                username: '',
                password: ''
            },
            errors: {
                general: ''
            },
            showError: false,
            isLoading: false,
            isSuccess: false,
            hasError: false,
            isShaking: false,
            animationTimeout: null,
            errorTimeout: null,
            mouseX: 0,
            mouseY: 0,
            elementsVisible: false,
            showNotification: false,
            notificationText: '',
            notificationTimeout: null,
            showPasswordRecovery: false,
            cooldownSeconds: 0,
            cooldownTimer: null,
            /* Боке над пейзажем: размер, положение в процентах, прозрачность и
               своя длительность дрейфа у каждого шара. Разные фазы (delay) не
               дают им двигаться строем - иначе фон читается как один слой. */
            backgroundOrbs: [
                { left: -4, top: -6, size: 260, alpha: 0.12, dur: 46, delay: 0, path: 'a' },
                { left: 78, top: 6, size: 190, alpha: 0.1, dur: 52, delay: -8, path: 'b' },
                { left: 92, top: 28, size: 120, alpha: 0.07, dur: 38, delay: -14, path: 'c' },
                { left: 63, top: 50, size: 150, alpha: 0.09, dur: 44, delay: -6, path: 'a' },
                { left: 87, top: 72, size: 220, alpha: 0.08, dur: 56, delay: -20, path: 'b' },
                { left: 70, top: 86, size: 90, alpha: 0.07, dur: 40, delay: -12, path: 'c' },
                { left: 46, top: 68, size: 72, alpha: 0.06, dur: 36, delay: -3, path: 'a' },
                { left: 30, top: 90, size: 130, alpha: 0.06, dur: 48, delay: -25, path: 'b' },
                { left: 16, top: 44, size: 110, alpha: 0.05, dur: 42, delay: -9, path: 'c' },
                { left: 4, top: 72, size: 62, alpha: 0.06, dur: 34, delay: -16, path: 'a' },
                { left: 52, top: 16, size: 56, alpha: 0.06, dur: 30, delay: -5, path: 'b' },
                { left: 36, top: 32, size: 40, alpha: 0.05, dur: 32, delay: -11, path: 'c' },
                { left: 84, top: 50, size: 46, alpha: 0.07, dur: 28, delay: -2, path: 'a' },
                { left: 24, top: 60, size: 36, alpha: 0.05, dur: 33, delay: -19, path: 'b' },
                { left: 66, top: 24, size: 34, alpha: 0.06, dur: 31, delay: -7, path: 'c' },
                { left: 95, top: 90, size: 160, alpha: 0.05, dur: 50, delay: -22, path: 'a' }
            ],
            /* Кроны вдоль кромки холмов. y - точка посадки на гребне, r задаёт
               разброс высот: ровный ряд одинаковых деревьев читается как забор. */
            midTrees: [
                { x: 90, y: 684, r: 15 }, { x: 140, y: 680, r: 11 }, { x: 250, y: 676, r: 17 },
                { x: 292, y: 674, r: 12 }, { x: 430, y: 672, r: 14 }, { x: 486, y: 668, r: 10 },
                { x: 610, y: 660, r: 18 }, { x: 660, y: 656, r: 12 }, { x: 790, y: 648, r: 15 },
                { x: 838, y: 650, r: 10 }, { x: 960, y: 656, r: 16 }, { x: 1012, y: 660, r: 11 },
                { x: 1140, y: 670, r: 17 }, { x: 1196, y: 672, r: 12 }, { x: 1330, y: 664, r: 14 },
                { x: 1386, y: 660, r: 10 }
            ],
            nearTrees: [
                { x: 130, y: 786, r: 30 }, { x: 205, y: 792, r: 20 }, { x: 400, y: 784, r: 26 },
                { x: 660, y: 770, r: 34 }, { x: 745, y: 766, r: 22 }, { x: 980, y: 766, r: 28 },
                { x: 1215, y: 778, r: 32 }, { x: 1300, y: 782, r: 21 }
            ],
            parkLights: [
                { x: 168, y: 700, r: 2.6, dur: 7, delay: 0 },
                { x: 356, y: 690, r: 2.2, dur: 9, delay: -3 },
                { x: 560, y: 676, r: 2.8, dur: 8, delay: -5 },
                { x: 806, y: 664, r: 2.3, dur: 10, delay: -1 },
                { x: 1046, y: 672, r: 2.7, dur: 7.5, delay: -6 },
                { x: 1268, y: 684, r: 2.2, dur: 9.5, delay: -2 },
                { x: 470, y: 792, r: 3.2, dur: 8.5, delay: -4 },
                { x: 880, y: 776, r: 3, dur: 11, delay: -7 }
            ],
            backgroundLines: [
                { d: 'M-40 250C260 210 520 300 820 258s420-98 700-126', w: 1.6, alpha: 0.18, dur: 40, delay: 0 },
                { d: 'M-40 430C300 400 560 470 900 424s380-70 620-92', w: 1.2, alpha: 0.13, dur: 52, delay: -12 },
                { d: 'M-40 120C240 96 480 150 760 118s460-58 760-74', w: 1.1, alpha: 0.1, dur: 46, delay: -22 },
                { d: 'M-40 560C280 540 540 596 880 552s400-56 640-70', w: 1, alpha: 0.09, dur: 58, delay: -30 }
            ]
        }
    },
    computed: {
        getButtonText() {
            if (this.isCoolingDown) return this.cooldownText;
            if (this.isLoading) return 'Вход...';
            if (this.isSuccess) return 'Успешно!';
            return 'Войти';
        },
        isCoolingDown() {
            return this.cooldownSeconds > 0;
        },
        // Кулдаун растёт по лестнице до часа, поэтому час выводим отдельным разрядом:
        // "60:00" читается как минуты и врёт про порядок ожидания.
        cooldownText() {
            const total = Math.max(0, this.cooldownSeconds);
            const h = Math.floor(total / 3600);
            const m = Math.floor((total % 3600) / 60);
            const s = total % 60;
            const pad = (n) => String(n).padStart(2, '0');
            return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${m}:${pad(s)}`;
        },
        displayError() {
            if (this.isCoolingDown) {
                return `Слишком много попыток. Повторите через ${this.cooldownText}`;
            }
            return this.errors.general;
        },
        bureauEmail() {
            return useContactsStore().email || FALLBACK_BUREAU_EMAIL;
        },
        bureauPhone() {
            return useContactsStore().phone || FALLBACK_BUREAU_PHONE;
        },
        parallaxStyle() {
            const moveX = (this.mouseX - window.innerWidth / 8) / 25;
            const moveY = (this.mouseY - window.innerHeight / 8) / 25;
            
            return {
                transform: `translate3d(${moveX}px, ${moveY}px, 0) scale(1.1)`
            };
        },
        titleStyle() {
            return {
                opacity: this.elementsVisible ? 1 : 0,
                transform: this.elementsVisible ? 'translateY(0)' : 'translateY(20px)',
                transition: 'opacity 0.6s ease, transform 0.6s ease',
                transitionDelay: '0.1s'
            };
        },
        subtitleStyle() {
            return {
                opacity: this.elementsVisible ? 1 : 0,
                transform: this.elementsVisible ? 'translateY(0)' : 'translateY(20px)',
                transition: 'opacity 0.6s ease, transform 0.6s ease',
                transitionDelay: '0.2s'
            };
        },
        input1Style() {
            return {
                opacity: this.elementsVisible ? 1 : 0,
                transform: this.elementsVisible ? 'translateX(0)' : 'translateX(-30px)',
                transition: 'opacity 0.6s ease, transform 0.6s ease',
                transitionDelay: '0.3s'
            };
        },
        input2Style() {
            return {
                opacity: this.elementsVisible ? 1 : 0,
                transform: this.elementsVisible ? 'translateX(0)' : 'translateX(-30px)',
                transition: 'opacity 0.6s ease, transform 0.6s ease',
                transitionDelay: '0.4s'
            };
        },
        linkStyle() {
            return {
                opacity: this.elementsVisible ? 1 : 0,
                transition: 'opacity 0.6s ease',
                transitionDelay: '0.5s'
            };
        },
        footerStyle() {
            return {
                opacity: this.elementsVisible ? 1 : 0,
                transform: this.elementsVisible ? 'translateY(0)' : 'translateY(30px)',
                transition: 'opacity 0.6s ease, transform 0.6s ease',
                transitionDelay: '0.6s'
            };
        },
        infoStyle() {
            return {
                opacity: this.elementsVisible ? 1 : 0,
                transform: this.elementsVisible ? 'translateX(0)' : 'translateX(30px)',
                transition: 'opacity 0.8s ease, transform 0.8s ease',
                transitionDelay: '0.7s'
            };
        }
    },
    mounted() {
        setTimeout(() => {
            this.elementsVisible = true;
        }, 100);
        useContactsStore().fetch();
        this.restoreCooldown();
    },
    beforeUnmount() {
        if (this.animationTimeout) {
            clearTimeout(this.animationTimeout);
        }
        if (this.errorTimeout) {
            clearTimeout(this.errorTimeout);
        }
        if (this.notificationTimeout) {
            clearTimeout(this.notificationTimeout);
        }
        // Останавливаем только интервал. localStorage НЕ чистим - блокировка должна
        // пережить перезагрузку/размонтирование (restoreCooldown поднимет её заново).
        if (this.cooldownTimer) {
            clearInterval(this.cooldownTimer);
            this.cooldownTimer = null;
        }
    },
    methods: {
        handleMouseMove(e) {
            this.mouseX = e.clientX;
            this.mouseY = e.clientY;
        },
        orbStyle(orb) {
            return {
                left: `${orb.left}%`,
                top: `${orb.top}%`,
                width: `${orb.size}px`,
                height: `${orb.size}px`,
                background: `rgba(255, 255, 255, ${orb.alpha})`,
                animationDuration: `${orb.dur}s`,
                animationDelay: `${orb.delay}s`,
                boxShadow: orb.size >= 120 ? '0 6px 24px rgba(0, 0, 0, 0.12)' : 'none'
            };
        },
        resetAnimations() {
            if (this.animationTimeout) {
                clearTimeout(this.animationTimeout);
                this.animationTimeout = null;
            }
            
            if (this.errorTimeout) {
                clearTimeout(this.errorTimeout);
                this.errorTimeout = null;
            }
            
            this.showError = false;
            this.isLoading = false;
            this.isSuccess = false;
            this.hasError = false;
            this.isShaking = false;
        },
        
        setupErrorAutoHide() {
            if (this.errorTimeout) {
                clearTimeout(this.errorTimeout);
            }

            this.errorTimeout = setTimeout(() => {
                this.showError = false;
                this.errors.general = '';
            }, 10000);
        },

        // startCooldown запускает обратный отсчёт блокировки входа (429): плашка
        // с оставшимся временем держится до нуля, кнопка входа заблокирована.
        // Auto-hide (10с) при кулдауне НЕ используем - сообщение живёт весь отсчёт.
        startCooldown(seconds) {
            this.clearCooldown();
            const sec = Math.floor(Number(seconds));
            if (!Number.isFinite(sec) || sec <= 0) return;
            if (this.errorTimeout) {
                clearTimeout(this.errorTimeout);
                this.errorTimeout = null;
            }
            this.cooldownSeconds = sec;
            this.showError = true;
            // Персистим момент окончания: F5 не сбросит таймер и не даст обойти
            // блокировку перезагрузкой (restoreCooldown поднимет остаток при mount).
            localStorage.setItem('loginCooldownUntil', String(Date.now() + sec * 1000));
            this.cooldownTimer = setInterval(() => {
                this.cooldownSeconds -= 1;
                if (this.cooldownSeconds <= 0) {
                    this.clearCooldown();
                    this.showError = false;
                    this.errors.general = '';
                }
            }, 1000);
        },

        clearCooldown() {
            if (this.cooldownTimer) {
                clearInterval(this.cooldownTimer);
                this.cooldownTimer = null;
            }
            this.cooldownSeconds = 0;
            localStorage.removeItem('loginCooldownUntil');
        },

        // restoreCooldown поднимает активную блокировку из localStorage при загрузке
        // страницы - чтобы перезагрузка не обнуляла таймер и не открывала отправку.
        restoreCooldown() {
            const until = parseInt(localStorage.getItem('loginCooldownUntil'), 10);
            if (!Number.isFinite(until)) return;
            const remaining = Math.ceil((until - Date.now()) / 1000);
            if (remaining > 0) {
                this.startCooldown(remaining);
            } else {
                localStorage.removeItem('loginCooldownUntil');
            }
        },
        
        showNotificationMessage(text) {
            if (this.notificationTimeout) {
                clearTimeout(this.notificationTimeout);
                this.notificationTimeout = null;
            }
            this.notificationText = text;
            this.showNotification = true;
            this.notificationTimeout = setTimeout(() => {
                this.showNotification = false;
                this.notificationTimeout = null;
            }, 2000);
        },

        async copyToClipboard(text) {
            try {
                await navigator.clipboard.writeText(text);
            } catch {
                const textArea = document.createElement('textarea');
                textArea.value = text;
                document.body.appendChild(textArea);
                textArea.select();
                document.execCommand('copy');
                document.body.removeChild(textArea);
            }
        },

        async copyEmail() {
            await this.copyToClipboard(this.bureauEmail);
            this.showNotificationMessage('E-mail скопирован');
        },

        async copyPhone() {
            await this.copyToClipboard(this.bureauPhone);
            this.showNotificationMessage('Номер телефона скопирован');
        },
        
        async handleSubmit() {
    // Enter в поле формы вызывает и @submit формы, и @keyup.enter инпута - без guard'а
    // это два параллельных логина (в auth_events двоились login_failed). isLoading
    // ставим ДО первого await, чтобы второй синхронный вызов отсёкся здесь.
    if (this.isLoading || this.isSuccess || this.isCoolingDown) return;
    this.resetAnimations();
    this.errors.general = '';

    if (!this.formData.username.trim() || !this.formData.password) {
        this.errors.general = 'Необходимо заполнить все поля';
        this.showError = true;
        this.hasError = true;
        this.isShaking = true;
        this.setupErrorAutoHide();
        this.animationTimeout = setTimeout(() => {
            this.isShaking = false;
            this.hasError = false;
        }, 500);
        return;
    }

    this.isLoading = true;
    await new Promise(resolve => setTimeout(resolve, 100));

    let timeoutId;
    
    try {
        const controller = new AbortController();
        timeoutId = setTimeout(() => controller.abort(), 10000); // 10 секунд таймаут
        
        const response = await apiRequest("/login", {
            method: "POST",
            headers: { 
                "Content-Type": "application/json",
                "Accept": "application/json"
            },
            body: JSON.stringify({ 
                username: this.formData.username, 
                password: this.formData.password 
            }),
            signal: controller.signal
        });

        clearTimeout(timeoutId);

        if (response.ok) {
            const data = await response.json();

            const authStore = useAuthStore()
            authStore.setTokens(data.token)

            this.isLoading = false;
            this.isSuccess = true;

            await new Promise(resolve => setTimeout(resolve, 1500));

            this.$emit('login-success', { token: data.token });

            // #974: если сюда привёл гард (открыли ссылку/push-уведомление без
            // сессии), возвращаем на исходный адрес, а не на дефолтную ленту.
            this.$router.push(resolveLoginRedirect(this.$route.query) || '/news');
        } else {
            // Проверяем статус код для определения типа ошибки
            if (response.status === 429) {
                // Retry-After (сек): и IP-лимитер входа, и блокировка учётки.
                // Запускаем обратный отсчёт; текст плашки берёт displayError.
                const retryAfter = parseInt(response.headers.get('Retry-After'), 10);
                const sec = Number.isFinite(retryAfter) && retryAfter > 0 ? retryAfter : 60;
                this.isLoading = false;
                this.startCooldown(sec);
                return;
            } else if (response.status === 401) {
                // X-Auth-Attempts-Remaining: остаток попыток до блокировки учётки
                // (есть только для существующего логина).
                const remaining = parseInt(response.headers.get('X-Auth-Attempts-Remaining'), 10);
                this.errors.general = Number.isFinite(remaining)
                    ? `Неверный логин или пароль. Осталось попыток: ${remaining}`
                    : 'Неверный логин или пароль';
            } else {
                try {
                    const errorText = await response.text();
                    if (errorText) {
                        try {
                            const errorData = JSON.parse(errorText);
                            // Тело ошибки приходит конвертом {success:false, error}. Читали
                            // только message - его в конверте нет, и на месте текста
                            // оказывался сам объект, то есть "[object Object]" на экране.
                            // Форму берём сырую: тут response.text(), а разворачивает конверт
                            // client.js только у response.json().
                            const message = typeof errorData === 'string'
                                ? errorData
                                : errorData?.error || errorData?.message;
                            this.errors.general = message || "Произошла ошибка";
                        } catch {
                            this.errors.general = errorText || "Произошла ошибка";
                        }
                    } else {
                        this.errors.general = `Ошибка ${response.status}: ${response.statusText}`;
                    }
                } catch {
                    this.errors.general = `Ошибка ${response.status}: ${response.statusText}`;
                }
            }
            this.isLoading = false;
            await this.showErrorWithDelay();
        }
    } catch (error) {
        if (timeoutId) clearTimeout(timeoutId);
        
        console.error("Network error:", error);
        
        // Определяем тип ошибки более точно
        if (error.name === 'AbortError') {
            this.errors.general = "Таймаут запроса. Сервер не отвечает.";
        } else if (error.toString().includes("Failed to fetch") || error.toString().includes("NetworkError")) {
            // Это может быть CORS ошибка
            this.errors.general = "Ошибка сети. Проверьте подключение и повторите позже.";
        } else if (error.toString().includes("Too many requests") || error.toString().includes("429")) {
            // Сетевой путь без заголовков - fallback-кулдаун 60с.
            this.isLoading = false;
            this.startCooldown(60);
            return;
        } else {
            this.errors.general = "Ошибка соединения. Проверьте подключение к интернету.";
        }

        this.isLoading = false;
        await this.showErrorWithDelay();
    }
},
        
        async showErrorWithDelay() {
            await new Promise(resolve => setTimeout(resolve, 100));
            this.showError = true;
            this.setupErrorAutoHide();
            this.showErrorAnimation();
        },
        
        showErrorAnimation() {
            this.hasError = true;
            this.isShaking = true;
            
            this.animationTimeout = setTimeout(() => {
                this.isShaking = false;
            }, 600);
            
            this.animationTimeout = setTimeout(() => {
                this.hasError = false;
                
                this.animationTimeout = setTimeout(() => {
                    this.hasError = false;
                    this.isShaking = false;
                }, 300);
            }, 700);
        },
    }
}
</script>

<style scoped>
    /* Ваши существующие стили остаются без изменений */
    .login {
        width: 100%;
        /* zoom-safe (#1097): 100vh/100dvh под корневым zoom считаются от
           НЕзумленной высоты -> экран логина выше зумленного вьюпорта (900px при
           2560), форма уезжает под фолд. --app-vh нормирован на zoom. */
        height: calc(var(--app-vh, 1vh) * 100);
        /* B.3 (#1097): svh стабилизирует высоту на мобилке (ретракт адрес-бара Яндекса
           не дёргает layout); min() держит zoom-корректность на десктопе (app-vh < svh
           под zoom). Отдельное объявление - при отсутствии svh каскад откатится на calc. */
        height: min(calc(var(--app-vh, 1vh) * 100), 100svh);
        background-color: var(--accent);
        padding: 40px;
        display: flex;
        position: relative;
        overflow: hidden;
        perspective: 1000px;
    }

    .login-background {
        position: absolute;
        width: 100%;
        height: 100%;
        top: 0;
        left: 0;
        z-index: 0;
    }

    /* Собственная графика вместо снимка: сетка постов с узлами на пересечениях
       и два световых пятна - под формой слева и в глубине справа. Рисуется
       градиентами, поэтому весит ноль и не боится масштаба экрана. */
    .login-pattern {
        position: absolute;
        width: 100%;
        height: 100%;
        top: 0;
        left: 0;
        background-image:
            radial-gradient(circle at 50% 50%, rgba(255, 255, 255, 0.55) 0 1.5px, transparent 2px),
            linear-gradient(to right, rgba(255, 255, 255, 0.07) 1px, transparent 1px),
            linear-gradient(to bottom, rgba(255, 255, 255, 0.07) 1px, transparent 1px),
            radial-gradient(115% 85% at 14% 26%, rgba(255, 255, 255, 0.22), transparent 62%),
            radial-gradient(95% 80% at 86% 96%, rgba(23, 29, 96, 0.4), transparent 68%);
        background-size: 96px 96px, 96px 96px, 96px 96px, 100% 100%, 100% 100%;
        background-position: 48px 48px, 0 0, 0 0, 0 0, 0 0;
        opacity: 0.9;
        z-index: 1;
        transition: transform 0.1s ease-out;
        will-change: transform;
    }

    /* Слои пейзажа и линий лежат между сеткой и формой. Отдельными элементами,
       а не фоном .login-pattern: его замок в спеке требует, чтобы слой рисовался
       градиентами и не тянул файл, а разметке нужны кривые силуэтов. */
    .login-scene {
        position: absolute;
        width: 100%;
        height: 100%;
        top: 0;
        left: 0;
        z-index: 1;
        pointer-events: none;
    }

    .login-lines {
        position: absolute;
        width: 100%;
        height: 100%;
        top: 0;
        left: 0;
        z-index: 2;
        pointer-events: none;
    }

    .login-scene__light {
        opacity: 0.85;
        animation-name: light-pulse;
        animation-iteration-count: infinite;
        animation-timing-function: ease-in-out;
    }

    .login-scene__glow {
        opacity: 0.18;
        animation-name: glow-pulse;
        animation-iteration-count: infinite;
        animation-timing-function: ease-in-out;
    }

    @keyframes light-pulse {
        0%, 100% {
            opacity: 0.85;
        }
        50% {
            opacity: 0.5;
        }
    }

    @keyframes glow-pulse {
        0%, 100% {
            opacity: 0.18;
        }
        50% {
            opacity: 0.08;
        }
    }

    .login-lines__group {
        animation-name: line-sway;
        animation-iteration-count: infinite;
        animation-timing-function: ease-in-out;
        will-change: transform;
    }

    .floating-shape {
        position: absolute;
        border-radius: 50%;
        z-index: 3;
        animation-iteration-count: infinite;
        animation-timing-function: ease-in-out;
        will-change: transform;
    }

    /* Три траектории вместо одной: шары одного пути двигаются согласованно, но
       разные пути и фазы убирают ощущение строя. Только transform - слои не
       пересчитывают раскладку. */
    .floating-shape--a {
        animation-name: orb-drift-a;
    }

    .floating-shape--b {
        animation-name: orb-drift-b;
    }

    .floating-shape--c {
        animation-name: orb-drift-c;
    }

    @keyframes orb-drift-a {
        0%, 100% {
            transform: translate3d(0, 0, 0);
        }
        50% {
            transform: translate3d(26px, -34px, 0);
        }
    }

    @keyframes orb-drift-b {
        0%, 100% {
            transform: translate3d(0, 0, 0);
        }
        33% {
            transform: translate3d(-22px, 16px, 0);
        }
        66% {
            transform: translate3d(15px, 28px, 0);
        }
    }

    @keyframes orb-drift-c {
        0%, 100% {
            transform: translate3d(0, 0, 0) scale(1);
        }
        50% {
            transform: translate3d(-16px, -20px, 0) scale(1.07);
        }
    }

    @keyframes line-sway {
        0%, 100% {
            transform: translate3d(0, 0, 0);
        }
        50% {
            transform: translate3d(0, 14px, 0);
        }
    }

    /* Пользователь, попросивший меньше движения на уровне системы, получает
       статичный фон - анимации здесь чисто декоративные. */
    @media (prefers-reduced-motion: reduce) {
        .floating-shape,
        .login-lines__group,
        .login-scene__light,
        .login-scene__glow {
            animation: none;
        }
    }

    .login__container {
        display: flex;
        width: 100%;
        justify-content: space-between;
    }

    .login__info {
        width: 550px;
        height: 480px;
        background-color: rgba(255,255,255,0.9);
        border-radius: 100px;
        box-shadow: 0 3px 10px rgba(0,0,0,0.25);
        z-index: 1000;
        padding: 50px;
        margin-top: 50px;
        position: relative;
    }

    .info-notifications {
        position: absolute;
        top: -40px;
        left: 50%;
        transform: translateX(-50%);
        z-index: 1001;
        pointer-events: none;
    }

    .notification {
        height: 25px;
        border-radius: 50px;
        background-color: #fff;
        font-size: 14px;
        color: #000;
        font-weight: 500;
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 0 15px;
        box-shadow: 0 3px 10px rgba(0,0,0,0.2);
        min-width: 150px;
        white-space: nowrap;
    }

    .notification-enter-active,
    .notification-leave-active {
        transition: opacity 0.3s ease, transform 0.3s ease;
    }

    .notification-enter-from,
    .notification-leave-to {
        opacity: 0;
        transform: translateY(-10px);
    }

    .notification-enter-to,
    .notification-leave-from {
        opacity: 1;
        transform: translateY(0);
    }

    .info__title {
        font-size: 40px;
        padding-bottom: 20px;
    }

    .help {
        font-size: 25px;
    }
    
    .contact {
        display: flex;
        gap: 10px;
        align-items: center;
        padding-bottom: 10px;
    }

    .contact__icon {
        width: 20px;
        height: 20px;
    }

    .contact__text {
        font-size: 16px;
        color: var(--accent-text);
        font-weight: 500;
        margin: 0;
    }

    .contact__text--clickable {
        cursor: pointer;
        transition: opacity 0.2s ease;
    }

    .contact__text--clickable:hover {
        opacity: 0.7;
        text-decoration: underline;
        text-underline-position: under;
    }

    .contact__text:hover {
        cursor: pointer;
        text-decoration: underline;
        text-underline-position: under;
    }

    .time {
        margin-top: 5px;
        font-size: 13px;
        /* 5.74 на белой карточке; прежний #a2a2a2 давал 2.55 - ниже нормы AA.
           Островок здесь светлый, значение берётся из светлой палитры. */
        color: var(--text-muted);
    }

    .info__text {
        line-height: 150%;
        font-size: 16px;
        padding-bottom: 40px;
    }

    .login__header {
        padding-bottom: 50px;
        position: relative;
        z-index: 3;
    }

    .login__title {
        font-size: 60px;
        color: #FFFFFF;
        font-weight: 900;
        text-shadow: 4px 4px rgba(0,0,0,0.1);
    }

    .login__subtitle {
        font-size: 20px;
        color: #FFFFFF;
        font-weight: 500;
    }

    .login__form {
        display: flex;
        flex-direction: column;
        position: relative;
        z-index: 3;
    }

    .inputs {
        display: flex;
        flex-direction: column;
        gap: 20px;
        padding-bottom: 10px;
    }

    .login__input {
        width: 450px;
        height: 70px;
        border-radius: 100px;
        background-color: #F7F7F7;
        padding: 5px 30px;
        display: flex;
        align-items: center;
        justify-content: flex-start;
        gap: 15px;
    }

    .input__icon {
        width: 20px;
        height: 20px;
        transition: transform .5s;
    }

    .input {
        width: 100%;
        height: 100%;
        border: none;
        outline: none;
        font-size: 19px;
        font-weight: 500;
        background-color: transparent;
    }

    .input:-webkit-autofill,
    .input:-webkit-autofill:hover, 
    .input:-webkit-autofill:focus, 
    .input:-webkit-autofill:active {
        -webkit-box-shadow: 0 0 0 30px #F7F7F7 inset !important;
        -webkit-text-fill-color: #000 !important;
        transition: background-color 5000s ease-in-out 0s;
    }

    .input::-webkit-contacts-auto-fill-button,
    .input::-webkit-credentials-auto-fill-button {
        visibility: hidden;
        display: none !important;
        pointer-events: none;
        position: absolute;
        right: 0;
    }

    .remember-password {
        font-size: 16px;
        font-weight: 400;
        color: #FFFFFF;
        text-decoration: none;
        text-underline-position: under;
        padding-left: 30px;
        height: fit-content;
        width: fit-content;
        padding-bottom: 15px;
        position: relative;
        z-index: 3;
    }

    .remember-password:hover {
        text-decoration: underline;
    }

    .login__footer {
        display: flex;
        flex-direction: column;
        position: relative;
        z-index: 3;
    }

    .error-container {
        height: 50px;
        display: flex;
        align-items: center;
        margin-bottom: 15px;
    }

    .error-message {
        background: rgba(255, 45, 45, 0.4);
        color: #fff;
        padding: 12px 20px;
        border-radius: 20px;
        font-size: 14px;
        width: fit-content;
        font-weight: 600;
        animation: errorFadeIn 0.5s ease-out;
        backdrop-filter: blur(5px);
        /* Моноширинные цифры: плашка не дёргается пока тикает таймер (M:SS). */
        font-variant-numeric: tabular-nums;
    }

    @keyframes errorFadeIn {
        0% {
            opacity: 0;
        }
        100% {
            opacity: 1;
        }
    }

    .footer__button {
        display: flex;
        gap: 20px;
        align-items: center;
    }

    .login__button {
        width: 200px;
        height: 60px;
        background-color: #FFFFFF;
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 10px;
        border: none;
        outline: none;
        border-radius: 50px;
        transition: all 0.3s ease;
        position: relative;
    }

    .login__button:disabled {
        cursor: not-allowed;
    }

    .login__button.loading {
        width: 60px;
        border-radius: 50%;
        background-color: #7981d4;
    }

    .login__button.success {
        background-color: #FFFFFF;
    }

    .button__text {
        font-size: 20px;
        color: var(--accent-text);
        font-weight: 800;
        transition: all 0.3s ease;
        /* Моноширинные цифры: ширина таймера на кнопке стабильна. */
        font-variant-numeric: tabular-nums;
    }

    .login__button.loading .button__text {
        opacity: 0;
        transform: translateX(-20px);
        color: #FFFFFF;
    }

    .login__button.success .button__text {
        color: var(--accent-text);
    }

    /* Во время таймера блокировки кнопка выглядит выключенной: серый текст и иконка. */
    .login__button.cooling .button__text {
        color: #999;
    }

    .login__button.cooling .input__icon {
        filter: grayscale(1);
        opacity: 0.45;
    }

    .login__button:hover:not(:disabled) {
        cursor: pointer;
        background-color: var(--color-border);
    }

    .login__button:hover:not(:disabled) .button__text {
        transform: translateX(-3px);
    }

    .login__button:hover:not(:disabled) .input__icon {
        transform: translateX(30px);
    }

    .spinner {
        width: 24px;
        height: 24px;
        border: 3px solid rgba(255, 255, 255, 0.3);
        border-radius: 50%;
        border-top: 3px solid #FFFFFF;
        animation: spin 1s linear infinite;
        position: absolute;
    }

    .success-checkmark {
        font-size: 24px;
        color: var(--accent-text);
        font-weight: bold;
        animation: scaleIn 0.3s ease-out;
    }

    @keyframes scaleIn {
        0% {
            transform: scale(0);
            opacity: 0;
        }
        100% {
            transform: scale(1);
            opacity: 1;
        }
    }

    @keyframes spin {
        0% { transform: rotate(0deg); }
        100% { transform: rotate(360deg); }
    }

    .custom-lock {
        position: relative;
        width: 20px;
        height: 20px;
        transition: all 0.5s;
    }

    .lock-body {
        position: absolute;
        bottom: 0;
        width: 16px;
        height: 12px;
        background-color: #FFFFFF;
        border-radius: 2px;
        left: 2px;
        transition: all 0.3s;
    }

    .lock-arc {
        position: absolute;
        top: 0;
        left: 4px;
        width: 12px;
        height: 8px;
        border: 3px solid #FFFFFF;
        border-bottom: none;
        border-radius: 10px 10px 0 0;
        transition: all 0.3s;
    }

    .login__button.loading ~ .custom-lock .lock-arc {
        animation: lockBounce .5s ease-in-out infinite;
    }

    .custom-lock.success .lock-arc {
        border-color: #63ee59;
        animation: lockSuccess 1s ease-in-out;
    }

    .custom-lock.success .lock-body {
        background-color: #63ee59;
        animation: lockBodySuccess 1s ease-in-out;
    }

    @keyframes lockSuccess {
        0% {
            transform: translateY(0);
            border-color: #FFFFFF;
        }
        50% {
            transform: translateY(-5px);
            border-color: #63ee59;
        }
        100% {
            transform: translateY(0);
            border-color: #63ee59;
        }
    }

    @keyframes lockBodySuccess {
        0% {
            background-color: #FFFFFF;
        }
        50% {
            background-color: #63ee59;
        }
        100% {
            background-color: #63ee59;
        }
    }

    @keyframes lockBounce {
        0%, 100% {
            transform: translateY(0);
        }
        50% {
            transform: translateY(-2px);
        }
    }

    .login__button.loading ~ .custom-lock .lock-arc {
        top: -3px;
    }

    .login__button:hover:not(:disabled) ~ .custom-lock:not(.error) .lock-arc {
        top: -3px;
    }

    .custom-lock.error .lock-body,
    .custom-lock.error .lock-arc {
        border-color: #ff4d4d;
    }

    .custom-lock.error .lock-body {
        background-color: #ff4d4d;
    }

    .custom-lock.shaking {
        animation: shake 0.6s cubic-bezier(.36,.07,.19,.97) both;
    }

    @keyframes shake {
        10%, 90% { transform: translateX(-2px); }
        20%, 80% { transform: translateX(4px); }
        30%, 50%, 70% { transform: translateX(-6px); }
        40%, 60% { transform: translateX(6px); }
    }

    .fade-enter-active, .fade-leave-active {
        transition: opacity 0.3s;
    }
    .fade-enter, .fade-leave-to {
        opacity: 0;
    }

    /* Tablet и меньше: stacked layout (форма сверху, info снизу) */
    @media (max-width: 1024px) {
        /* Прокрутку stacked-раскладки держит документ, а не .login__container:
           свой скролл-контейнер клипал по СВОЕЙ рамке (уже экрана на padding),
           и элементы, выезжающие по X из-за края экрана, обрезались внутри него. */
        .login {
            height: auto;
            min-height: calc(var(--app-vh, 1vh) * 100);
            min-height: min(calc(var(--app-vh, 1vh) * 100), 100svh);
        }

        .login__container {
            flex-direction: column;
            align-items: center;
            gap: 30px;
            padding-bottom: 40px;
        }

        .login__info {
            width: 100%;
            max-width: 550px;
            margin-top: 0;
        }
    }

    /* Mobile: <768px */
    @media (max-width: 768px) {
        .login {
            padding: 24px 16px;
        }

        /* На телефоне низ экрана занимает карточка контактов, и горизонт целиком
           уходил под неё. Сцена растягивается и приподнимается, чтобы кромка с
           деревьями осталась в видимой полосе над карточкой. */
        .login-scene {
            height: 122%;
            top: -22%;
        }

        .login__header {
            padding-bottom: 30px;
        }

        .login__title {
            font-size: 40px;
        }

        .login__subtitle {
            font-size: 16px;
        }

        .login__input {
            width: 100%;
            max-width: 100%;
            height: 60px;
            padding: 5px 20px;
        }

        .input {
            font-size: 16px;
        }

        .remember-password {
            padding-left: 20px;
            font-size: 14px;
        }

        .login__info {
            height: auto;
            border-radius: 30px;
            padding: 30px 24px;
        }

        .info__title {
            font-size: 28px;
            padding-bottom: 14px;
        }

        .help {
            font-size: 20px;
        }

        .info__text {
            font-size: 14px;
            padding-bottom: 24px;
        }

        .contact__text {
            font-size: 14px;
        }

        .time {
            font-size: 12px;
        }

        /* Floating shapes занимают место и тормозят на мобильных - убираем */
        .floating-shape {
            display: none;
        }
    }

    /* Mobile small: <=480px */
    @media (max-width: 480px) {
        .login {
            padding: 20px 14px;
        }

        .login__header {
            padding-bottom: 24px;
        }

        .login__title {
            font-size: 32px;
            text-shadow: 2px 2px rgba(0,0,0,0.1);
        }

        .login__subtitle {
            font-size: 14px;
        }

        .inputs {
            gap: 14px;
        }

        .login__input {
            height: 56px;
            border-radius: 30px;
            padding: 5px 18px;
        }

        .error-container {
            height: auto;
            min-height: 40px;
            margin-bottom: 12px;
        }

        .error-message {
            font-size: 13px;
            padding: 10px 16px;
        }

        .footer__button {
            gap: 12px;
        }

        .login__button {
            width: 170px;
            height: 54px;
        }

        .button__text {
            font-size: 18px;
        }

        .login__info {
            border-radius: 24px;
            padding: 24px 18px;
        }

        .info__title {
            font-size: 24px;
        }

        .help {
            font-size: 18px;
        }
    }
</style>