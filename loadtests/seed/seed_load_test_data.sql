-- Datos sinteticos para pruebas de carga con k6.
-- Se ejecuta directamente contra Postgres (bypass de la API) para no
-- consumir el rate limit de /api/v1/auth/register (100 req/min por IP)
-- ni depender del flujo de verificacion de correo por Mailpit.
--
-- Uso:
--   docker compose exec -T postgres psql -U mooc_user -d mooc_db < loadtests/seed/seed_load_test_data.sql
-- o, contra Cloud SQL via el Auth Proxy:
--   psql "postgres://mooc_user:PASSWORD@127.0.0.1:5432/mooc_db" -f loadtests/seed/seed_load_test_data.sql
--
-- Es idempotente: se puede correr varias veces sin duplicar datos.

BEGIN;

-- 1) Pool de estudiantes de prueba, ya verificados y activos.
--    Password para TODOS: LoadTest123!
--    Hash bcrypt precalculado (compatible con golang.org/x/crypto/bcrypt).
DO $$
DECLARE
  i INT;
  test_email TEXT;
  password_hash TEXT := '$2b$10$7ALjOU2uvXXmW7KSElPwxen0DDBrPEWBlzdSkurk/TEnjMEMFIgjO';
BEGIN
  FOR i IN 1..300 LOOP
    test_email := 'loadtest' || LPAD(i::TEXT, 4, '0') || '@mooc.test';
    INSERT INTO users (email, password_hash, full_name, role, status, email_verified_at)
    VALUES (test_email, password_hash, 'Load Test Student ' || i, 'student', 'active', NOW())
    ON CONFLICT (email) DO NOTHING;
  END LOOP;
END $$;

-- 2) Un profesor de prueba para autorar el curso usado en las pruebas.
INSERT INTO users (id, email, password_hash, full_name, role, status, email_verified_at)
VALUES (
  '10000000-0000-0000-0000-000000000090',
  'loadtest-teacher@mooc.test',
  '$2b$10$7ALjOU2uvXXmW7KSElPwxen0DDBrPEWBlzdSkurk/TEnjMEMFIgjO',
  'Load Test Teacher',
  'teacher',
  'active',
  NOW()
)
ON CONFLICT (id) DO NOTHING;

