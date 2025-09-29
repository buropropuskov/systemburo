use actix_web::{web, HttpResponse, Responder, HttpRequest, error, Error};
use chrono::{Utc, NaiveDate};
use sqlx::PgPool;
use serde_json::json;
use log;
use crate::{
    models::{
        UserRegister, UserLogin, Application, ApplicationWithCars, Car,
        LoginResponse, UserInfo, UpdateUserTypeRequest, UpdatePasswordRequest,
        UpdateOrganizationRequest, UpdateCompanyRequest, OrganizationInfo,
        Company, OrganizationWithUsers, NewOrganization, NewCompany,
        CompanyWithUsers, UserType, UpdateUserRequest, ApplicationSubmitRequest,
        CarWithUnloadPlaces, UnloadPlace, CarUnloadPlace, UserData
    },
    auth::{hash_password, verify_password, create_token, decode_token}
};

// Регистрация пользователя
pub async fn register(
    pool: web::Data<PgPool>, 
    form: web::Json<UserRegister>
) -> impl Responder {
    let hashed_password = hash_password(&form.password);
    let result = sqlx::query!(
        r#"INSERT INTO users 
           (username, password, organization_id, company_id, type_id, 
            last_name, first_name, middle_name, position, email, phone) 
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)"#,
        form.username,
        hashed_password,
        form.organization_id,
        form.company_id,
        form.type_id,
        form.last_name,
        form.first_name,
        form.middle_name,
        form.position,
        form.email,
        form.phone,
    )
    .execute(pool.get_ref())
    .await;

    match result {
        Ok(_) => HttpResponse::Ok().json("User registered successfully"),
        Err(e) => {
            if e.to_string().contains("users_username_key") {
                HttpResponse::BadRequest().json(json!({"message": "Пользователь с таким логином уже существует"}))
            } else {
                log::error!("Registration failed: {}", e);
                HttpResponse::InternalServerError().json("Registration failed")
            }
        }
    }
}

// Авторизация пользователя
pub async fn login(
    pool: web::Data<PgPool>, 
    form: web::Json<UserLogin>
) -> impl Responder {
    let user = sqlx::query!(
        r#"SELECT u.username, u.password, o.name as organization, 
                c.name as company, u.type_id, ut.code as user_type, 
                u.organization_id, u.company_id
         FROM users u 
         JOIN organizations o ON u.organization_id = o.id
         JOIN companies c ON u.company_id = c.id
         JOIN user_types ut ON u.type_id = ut.id
         WHERE u.username = $1"#,
        form.username
    )
    .fetch_one(pool.get_ref())
    .await;

    match user {
        Ok(user) => {
            if verify_password(&user.password, &form.password) {
                let token = create_token(&user.username, user.type_id);
                HttpResponse::Ok().json(LoginResponse {
                    token,
                    organization: user.organization,
                    organization_id: user.organization_id,
                    company: user.company,
                    company_id: user.company_id,
                    type_id: user.type_id,
                    user_type: user.user_type,
                })
            } else {
                HttpResponse::Unauthorized().json("Invalid credentials")
            }
        }
        Err(_) => HttpResponse::Unauthorized().json("User not found"),
    }
}

pub async fn get_user_types(pool: web::Data<PgPool>) -> impl Responder {
    match sqlx::query_as!(
        UserType,
        "SELECT id, name, code FROM user_types ORDER BY id"
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(types) => HttpResponse::Ok().json(types),
        Err(e) => {
            log::error!("Failed to fetch user types: {}", e);
            HttpResponse::InternalServerError().json("Error fetching user types")
        }
    }
}

pub async fn update_user_type(
    pool: web::Data<PgPool>,
    path: web::Path<String>,
    req: HttpRequest,
    form: web::Json<UpdateUserTypeRequest>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        // Проверка прав доступа (только для manager и buropropuskov)
                        let user = sqlx::query!(
                            r#"SELECT ut.code as user_type 
                               FROM users u
                               JOIN user_types ut ON u.type_id = ut.id
                               WHERE u.username = $1"#,
                            claims.sub
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?;

                        if user.user_type != "manager" && user.user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }

                        let username = path.into_inner();
                        
                        // Проверка существования типа
                        let type_exists = sqlx::query!(
                            "SELECT EXISTS(SELECT 1 FROM user_types WHERE id = $1) as exists",
                            form.type_id
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error checking user type"))?;

                        if !type_exists.exists.unwrap_or(false) {
                            return Err(error::ErrorBadRequest("Invalid user type"));
                        }

                        // Обновление типа в базе данных
                        sqlx::query!(
                            "UPDATE users SET type_id = $1 WHERE username = $2",
                            form.type_id,
                            username
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error updating user type"))?;

                        Ok(HttpResponse::Ok().json("User type updated successfully"))
                    }
                    Err(_) => Err(error::ErrorUnauthorized("Invalid or missing token")),
                }
            } else {
                Err(error::ErrorUnauthorized("Invalid or missing token"))
            }
        } else {
            Err(error::ErrorUnauthorized("Invalid or missing token"))
        }
    } else {
        Err(error::ErrorUnauthorized("Missing Authorization header"))
    }
}

pub async fn get_current_user(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    // Извлекаем токен из заголовка
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    // Декодируем токен с обработкой ошибки
    let claims = decode_token(token)
        .map_err(|e| error::ErrorUnauthorized(format!("Invalid token: {}", e)))?;

    // Получаем данные пользователя из БД
    let user = sqlx::query_as!(
         UserInfo,
    r#"
    SELECT 
        u.username, 
        o.name as organization, 
        u.organization_id, 
        c.name as company,
        u.company_id,
        u.type_id,
        ut.name as user_type,
        u.last_name,
        u.first_name,
        u.middle_name,
        u.position,
        u.email,
        u.phone
    FROM users u 
    JOIN organizations o ON u.organization_id = o.id
    JOIN companies c ON u.company_id = c.id
    JOIN user_types ut ON u.type_id = ut.id
    WHERE u.username = $1
    "#,
    claims.sub
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch user: {}", e);
        error::ErrorInternalServerError("Error fetching user")
    })?;

    Ok(HttpResponse::Ok().json(user))
}

