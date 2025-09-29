<template>
    <div class="notifications">
        <header class="notifications__header">
            <h3>Уведомления</h3>
            <a href="#" class="notifications__clear">Очистить</a>
        </header>
        <div class="notifications__list">
            <p class="notifications__message">Уведомлений нет</p>
        </div>
    </div>
</template>
<script>
export default {
    
}
</script>
<style scoped>
    .notifications {
        height: 200px;
        width: 38%;
        background-color: #FFF;
        border-radius: 30px;
        border: 1px solid #e6e6e6;
        box-shadow: 0 3px 10px rgba(0,0,0,0.05);
    }

    .notifications__header {
        height: 40px;
        width: 100%;
        border-bottom: 1px solid #e6e6e6;
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0 20px;
    }

    .notifications__clear {
        color: #a2a2a2;
        text-decoration: none;
        font-size: 12px;
        transition: .2s;
        font-weight: 600;
    }

    .notifications__clear:hover {
        color: #333;
    }

    .notifications__list {
        width: 100%;
        height: 155px;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .notifications__message {
        color: #a2a2a2;
    }
</style>