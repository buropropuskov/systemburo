<template>
    <div class="login" @mousemove="handleMouseMove">
        <div class="login-background">
            <div class="floating-shape shape-1"></div>
            <div class="floating-shape shape-2"></div>
            <div class="floating-shape shape-3"></div>
            <div class="floating-shape shape-4"></div>
            <div class="floating-shape shape-5"></div>
            <div class="floating-shape shape-6"></div>
            <div class="floating-shape shape-7"></div>
        </div>
        
        <div class="background-image" :style="parallaxStyle"></div>
        
        <div class="login__container">
            <div class="login__active">
                <div class="login__header">
                    <h1 class="login__title" :style="titleStyle">Войдите в аккаунт</h1>
                    <h3 class="login__subtitle" :style="subtitleStyle">для продолжения</h3>
                </div>
                <form class="login__form" autocomplete="off" @submit.prevent="handleSubmit">
                    <!-- Скрытое поле для обмана браузерного автозаполнения -->
                    <input type="text" style="display:none" autocomplete="off" />
                    
                    <div class="inputs">
                        <div class="login__input" :style="input1Style">
                            <img src="@/assets/icons/login.png" alt="" class="input__icon" />
                            <input 
                                v-model="formData.username" 
                                class="input" 
                                type="text" 
                                autocomplete="off" 
                                autocorrect="off" 
                                autocapitalize="off" 
                                spellcheck="false"
                                placeholder="Логин"
                                :name="'username_' + randomSuffix"
                                @keydown="preventSqlInjection"
                                @paste="preventSqlInjectionPaste"
                            />
                        </div>
                        <div class="login__input" :style="input2Style">
                            <img src="@/assets/icons/password.png" alt="" class="input__icon" />
                            <input 
                                v-model="formData.password" 
                                class="input" 
                                type="password" 
                                autocomplete="new-password" 
                                autocorrect="off" 
                                autocapitalize="off" 
                                spellcheck="false"
                                placeholder="Пароль"
                                :name="'password_' + randomSuffix"
                            />
                        </div>
                    </div>
                    
                    <a href="#" class="remember-password" :style="linkStyle" @click.prevent="openForgotModal">Забыли пароль?</a>
                    
                    <div class="login__footer" :style="footerStyle">
                        <div class="error-container">
                            <transition name="fade">
                                <div v-if="showError" class="error-message">
                                    {{ errors.general }}
                                </div>
                            </transition>
                        </div>
                        
                        <div class="footer__button">
                            <button class="login__button" :class="{'loading': isLoading, 'success': isSuccess}" :disabled="isLoading || isSuccess">
                                <p class="button__text">{{ getButtonText }}</p>
                                <img v-if="!isLoading && !isSuccess" src="@/assets/icons/key-blue.png" alt="" class="input__icon"/>
                                <div v-if="isLoading" class="spinner"></div>
                            </button>
                            <div class="custom-lock" :class="{'error': hasError, 'shaking': isShaking, 'success': isSuccess}">
                                <div class="lock-arc"></div>
                                <div class="lock-body"></div>
                            </div>
                        </div>
                    </div>
                </form>
            </div>
            <div class="login__info" :style="infoStyle">
                <!-- Единое уведомление с out-in анимацией -->
                <div class="info-notifications">
                    <transition name="notification" mode="out-in">
                        <div v-if="showNotification" class="notification" :key="notificationText">
                            {{ notificationText }}
                        </div>
                    </transition>
                </div>

                <h2 class="info__title">Добро пожаловать!</h2>
                <p class="info__text">
                    Для продолжения войдите в свою учётную запись,
                    используя выданные вам данные.
                </p>
                <h3 class="info__title help">Помощь и поддержка</h3>
                <p class="info__text">
                  Обратитесь к нам, чтобы получить учётную запись, восстановить доступ или решить другие проблемы:
                </p>
                <div class="info__contacts">
                    <div class="contact">
                        <img src="@/assets/icons/email-blue.png" class="contact__icon" alt="" />
                        <p class="contact__text" @click="copyEmail">buropropuskov@dreamisland.ru</p>
                    </div>
                    <div class="contact">
                        <img src="@/assets/icons/phone-blue.png" class="contact__icon" alt="" />
                        <p class="contact__text" @click="copyPhone">+7 (910) 083 00-55</p>
                    </div>
                    <p class="time">
                        ПН-ПТ: <strong>08:00 - 22:00</strong> СБ-ВС и ПРАЗДНИКИ: <strong>08:00 - 20:00</strong>
                    </p>
                </div>
            </div>
        </div>

        <!-- Модальное окно восстановления пароля -->
        <PasswordRecoveryModal 
            :show="showForgotModal" 
            @close="closeForgotModal" 
        />
    </div>
