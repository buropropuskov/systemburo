// routes.rs
use actix_web::web;
use crate::handlers::auth::*;
use crate::handlers::applications::*;
use crate::handlers::cars::*;
use crate::handlers::companies::*;
use crate::handlers::organizations::*;
use crate::handlers::users::*;
use crate::handlers::user_types::*;
use crate::handlers::unload_places::*;
use crate::handlers::number_format::*;
use crate::handlers::unique_cars::*;
use crate::handlers::unique_employees::*; // Добавляем импорт для сотрудников
use crate::handlers::table_constructor::*;
use crate::handlers::citizenship::*;

pub fn config(cfg: &mut web::ServiceConfig) {
    cfg
        // Авторизация и регистрация
        .service(web::resource("/register").route(web::post().to(register)))
        .service(web::resource("/login").route(web::post().to(login)))
        .service(web::resource("/user-data").route(web::get().to(get_current_user_data)))
        .service(web::resource("/users/me").route(web::get().to(get_current_user)))
        .service(web::resource("/user-types").route(web::get().to(get_user_types)))

        // Управление типами пользователей (CRUD)
        .service(
            web::scope("/user-types-management")
                .route("", web::get().to(get_user_types_with_count))
                .route("", web::post().to(create_user_type))
                .route("/{id}", web::put().to(update_user_type_by_id))
                .route("/{id}", web::delete().to(delete_user_type))
        )

        // Заявки
        .service(web::resource("/submit").route(web::post().to(submit_application)))
        .service(web::resource("/submit-v2").route(web::post().to(submit_application_v2)))
        .service(web::resource("/submit-employee-application").route(web::post().to(submit_employee_application))) // Новая ручка для сотрудников
        .service(web::resource("/applications/all-cars").route(web::get().to(get_all_cars_for_account)))
        .service(web::resource("/applications/active-cars").route(web::get().to(get_active_cars_for_table)))
        .service(
            web::resource("/applications/{application_id}")
                .route(web::put().to(update_application))
        )
        .service(
            web::resource("/applications/{application_id}/cars/{car_id}")
                .route(web::put().to(update_car))
                .route(web::delete().to(delete_car))
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
                .route("/{id}", web::put().to(update_unload_place))
                .route("/{id}", web::delete().to(delete_unload_place))
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
        .service(web::resource("/users/{username}").route(web::delete().to(delete_user))
        )

        // Форматы номеров
        .service(
            web::scope("/license-plate-formats")
                .route("", web::get().to(get_license_plate_formats))
                .route("", web::post().to(create_license_plate_format))
                .route("/{id}", web::put().to(update_license_plate_format))
                .route("/{id}", web::delete().to(delete_license_plate_format))
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
                .route("/{id}", web::put().to(update_system_table))
                .route("/{id}", web::delete().to(delete_system_table))
                .route("/name/{name}", web::get().to(get_system_table_by_name))
        );
}