-- 3) Curso publicado con jerarquia minima (Modulo -> Unidad -> Recurso visible),
--    fijado con IDs deterministicos para que el script de k6 no tenga que
--    descubrirlos en tiempo de ejecucion.
INSERT INTO courses (id, slug, title, summary, created_by_teacher_id)
VALUES (
  '10000000-0000-0000-0000-000000000001',
  'curso-prueba-de-carga',
  'Curso de Prueba de Carga',
  'Curso sintetico usado unicamente para pruebas de carga automatizadas con k6.',
  '10000000-0000-0000-0000-000000000090'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO course_versions (id, course_id, version_number, status, passing_score, published_at)
VALUES (
  '10000000-0000-0000-0000-000000000002',
  '10000000-0000-0000-0000-000000000001',
  1,
  'published',
  70.00,
  NOW()
)
ON CONFLICT (id) DO NOTHING;

UPDATE courses
SET current_published_version_id = '10000000-0000-0000-0000-000000000002'
WHERE id = '10000000-0000-0000-0000-000000000001';

INSERT INTO course_modules (id, version_id, stable_id, title, description, position)
VALUES (
  '10000000-0000-0000-0000-000000000003',
  '10000000-0000-0000-0000-000000000002',
  '10000000-0000-0000-0000-000000000013',
  'Modulo 1 - Introduccion',
  'Modulo sintetico de prueba de carga.',
  1
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO course_units (id, module_id, stable_id, title, position)
VALUES (
  '10000000-0000-0000-0000-000000000004',
  '10000000-0000-0000-0000-000000000003',
  '10000000-0000-0000-0000-000000000014',
  'Unidad 1',
  1
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO course_resources (
  id, unit_id, stable_id, title, type, canonical_markdown,
  is_visible, is_mandatory, is_downloadable, position, processing_status
)
VALUES (
  '10000000-0000-0000-0000-000000000005',
  '10000000-0000-0000-0000-000000000004',
  '10000000-0000-0000-0000-000000000015',
  'Lectura introductoria',
  'text',
  'Contenido sintetico usado por la prueba de carga (heartbeat de progreso).',
  TRUE,
  TRUE,
  FALSE,
  1,
  'completed'
)
ON CONFLICT (id) DO NOTHING;

-- 4) Estudiante "vitrina" ya aprobado, con insignia emitida, para poder
--    probar GET /api/v1/badges/verify/{code} (publico, sin auth) bajo
--    carga sin tener que simular el flujo completo de aprobacion.
INSERT INTO users (id, email, password_hash, full_name, role, status, email_verified_at)
VALUES (
  '10000000-0000-0000-0000-000000000091',
  'loadtest-showcase@mooc.test',
  '$2b$10$7ALjOU2uvXXmW7KSElPwxen0DDBrPEWBlzdSkurk/TEnjMEMFIgjO',
  'Load Test Showcase Student',
  'student',
  'active',
  NOW()
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO enrollments (student_id, course_id, status, academic_status, progress_percentage)
VALUES (
  '10000000-0000-0000-0000-000000000091',
  '10000000-0000-0000-0000-000000000001',
  'active',
  'approved',
  100.00
)
ON CONFLICT (student_id, course_id) DO NOTHING;

INSERT INTO badges (
  id, student_id, course_id, course_version_id, verification_code,
  image_url, verification_url
)
VALUES (
  '10000000-0000-0000-0000-000000000099',
  '10000000-0000-0000-0000-000000000091',
  '10000000-0000-0000-0000-000000000001',
  '10000000-0000-0000-0000-000000000002',
  '10000000-0000-0000-0000-000000000199',
  'https://placeholder.local/badges/loadtest.png',
  'https://placeholder.local/badges/verify/10000000-0000-0000-0000-000000000199'
)
ON CONFLICT (id) DO NOTHING;

-- 5) Recurso de tipo quiz + quiz con 3 preguntas (2 opciones c/u) para poder
--    ejercitar el flujo de presentacion de quiz en las pruebas de carga
--    (Escenario 1: actividad academica concurrente).
INSERT INTO course_resources (
  id, unit_id, stable_id, title, type, canonical_markdown,
  is_visible, is_mandatory, is_downloadable, position, processing_status
)
VALUES (
  '10000000-0000-0000-0000-000000000006',
  '10000000-0000-0000-0000-000000000004',
  '10000000-0000-0000-0000-000000000016',
  'Quiz de practica',
  'quiz',
  NULL,
  TRUE,
  TRUE,
  FALSE,
  2,
  'completed'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO quizzes (id, resource_id, max_attempts, passing_score)
VALUES (
  '10000000-0000-0000-0000-000000000007',
  '10000000-0000-0000-0000-000000000006',
  3,
  70.00
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO quiz_questions (id, quiz_id, question_text, position, points)
VALUES
  ('10000000-0000-0000-0000-000000000021', '10000000-0000-0000-0000-000000000007', 'Pregunta sintetica 1 de prueba de carga', 1, 1.00),
  ('10000000-0000-0000-0000-000000000022', '10000000-0000-0000-0000-000000000007', 'Pregunta sintetica 2 de prueba de carga', 2, 1.00),
  ('10000000-0000-0000-0000-000000000023', '10000000-0000-0000-0000-000000000007', 'Pregunta sintetica 3 de prueba de carga', 3, 1.00)
ON CONFLICT (id) DO NOTHING;

INSERT INTO quiz_options (id, question_id, option_text, is_correct, position)
VALUES
  ('10000000-0000-0000-0000-000000000031', '10000000-0000-0000-0000-000000000021', 'Opcion correcta 1', TRUE, 1),
  ('10000000-0000-0000-0000-000000000032', '10000000-0000-0000-0000-000000000021', 'Opcion incorrecta 1', FALSE, 2),
  ('10000000-0000-0000-0000-000000000034', '10000000-0000-0000-0000-000000000022', 'Opcion correcta 2', TRUE, 1),
  ('10000000-0000-0000-0000-000000000035', '10000000-0000-0000-0000-000000000022', 'Opcion incorrecta 2', FALSE, 2),
  ('10000000-0000-0000-0000-000000000037', '10000000-0000-0000-0000-000000000023', 'Opcion correcta 3', TRUE, 1),
  ('10000000-0000-0000-0000-000000000038', '10000000-0000-0000-0000-000000000023', 'Opcion incorrecta 3', FALSE, 2)
ON CONFLICT (id) DO NOTHING;

COMMIT;

-- Verificacion rapida:
--   SELECT count(*) FROM users WHERE email LIKE 'loadtest%@mooc.test';  -- deberia dar 302 (300 + teacher + showcase)
--   SELECT id, slug, current_published_version_id FROM courses WHERE id = '10000000-0000-0000-0000-000000000001';
--   SELECT count(*) FROM quiz_questions WHERE quiz_id = '10000000-0000-0000-0000-000000000007';  -- deberia dar 3