pub async fn get_current_user_data(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    let token = req.headers().get("Authorization")
        .ok_or_else(|| error::ErrorUnauthorized("Missing Authorization header"))?
        .to_str()
        .map_err(|_| error::ErrorUnauthorized("Invalid Authorization header"))?
        .strip_prefix("Bearer ")
        .ok_or_else(|| error::ErrorUnauthorized("Invalid token format"))?;

    let claims = decode_token(token)
        .map_err(|e| error::ErrorUnauthorized(format!("Invalid token: {}", e)))?;

    let user = sqlx::query_as!(
        UserData,
        r#"
        SELECT 
            u.username,
            o.name as organization,
            u.organization_id,
            c.name as company,
            u.company_id,
            u.last_name,
            u.first_name,
            u.middle_name,
            u.phone
        FROM users u 
        JOIN organizations o ON u.organization_id = o.id
        JOIN companies c ON u.company_id = c.id
        WHERE u.username = $1
        "#,
        claims.sub
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to fetch user data: {}", e);
        error::ErrorInternalServerError("Error fetching user data")
    })?;

    Ok(HttpResponse::Ok().json(user))
}

// Убедитесь, что обработчик submit_application_v2 правильно обрабатывает ошибки

pub async fn submit_application_v2(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<ApplicationSubmitRequest>,
) -> Result<HttpResponse, Error> {
    // Проверяем авторизацию
    let token = if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                token
            } else {
                return Ok(HttpResponse::BadRequest().json(json!({
                    "message": "Invalid token format"
                })));
            }
        } else {
            return Ok(HttpResponse::BadRequest().json(json!({
                "message": "Invalid Authorization header"
            })));
        }
    } else {
        return Ok(HttpResponse::Unauthorized().json(json!({
            "message": "Missing Authorization header"
        })));
    };

    // Декодируем токен
    let claims = match decode_token(&token) {
        Ok(claims) => claims,
        Err(e) => {
            log::error!("Invalid token: {}", e);
            return Ok(HttpResponse::Unauthorized().json(json!({
                "message": "Invalid token"
            })));
        }
    };

    let mut transaction = match pool.begin().await {
        Ok(transaction) => transaction,
        Err(e) => {
            log::error!("Failed to start transaction: {}", e);
            return Ok(HttpResponse::InternalServerError().json(json!({
                "message": "Failed to start transaction"
            })));
        }
    };

    // Валидация обязательных полей заявки
    if form.application.organization.is_none() {
        return Ok(HttpResponse::BadRequest().json(json!({
            "message": "Organization is required"
        })));
    }
    
    if form.application.responsible_person.is_none() {
        return Ok(HttpResponse::BadRequest().json(json!({
            "message": "Responsible person is required"
        })));
    }
    
    if form.application.contact_phone.is_none() {
        return Ok(HttpResponse::BadRequest().json(json!({
            "message": "Contact phone is required"
        })));
    }
    
    if form.application.entry_date_from.is_none() {
        return Ok(HttpResponse::BadRequest().json(json!({
            "message": "Start date is required"
        })));
    }
    
    if form.application.entry_date_to.is_none() {
        return Ok(HttpResponse::BadRequest().json(json!({
            "message": "End date is required"
        })));
    }
    
    if form.application.entry_time_from.is_none() {
        return Ok(HttpResponse::BadRequest().json(json!({
            "message": "Start time is required"
        })));
    }
    
    if form.application.entry_time_to.is_none() {
        return Ok(HttpResponse::BadRequest().json(json!({
            "message": "End time is required"
        })));
    }

    // Вставка заявки
    let application_id = match sqlx::query!(
        "INSERT INTO applications (organization, responsible_person, contact_phone, 
         entry_date_from, entry_date_to, entry_time_from, entry_time_to) 
         VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id",
        form.application.organization.as_ref().unwrap(),
        form.application.responsible_person.as_ref().unwrap(),
        form.application.contact_phone.as_ref().unwrap(),
        form.application.entry_date_from.unwrap(),
        form.application.entry_date_to.unwrap(),
        form.application.entry_time_from.unwrap(),
        form.application.entry_time_to.unwrap()
    )
    .fetch_one(&mut *transaction)
    .await
    {
        Ok(row) => row.id,
        Err(e) => {
            log::error!("Error creating application: {}", e);
            return Ok(HttpResponse::InternalServerError().json(json!({
                "message": "Error creating application",
                "details": e.to_string()
            })));
        }
    };

    // Вставка машин
    for car_with_places in &form.cars {
        let car = &car_with_places.car;
        
        if car.car_number.is_none() || car.car_brand.is_none() {
            return Ok(HttpResponse::BadRequest().json(json!({
                "message": "Car number and brand are required"
            })));
        }

        let car_id = match sqlx::query!(
            "INSERT INTO cars (application_id, car_number, car_brand, 
             entry_date_from, entry_date_to, entry_time_from, entry_time_to) 
             VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id",
            application_id,
            car.car_number.as_ref().unwrap(),
            car.car_brand.as_ref().unwrap(),
            form.application.entry_date_from.unwrap(),
            form.application.entry_date_to.unwrap(),
            form.application.entry_time_from.unwrap(),
            form.application.entry_time_to.unwrap()
        )
        .fetch_one(&mut *transaction)
        .await
        {
            Ok(row) => row.id,
            Err(e) => {
                log::error!("Error adding car to application: {}", e);
                return Ok(HttpResponse::InternalServerError().json(json!({
                    "message": "Error adding car to application",
                    "details": e.to_string()
                })));
            }
        };

        // Вставка мест разгрузки
        for (index, unload_place) in car_with_places.unload_places.iter().enumerate() {
            if let Err(e) = sqlx::query!(
                "INSERT INTO car_unload_places (car_id, unload_place_id, order_index, planned_time, notes) 
                 VALUES ($1, $2, $3, $4, $5)",
                car_id,
                unload_place.unload_place_id,
                unload_place.order_index,
                unload_place.planned_time,
                unload_place.notes
            )
            .execute(&mut *transaction)
            .await
            {
                log::error!("Error adding unload place: {}", e);
                return Ok(HttpResponse::InternalServerError().json(json!({
                    "message": "Error adding unload place",
                    "details": e.to_string()
                })));
            }
        }
    }

    if let Err(e) = transaction.commit().await {
        log::error!("Failed to commit transaction: {}", e);
        return Ok(HttpResponse::InternalServerError().json(json!({
            "message": "Failed to commit transaction"
        })));
    }

    Ok(HttpResponse::Ok().json(json!({
        "success": true,
        "message": "Application submitted successfully",
        "application_id": application_id
    })))
}

