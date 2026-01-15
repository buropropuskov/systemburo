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
                <form class="login__form" @submit.prevent="handleSubmit">
                    <div class="inputs">
                        <div class="login__input" :style="input1Style">
                            <img src="@/assets/icons/login.png" alt="" class="input__icon" />
                            <input v-model="formData.username" class="input" type="text" 
                                autocomplete="off" 
                                autocorrect="off" 
                                autocapitalize="off" 
                                spellcheck="false"
                                placeholder="Логин" />
                        </div>
                        <div class="login__input" :style="input2Style">
                            <img src="@/assets/icons/password.png" alt="" class="input__icon" />
                            <input v-model="formData.password" class="input" type="password" 
                                autocomplete="new-password" 
                                autocorrect="off" 
                                autocapitalize="off" 
                                spellcheck="false"
                                placeholder="Пароль" />
                        </div>
                    </div>
                    
                    <a href="#" class="remember-password" :style="linkStyle">Забыли пароль?</a>
                    
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
                <h2 class="info__title">Добро пожаловать!</h2>
                <p class="info__text">
                    Для продолжения, необходимо войти в аккаунт,
                    используя выданные Вам данные.
                </p>
                <h3 class="info__title help">Помощь и поддержка</h3>
                <p class="info__text">
                  Обращайтесь к нам, чтобы получить учётную запись, восстановить доступ или решить другие проблемы:
                </p>
                <div class="info__contacts">
                    <div class="contact">
                        <img src="@/assets/icons/email-blue.png" class="contact__icon" alt="" />
                        <p class="contact__text">buropropuskov@dreamisland.ru</p>
                    </div>
                    <div class="contact">
                        <img src="@/assets/icons/phone-blue.png" class="contact__icon" alt="" />
                        <p class="contact__text">+7 (910) 083 00-55</p>
                    </div>
                    <p class="time">
                        ПН-ПТ: <strong>08:00 - 22:00</strong> СБ-ВС и ПРАЗДНИКИ: <strong>08:00 - 20:00</strong>
                    </p>
                </div>
            </div>
        </div>
    </div>
</template>

<script>
export default {
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
            mouseX: 0,
            mouseY: 0,
            elementsVisible: false
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
            
            this.showError = false;
            this.isLoading = false;
            this.isSuccess = false;
            this.hasError = false;
            this.isShaking = false;
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
            
            this.isLoading = true;
            
            try {
                const response = await fetch("http://localhost:8080/login", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ 
                        username: this.formData.username, 
                        password: this.formData.password 
                    }),
                });

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
                    this.$router.push('/personal-cabinet');
                } else {
                    const errorData = await response.json();
                    console.error("Login error:", errorData);
                    this.errors.general = errorData.message || "Неверный логин и/или пароль";
                    this.isLoading = false;
                    await this.showErrorWithDelay();
                }
            } catch (error) {
                console.error("Network error:", error);
                this.errors.general = "Ошибка сети";
                this.isLoading = false;
                await this.showErrorWithDelay();
            }
        },
        
        async showErrorWithDelay() {
            await new Promise(resolve => setTimeout(resolve, 100));
            this.showError = true;
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
                    this.resetAnimations();
                }, 300);
            }, 700);
        },
    },
    beforeUnmount() {
        if (this.animationTimeout) {
            clearTimeout(this.animationTimeout);
        }
    }
}
</script>

<style scoped>
    .login {
        width: 100%;
        height: 100vh;
        background-color: #4F5BDF;
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
        transition: transform 0.1s ease-out;
        will-change: transform;
    }

    .floating-shape {
        position: absolute;
        border-radius: 50%;
        background: rgba(255, 255, 255, 0.3);
        animation: float 15s infinite ease-in-out;
        z-index: 2;
        box-shadow: 0px 3px 10px rgba(0,0,0,0.1);
    }

    .shape-1 {
        width: 250px;
        height: 250px;
        top: -50px;
        left: -50px;
        animation-delay: 0s;
    }

    .shape-2 {
        width: 150px;
        height: 150px;
        bottom: 50px;
        right: 100px;
        animation-delay: -5s;
    }

    .shape-3 {
        width: 100px;
        height: 100px;
        top: 50%;
        left: 70%;
        animation-delay: -10s;
    }

    .shape-4 {
    width: 80px;
    height: 80px;
    top: 20%;
    right: 10%;
    animation-delay: -2s;
    background: rgba(255, 255, 255, 0.15);
    box-shadow: 0px 4px 15px rgba(0,0,0,0.2);
}

.shape-5 {
    width: 120px;
    height: 120px;
    bottom: 40%;
    left: 15%;
    animation-delay: -7s;
    background: rgba(255, 255, 255, 0.08);
    box-shadow: 0px 5px 20px rgba(0,0,0,0.15);
}

.shape-6 {
    width: 60px;
    height: 60px;
    top: 70%;
    right: 25%;
    animation-delay: -12s;
    background: rgba(255, 255, 255, 0.12);
    box-shadow: 0px 3px 12px rgba(0,0,0,0.18);
}

.shape-7 {
    width: 180px;
    height: 180px;
    bottom: -30px;
    right: -30px;
    animation-delay: -8s;
    background: rgba(255, 255, 255, 0.05);
    box-shadow: 0px 6px 25px rgba(0,0,0,0.1);
}

    @keyframes float {
        0%, 100% {
            transform: translate(0, 0) rotate(0deg);
        }
        33% {
            transform: translate(30px, -50px) rotate(120deg);
        }
        66% {
            transform: translate(-20px, 20px) rotate(240deg);
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
    }

    .info__title {
        font-size: 40px;
        padding-bottom: 20px;
    }

    .help {
        font-size: 30px;
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
        color: #4F5BDF;
        font-weight: 500;
    }

    .contact__text:hover {
        cursor: pointer;
        text-decoration: underline;
        text-underline-position: under;
    }

    .time {
        margin-top: 5px;
        font-size: 13px;
        color: #a2a2a2;
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
        font-weight: 400;
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
        justify-content: center;
        gap: 10px;
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
        font-size: 18px;
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
        background: rgba(255, 45, 45, 0.2);
        color: #ff4d4d;
        padding: 12px 20px;
        border-radius: 10px;
        border-left: 6px solid #ff4d4d;
        width: 440px;
        font-weight: 600;
        animation: errorFadeIn 0.5s ease-out;
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
        color: #4F5BDF;
        font-weight: 800;
        transition: all 0.3s ease;
    }

    .login__button.loading .button__text {
        opacity: 0;
        transform: translateX(-20px);
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
        color: #4F5BDF;
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
            transform: translateY(-3px);
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
</style>