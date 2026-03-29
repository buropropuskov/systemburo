use actix_web::web;

use crate::handlers::auth::*;
use crate::handlers::applications::*;
use crate::handlers::companies::*;
use crate::handlers::organizations::*;
use crate::handlers::users::*;
use crate::handlers::user_types::*;
use crate::handlers::unload_places::*;
use crate::handlers::number_format::*;
use crate::handlers::unique_cars::*;
use crate::handlers::unique_employees::*;
use crate::handlers::table_constructor::*;
use crate::handlers::citizenship::*;
use crate::handlers::unique_attachments::*;
use crate::handlers::cars::*;
use crate::handlers::employees::*;
use crate::handlers::employees_history::*;
use crate::handlers::feedback::*;
use crate::handlers::application_approvers::*;
use crate::handlers::application_history::*;
use crate::handlers::application_viewers::*;
use crate::handlers::notifications::*;  
use crate::handlers::news::*;
use crate::handlers::request_logs::*;

use crate::handlers::cars_history::{
    get_car_history,
    add_car_history_entry,
    get_all_cars_history,
    get_cars_current_status,
    get_unified_car_history,
};

pub fn config(cfg: &mut web::ServiceConfig) {
    cfg
        // Авторизация и регистрация
        .service(web::resource("/register").route(web::post().to(register)))
        .service(web::resource("/login").route(web::post().to(login)))
        .service(web::resource("/refresh-token").route(web::post().to(refresh_token)))
        .service(web::resource("/logout").route(web::post().to(logout)))
        .service(web::resource("/user-data").route(web::get().to(get_current_user_data)))
        .service(web::resource("/users/me").route(web::get().to(get_this_user)))
        .service(web::resource("/user-types").route(web::get().to(get_user_types)))

        
        // Новости и объявления
        .service(
            web::scope("/news")
                .route("", web::get().to(get_news))
                .route("", web::post().to(create_news))
                .route("/all", web::get().to(get_all_news_for_manage))
                .route("/{id}", web::put().to(update_news))
                .route("/{id}", web::delete().to(delete_news))
        )
        .service(
            web::scope("/announcements")
                .route("/active", web::get().to(get_active_announcement))
                .route("", web::post().to(create_announcement))
                .route("/all", web::get().to(get_all_announcements))
                .route("/set-active", web::post().to(set_active_announcement))
                .route("/{id}", web::put().to(update_announcement))
                .route("/{id}", web::delete().to(delete_announcement))
        )
// В функции config добавить:
// src/routes.rs (добавить эндпоинты для request_logs)
// В функции config добавить:
.service(
    web::scope("/request-logs")
        .route("", web::get().to(get_request_logs))
        .route("/users", web::get().to(get_logs_users))
        .route("/stats", web::get().to(get_logs_stats))
        .route("/realtime", web::get().to(get_realtime_stats))
        .route("/timeline", web::get().to(get_request_timeline))
        .route("/export", web::get().to(export_logs_txt))
)
        // Обратная связь
        .service(
            web::scope("/feedback")
                .route("", web::post().to(create_feedback))
                .route("/all", web::get().to(get_all_feedback))
                .route("/stats", web::get().to(get_feedback_stats))
                .route("/my", web::get().to(get_my_feedback))
                .route("/{id}/status", web::put().to(update_feedback_status))
                .route("/{id}/read", web::put().to(mark_feedback_as_read))
        )

        // Управление типами пользователей (CRUD)
        .service(
            web::scope("/user-types-management")
                .route("", web::get().to(get_user_types_with_count))
                .route("", web::post().to(create_user_type))
                .route("/{id}", web::put().to(update_user_type_by_id))
                .route("/{id}", web::delete().to(delete_user_type))
        )

        // Заявки
        .service(
            web::scope("/applications")
                .route("", web::get().to(get_applications))
                .route("", web::post().to(create_application))
                .route("/submit-complete-application", web::post().to(submit_complete_application))
                .route("/user", web::get().to(get_user_applications))
                .route("/{id}", web::get().to(get_application_by_id))
                .route("/{id}", web::put().to(update_application))
                .route("/{id}/responsible-users", web::get().to(get_application_responsible_users))
                .route("/{id}/details", web::get().to(get_application_details))
                .route("/{id}/attachments", web::get().to(get_application_attachments))
                .route("/{id}/update-items-status", web::post().to(update_application_items_status))
                // Маршруты для согласования и принятия
                .route("/{id}/forward", web::post().to(forward_application))
                .route("/{id}/approve", web::post().to(approve_application_by_user))
                .route("/{id}/check-approval-status", web::get().to(check_approval_status))
                .route("/{id}/take-to-work", web::post().to(take_application_to_work))
                .route("/{id}/revoke-from-work", web::post().to(revoke_application_from_work))
                .route("/{id}/restore-to-work", web::post().to(restore_application_to_work))
                // Маршруты для истории
                .route("/{id}/history", web::get().to(get_application_history))
                .route("/{id}/revoke-approval", web::post().to(revoke_approval))
                .route("/history", web::post().to(add_history_entry))
                .route("/{id}/viewers", web::get().to(get_application_viewers))
        )

        // Машины
        .service(
            web::scope("/cars")
                .route("/active-for-tables", web::get().to(get_active_cars_for_tables))
                .route("/fact-for-tables", web::get().to(get_fact_cars_for_tables))
                .route("/unload-places", web::get().to(get_car_unload_places))
                .route("/fact-unload-places", web::get().to(get_fact_car_unload_places))
                .route("/check-active", web::get().to(check_active_car))
                // Маршруты для истории машин (из cars_history)
                .route("/{id}/history", web::get().to(get_car_history))
                .route("/{id}/history", web::post().to(add_car_history_entry))
                .route("/history/all", web::get().to(get_all_cars_history))
                .route("/history/current-status", web::get().to(get_cars_current_status))
                .route("/history/unified", web::get().to(get_unified_car_history))
                // Эти маршруты используют функции из cars (обновление статуса, активация и т.д.)
                .route("/{id}/territory-status", web::put().to(update_car_territory_status))
                .route("/{id}/deactivate", web::put().to(deactivate_car))
                .route("/{id}/activate", web::put().to(activate_car))
                .route("/{id}/restore", web::put().to(restore_car))
        )

        // Сотрудники
        .service(
            web::scope("/employees")
                .route("/active-for-table/{table_id}", web::get().to(get_active_employees_for_table))
                .route("/{id}/history", web::get().to(get_employee_history))
                .route("/history/unified", web::get().to(get_unified_employee_history))
                .route("/history/all", web::get().to(get_all_employees_entry_exit_history))
                .route("/history/current-status", web::get().to(get_employees_current_status))
                .route("/{id}/territory-status", web::put().to(update_employee_territory_status))
                .route("/{id}/deactivate", web::put().to(deactivate_employee))
                .route("/{id}/activate", web::put().to(activate_employee))
                .route("/{id}/restore", web::put().to(restore_employee))
                .route("/history/table/{table_id}", web::get().to(get_table_employees_history))
        )

        // Уникальные машины
        .service(
            web::scope("/unique-cars")
                .route("", web::get().to(get_unique_cars))
                .route("", web::post().to(create_unique_car))
                .route("/batch", web::post().to(create_unique_cars_batch))
                .route("/{id}", web::put().to(update_unique_car))
                .route("/{id}", web::delete().to(delete_unique_car))
                .route("/ownership-info", web::get().to(get_car_ownership_info))
        )
        .service(web::resource("/unique-cars/by-number").route(web::put().to(update_unique_car_by_number)))

        // Уникальные сотрудники
        .service(
            web::scope("/unique-employees")
                .route("", web::get().to(get_unique_employees))
                .route("", web::post().to(create_unique_employee))
                .route("/{id}", web::put().to(update_unique_employee))
                .route("/{id}", web::delete().to(delete_unique_employee))
                .route("/ownership-info", web::get().to(get_employee_ownership_info))
        )

        // Места разгрузки
        .service(
            web::scope("/unload-places")
                .route("", web::get().to(get_unload_places))
                .route("", web::post().to(create_unload_place))
                .route("/{id}", web::get().to(get_unload_place_by_id))
                .route("/{id}", web::put().to(update_unload_place))
                .route("/{id}", web::delete().to(delete_unload_place))
                // Маршруты для временных слотов мест разгрузки
                .route("/{id}/time-slots", web::get().to(get_unload_place_time_slots))
                .route("/{id}/time-slots", web::post().to(add_unload_place_time_slot))
                .route("/{place_id}/time-slots/{slot_id}", web::put().to(update_unload_place_time_slot))
                .route("/{place_id}/time-slots/{slot_id}", web::delete().to(delete_unload_place_time_slot))
                // Маршруты для фотографий мест разгрузки
                .route("/{id}/photos", web::post().to(upload_unload_place_photo))
                .route("/{place_id}/photos/{photo_id}", web::delete().to(delete_unload_place_photo))
                .route("/{place_id}/photos/{photo_id}/main", web::post().to(set_main_unload_place_photo))
        )

        // Организации
        .service(
            web::scope("/organizations")
                .route("", web::get().to(get_all_organizations))
                .route("", web::post().to(create_organization))
                .route("/{id}", web::put().to(update_organization))
                .route("/{id}", web::delete().to(delete_organization))
                .route("/with-users", web::get().to(get_organizations_with_users))
                .route("/with-users-extended", web::get().to(get_organizations_with_users_extended))
                .route("/{id}/unload-places", web::get().to(get_organization_unload_places))
                .route("/{id}/unload-places", web::put().to(update_organization_unload_places))
                .route("/{id}/users", web::get().to(get_organization_users))
                .route("/{id}/users", web::put().to(update_organization_users))
                .route("/{id}/tables", web::get().to(get_organization_tables))
                .route("/{id}/tables", web::put().to(update_organization_tables))
        )
        .service(web::resource("/get-organization").route(web::get().to(get_organization)))

        // Компании
        .service(
            web::scope("/companies")
                .route("", web::get().to(get_all_companies))
                .route("", web::post().to(create_company))
                .route("/{id}", web::put().to(update_company))
                .route("/{id}", web::delete().to(delete_company))
                .route("/with-users", web::get().to(get_companies_with_users))
                .route("/with-users-extended", web::get().to(get_companies_with_users_extended))
                .route("/{id}/unload-places", web::get().to(get_company_unload_places))
                .route("/{id}/unload-places", web::put().to(update_company_unload_places))
                .route("/{id}/users", web::get().to(get_company_users))
                .route("/{id}/users", web::put().to(update_company_users))
                .route("/{id}/tables", web::get().to(get_company_tables))
                .route("/{id}/tables", web::put().to(update_company_tables))
        )

        // Пользователи (CRUD)
        .service(web::resource("/users/all").route(web::get().to(get_all_users)))
        .service(
            web::resource("/users/{username}/type")
                .route(web::put().to(update_user_type))
        )
        .service(
            web::resource("/users/{username}/password")
                .route(web::put().to(update_user_password))
        )
        .service(
            web::resource("/users/{username}/organization")
                .route(web::put().to(update_user_organization))
        )
        .service(
            web::resource("/users/{username}/company")
                .route(web::put().to(update_user_company))
        )
        .service(
            web::resource("/users/{username}/info")
                .route(web::put().to(update_user_info))
        )
        .service(web::resource("/users/{username}").route(web::delete().to(delete_user)))
        // Получение текущего пользователя
        .service(web::resource("/users/current").route(web::get().to(get_current_user)))

        // Форматы номеров
        .service(
            web::scope("/license-plate-formats")
                .route("", web::get().to(get_license_plate_formats))
                .route("", web::post().to(create_license_plate_format))
                .route("/{id}", web::put().to(update_license_plate_format))
                .route("/{id}", web::delete().to(delete_license_plate_format))
        )

        // Принимающие заявки
        .service(
            web::scope("/application-approvers")
                .route("", web::get().to(get_application_approvers))
                .route("/available-users", web::get().to(get_available_users_for_approvers))
                .route("", web::post().to(create_application_approver))
                .route("/{id}", web::delete().to(delete_application_approver))
        )

        // Гражданства
        .service(
            web::scope("/citizenships")
                .route("", web::get().to(get_citizenships))
                .route("", web::post().to(create_citizenship))
                .route("/{id}", web::put().to(update_citizenship))
                .route("/{id}", web::delete().to(delete_citizenship))
                .route("/clear-default", web::post().to(clear_default_citizenships))
        )

        // Конструктор таблиц
        .service(
            web::scope("/system-tables")
                .route("", web::get().to(get_system_tables))
                .route("", web::post().to(create_system_table))
                .route("/{id}", web::get().to(get_system_table_by_id))
                .route("/{id}", web::put().to(update_system_table))
                .route("/{id}", web::delete().to(delete_system_table))
                .route("/name/{name}", web::get().to(get_system_table_by_name))
                // Маршруты для временных слотов таблиц
                .route("/{id}/time-slots", web::get().to(get_system_table_time_slots))
                .route("/{id}/time-slots", web::post().to(add_system_table_time_slot))
                .route("/{table_id}/time-slots/{slot_id}", web::put().to(update_system_table_time_slot))
                .route("/{table_id}/time-slots/{slot_id}", web::delete().to(delete_system_table_time_slot))
                // Маршруты для фотографий таблиц
                .route("/{id}/photos", web::post().to(upload_system_table_photo))
                .route("/{table_id}/photos/{photo_id}", web::delete().to(delete_system_table_photo))
                .route("/{table_id}/photos/{photo_id}/main", web::post().to(set_main_system_table_photo))
        )

        // Управление вложениями (бланками заявок)
        .service(
            web::scope("/attachments")
                .route("", web::get().to(get_attachments))
                .route("/all", web::get().to(get_all_attachments))
                .route("", web::post().to(create_attachment))
                .route("/{id}", web::put().to(update_attachment))
                .route("/{id}", web::delete().to(delete_attachment))
                .route("/{id}/restore", web::put().to(restore_attachment))
                .route("/{id}", web::get().to(get_attachment_by_id))
                .route("/{id}/cars", web::get().to(get_attachment_cars))
                .route("/{id}/employees", web::get().to(get_attachment_employees))
                .route("/{id}/items", web::get().to(get_attachment_items))
        )

        // Уведомления
        .service(
            web::scope("/notifications")
                .route("", web::get().to(get_notifications))
                .route("", web::delete().to(clear_all_notifications))
                .route("/{id}/read", web::put().to(mark_notification_read))
                .route("/{id}", web::delete().to(delete_notification))
        );
}