// Обновляем обработчик получения организации
pub async fn get_organization(
    req: HttpRequest, 
    pool: web::Data<PgPool>
) -> impl Responder {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let user = sqlx::query!(
                            "SELECT o.name as organization, u.organization_id 
                             FROM users u JOIN organizations o ON u.organization_id = o.id 
                             WHERE u.username = $1", 
                            claims.sub
                        )
                        .fetch_one(pool.get_ref())
                        .await;

                        match user {
                            Ok(user) => HttpResponse::Ok().json(json!({ 
                                "organization": user.organization,
                                "organization_id": user.organization_id
                            })),
                            Err(_) => HttpResponse::InternalServerError().json("Не удалось получить организацию"),
                        }
                    }
                    Err(_) => HttpResponse::Unauthorized().json("Invalid or missing token"),
                }
            } else {
                HttpResponse::Unauthorized().json("Invalid or missing token")
            }
        } else {
            HttpResponse::Unauthorized().json("Invalid or missing token")
        }
    } else {
        HttpResponse::Unauthorized().json("Missing Authorization header")
    }
}

// Отправка заявки - ПОЛНОСТЬЮ ПЕРЕПИСАНА
pub async fn submit_application(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<ApplicationSubmitRequest>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(_claims) => {
                        let mut transaction = pool.begin().await.map_err(|e| {
                            log::error!("Failed to start transaction: {}", e);
                            error::ErrorInternalServerError("Failed to start transaction")
                        })?;

                        // Проверка обязательных полей заявки
                        if form.application.organization.is_none() || 
                           form.application.responsible_person.is_none() || 
                           form.application.contact_phone.is_none() || 
                           form.application.entry_date_from.is_none() || 
                           form.application.entry_date_to.is_none() ||
                           form.application.entry_time_from.is_none() ||
                           form.application.entry_time_to.is_none() {
                            return Err(error::ErrorBadRequest("Missing required fields in application"));
                        }

                        // Вставка заявки
                        let application_id = match sqlx::query!(
                            "INSERT INTO applications (organization, responsible_person, contact_phone, 
                             entry_date_from, entry_date_to, entry_time_from, entry_time_to) 
                             VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id",
                            form.application.organization.as_ref().unwrap(),
                            form.application.responsible_person.as_ref().unwrap(),
                            form.application.contact_phone.as_ref().unwrap(),
                            form.application.entry_date_from.unwrap(),
                            form.application.entry_date_to.unwrap(),
                            form.application.entry_time_from.unwrap(),
                            form.application.entry_time_to.unwrap()
                        )
                        .fetch_one(&mut *transaction)
                        .await
                        {
                            Ok(row) => row.id,
                            Err(e) => {
                                log::error!("Error creating application: {}", e);
                                return Err(error::ErrorInternalServerError(
                                    "Error creating application",
                                ));
                            }
                        };

                        // Вставка машин с местами разгрузки
                        for car_with_places in &form.cars {
                            let car = &car_with_places.car;
                            
                            // Проверка обязательных полей для машины
                            if car.car_number.is_none() || car.car_brand.is_none() {
                                return Err(error::ErrorBadRequest("Missing required fields for car"));
                            }

                            // Вставка машины
                            let car_id = match sqlx::query!(
                                "INSERT INTO cars (application_id, car_number, car_brand, 
                                 entry_date_from, entry_date_to, entry_time_from, entry_time_to) 
                                 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id",
                                application_id,
                                car.car_number.as_ref().unwrap(),
                                car.car_brand.as_ref().unwrap(),
                                form.application.entry_date_from.unwrap(),
                                form.application.entry_date_to.unwrap(),
                                form.application.entry_time_from.unwrap(),
                                form.application.entry_time_to.unwrap()
                            )
                            .fetch_one(&mut *transaction)
                            .await
                            {
                                Ok(row) => row.id,
                                Err(e) => {
                                    log::error!("Error adding car to application: {}", e);
                                    return Err(error::ErrorInternalServerError(
                                        "Error adding car to application",
                                    ));
                                }
                            };

                            // Вставка мест разгрузки для машины
                            for (index, unload_place) in car_with_places.unload_places.iter().enumerate() {
                                sqlx::query!(
                                    "INSERT INTO car_unload_places (car_id, unload_place_id, order_index, planned_time, notes) 
                                     VALUES ($1, $2, $3, $4, $5)",
                                    car_id,
                                    unload_place.unload_place_id,
                                    (index + 1) as i32,
                                    unload_place.planned_time,
                                    unload_place.notes
                                )
                                .execute(&mut *transaction)
                                .await
                                .map_err(|e| {
                                    log::error!("Error adding unload place: {}", e);
                                    error::ErrorInternalServerError("Error adding unload place")
                                })?;
                            }
                        }

                        transaction.commit().await.map_err(|e| {
                            log::error!("Failed to commit transaction: {}", e);
                            error::ErrorInternalServerError("Failed to commit transaction")
                        })?;

                        Ok(HttpResponse::Ok().json(json!({
                            "success": true,
                            "message": "Application submitted successfully"
                        })))
                    }
                    Err(e) => {
                        log::error!("Invalid token: {}", e);
                        Err(error::ErrorUnauthorized(json!({
                            "message": "Invalid or missing token"
                        })))
                    }
                }
            } else {
                Err(error::ErrorUnauthorized(json!({
                    "message": "Invalid or missing token"
                })))
            }
        } else {
            Err(error::ErrorUnauthorized(json!({
                "message": "Invalid or missing token"
            })))
        }
    } else {
        Err(error::ErrorUnauthorized(json!({
            "message": "Missing Authorization header"
        })))
    }
}

