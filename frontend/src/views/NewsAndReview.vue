<template>
    <section class="news">
        <div class="news-container">
            <div class="news-header">
                <h2 class="news-title">Последние новости и обновления</h2>
                <RefreshButton />
            </div>

            <div class="divider"></div>

            <div class="news-list">
                <article 
                    v-for="(news, index) in newsList" 
                    :key="index"
                    class="news-item"
                >
                    <time class="news-date">{{ formatDate(news.date) }}</time>
                    <h3 class="news-item-title">{{ news.title }}</h3>
                    <p class="news-item-description">
                        {{ news.description }}
                    </p>
                    <button 
                        class="news-details-button"
                        @click="handleDetailsClick(news)"
                    >
                        Подробнее
                    </button>
                    
                    <div 
                        v-if="index < newsList.length - 1" 
                        class="divider divider-middle"
                    ></div>
                </article>
            </div>
        </div>
    </section>
</template>

<script>
import RefreshButton from '../components/RefreshButton.vue'

export default {
    name: 'LatestNews',
    components: {
        RefreshButton
    },
    data() {
        return {
            newsList: [
                {
                    date: '2025-08-22T11:00:00',
                    title: 'Новый раздел "Сотрудники"',
                    description: 'В системе добавлен новый персональный раздел «Сотрудники». Теперь каждый пользователь может создать и вести собственную базу сотрудников для использования при формировании заявок.',
                    id: 1
                },
                {
                    date: '2025-08-22T11:00:00',
                    title: 'Новый раздел "Сотрудники"',
                    description: 'В системе добавлен новый персональный раздел «Сотрудники». Теперь каждый пользователь может создать и вести собственную базу сотрудников для использования при формировании заявок.',
                    id: 2
                },
                {
                    date: '2025-08-22T11:00:00',
                    title: 'Изменения в режиме работы Дебаркадера №2',
                    description: 'С 01.08.2025 дебаркадер №2 закрывается. Для того, чтобы произвести разгрузку на дебаркадере, необходимо за 30 минут до прибытия сообщить об этом в Бюро пропусков или по факту на КПП №4.',
                    id: 3
                }
            ]
        }
    },
    methods: {
        formatDate(dateString) {
            const date = new Date(dateString);
            return date.toLocaleDateString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric',
                hour: '2-digit',
                minute: '2-digit'
            }).replace(',', '');
        },
        handleDetailsClick(news) {
            // Обработка клика на кнопку "Подробнее"
            console.log('Подробнее о новости:', news.id);
            // В реальном приложении здесь может быть:
            // this.$router.push(`/news/${news.id}`);
            // или this.$emit('show-details', news);
        }
    }
}
</script>

<style scoped>
.news {
    padding: 20px;
    font-family: 'Montserrat', sans-serif;
}

.news-container {
    width: 775px;
    background: #FFFFFF;
    border: 1px solid #E6E6E6;
    box-shadow: 0px 3px 10px rgba(0, 0, 0, 0.05);
    border-radius: 30px;
    padding: 15px;
    position: relative;
    overflow: hidden;
}

.news-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 10px;
}

.news-title {
    margin: 0;
    font-weight: 700;
    font-size: 18px;
    line-height: 22px;
    color: #000000;
}

.divider {
    height: 1px;
    background: #E6E6E6;
    margin: 10px 0 20px;
}

.divider-middle {
    margin: 20px 0;
    background: #E6E6E6;
}

.news-list {
    max-height: 400px;
    overflow-y: auto;
    padding-right: 5px;
}

/* Стилизация скроллбара */
.news-list::-webkit-scrollbar {
    width: 4px;
}

.news-list::-webkit-scrollbar-track {
    background: #f1f1f1;
    border-radius: 2px;
}

.news-list::-webkit-scrollbar-thumb {
    background: #D9E2FF;
    border-radius: 2px;
}

.news-item {
    margin-bottom: 20px;
    position: relative;
}

.news-date {
    display: block;
    font-weight: 500;
    font-size: 12px;
    line-height: 15px;
    color: #A2A2A2;
    margin-bottom: 5px;
}

.news-item-title {
    margin: 0 0 8px 0;
    font-weight: 700;
    font-size: 16px;
    line-height: 20px;
    color: #4F5BDF;
}

.news-item-description {
    margin: 0 0 12px 0;
    font-weight: 400;
    font-size: 14px;
    line-height: 17px;
    color: #000000;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
}

.news-details-button {
    background: #4F5BDF;
    border: none;
    border-radius: 30px;
    cursor: pointer;
    padding: 4px 16px;
    font-family: 'Montserrat', sans-serif;
    font-weight: 500;
    font-size: 13px;
    line-height: 16px;
    color: #FFFFFF;
    transition: background-color 0.3s ease, transform 0.2s ease;
}

.news-details-button:hover {
    background: #3a45c5;
    transform: translateY(-1px);
}

.news-details-button:active {
    transform: translateY(0);
}
</style>