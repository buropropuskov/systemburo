-- Seed 1000 тестовых пользователей для нагрузочного тестирования.
-- Пароль: LoadTest123456789012345678901234 (Argon2id хеш)
-- Запуск: docker compose exec -i db psql -U postgres -d auto_registry < load-tests/seed.sql

DO $$
DECLARE
  i INTEGER;
  org_id INTEGER;
  company_id INTEGER;
  -- Argon2id хеш для 'LoadTest123456789012345678901234'
  -- Сгенерировать актуальный хеш: go run ./cmd/genhash 'LoadTest123456789012345678901234'
  pass_hash TEXT := '$argon2id$v=19$m=19456,t=2,p=1$PLACEHOLDER_GENERATE_WITH_GO$PLACEHOLDER';
BEGIN
  INSERT INTO organizations (name, created_at, updated_at)
  VALUES ('LoadTest Organization', NOW(), NOW())
  ON CONFLICT DO NOTHING
  RETURNING id INTO org_id;

  IF org_id IS NULL THEN
    SELECT id INTO org_id FROM organizations WHERE name = 'LoadTest Organization';
  END IF;

  INSERT INTO companies (name, created_at, updated_at)
  VALUES ('LoadTest Company', NOW(), NOW())
  ON CONFLICT DO NOTHING
  RETURNING id INTO company_id;

  IF company_id IS NULL THEN
    SELECT id INTO company_id FROM companies WHERE name = 'LoadTest Company';
  END IF;

  FOR i IN 0..999 LOOP
    INSERT INTO users (username, password, organization_id, company_id, type_id, created_at, updated_at)
    VALUES (
      'loadtest_user_' || i,
      pass_hash,
      org_id,
      company_id,
      1,
      NOW(),
      NOW()
    )
    ON CONFLICT (username) DO NOTHING;
  END LOOP;

  RAISE NOTICE 'Seeded 1000 load test users in org % company %', org_id, company_id;
END $$;