// Функция для получения всех заявок для аккаунта - ОБНОВЛЕНА
pub async fn get_all_cars_for_account(pool: web::Data<PgPool>) -> impl Responder {
    let applications_result = sqlx::query!(
        r#"SELECT id, organization, responsible_person, contact_phone, 
                  entry_date_from, entry_date_to, entry_time_from, entry_time_to, submission_datetime
         FROM applications"#
    )
    .fetch_all(pool.get_ref())
    .await;

    match applications_result {
        Ok(application_data) => {
            let mut applications_with_cars = Vec::new();

            for app in application_data {
                let cars = sqlx::query!(
                    r#"SELECT id, application_id, car_number, car_brand, 
                              entry_date_from, entry_date_to, entry_time_from, entry_time_to, 
                              status, date_added, date_removed
                     FROM cars WHERE application_id = $1"#,
                    app.id
                )
                .fetch_all(pool.get_ref())
                .await
                .unwrap_or_else(|_| vec![]);

                let mut cars_with_places = Vec::new();

                for car in cars {
                    let unload_places = sqlx::query!(
                        r#"SELECT cup.id, cup.car_id, cup.unload_place_id, cup.order_index, 
                                  cup.planned_time, cup.notes, up.name as unload_place_name
                           FROM car_unload_places cup
                           JOIN unload_places up ON cup.unload_place_id = up.id
                           WHERE cup.car_id = $1
                           ORDER BY cup.order_index"#,
                        car.id
                    )
                    .fetch_all(pool.get_ref())
                    .await
                    .unwrap_or_else(|_| vec![]);

                    let car_model = Car {
                        id: Some(car.id),
                        application_id: Some(car.application_id),
                        car_number: Some(car.car_number),
                        car_brand: Some(car.car_brand),
                        unload_place: None,
                        entry_date_from: Some(car.entry_date_from),
                        entry_date_to: Some(car.entry_date_to),
                        entry_time_from: Some(car.entry_time_from),
                        entry_time_to: Some(car.entry_time_to),
                        status: car.status,
                        date_added: car.date_added,
                        date_removed: car.date_removed,
                        unload_places: Some(unload_places.into_iter().map(|up| CarUnloadPlace {
                            id: Some(up.id),
                            car_id: Some(up.car_id),
                            unload_place_id: up.unload_place_id,
                            unload_place_name: Some(up.unload_place_name),
                            order_index: up.order_index,
                            planned_time: up.planned_time,
                            notes: up.notes,
                        }).collect()),
                    };

                    cars_with_places.push(car_model);
                }

                let application_with_cars = ApplicationWithCars {
                    id: Some(app.id),
                    organization: Some(app.organization),
                    responsible_person: Some(app.responsible_person),
                    contact_phone: Some(app.contact_phone),
                    entry_date_from: Some(app.entry_date_from),
                    entry_date_to: Some(app.entry_date_to),
                    entry_time_from: Some(app.entry_time_from),
                    entry_time_to: Some(app.entry_time_to),
                    submission_datetime: app.submission_datetime,
                    cars: cars_with_places,
                };

                applications_with_cars.push(application_with_cars);
            }

            HttpResponse::Ok().json(applications_with_cars)
        },
        Err(e) => {
            log::error!("Error fetching all cars: {}", e);
            HttpResponse::InternalServerError().json("Error fetching all cars for account")
        },
    }
}

