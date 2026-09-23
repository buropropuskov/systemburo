#!/usr/bin/env python3
"""Генерация диаграмм через PlantUML web-сервис (в стиле оригинальной диаграммы А.4).

Создаёт:
- 2.12_activity_billing_swimlane.png — диаграмма активности «Обработка платежа»
- 2.13_usecase_admin.png — Use Case «Администрирование и биллинг» (PlantUML)
- 2.9_user_flow_registration.png — activity-диаграмма user flow регистрации
- 2.10_user_flow_analysis.png — activity-диаграмма анализа документа
- 2.11_user_flow_results.png — activity-диаграмма работы с результатами
- 2.3.5_user_story_map.png — user story map (через component diagram)
- 2.16_activity_notes.png — диаграмма активности работы с заметками (доп. в приложения)
- 2.17_sequence_analyze.png — sequence-диаграмма процесса анализа (доп. в приложения)
"""

from pathlib import Path
from plantuml import PlantUML

IMAGES = Path("/home/washka/project/diplom/Sergey/images")
server = PlantUML(url="http://www.plantuml.com/plantuml/png/")

# Стиль как на эталоне (Рисунок А.4): светло-голубые блоки со скруглёнными углами,
# тёмно-синяя рамка, чёрные стрелки с обычными наконечниками, белый фон дорожек.
# НЕ используем linetype:ortho — классические стрелки читаются лучше.
STYLE = r"""
skinparam defaultFontName "Liberation Sans"
skinparam defaultFontSize 12
skinparam backgroundColor #FFFFFF
skinparam shadowing false

skinparam ArrowColor #000000
skinparam ArrowFontColor #000000
skinparam ArrowThickness 1.0

skinparam activity {
  BackgroundColor       #DCE6F1
  BorderColor           #365F91
  FontColor             #000000
  StartColor            #000000
  EndColor              #000000
  DiamondBackgroundColor #DCE6F1
  DiamondBorderColor    #365F91
  DiamondFontColor      #000000
  BarColor              #365F91
  ArrowColor            #000000
}

skinparam usecase {
  BackgroundColor #DCE6F1
  BorderColor     #365F91
  FontColor       #000000
  ArrowColor      #000000
}

skinparam actor {
  BackgroundColor #FFFFFF
  BorderColor     #000000
  FontColor       #000000
}

skinparam rectangle {
  BackgroundColor #FFFFFF
  BorderColor     #365F91
  FontColor       #000000
}

skinparam partition {
  BackgroundColor #FFFFFF
  BorderColor     #365F91
  FontColor       #000000
}

skinparam swimlane {
  BorderColor          #365F91
  TitleFontColor       #000000
  TitleBackgroundColor #F2F2F2
}

skinparam sequence {
  ArrowColor                 #000000
  LifeLineBorderColor        #365F91
  LifeLineBackgroundColor    #FFFFFF
  ParticipantBackgroundColor #DCE6F1
  ParticipantBorderColor     #365F91
  ParticipantFontColor       #000000
  ActorBackgroundColor       #FFFFFF
  ActorBorderColor           #000000
  ActorFontColor             #000000
  MessageAlignment           direction
}

skinparam note {
  BackgroundColor #FFFACD
  BorderColor     #365F91
  FontColor       #000000
}
"""


def wrap(uml_body: str) -> str:
    """Встроить общий STYLE между @startuml и остальной разметкой."""
    start = "@startuml"
    end = "@enduml"
    body = uml_body.strip()
    if body.startswith(start):
        body = body[len(start):].lstrip()
    if body.endswith(end):
        body = body[: -len(end)].rstrip()
    return f"{start}\n{STYLE}\n{body}\n{end}\n"


def render(uml: str, filename: str) -> Path:
    out = IMAGES / filename
    data = server.processes(wrap(uml))
    with open(out, "wb") as f:
        f.write(data)
    size = out.stat().st_size
    print(f"  OK {filename}  ({size // 1024} KB)")
    return out


# ============================================================
# 2.12 — swimlane обработки платежа (замена старой версии)
# ============================================================

