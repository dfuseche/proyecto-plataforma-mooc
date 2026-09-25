-- Reinicia los intentos de quiz de los usuarios de carga contra el quiz
-- sembrado, para que run_escenario1_niveles.ps1 pueda ejercitar de verdad
-- el flujo de quiz (start-attempt / submit / reenvio duplicado) en cada
-- corrida completa.
--
-- Necesario porque max_attempts=3 (ver seed_load_test_data.sql) y los
-- tokens de menor numero (loadtest0001...) se reutilizan primero en cada
-- corrida: una vez agotan sus 3 intentos, start-attempt les responde 409
-- para siempre y el script deja de medir el flujo real (quiz_submit queda
-- en 0 muestras, como paso el 2026-09-25 antes de este fix).
--
-- Correr ANTES de cada ejecucion completa de run_escenario1_niveles.ps1:
--   psql "postgres://mooc_user:TU_PASSWORD@127.0.0.1:5432/mooc_db" \
--     -f loadtests/seed/reset_quiz_attempts.sql

DELETE FROM quiz_attempts
WHERE quiz_id = '10000000-0000-0000-0000-000000000007'
  AND student_id IN (
    SELECT id FROM users WHERE email LIKE 'loadtest%@mooc.test'
  );

SELECT count(*) AS intentos_restantes
FROM quiz_attempts
WHERE quiz_id = '10000000-0000-0000-0000-000000000007';
