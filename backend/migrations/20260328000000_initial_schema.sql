-- Справочники (без зависимостей)
CREATE TABLE user_types (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    name VARCHAR NOT NULL,
    code VARCHAR NOT NULL UNIQUE
);

CREATE TABLE organizations (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    name VARCHAR NOT NULL
);

CREATE TABLE companies (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    name VARCHAR NOT NULL
);

CREATE TABLE citizenships (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    name VARCHAR NOT NULL,
    icon VARCHAR,
    is_active BOOLEAN DEFAULT TRUE,
    is_default BOOLEAN DEFAULT FALSE,
    patent_required BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE license_plate_formats (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    name VARCHAR NOT NULL,
    country_code VARCHAR,
    icon VARCHAR,
    is_active BOOLEAN DEFAULT TRUE,
    is_default BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE license_plate_format_cells (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    format_id INTEGER NOT NULL REFERENCES license_plate_formats(id) ON DELETE CASCADE,
    cell_order INTEGER NOT NULL,
    cell_type VARCHAR NOT NULL,
    min_length INTEGER NOT NULL DEFAULT 1,
    max_length INTEGER NOT NULL DEFAULT 1,
    allowed_letters VARCHAR,
    alphabet_type VARCHAR,
    language VARCHAR,
    padding_char VARCHAR,
    padding_side VARCHAR,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Пользователи
CREATE TABLE users (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    username VARCHAR NOT NULL UNIQUE,
    password VARCHAR NOT NULL,
    organization_id INTEGER NOT NULL REFERENCES organizations(id),
    company_id INTEGER NOT NULL REFERENCES companies(id),
    type_id INTEGER NOT NULL REFERENCES user_types(id),
    last_name VARCHAR,
    first_name VARCHAR,
    middle_name VARCHAR,
    position VARCHAR,
    email VARCHAR,
    phone VARCHAR
);

CREATE TABLE refresh_tokens (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    is_revoked BOOLEAN DEFAULT FALSE
);

-- Места разгрузки
CREATE TABLE unload_places (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    name VARCHAR NOT NULL,
    description TEXT,
    map_link VARCHAR,
    status VARCHAR,
    status_comment VARCHAR,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE unload_place_time_slots (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    unload_place_id INTEGER NOT NULL REFERENCES unload_places(id) ON DELETE CASCADE,
    day_of_week INTEGER NOT NULL,
    open_time TIME NOT NULL,
    close_time TIME NOT NULL,
    is_next_day BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE unload_place_photos (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    unload_place_id INTEGER NOT NULL REFERENCES unload_places(id) ON DELETE CASCADE,
    photo_url VARCHAR NOT NULL,
    file_name VARCHAR NOT NULL,
    file_size INTEGER DEFAULT 0,
    mime_type VARCHAR,
    is_main BOOLEAN DEFAULT FALSE,
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    uploaded_by INTEGER REFERENCES users(id)
);

-- Системные таблицы
CREATE TABLE system_tables (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    name VARCHAR NOT NULL,
    display_name VARCHAR NOT NULL DEFAULT '',
    table_type VARCHAR NOT NULL DEFAULT '',
    show_fact_table BOOLEAN DEFAULT FALSE,
    fact_table_hint VARCHAR,
    instruction TEXT,
    map_link VARCHAR,
    status VARCHAR,
    status_comment VARCHAR,
    location_description VARCHAR,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE system_table_time_slots (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    table_id INTEGER NOT NULL REFERENCES system_tables(id) ON DELETE CASCADE,
    day_of_week INTEGER NOT NULL,
    open_time TIME NOT NULL,
    close_time TIME NOT NULL,
    is_next_day BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE system_table_photos (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    table_id INTEGER NOT NULL REFERENCES system_tables(id) ON DELETE CASCADE,
    photo_url VARCHAR NOT NULL,
    file_name VARCHAR NOT NULL,
    file_size INTEGER DEFAULT 0,
    mime_type VARCHAR,
    is_main BOOLEAN DEFAULT FALSE,
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    uploaded_by INTEGER REFERENCES users(id)
);

-- Поля таблиц (конструктор)
CREATE TABLE table_fields (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    table_id INTEGER REFERENCES system_tables(id) ON DELETE CASCADE,
    field_name VARCHAR NOT NULL,
    field_type VARCHAR NOT NULL,
    display_order INTEGER DEFAULT 0,
    is_visible BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Junction tables: организации/компании <-> места/таблицы/пользователи
CREATE TABLE organization_unload_places (
    organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    unload_place_id INTEGER NOT NULL REFERENCES unload_places(id) ON DELETE CASCADE,
    PRIMARY KEY (organization_id, unload_place_id)
);

CREATE TABLE companies_unload_places (
    company_id INTEGER NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    unload_place_id INTEGER NOT NULL REFERENCES unload_places(id) ON DELETE CASCADE,
    PRIMARY KEY (company_id, unload_place_id)
);

CREATE TABLE organization_tables (
    organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    table_id INTEGER NOT NULL REFERENCES system_tables(id) ON DELETE CASCADE,
    PRIMARY KEY (organization_id, table_id)
);

CREATE TABLE companies_tables (
    company_id INTEGER NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    table_id INTEGER NOT NULL REFERENCES system_tables(id) ON DELETE CASCADE,
    PRIMARY KEY (company_id, table_id)
);

CREATE TABLE organization_users (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_primary BOOLEAN DEFAULT FALSE,
    required_approval BOOLEAN DEFAULT FALSE
);

CREATE TABLE companies_users (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    company_id INTEGER NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_primary BOOLEAN DEFAULT FALSE,
    required_approval BOOLEAN DEFAULT FALSE
);

-- Уникальные машины и сотрудники (справочники)
CREATE TABLE unique_cars (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    number VARCHAR NOT NULL,
    mark VARCHAR NOT NULL DEFAULT '',
    organization_id INTEGER REFERENCES organizations(id),
    company_id INTEGER REFERENCES companies(id),
    format_id INTEGER REFERENCES license_plate_formats(id),
    user_id INTEGER REFERENCES users(id),
    status BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE unique_employees (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    last_name VARCHAR,
    first_name VARCHAR,
    middle_name VARCHAR,
    citizenship_id INTEGER REFERENCES citizenships(id),
    position VARCHAR,
    passport_series_number VARCHAR,
    patent_number VARCHAR,
    other_permission VARCHAR,
    organization_id INTEGER REFERENCES organizations(id),
    company_id INTEGER REFERENCES companies(id),
    user_id INTEGER REFERENCES users(id),
    status BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Вложения (бланки/шаблоны)
CREATE TABLE unique_attachments (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    attachment_type VARCHAR NOT NULL,
    name VARCHAR NOT NULL,
    display_name VARCHAR NOT NULL DEFAULT '',
    title VARCHAR NOT NULL DEFAULT '',
    instruction TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    status INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Заявки
CREATE TABLE applications (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    application_number VARCHAR NOT NULL UNIQUE DEFAULT '',
    confirmation VARCHAR NOT NULL DEFAULT '',
    sending_datetime TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    reading_datetime TIMESTAMPTZ,
    confirmation_datetime TIMESTAMPTZ,
    organization_id INTEGER REFERENCES organizations(id),
    company_id INTEGER REFERENCES companies(id),
    sender_user_id INTEGER NOT NULL REFERENCES users(id),
    status VARCHAR NOT NULL DEFAULT 'Непрочитано',
    responsible_user_id INTEGER REFERENCES users(id),
    responsible_comment TEXT,
    data_approval BOOLEAN NOT NULL DEFAULT FALSE,
    message TEXT
);

CREATE TABLE attachments (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    application_id INTEGER NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    unique_attachment_id INTEGER REFERENCES unique_attachments(id),
    attachment_type VARCHAR NOT NULL DEFAULT '',
    attachment_name VARCHAR NOT NULL DEFAULT '',
    attachment_display_name VARCHAR,
    entry_date_from DATE,
    entry_date_to DATE,
    entry_time_from TIME,
    entry_time_to TIME,
    status INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Машины (в заявках)
CREATE TABLE cars (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    attachment_id INTEGER REFERENCES attachments(id) ON DELETE CASCADE,
    car_number VARCHAR NOT NULL,
    car_brand VARCHAR NOT NULL DEFAULT '',
    unload_place VARCHAR,
    entry_date_from DATE NOT NULL DEFAULT CURRENT_DATE,
    entry_date_to DATE NOT NULL DEFAULT CURRENT_DATE,
    entry_time_from TIME NOT NULL DEFAULT '00:00:00',
    entry_time_to TIME NOT NULL DEFAULT '23:59:59',
    territory_entry_time TIMESTAMP,
    territory_status INTEGER DEFAULT 0,
    status INTEGER DEFAULT 1,
    date_added TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    date_removed TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE car_unload_places (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    car_id INTEGER NOT NULL REFERENCES cars(id) ON DELETE CASCADE,
    unload_place_id INTEGER NOT NULL REFERENCES unload_places(id),
    order_index INTEGER NOT NULL DEFAULT 0,
    planned_time VARCHAR,
    notes TEXT
);

CREATE TABLE cars_history (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    car_id INTEGER NOT NULL REFERENCES cars(id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users(id),
    action_type VARCHAR NOT NULL,
    field_name VARCHAR,
    old_value TEXT,
    new_value TEXT,
    comment TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    metadata JSONB
);

-- Сотрудники (в заявках)
CREATE TABLE employees (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    attachment_id INTEGER REFERENCES attachments(id) ON DELETE CASCADE,
    last_name VARCHAR NOT NULL,
    first_name VARCHAR NOT NULL DEFAULT '',
    middle_name VARCHAR,
    citizenship_id INTEGER REFERENCES citizenships(id),
    position VARCHAR,
    passport_series_number VARCHAR,
    patent_number VARCHAR,
    other_permission VARCHAR,
    territory_entry_time TIMESTAMPTZ,
    territory_status INTEGER NOT NULL DEFAULT 0,
    status INTEGER DEFAULT 1,
    date_created TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    date_deleted TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE employee_target_tables (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    employee_id INTEGER NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    table_id INTEGER NOT NULL REFERENCES system_tables(id),
    order_index INTEGER NOT NULL DEFAULT 0
);

-- Предметы (в заявках)
CREATE TABLE items (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    attachment_id INTEGER REFERENCES attachments(id) ON DELETE CASCADE,
    name VARCHAR NOT NULL,
    count INTEGER NOT NULL DEFAULT 1,
    date_created DATE DEFAULT CURRENT_DATE,
    date_deleted DATE
);

-- Согласование заявок
CREATE TABLE application_approvers (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    user_id INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by INTEGER REFERENCES users(id)
);

CREATE TABLE application_responsible_users (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    application_id INTEGER NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id),
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    required_approval BOOLEAN NOT NULL DEFAULT FALSE,
    approval_status VARCHAR DEFAULT 'pending',
    approval_comment VARCHAR,
    approval_datetime TIMESTAMPTZ,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by INTEGER REFERENCES users(id),
    UNIQUE (application_id, user_id)
);

CREATE TABLE application_viewers (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    application_id INTEGER NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by INTEGER REFERENCES users(id)
);

CREATE TABLE application_history (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    application_id INTEGER NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id),
    action_type VARCHAR NOT NULL,
    action_status VARCHAR,
    old_value TEXT,
    new_value TEXT,
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    metadata JSONB
);

-- Обратная связь
CREATE TABLE feedback (
    id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    user_id INTEGER NOT NULL REFERENCES users(id),
    message TEXT NOT NULL,
    status VARCHAR NOT NULL DEFAULT 'new',
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Seed: базовые типы пользователей
INSERT INTO user_types (name, code) VALUES
    ('Менеджер', 'manager'),
    ('Бюро пропусков', 'buropropuskov'),
    ('Диспетчер', 'dispatcher'),
    ('Сотрудник', 'employee');

-- Seed: тестовая организация и компания (для FK в users)
INSERT INTO organizations (name) VALUES ('Тестовая организация');
INSERT INTO companies (name) VALUES ('Тестовая компания');