BILLING_SWIMLANE = """
@startuml
skinparam defaultFontName "Liberation Sans"
skinparam defaultFontSize 12
|Пользователь|
start
:Открыть страницу тарифов;
:Выбрать тариф;
|Фронтенд|
:Запрос на создание заказа;
|Бэкенд|
:Создать заказ в БД;
:Запросить платёж у шлюза;
|Платёжный шлюз|
:Создать платёжную сессию;
:Вернуть ссылку на оплату;
|Бэкенд|
:Сохранить идентификатор платежа;
|Фронтенд|
:Перенаправить на форму оплаты;
|Пользователь|
:Ввести реквизиты и подтвердить;
|Платёжный шлюз|
:Обработать платёж;
if (Успех?) then (да)
  :Отправить уведомление;
  |Бэкенд|
  :Проверить подпись уведомления;
  :Обновить статус заказа;
  :Продлить подписку;
  |Фронтенд|
  :Отобразить активную подписку;
  |Пользователь|
  :Видит активную подписку;
  stop
else (нет)
  :Отправить уведомление об ошибке;
  |Бэкенд|
  :Сохранить статус «отклонено»;
  |Фронтенд|
  :Показать сообщение об ошибке;
  |Пользователь|
  :Видит ошибку;
  stop
endif
@enduml
"""


# ============================================================
# 2.13 — Use Case «Администрирование и биллинг»
# ============================================================

USECASE_ADMIN = """
@startuml
left to right direction

actor "Администратор" as Admin

rectangle "Dockee: администрирование" {
  usecase "Управление\\nпользователями" as UC_Users
  usecase "Блокировка\\nпользователя"   as UC_Block
  usecase "Подтвердить\\nблокировку"    as UC_Confirm
  usecase "Настройка тарифов"           as UC_Tariffs
  usecase "Модерация жалоб\\nна ответы ИИ" as UC_Moderate
  usecase "Просмотр метрик\\nсервиса"    as UC_Metrics
  usecase "Управление правами\\nдоступа"  as UC_Roles
}

Admin --> UC_Users
Admin --> UC_Block
Admin --> UC_Tariffs
Admin --> UC_Moderate
Admin --> UC_Metrics
Admin --> UC_Roles
UC_Block ..> UC_Confirm : include
@enduml
"""


USECASE_MANAGER = """
@startuml
left to right direction

actor "Менеджер\\nподдержки" as Manager

rectangle "Dockee: поддержка клиентов" {
  usecase "Обработка заявок\\nна enterprise-тариф" as UC_Enterprise
  usecase "Проверить оплату"                        as UC_CheckPay
  usecase "Работа с обращениями\\nпользователей"    as UC_Tickets
  usecase "Просмотр истории\\nанализов"             as UC_History
  usecase "Ответ пользователю"                      as UC_Reply
}

Manager --> UC_Enterprise
Manager --> UC_Tickets
Manager --> UC_History
UC_Enterprise ..> UC_CheckPay : include
UC_Tickets ..> UC_Reply : include
@enduml
"""


USECASE_BILLING = """
@startuml
left to right direction

actor "Платёжный шлюз" as Billing

rectangle "Dockee: подсистема биллинга" {
  usecase "Регистрация платежа"       as UC_Pay
  usecase "Обработка уведомлений"     as UC_Webhook
  usecase "Формирование счёта"        as UC_Invoice
  usecase "Автопродление подписки"    as UC_Renew
  usecase "Начисление реф. бонуса"    as UC_Referral
  usecase "Возврат средств"           as UC_Refund
}

Billing --> UC_Pay
Billing --> UC_Webhook
Billing --> UC_Invoice
Billing --> UC_Renew
Billing --> UC_Referral
Billing --> UC_Refund
@enduml
"""


USECASE_OVERVIEW = """
@startuml
left to right direction

actor "Пользователь"    as User
actor "Менеджер\\nподдержки" as Manager
actor "Администратор"   as Admin
actor "Платёжный шлюз"  as Billing

rectangle "Сервис Dockee" {
  usecase "Работа с документами\\nи анализом"         as UC_Main
  usecase "Управление аккаунтом\\nи подпиской"        as UC_Account
  usecase "Поддержка клиентов"                        as UC_Support
  usecase "Администрирование\\nсервиса"               as UC_Admin
  usecase "Обработка платежей"                        as UC_Pay
}

User    --> UC_Main
User    --> UC_Account
Manager --> UC_Support
Admin   --> UC_Admin
Billing --> UC_Pay

UC_Account ..> UC_Pay : include
UC_Support ..> UC_Account : extend
@enduml
"""


# ============================================================
# 2.9 — User flow «Регистрация и выбор тарифа» (activity)
# ============================================================