// Функция для получения активных машин для таблицы - ОБНОВЛЕНА
pub async fn get_active_cars_for_table(pool: web::Data<PgPool>) -> impl Responder {
    let applications_result = sqlx::query!(
        r#"SELECT id, organization, responsible_person, contact_phone, 
                  entry_date_from, entry_date_to, entry_time_from, entry_time_to, submission_datetime 
           FROM applications"#
    )
    .fetch_all(pool.get_ref())
    .await;

    match applications_result {
        Ok(application_data) => {
            let mut applications_with_cars = Vec::new();

            for app in application_data {
                let cars = sqlx::query!(
                    r#"SELECT id, application_id, car_number, car_brand, 
                              entry_date_from, entry_date_to, entry_time_from, entry_time_to, 
                              status, date_added, date_removed
                       FROM cars WHERE application_id = $1 AND status = 1"#,
                    app.id
                )
                .fetch_all(pool.get_ref())
                .await
                .unwrap_or_else(|_| vec![]);

                let mut cars_with_places = Vec::new();

                for car in cars {
                    let unload_places = sqlx::query!(
                        r#"SELECT cup.id, cup.car_id, cup.unload_place_id, cup.order_index, 
                                  cup.planned_time, cup.notes, up.name as unload_place_name
                           FROM car_unload_places cup
                           JOIN unload_places up ON cup.unload_place_id = up.id
                           WHERE cup.car_id = $1
                           ORDER BY cup.order_index"#,
                        car.id
                    )
                    .fetch_all(pool.get_ref())
                    .await
                    .unwrap_or_else(|_| vec![]);

                    let car_model = Car {
                        id: Some(car.id),
                        application_id: Some(car.application_id),
                        car_number: Some(car.car_number),
                        car_brand: Some(car.car_brand),
                        unload_place: None,
                        entry_date_from: Some(car.entry_date_from),
                        entry_date_to: Some(car.entry_date_to),
                        entry_time_from: Some(car.entry_time_from),
                        entry_time_to: Some(car.entry_time_to),
                        status: car.status,
                        date_added: car.date_added,
                        date_removed: car.date_removed,
                        unload_places: Some(unload_places.into_iter().map(|up| CarUnloadPlace {
                            id: Some(up.id),
                            car_id: Some(up.car_id),
                            unload_place_id: up.unload_place_id,
                            unload_place_name: Some(up.unload_place_name),
                            order_index: up.order_index,
                            planned_time: up.planned_time,
                            notes: up.notes,
                        }).collect()),
                    };

                    cars_with_places.push(car_model);
                }

                let application_with_cars = ApplicationWithCars {
                    id: Some(app.id),
                    organization: Some(app.organization),
                    responsible_person: Some(app.responsible_person),
                    contact_phone: Some(app.contact_phone),
                    entry_date_from: Some(app.entry_date_from),
                    entry_date_to: Some(app.entry_date_to),
                    entry_time_from: Some(app.entry_time_from),
                    entry_time_to: Some(app.entry_time_to),
                    submission_datetime: app.submission_datetime,
                    cars: cars_with_places,
                };

                applications_with_cars.push(application_with_cars);
            }

            HttpResponse::Ok().json(applications_with_cars)
        },
        Err(e) => {
            log::error!("Error fetching active cars: {}", e);
            HttpResponse::InternalServerError().json("Error fetching active cars for table")
        },
    }
}

// Новый обработчик для получения мест разгрузки
pub async fn get_unload_places(pool: web::Data<PgPool>) -> impl Responder {
    match sqlx::query_as!(
        UnloadPlace,
        "SELECT id, name, description, is_active FROM unload_places WHERE is_active = true ORDER BY name"
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(places) => HttpResponse::Ok().json(places),
        Err(e) => {
            log::error!("Failed to fetch unload places: {}", e);
            HttpResponse::InternalServerError().json("Error fetching unload places")
        }
    }
}

// Обработчик для обновления данных машины - ОБНОВЛЕН
pub async fn update_car(
    pool: web::Data<PgPool>,
    path: web::Path<(i32, i32)>,
    car_data: web::Json<CarWithUnloadPlaces>
) -> Result<HttpResponse, Error> {
    let (application_id, car_id) = path.into_inner();
    let mut transaction = pool.begin().await.map_err(|e| {
        log::error!("Failed to start transaction: {}", e);
        error::ErrorInternalServerError("Failed to start transaction")
    })?;

    // Устанавливаем дату удаления, если статус равен 0
    let date_removed: Option<NaiveDate> = if car_data.car.status == Some(0) {
        Some(Utc::now().date_naive())
    } else {
        None
    };

    // Обновление данных машины
    let result = sqlx::query!(
        "UPDATE cars 
         SET car_number = $1, 
             car_brand = $2, 
             entry_date_from = $3,
             entry_date_to = $4,
             entry_time_from = $5,
             entry_time_to = $6,
             status = $7, 
             date_removed = $8
         WHERE application_id = $9 AND id = $10",
        car_data.car.car_number,
        car_data.car.car_brand,
        car_data.car.entry_date_from,
        car_data.car.entry_date_to,
        car_data.car.entry_time_from,
        car_data.car.entry_time_to,
        car_data.car.status,
        date_removed,
        application_id,
        car_id
    )
    .execute(&mut *transaction)
    .await;

    if result.is_err() {
        return Err(error::ErrorInternalServerError("Error updating car"));
    }

    // Обновление мест разгрузки
    sqlx::query!("DELETE FROM car_unload_places WHERE car_id = $1", car_id)
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Error deleting old unload places: {}", e);
            error::ErrorInternalServerError("Error updating unload places")
        })?;

    for (index, unload_place) in car_data.unload_places.iter().enumerate() {
        sqlx::query!(
            "INSERT INTO car_unload_places (car_id, unload_place_id, order_index, planned_time, notes) 
             VALUES ($1, $2, $3, $4, $5)",
            car_id,
            unload_place.unload_place_id,
            (index + 1) as i32,
            unload_place.planned_time,
            unload_place.notes
        )
        .execute(&mut *transaction)
        .await
        .map_err(|e| {
            log::error!("Error adding unload place: {}", e);
            error::ErrorInternalServerError("Error updating unload places")
        })?;
    }

    transaction.commit().await.map_err(|e| {
        log::error!("Failed to commit transaction: {}", e);
        error::ErrorInternalServerError("Failed to commit transaction")
    })?;

    Ok(HttpResponse::Ok().json("Car updated successfully"))
}