</template>

<script>
import PasswordRecoveryModal from './PasswordRecoveryModal.vue';

export default {
    components: {
        PasswordRecoveryModal
    },
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
            // Единое уведомление для основной панели
            showNotification: false,
            notificationText: '',
            notificationTimeout: null,
            // Модальное окно
            showForgotModal: false,
            resizeTimeout: null,
            randomSuffix: Math.random().toString(36).substring(2, 10) // уникальный суффикс для name
        }
    },
    computed: {
        getButtonText() {
            if (this.isLoading) return 'Вход...';
            if (this.isSuccess) return 'Успешно!';
            return 'Войти';
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
                transitionDelay: '0.1s',
                willChange: 'transform, opacity'
            };
        },
        subtitleStyle() {
            return {
                opacity: this.elementsVisible ? 1 : 0,
                transform: this.elementsVisible ? 'translateY(0)' : 'translateY(20px)',
                transition: 'opacity 0.6s ease, transform 0.6s ease',
                transitionDelay: '0.2s',
                willChange: 'transform, opacity'
            };
        },
        input1Style() {
            return {
                opacity: this.elementsVisible ? 1 : 0,
                transform: this.elementsVisible ? 'translateX(0)' : 'translateX(-30px)',
                transition: 'opacity 0.6s ease, transform 0.6s ease',
                transitionDelay: '0.3s',
                willChange: 'transform, opacity'
            };
        },
        input2Style() {
            return {
                opacity: this.elementsVisible ? 1 : 0,
                transform: this.elementsVisible ? 'translateX(0)' : 'translateX(-30px)',
                transition: 'opacity 0.6s ease, transform 0.6s ease',
                transitionDelay: '0.4s',
                willChange: 'transform, opacity'
            };
        },
        linkStyle() {
            return {
                opacity: this.elementsVisible ? 1 : 0,
                transition: 'opacity 0.6s ease',
                transitionDelay: '0.5s',
                willChange: 'opacity'
            };
        },
        footerStyle() {
            return {
                opacity: this.elementsVisible ? 1 : 0,
                transform: this.elementsVisible ? 'translateY(0)' : 'translateY(30px)',
                transition: 'opacity 0.6s ease, transform 0.6s ease',
                transitionDelay: '0.6s',
                willChange: 'transform, opacity'
            };
        },
        infoStyle() {
            return {
                opacity: this.elementsVisible ? 1 : 0,
                transform: this.elementsVisible ? 'translateX(0)' : 'translateX(30px)',
                transition: 'opacity 0.8s ease, transform 0.8s ease',
                transitionDelay: '0.7s',
                willChange: 'transform, opacity'
            };
        }
    },
    mounted() {
        setTimeout(() => {
            this.elementsVisible = true;
        }, 100);
        
        window.addEventListener('resize', this.handleResize);
    },
    methods: {
        handleMouseMove(e) {
            this.mouseX = e.clientX;
            this.mouseY = e.clientY;
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
        
        // Уведомление для основной панели (единое)
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
        
        async copyEmail(event) {
            event.stopPropagation();
            const email = 'buropropuskov@dreamisland.ru';
            try {
                await navigator.clipboard.writeText(email);
                this.showNotificationMessage('E-mail скопирован');
            } catch (err) {
                const textArea = document.createElement('textarea');
                textArea.value = email;
                document.body.appendChild(textArea);
                textArea.select();
                document.execCommand('copy');
                document.body.removeChild(textArea);
                this.showNotificationMessage('E-mail скопирован');
            }
        },
        
        async copyPhone(event) {
            event.stopPropagation();
            const phone = '+7 (910) 083 00-55';
            try {
                await navigator.clipboard.writeText(phone);
                this.showNotificationMessage('Номер телефона скопирован');
            } catch (err) {
                const textArea = document.createElement('textarea');
                textArea.value = phone;
                document.body.appendChild(textArea);
                textArea.select();
                document.execCommand('copy');
                document.body.removeChild(textArea);
                this.showNotificationMessage('Номер телефона скопирован');
            }
        },
        
        // Методы для модального окна
        openForgotModal() {
            this.showForgotModal = true;
        },
        
        closeForgotModal() {
            this.showForgotModal = false;
        },
        
        // Обработчик ресайза с debounce
        handleResize() {
            document.body.classList.add('no-transition');
            if (this.resizeTimeout) {
                clearTimeout(this.resizeTimeout);
            }
            this.resizeTimeout = setTimeout(() => {
                document.body.classList.remove('no-transition');
            }, 150);
        },
        
        // Предотвращение ввода опасных символов на лету (для логина)
        preventSqlInjection(e) {
            const key = e.key;
            // Разрешены: буквы (латиница), цифры, дефис, подчёркивание, точка
            const allowedRegex = /^[a-zA-Z0-9\-_.]$/;
            // Управляющие клавиши (Backspace, Delete, Tab, стрелки и т.д.) разрешаем
            const controlKeys = ['Backspace', 'Delete', 'Tab', 'ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End', 'Enter', 'Escape'];
            if (controlKeys.includes(key) || key.startsWith('F') && key.length === 2) {
                return;
            }
            // Если символ не разрешён, предотвращаем ввод
            if (!allowedRegex.test(key) && key.length === 1) {
                e.preventDefault();
                // Можно показать подсказку, но не обязательно
            }
        },
        
        // Предотвращение вставки опасных символов
        preventSqlInjectionPaste(e) {
            // Получаем вставляемый текст
            const pastedText = (e.clipboardData || window.clipboardData).getData('text');
            // Оставляем только разрешённые символы
            const cleaned = pastedText.replace(/[^a-zA-Z0-9\-_.]/g, '');
            if (cleaned !== pastedText) {
                e.preventDefault();
                // Вставляем очищенный текст
                const start = this.formData.username.substring(0, e.target.selectionStart);
                const end = this.formData.username.substring(e.target.selectionEnd);
                this.formData.username = start + cleaned + end;
                // Можно показать уведомление, что символы удалены
            }
        },
        
        // Санитизация логина перед отправкой
        sanitizeUsername(username) {
            // Удаляем все символы, кроме разрешённых (буквы, цифры, дефис, подчёркивание, точка)
            return username.replace(/[^a-zA-Z0-9\-_.]/g, '');
        },
        
        async handleSubmit() {
            this.resetAnimations();
            this.errors.general = '';
            
            await new Promise(resolve => setTimeout(resolve, 100));
            
            if (!this.formData.username || !this.formData.password) {
                this.errors.general = 'Необходимо заполнить все поля';
                await this.showErrorWithDelay();
                return;
            }
            
            // Санитизация логина (удаляем недопустимые символы)
            const sanitizedUsername = this.sanitizeUsername(this.formData.username);
            if (sanitizedUsername !== this.formData.username) {
                // Если были удалены символы, показываем предупреждение и обновляем поле
                this.formData.username = sanitizedUsername;
                this.errors.general = 'Логин содержит недопустимые символы. Разрешены только буквы, цифры, дефис, подчёркивание и точка.';
                await this.showErrorWithDelay();
                return;
            }
            
            this.isLoading = true;
            let timeoutId;
            
            try {
                const controller = new AbortController();
                timeoutId = setTimeout(() => controller.abort(), 10000);
                
                const response = await fetch("http://localhost:8080/login", {
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
                    
                    localStorage.setItem("token", data.token);
                    localStorage.setItem("refreshToken", data.refreshToken);
                    
                    this.isLoading = false;
                    this.isSuccess = true;
                    
                    await new Promise(resolve => setTimeout(resolve, 1500));
                    
                    this.$emit('login-success', {
                        token: data.token,
                        refreshToken: data.refreshToken
                    });
                    
                    this.$root.$forceUpdate(); 
                   this.$router.push('/news');
                } else {
                    if (response.status === 429) {
                        this.errors.general = "Вы отправляете слишком много запросов. Пожалуйста, подождите.";
                    } else if (response.status === 401) {
                        this.errors.general = "Неверный логин и/или пароль";
                    } else {
                        try {
                            const errorText = await response.text();
                            if (errorText) {
                                try {
                                    const errorData = JSON.parse(errorText);
                                    this.errors.general = errorData.message || errorData || "Произошла ошибка";
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
                
                if (error.name === 'AbortError') {
                    this.errors.general = "Таймаут запроса. Сервер не отвечает.";
                } else if (error.toString().includes("Failed to fetch") || error.toString().includes("NetworkError")) {
                    this.errors.general = "Ошибка сети. Проверьте подключение и повторите позже.";
                } else if (error.toString().includes("Too many requests") || error.toString().includes("429")) {
                    this.errors.general = "Вы отправляете слишком много запросов. Подождите.";
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
        if (this.resizeTimeout) {
            clearTimeout(this.resizeTimeout);
        }
        window.removeEventListener('resize', this.handleResize);
        document.body.classList.remove('no-transition');
    }
}
</script>

<style scoped>
/* Глобальное правило для отключения transition и анимаций при ресайзе */
:global(body.no-transition *),
:global(body.no-transition *::before),
:global(body.no-transition *::after) {
    transition: none !important;
    animation: none !important;
}

/* Все размеры переведены в vh относительно базовой высоты 648px */
.login {
    width: 100%;
    height: 100vh;
    background-color: #4F5BDF;
    padding: 6.17vh; /* 40px / 6.48 ≈ 6.17 */
    display: flex;
    position: relative;
    perspective: 1000px;
    overflow: hidden;
}

.login-background {
    position: absolute;
    width: 100%;
    height: 100%;
    top: 0;
    left: 0;
    z-index: 0;
}

.background-image {
    position: absolute;
    width: 100%;
    height: 100%;
    top: 0;
    left: 0;
    background-image: url('@/assets/background.png');
    background-size: cover;
    background-position: center;
    background-repeat: no-repeat;
    opacity: 0.35;
    z-index: 1;
    will-change: transform;
}

.floating-shape {
    position: absolute;
    border-radius: 50%;
    background: rgba(255, 255, 255, 0.3);
    animation: float 15s infinite ease-in-out;
    z-index: 2;
    box-shadow: 0px 0.46vh 1.54vh rgba(0,0,0,0.1);
    will-change: transform;
}

.shape-1 {
    width: 38.58vh;
    height: 38.58vh;
    top: -7.72vh;
    left: -7.72vh;
    animation-delay: 0s;
}

.shape-2 {
    width: 23.15vh;
    height: 23.15vh;
    bottom: 7.72vh;
    right: 15.43vh;
    animation-delay: -5s;
}

.shape-3 {
    width: 15.43vh;
    height: 15.43vh;
    top: 50%;
    left: 70%;
    animation-delay: -10s;
}

.shape-4 {
    width: 12.35vh;
    height: 12.35vh;
    top: 20%;
    right: 10%;
    animation-delay: -2s;
    background: rgba(255, 255, 255, 0.15);
    box-shadow: 0px 0.62vh 2.31vh rgba(0,0,0,0.2);
}

.shape-5 {
    width: 18.52vh;
    height: 18.52vh;
    bottom: 40%;
    left: 15%;
    animation-delay: -7s;
    background: rgba(255, 255, 255, 0.08);
    box-shadow: 0px 0.77vh 3.09vh rgba(0,0,0,0.15);
}

.shape-6 {
    width: 9.26vh;
    height: 9.26vh;
    top: 70%;
    right: 25%;
    animation-delay: -12s;
    background: rgba(255, 255, 255, 0.12);
    box-shadow: 0px 0.46vh 1.85vh rgba(0,0,0,0.18);
}

.shape-7 {
    width: 27.78vh;
    height: 27.78vh;
    bottom: -4.63vh;
    right: -4.63vh;
    animation-delay: -8s;
    background: rgba(255, 255, 255, 0.05);
    box-shadow: 0px 0.93vh 3.86vh rgba(0,0,0,0.1);
}

@keyframes float {
    0%, 100% {
        transform: translate(0, 0) rotate(0deg);
    }
    33% {
        transform: translate(4.63vh, -7.72vh) rotate(120deg);
    }
    66% {
        transform: translate(-3.09vh, 3.09vh) rotate(240deg);
    }
}

.login__container {
    display: flex;
    width: 100%;
    justify-content: space-between;
}

.login__info {
    width: 84.88vh;
    height: 74.07vh;
    background-color: rgba(255,255,255,0.9);
    border-radius: 15.43vh;
    box-shadow: 0 0.46vh 1.54vh rgba(0,0,0,0.25);
    z-index: 1000;
    padding: 7.72vh;
    margin-top: 7.72vh;
    position: relative;
}

.info-notifications {
    position: absolute;
    top: -5.4vh;
    left: 50%;
    transform: translateX(-50%);
    z-index: 5000;
    pointer-events: none;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1.23vh;
}

.notification {
    height: 3.86vh;
    border-radius: 7.72vh;
    background-color: #fff;
    font-size: 2.16vh;
    color: #000;
    font-weight: 500;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0 2.31vh;
    box-shadow: 0 0.46vh 1.54vh rgba(0,0,0,0.2);
    min-width: 23.15vh;
    white-space: nowrap;
    will-change: transform, opacity;
    transition: opacity 0.3s ease, transform 0.3s ease;
}

.notification-enter-active, .notification-leave-active {
    transition: opacity 0.3s ease, transform 0.3s ease;
}

.notification-enter-from, .notification-leave-to {
    opacity: 0;
    transform: translateY(-1.54vh);
}

.notification-enter-to, .notification-leave-from {
    opacity: 1;
    transform: translateY(0);
}

.info__title {
    font-size: 6.17vh;
    padding-bottom: 3.09vh;
}

.help {
    font-size: 3.7vh;
}

.contact {
    display: flex;
    gap: 1.54vh;
    align-items: center;
    padding-bottom: 1.54vh;
}

.contact__text {
    font-size: 2.47vh;
    color: #4F5BDF;
    font-weight: 500;
    cursor: pointer;
    transition: opacity 0.2s ease;
}

.contact__text:hover {
    opacity: 0.8;
    text-decoration: underline;
    text-underline-position: under;
}

.contact__icon {
    width: 3.09vh;
    height: 3.09vh;
}

.time {
    margin-top: 0.77vh;
    font-size: 2.01vh;
    color: #a2a2a2;
}

.info__text {
    line-height: 150%;
    font-size: 2.47vh;
    padding-bottom: 6.17vh;
}

.login__header {
    padding-bottom: 7.72vh;
    position: relative;
    z-index: 3;
}

.login__title {
    font-size: 9.26vh;
    color: #FFFFFF;
    font-weight: 900;
    text-shadow: 0.62vh 0.62vh rgba(0,0,0,0.1);
}

.login__subtitle {
    font-size: 3.09vh;
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
    gap: 3.09vh;
    padding-bottom: 1.54vh;
}

.login__input {
    width: 69.44vh;
    height: 10.8vh;
    border-radius: 15.43vh;
    background-color: #F7F7F7;
    padding: 0.77vh 4.63vh;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 2.31vh;
}

.input__icon {
    width: 3.09vh;
    height: 3.09vh;
    transition: transform 0.5s;
    will-change: transform;
}

.input {
    width: 100%;
    height: 100%;
    border: none;
    outline: none;
    font-size: 2.93vh;
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
    font-size: 2.47vh;
    font-weight: 400;
    color: #FFFFFF;
    text-decoration: none;
    text-underline-position: under;
    padding-left: 4.63vh;
    height: fit-content;
    width: fit-content;
    padding-bottom: 2.31vh;
    position: relative;
    z-index: 3;
    cursor: pointer;
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
    height: 7.72vh;
    display: flex;
    align-items: center;
    margin-bottom: 2.31vh;
}

.error-message {
    background: rgba(255, 45, 45, 0.4);
    color: #fff;
    padding: 1.85vh 3.09vh;
    border-radius: 3.09vh;
    font-size: 2.16vh;
    width: fit-content;
    font-weight: 600;
    backdrop-filter: blur(5px);
    animation: errorFadeIn 0.5s ease-out;
}

@keyframes errorFadeIn {
    0% { opacity: 0; }
    100% { opacity: 1; }
}

.footer__button {
    display: flex;
    gap: 3.09vh;
    align-items: center;
}

.login__button {
    width: 30.86vh;
    height: 9.26vh;
    background-color: #FFFFFF;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 1.54vh;
    border: none;
    outline: none;
    border-radius: 7.72vh;
    transition: background-color 0.3s ease, width 0.3s ease, border-radius 0.3s ease, transform 0.3s ease;
    position: relative;
    will-change: width, background-color, border-radius, transform;
}

.login__button:disabled {
    cursor: not-allowed;
}

.login__button.loading {
    width: 9.26vh;
    border-radius: 50%;
    background-color: #7981d4;
}

.login__button.success {
    background-color: #FFFFFF;
}

.button__text {
    font-size: 3.09vh;
    color: #4F5BDF;
    font-weight: 800;
    transition: opacity 0.3s ease, transform 0.3s ease, color 0.3s ease;
    will-change: transform, opacity, color;
}

.login__button.loading .button__text {
    opacity: 0;
    transform: translateX(-3.09vh);
    color: #FFFFFF;
}

.login__button.success .button__text {
    color: #4F5BDF;
}

.login__button:hover:not(:disabled) {
    cursor: pointer;
    background-color: #e6e6e6;
}

.login__button:hover:not(:disabled) .button__text {
    transform: translateX(-0.46vh);
}

.login__button:hover:not(:disabled) .input__icon {
    transform: translateX(4.63vh);
}

.spinner {
    width: 3.7vh;
    height: 3.7vh;
    border: 0.46vh solid rgba(255, 255, 255, 0.3);
    border-radius: 50%;
    border-top: 0.46vh solid #FFFFFF;
    animation: spin 1s linear infinite;
    position: absolute;
}

.success-checkmark {
    font-size: 3.7vh;
    color: #4F5BDF;
    font-weight: bold;
    animation: scaleIn 0.3s ease-out;
}

@keyframes scaleIn {
    0% { transform: scale(0); opacity: 0; }
    100% { transform: scale(1); opacity: 1; }
}

@keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
}

.custom-lock {
    position: relative;
    width: 3.09vh;
    height: 3.09vh;
    transition: border-color 0.3s ease, background-color 0.3s ease, transform 0.3s ease;
    will-change: transform;
}

.lock-body {
    position: absolute;
    bottom: 0;
    width: 2.47vh;
    height: 1.85vh;
    background-color: #FFFFFF;
    border-radius: 0.31vh;
    left: 0.31vh;
    transition: background-color 0.3s ease;
}

.lock-arc {
    position: absolute;
    top: 0;
    left: 0.62vh;
    width: 1.85vh;
    height: 1.23vh;
    border: 0.46vh solid #FFFFFF;
    border-bottom: none;
    border-radius: 1.54vh 1.54vh 0 0;
    transition: border-color 0.3s ease, top 0.3s ease;
}

.login__button:hover:not(:disabled) ~ .custom-lock .lock-arc {
    top: -0.46vh;
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
        transform: translateY(-0.77vh);
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
        transform: translateY(-0.31vh);
    }
}

.login__button.loading ~ .custom-lock .lock-arc {
    top: -0.46vh;
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
    10%, 90% { transform: translateX(-0.31vh); }
    20%, 80% { transform: translateX(0.62vh); }
    30%, 50%, 70% { transform: translateX(-0.93vh); }
    40%, 60% { transform: translateX(0.93vh); }
}

.fade-enter-active, .fade-leave-active {
    transition: opacity 0.3s;
}
.fade-enter, .fade-leave-to {
    opacity: 0;
}
</style>