FLOW_REGISTRATION = """
@startuml
skinparam defaultFontName "Liberation Sans"
skinparam defaultFontSize 12
|Пользователь|
start
:Посетить лендинг;
:Нажать «Войти / Зарегистрироваться»;
:Ввести email и пароль;
|Система|
:Отправить письмо для подтверждения;
|Пользователь|
if (Email подтверждён?) then (да)
  :Выбрать тариф;
  if (Тариф платный?) then (да)
    |Биллинг|
    :Создать платёж;
    |Пользователь|
    :Оплатить на стороне шлюза;
    |Биллинг|
    :Уведомить об успешной оплате;
  endif
  |Система|
  :Активировать подписку;
  |Пользователь|
  :Попасть на рабочий стол;
  stop
else (нет)
  :Повторная отправка письма;
  |Система|
  :Повторная отправка;
  stop
endif
@enduml
"""


# ============================================================
# 2.10 — User flow «Загрузка и анализ документа» (activity)
# ============================================================

FLOW_ANALYSIS = """
@startuml
skinparam defaultFontName "Liberation Sans"
skinparam defaultFontSize 12
|Пользователь|
start
:Открыть рабочий стол;
:Выбрать или перетащить файл;
|Фронтенд|
if (Формат корректен?) then (нет)
  :Показать сообщение об ошибке;
  stop
else (да)
  :Проверить размер;
  if (Размер допустим?) then (нет)
    :Показать сообщение «Файл более 50 МБ»;
    stop
  else (да)
    :Отправить файл на сервер;
    |Бэкенд|
    :Сохранить документ;
    :Извлечь текст;
    :Сформировать запрос;
    |ИИ-сервис|
    :Проанализировать документ;
    :Вернуть список рисков;
    |Бэкенд|
    :Сохранить результаты;
    |Фронтенд|
    :Подсветить риски в тексте;
    :Показать панель замечаний;
    |Пользователь|
    :Работать с результатами;
    stop
  endif
endif
@enduml
"""


# ============================================================
# 2.11 — User flow «Работа с результатами»
# ============================================================

FLOW_RESULTS = """
@startuml
skinparam defaultFontName "Liberation Sans"
skinparam defaultFontSize 12
|Пользователь|
start
:Открыть документ с рисками;
repeat
  :Кликнуть на замечание;
  |Фронтенд|
  :Прокрутить документ к фрагменту;
  :Отобразить пояснение;
  |Пользователь|
  if (Нужен вопрос ИИ?) then (да)
    :Задать уточняющий вопрос;
    |ИИ-сервис|
    :Вернуть ответ;
    |Пользователь|
    :Прочитать ответ;
  endif
  :Добавить заметку;
  :Изменить статус замечания;
repeat while (Все замечания просмотрены?) is (нет) not (да)
|Фронтенд|
:Сформировать PDF-отчёт;
|Пользователь|
:Скачать отчёт;
stop
@enduml
"""


# ============================================================
# 2.3.5 — User story map (таблица в виде mindmap/wbs)
# ============================================================

USER_STORY_MAP = """
@startuml
skinparam defaultFontName "Liberation Sans"
skinparam defaultFontSize 11
skinparam rectangle {
  BackgroundColor<<backbone>> #DBEAFE
  BorderColor<<backbone>>     #1D4E89
  BackgroundColor<<mvp>>       #DCFCE7
  BorderColor<<mvp>>           #065F46
  BackgroundColor<<v2>>        #FEF3C7
  BorderColor<<v2>>            #92400E
  BackgroundColor<<v3>>        #FEE2E2
  BorderColor<<v3>>            #991B1B
}

title Карта пользовательских историй сервиса Dockee

' Backbone: этапы пути пользователя
rectangle "Знакомство" as B1 <<backbone>>
rectangle "Регистрация" as B2 <<backbone>>
rectangle "Загрузка документа" as B3 <<backbone>>
rectangle "Работа с результатами" as B4 <<backbone>>
rectangle "Управление файлами" as B5 <<backbone>>
rectangle "Поддержка" as B6 <<backbone>>

B1 -right[hidden]- B2
B2 -right[hidden]- B3
B3 -right[hidden]- B4
B4 -right[hidden]- B5
B5 -right[hidden]- B6

' MVP row
rectangle "Посмотреть\\nлендинг" as R1_1 <<mvp>>
rectangle "Регистрация\\nemail+пароль" as R1_2 <<mvp>>
rectangle "Загрузить PDF/\\nDOCX" as R1_3 <<mvp>>
rectangle "Увидеть\\nподсветку\\nрисков" as R1_4 <<mvp>>
rectangle "Список\\nдокументов" as R1_5 <<mvp>>
rectangle "Email-\\nподдержка" as R1_6 <<mvp>>

B1 -down- R1_1
B2 -down- R1_2
B3 -down- R1_3
B4 -down- R1_4
B5 -down- R1_5
B6 -down- R1_6

' Release 2
rectangle "Тарифы\\nи цены" as R2_1 <<v2>>
rectangle "Подтверждение\\nemail" as R2_2 <<v2>>
rectangle "Drag-n-drop\\nзагрузка" as R2_3 <<v2>>
rectangle "Чат с ИИ-\\nпомощником" as R2_4 <<v2>>
rectangle "Поиск и\\nсортировка" as R2_5 <<v2>>
rectangle "База знаний\\nв интерфейсе" as R2_6 <<v2>>

R1_1 -down- R2_1
R1_2 -down- R2_2
R1_3 -down- R2_3
R1_4 -down- R2_4
R1_5 -down- R2_5
R1_6 -down- R2_6

' Release 3
rectangle "Блок\\nотзывов" as R3_1 <<v3>>
rectangle "Выбор\\nтарифа" as R3_2 <<v3>>
rectangle "Ограничение\\nразмера\\n(50 МБ)" as R3_3 <<v3>>
rectangle "Экспорт\\nотчёта PDF" as R3_4 <<v3>>
rectangle "Папки и\\nкорзина" as R3_5 <<v3>>
rectangle "Личный\\nкабинет" as R3_6 <<v3>>

R2_1 -down- R3_1
R2_2 -down- R3_2
R2_3 -down- R3_3
R2_4 -down- R3_4
R2_5 -down- R3_5
R2_6 -down- R3_6

legend right
  <size:11>Легенда</size>
  |= Цвет |= Выпуск |
  |<back:#DBEAFE>  </back>| Этап (backbone) |
  |<back:#DCFCE7>  </back>| Релиз 1 — MVP |
  |<back:#FEF3C7>  </back>| Релиз 2 — расширение |
  |<back:#FEE2E2>  </back>| Релиз 3 — развитая версия |
endlegend

@enduml
"""