// Обработчик для удаления машины
pub async fn delete_car(
    pool: web::Data<PgPool>,
    path: web::Path<(i32, i32)>
) -> Result<HttpResponse, Error> {
    let (application_id, car_id) = path.into_inner();

    let result = sqlx::query!(
        "UPDATE cars SET status = 0, date_removed = CURRENT_DATE WHERE application_id = $1 AND id = $2",
        application_id,
        car_id
    )
    .execute(pool.get_ref())
    .await;

    match result {
        Ok(_) => Ok(HttpResponse::Ok().json("Car status updated to removed")),
        Err(e) => {
            eprintln!("Ошибка при обновлении статуса машины: {:?}", e);
            Err(error::ErrorInternalServerError("Error updating car status"))
        }
    }
}

// Обработчик для обновления заявки - НУЖНО ОБНОВИТЬ (оставлен старый для примера)
pub async fn update_application(
    pool: web::Data<PgPool>,
    path: web::Path<i32>,
    application_data: web::Json<Application>
) -> Result<HttpResponse, Error> {
    let application_id = path.into_inner();

    let result = sqlx::query!(
        "UPDATE applications 
         SET entry_date_from = $1, entry_date_to = $2, entry_time_from = $3, entry_time_to = $4
         WHERE id = $5",
        application_data.entry_date_from,
        application_data.entry_date_to,
        application_data.entry_time_from,
        application_data.entry_time_to,
        application_id
    )
    .execute(pool.get_ref())
    .await;

    match result {
        Ok(_) => Ok(HttpResponse::Ok().json("Application updated successfully")),
        Err(_) => Err(actix_web::error::ErrorInternalServerError("Failed to update application")),
    }
}

// Остальные обработчики остаются без изменений
pub async fn get_all_users(
    pool: web::Data<PgPool>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        // Проверка прав доступа
                        let user = sqlx::query!(
                            r#"SELECT ut.code as user_type 
                               FROM users u
                               JOIN user_types ut ON u.type_id = ut.id
                               WHERE u.username = $1"#,
                            claims.sub
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?;

                        if user.user_type != "manager" && user.user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }

                        // Получение списка всех пользователей
                        let users = sqlx::query_as!(
                            UserInfo,
    r#"
    SELECT 
        u.username, 
        o.name as organization, 
        u.organization_id, 
        c.name as company,
        u.company_id,
        u.type_id,
        ut.name as user_type,
        u.last_name,
        u.first_name,
        u.middle_name,
        u.position,
        u.email,
        u.phone
    FROM users u 
    JOIN organizations o ON u.organization_id = o.id
    JOIN companies c ON u.company_id = c.id
    JOIN user_types ut ON u.type_id = ut.id
    ORDER BY u.username
    "#
                        )
                        .fetch_all(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to fetch users: {}", e);
                            error::ErrorInternalServerError("Error fetching users")
                        })?;

                        Ok(HttpResponse::Ok().json(users))
                    }
                    Err(_) => Err(error::ErrorUnauthorized("Invalid or missing token")),
                }
            } else {
                Err(error::ErrorUnauthorized("Invalid or missing token"))
            }
        } else {
            Err(error::ErrorUnauthorized("Invalid or missing token"))
        }
    } else {
        Err(error::ErrorUnauthorized("Missing Authorization header"))
    }
}

pub async fn update_user_password(
    pool: web::Data<PgPool>,
    path: web::Path<String>,
    req: HttpRequest,
    form: web::Json<UpdatePasswordRequest>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        // Проверка прав доступа
                        let user = sqlx::query!(
                            r#"SELECT ut.code as user_type 
                               FROM users u
                               JOIN user_types ut ON u.type_id = ut.id
                               WHERE u.username = $1"#,
                            claims.sub
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?;

                        if user.user_type != "manager" && user.user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }

                        let username = path.into_inner();
                        let hashed_password = hash_password(&form.password);
                        
                        // Обновление пароля в базе данных
                        sqlx::query!(
                            "UPDATE users SET password = $1 WHERE username = $2",
                            hashed_password,
                            username
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error updating password"))?;

                        Ok(HttpResponse::Ok().json("Password updated successfully"))
                    }
                    Err(_) => Err(error::ErrorUnauthorized("Invalid or missing token")),
                }
            } else {
                Err(error::ErrorUnauthorized("Invalid or missing token"))
            }
        } else {
            Err(error::ErrorUnauthorized("Invalid or missing token"))
        }
    } else {
        Err(error::ErrorUnauthorized("Missing Authorization header"))
    }
}

pub async fn update_user_organization(
    pool: web::Data<PgPool>,
    path: web::Path<String>,
    req: HttpRequest,
    form: web::Json<UpdateOrganizationRequest>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        // Проверка прав доступа
                        let user = sqlx::query!(
                            r#"SELECT ut.code as user_type 
                               FROM users u
                               JOIN user_types ut ON u.type_id = ut.id
                               WHERE u.username = $1"#,
                            claims.sub
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?;

                        if user.user_type != "manager" && user.user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }

                        let username = path.into_inner();
                        
                        // Обновление организации в базе данных
                        sqlx::query!(
                            "UPDATE users SET organization_id = $1 WHERE username = $2",
                            form.organization_id,
                            username
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error updating organization"))?;

                        Ok(HttpResponse::Ok().json("Organization updated successfully"))
                    }
                    Err(_) => Err(error::ErrorUnauthorized("Invalid or missing token")),
                }
            } else {
                Err(error::ErrorUnauthorized("Invalid or missing token"))
            }
        } else {
            Err(error::ErrorUnauthorized("Invalid or missing token"))
        }
    } else {
        Err(error::ErrorUnauthorized("Missing Authorization header"))
    }
}

