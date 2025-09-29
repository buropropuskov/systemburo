use actix_web::web;
use crate::handlers::*;

pub fn config(cfg: &mut web::ServiceConfig) {
    cfg
        .service(web::resource("/submit-v2").route(web::post().to(submit_application_v2))) 
        .service(web::resource("/user-data").route(web::get().to(get_current_user_data)))
        .service(web::resource("/register").route(web::post().to(register)))
        .service(web::resource("/login").route(web::post().to(login)))
        .service(
            web::scope("/organizations")
                .route("", web::get().to(get_all_organizations))
                .route("", web::post().to(create_organization))
                .route("/{id}", web::put().to(update_organization))
                .route("/{id}", web::delete().to(delete_organization))
                .route("/with-users", web::get().to(get_organizations_with_users))
        )
        .service(web::resource("/get-organization").route(web::get().to(get_organization)))
        .service(web::resource("/submit").route(web::post().to(submit_application))) // ОБНОВЛЕНО
        .service(web::resource("/applications/all-cars").route(web::get().to(get_all_cars_for_account)))
        .service(web::resource("/applications/active-cars").route(web::get().to(get_active_cars_for_table))) // ОБНОВЛЕНО
        .service(
            web::scope("/companies")
                .route("", web::get().to(get_all_companies))
                .route("", web::post().to(create_company))
                .route("/{id}", web::put().to(update_company))
                .route("/{id}", web::delete().to(delete_company))
                .route("/with-users", web::get().to(get_companies_with_users))
        )
        // НОВЫЕ МАРШРУТЫ ДЛЯ МЕСТ РАЗГРУЗКИ
        .service(web::resource("/unload-places").route(web::get().to(get_unload_places)))
        .service(
            web::resource("/applications/{application_id}/cars/{car_id}")
                .route(web::delete().to(delete_car))
                .route(web::put().to(update_car)), // ОБНОВЛЕНО
        )
        .service(
            web::resource("/applications/{application_id}")
                .route(web::put().to(update_application)) // НУЖНО ОБНОВИТЬ
        )
        .service(web::resource("/users/all").route(web::get().to(get_all_users)))
        .service(web::resource("/user-types").route(web::get().to(get_user_types)))
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
        .service(web::resource("/users/me").route(web::get().to(get_current_user)))
        .service(web::resource("/users/{username}").route(web::delete().to(delete_user)))
        .service(
            web::resource("/users/{username}/info")
                .route(web::put().to(update_user_info))
        );
}