# ============================================================
# Доп. 2.16 — Activity «Работа с заметками и статусами»
# ============================================================

ACTIVITY_NOTES = """
@startuml
skinparam defaultFontName "Liberation Sans"
skinparam defaultFontSize 12
|Пользователь|
start
:Открыть документ с замечаниями;
repeat
  :Выбрать замечание;
  if (Нужна заметка?) then (да)
    :Ввести текст заметки;
    |Фронтенд|
    :Отправить запрос сохранения;
    |Бэкенд|
    :Сохранить заметку в БД;
    |Фронтенд|
    :Показать сохранённую заметку;
  endif
  |Пользователь|
  if (Изменить статус?) then (да)
    :Выбрать статус (принять/отклонить/уточнить);
    |Фронтенд|
    :Отправить обновление статуса;
    |Бэкенд|
    :Сохранить статус;
    |Фронтенд|
    :Отобразить значок статуса;
  endif
repeat while (Есть ещё замечания?) is (да) not (нет)
:Перейти к экспорту отчёта;
stop
@enduml
"""


# ============================================================
# Доп. 2.17 — Sequence «Анализ документа»
# ============================================================

SEQ_ANALYZE = """
@startuml
skinparam defaultFontName "Liberation Sans"
skinparam defaultFontSize 12
actor Пользователь as U
participant Фронтенд as F
participant "API-сервис" as A
database БД as D
participant "Файловое\\nхранилище" as S
participant "ИИ-сервис" as AI

U -> F : Загрузить файл
F -> F : Проверить формат и размер
F -> A : POST /documents (файл)
A -> S : Сохранить файл
A -> D : Создать запись о документе
A --> F : documentId
F -> A : POST /documents/{id}/analyze
A -> S : Получить текст документа
A -> AI : Запрос анализа
AI --> A : JSON с рисками
A -> D : Сохранить риски
A --> F : Массив рисков
F -> U : Отобразить подсветку\\nи панель замечаний
@enduml
"""


# ============================================================
# Entry point
# ============================================================

if __name__ == "__main__":
    IMAGES.mkdir(parents=True, exist_ok=True)
    for diag, filename in [
        (BILLING_SWIMLANE, "2.12_activity_billing_swimlane.png"),
        (USECASE_ADMIN,    "2.13_usecase_admin.png"),
        (USECASE_MANAGER,  "2.13b_usecase_manager.png"),
        (USECASE_BILLING,  "2.13c_usecase_billing.png"),
        (USECASE_OVERVIEW, "2.13d_usecase_overview.png"),
        (ACTIVITY_NOTES,   "2.16_activity_notes.png"),
        (SEQ_ANALYZE,      "2.17_sequence_analyze.png"),
    ]:
        try:
            render(diag, filename)
        except Exception as e:
            print(f"  FAIL {filename}: {e}")