pub async fn update_user_info(
    pool: web::Data<PgPool>,
    path: web::Path<String>,
    req: HttpRequest,
    form: web::Json<UpdateUserRequest>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        // Проверка прав доступа
                        let user = sqlx::query!(
                            r#"SELECT ut.code as user_type 
                               FROM users u
                               JOIN user_types ut ON u.type_id = ut.id
                               WHERE u.username = $1"#,
                            claims.sub
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?;

                        if user.user_type != "manager" && user.user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }

                        let username = path.into_inner();
                        
                        // Обновление информации о пользователе
                        sqlx::query!(
                            r#"UPDATE users SET 
                                last_name = $1, 
                                first_name = $2, 
                                middle_name = $3, 
                                position = $4, 
                                email = $5, 
                                phone = $6 
                            WHERE username = $7"#,
                            form.last_name,
                            form.first_name,
                            form.middle_name,
                            form.position,
                            form.email,
                            form.phone,
                            username
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error updating user info"))?;

                        Ok(HttpResponse::Ok().json("User info updated successfully"))
                    }
                    Err(_) => Err(error::ErrorUnauthorized("Invalid or missing token")),
                }
            } else {
                Err(error::ErrorUnauthorized("Invalid or missing token"))
            }
        } else {
            Err(error::ErrorUnauthorized("Invalid or missing token"))
        }
    } else {
        Err(error::ErrorUnauthorized("Missing Authorization header"))
    }
}

pub async fn update_user_company(
    pool: web::Data<PgPool>,
    path: web::Path<String>,
    req: HttpRequest,
    form: web::Json<UpdateCompanyRequest>,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        // Проверка прав доступа
                        let user = sqlx::query!(
                            r#"SELECT ut.code as user_type 
                               FROM users u
                               JOIN user_types ut ON u.type_id = ut.id
                               WHERE u.username = $1"#,
                            claims.sub
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?;

                        if user.user_type != "manager" && user.user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }

                        let username = path.into_inner();
                        
                        sqlx::query!(
                            "UPDATE users SET company_id = $1 WHERE username = $2",
                            form.company_id,
                            username
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorInternalServerError("Error updating company"))?;

                        Ok(HttpResponse::Ok().json("Company updated successfully"))
                    }
                    Err(_) => Err(error::ErrorUnauthorized("Invalid or missing token")),
                }
            } else {
                Err(error::ErrorUnauthorized("Invalid or missing token"))
            }
        } else {
            Err(error::ErrorUnauthorized("Invalid or missing token"))
        }
    } else {
        Err(error::ErrorUnauthorized("Missing Authorization header"))
    }
}

pub async fn get_all_organizations(pool: web::Data<PgPool>) -> impl Responder {
    match sqlx::query_as!(
        OrganizationInfo,
        "SELECT id, name FROM organizations ORDER BY name"
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(orgs) => HttpResponse::Ok().json(orgs),
        Err(e) => {
            log::error!("Failed to fetch organizations: {}", e);
            HttpResponse::InternalServerError().json("Error fetching organizations")
        }
    }
}

pub async fn get_organizations_with_users(pool: web::Data<PgPool>) -> impl Responder {
    match sqlx::query_as!(
        OrganizationWithUsers,
        r#"
        SELECT o.id, o.name, COUNT(u.id) as "user_count!"
        FROM organizations o
        LEFT JOIN users u ON u.organization_id = o.id
        GROUP BY o.id
        ORDER BY o.name
        "#
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(orgs) => HttpResponse::Ok().json(orgs),
        Err(e) => {
            log::error!("Failed to fetch organizations with users: {}", e);
            HttpResponse::InternalServerError().json("Error fetching organizations")
        }
    }
}

pub async fn create_organization(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<NewOrganization>,
) -> Result<HttpResponse, Error> {
    check_admin_permissions(&req, &pool).await?;

    let org = sqlx::query_as!(
        OrganizationInfo,
        "INSERT INTO organizations (name) VALUES ($1) RETURNING id, name",
        form.name
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to create organization: {}", e);
        error::ErrorInternalServerError("Error creating organization")
    })?;

    Ok(HttpResponse::Ok().json(org))
}

pub async fn get_all_companies(pool: web::Data<PgPool>) -> impl Responder {
    match sqlx::query_as!(
        Company,
        "SELECT id, name FROM companies ORDER BY name"
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(companies) => HttpResponse::Ok().json(companies),
        Err(e) => {
            log::error!("Failed to fetch companies: {}", e);
            HttpResponse::InternalServerError().json("Error fetching companies")
        }
    }
}

pub async fn update_organization(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<NewOrganization>,
) -> Result<HttpResponse, Error> {
    check_admin_permissions(&req, &pool).await?;

    let id = path.into_inner();
    let org = sqlx::query_as!(
        OrganizationInfo,
        "UPDATE organizations SET name = $1 WHERE id = $2 RETURNING id, name",
        form.name,
        id
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to update organization: {}", e);
        error::ErrorInternalServerError("Error updating organization")
    })?;

    Ok(HttpResponse::Ok().json(org))
}

pub async fn delete_organization(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
) -> Result<HttpResponse, Error> {
    check_admin_permissions(&req, &pool).await?;

    let id = path.into_inner();
    
    let user_count: i64 = sqlx::query!(
    "SELECT COUNT(*) as count FROM users WHERE organization_id = $1",
    id
    )
    .fetch_one(pool.get_ref())
    .await
    .map(|r| r.count.unwrap_or(0))
    .map_err(|_| error::ErrorInternalServerError("Error checking users"))?;

    if user_count > 0 {
        return Err(error::ErrorBadRequest("Cannot delete organization with users"));
    }

    sqlx::query!("DELETE FROM organizations WHERE id = $1", id)
        .execute(pool.get_ref())
        .await
        .map_err(|e| {
            log::error!("Failed to delete organization: {}", e);
            error::ErrorInternalServerError("Error deleting organization")
        })?;

    Ok(HttpResponse::Ok().json(json!({"message": "Organization deleted"})))
}

