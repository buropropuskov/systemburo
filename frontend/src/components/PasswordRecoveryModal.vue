<template>
    <transition name="modal-fade">
        <div v-if="show" class="modal-overlay" @click.self="$emit('close')">
            <div class="modal-content">
                <h2 class="modal-title">Восстановление доступа</h2>

                <p class="modal-text">
                    Если вы забыли логин или пароль учётной записи, напишите нам или позвоните:
                </p>

                <div class="modal-contacts">
                    <div class="contact" @click="copyEmail">
                        <img src="@/assets/icons/email-blue.png" class="contact__icon" alt="" />
                        <p class="contact__text">buropropuskov@dreamisland.ru</p>
                    </div>
                    <div class="contact" @click="copyPhone">
                        <img src="@/assets/icons/phone-blue.png" class="contact__icon" alt="" />
                        <p class="contact__text">+7 (910) 083 00-55</p>
                    </div>
                </div>

                <button class="modal-button" @click="$emit('close')">Понятно</button>

                <!-- Единое уведомление с out-in анимацией -->
                <div class="modal-notifications">
                    <transition name="notification" mode="out-in">
                        <div v-if="showNotification" class="notification" :key="notificationText">
                            {{ notificationText }}
                        </div>
                    </transition>
                </div>
            </div>
        </div>
    </transition>
</template>

<script>
export default {
    name: 'PasswordRecoveryModal',
    props: {
        show: {
            type: Boolean,
            required: true
        }
    },
    data() {
        return {
            showNotification: false,
            notificationText: '',
            notificationTimeout: null
        };
    },
    methods: {
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
        }
    },
    beforeUnmount() {
        if (this.notificationTimeout) {
            clearTimeout(this.notificationTimeout);
        }
    }
};
</script>

<style scoped>
.modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background-color: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 4000;
}

.modal-content {
    background: rgba(255,255,255,1);
    border-radius: 7.72vh;
    width: 90%;
    max-width: 71vh;
    padding: 6.17vh;
    position: relative;
    backdrop-filter: blur(3px);
}

.modal-close {
    position: absolute;
    top: 3.09vh;
    right: 3.09vh;
    background: none;
    border: none;
    cursor: pointer;
    color: #999;
    padding: 1.23vh;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    transition: background-color 0.2s;
}

.modal-close:hover {
    background-color: #f0f0f0;
    color: #333;
}

.modal-title {
    font-size: 4.94vh;
    font-weight: 800;
    color: #333;
    padding: 1vh 3.3vh;
    margin-bottom: 4.63vh;
    text-align: center;
    border: 0.31vh solid #e6e6e6;
    border-radius: 7.72vh;
}

.modal-text {
    font-size: 2.47vh;
    line-height: 1.5;
    font-weight: 400;
    color: #000;
    margin-bottom: 3.7vh;
    text-align: center;
}

.modal-contacts {
    display: flex;
    flex-direction: column;
    gap: 1.54vh;
    margin-bottom: 4.94vh;
}

.modal-contacts .contact {
    display: flex;
    gap: 1.54vh;
    align-items: center;
    justify-content: center;
    padding-bottom: 0;
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

.modal-button {
    background: #4F5BDF;
    color: white;
    border: none;
    border-radius: 6.17vh;
    padding: 2.16vh 6.17vh;
    font-size: 2.78vh;
    font-weight: 600;
    cursor: pointer;
    width: 100%;
    max-width: 30.86vh;
    margin: 0 auto;
    display: block;
    transition: background-color 0.2s;
}

.modal-button:hover {
    background-color: #3f4bc9;
}

.modal-notifications {
    position: absolute;
    top: -5.4vh;
    left: 50%;
    transform: translateX(-50%);
    z-index: 10;
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

.modal-fade-enter-active, .modal-fade-leave-active {
    transition: opacity 0.3s ease;
}
.modal-fade-enter-from, .modal-fade-leave-to {
    opacity: 0;
}
.modal-fade-enter-to, .modal-fade-leave-from {
    opacity: 1;
}
</style>