pub async fn get_companies_with_users(pool: web::Data<PgPool>) -> impl Responder {
    match sqlx::query_as!(
        CompanyWithUsers,
        r#"
        SELECT c.id, c.name, COUNT(u.id) as "user_count!"
        FROM companies c
        LEFT JOIN users u ON u.company_id = c.id
        GROUP BY c.id
        ORDER BY c.name
        "#
    )
    .fetch_all(pool.get_ref())
    .await {
        Ok(companies) => HttpResponse::Ok().json(companies),
        Err(e) => {
            log::error!("Failed to fetch companies with users: {}", e);
            HttpResponse::InternalServerError().json("Error fetching companies")
        }
    }
}

pub async fn create_company(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    form: web::Json<NewCompany>,
) -> Result<HttpResponse, Error> {
    check_admin_permissions(&req, &pool).await?;

    let company = sqlx::query_as!(
        Company,
        "INSERT INTO companies (name) VALUES ($1) RETURNING id, name",
        form.name
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to create company: {}", e);
        error::ErrorInternalServerError("Error creating company")
    })?;

    Ok(HttpResponse::Ok().json(company))
}

pub async fn update_company(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
    form: web::Json<NewCompany>,
) -> Result<HttpResponse, Error> {
    check_admin_permissions(&req, &pool).await?;

    let id = path.into_inner();
    let company = sqlx::query_as!(
        Company,
        "UPDATE companies SET name = $1 WHERE id = $2 RETURNING id, name",
        form.name,
        id
    )
    .fetch_one(pool.get_ref())
    .await
    .map_err(|e| {
        log::error!("Failed to update company: {}", e);
        error::ErrorInternalServerError("Error updating company")
    })?;

    Ok(HttpResponse::Ok().json(company))
}

pub async fn delete_company(
    pool: web::Data<PgPool>,
    req: HttpRequest,
    path: web::Path<i32>,
) -> Result<HttpResponse, Error> {
    check_admin_permissions(&req, &pool).await?;

    let id = path.into_inner();
    
    let user_count: i64 = sqlx::query!(
    "SELECT COUNT(*) as count FROM users WHERE company_id = $1",
    id
)
.fetch_one(pool.get_ref())
.await
.map(|r| r.count.unwrap_or(0))
.map_err(|_| error::ErrorInternalServerError("Error checking users"))?;

if user_count > 0 {
    return Err(error::ErrorBadRequest("Cannot delete company with users"));
}

    sqlx::query!("DELETE FROM companies WHERE id = $1", id)
        .execute(pool.get_ref())
        .await
        .map_err(|e| {
            log::error!("Failed to delete company: {}", e);
            error::ErrorInternalServerError("Error deleting company")
        })?;

    Ok(HttpResponse::Ok().json(json!({"message": "Company deleted"})))
}

async fn check_admin_permissions(
    req: &HttpRequest,
    pool: &web::Data<PgPool>,
) -> Result<(), Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        let user = sqlx::query!(
                            r#"SELECT ut.code as user_type 
                               FROM users u
                               JOIN user_types ut ON u.type_id = ut.id
                               WHERE u.username = $1"#,
                            claims.sub
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?;

                        if user.user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }
                        Ok(())
                    }
                    Err(_) => Err(error::ErrorUnauthorized("Invalid or missing token")),
                }
            } else {
                Err(error::ErrorUnauthorized("Invalid or missing token"))
            }
        } else {
            Err(error::ErrorUnauthorized("Invalid or missing token"))
        }
    } else {
        Err(error::ErrorUnauthorized("Missing Authorization header"))
    }
}

pub async fn delete_user(
    pool: web::Data<PgPool>,
    path: web::Path<String>,
    req: HttpRequest,
) -> Result<HttpResponse, Error> {
    if let Some(auth_header) = req.headers().get("Authorization") {
        if let Ok(auth_str) = auth_header.to_str() {
            if let Some(token) = auth_str.strip_prefix("Bearer ") {
                match decode_token(&token) {
                    Ok(claims) => {
                        // Проверка прав доступа
                        let user = sqlx::query!(
                            r#"SELECT ut.code as user_type 
                               FROM users u
                               JOIN user_types ut ON u.type_id = ut.id
                               WHERE u.username = $1"#,
                            claims.sub
                        )
                        .fetch_one(pool.get_ref())
                        .await
                        .map_err(|_| error::ErrorUnauthorized("User not found"))?;

                        if user.user_type != "manager" && user.user_type != "buropropuskov" {
                            return Err(error::ErrorForbidden("Insufficient permissions"));
                        }

                        let username = path.into_inner();
                        
                        // Удаление пользователя
                        sqlx::query!(
                            "DELETE FROM users WHERE username = $1",
                            username
                        )
                        .execute(pool.get_ref())
                        .await
                        .map_err(|e| {
                            log::error!("Failed to delete user: {}", e);
                            error::ErrorInternalServerError("Error deleting user")
                        })?;

                        Ok(HttpResponse::Ok().json(json!({"message": "User deleted successfully"})))
                    }
                    Err(_) => Err(error::ErrorUnauthorized("Invalid or missing token")),
                }
            } else {
                Err(error::ErrorUnauthorized("Invalid or missing token"))
            }
        } else {
            Err(error::ErrorUnauthorized("Invalid or missing token"))
        }
    } else {
        Err(error::ErrorUnauthorized("Missing Authorization header"))
    }
}