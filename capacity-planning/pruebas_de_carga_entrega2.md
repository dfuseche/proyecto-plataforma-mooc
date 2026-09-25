# Pruebas de capacidad — Entrega 2 (ISIS4426)

> Estado: **Escenario 1 en curso.** Este documento se llena con datos reales
> a medida que se ejecutan las corridas; ninguna cifra de este archivo debe
> darse por buena hasta que la sección correspondiente tenga resultados
> pegados y enlazados (no estimados ni inventados).

## Herramienta de generación de carga

- **k6** (registrar versión exacta con `k6 version` al momento de correr las pruebas).
- Justificación: soporta HTTP con scripting en JS, exporta resúmenes reproducibles en JSON (`--summary-export`), permite checks funcionales (no solo códigos HTTP) y tags por endpoint para separar métricas — necesario para diferenciar catálogo/inscripción/heartbeat/quiz/insignia como pide el enunciado.
- Ubicación del generador: **la laptop del equipo** (fuera de Web Server y Worker Server). Registrar aquí sus recursos (CPU/RAM) y confirmar que no fue el cuello de botella (CPU del generador <70% durante las corridas más altas — verificar con Administrador de tareas o `Get-Counter` en paralelo).

## Datos sintéticos

- 300 estudiantes de carga ya verificados (`loadtest0001@mooc.test` … `loadtest0300@mooc.test`), un profesor y un estudiante "vitrina".
- 1 curso publicado con jerarquía mínima: 1 módulo, 1 unidad, 1 recurso de texto, 1 recurso de quiz (3 preguntas, 2 opciones c/u, `max_attempts=3`, `passing_score=70`).
- 1 insignia ya emitida para el estudiante vitrina.
- Script de carga: `loadtests/seed/seed_load_test_data.sql` (idempotente, ver `loadtests/README.md`).

## Escenario 1 — Actividad académica concurrente

### Definición del escenario

**Recorrido simulado por VU** (`loadtests/k6/load-test.js`):

| Rama | % de iteraciones | Operaciones |
|---|---|---|
| Catálogo público | 60% | `GET /api/v1/courses`, `GET /api/v1/courses/{id}` |
| Aprendizaje autenticado | 25% | `POST /enrollments` → `POST /heartbeat` → (30% de estas) `POST /quizzes/{id}/start-attempt` → `POST /attempts/{id}/submit` → **reenvío duplicado del mismo intento** (debe rechazarse) |
| Verificación de insignia | 15% | `GET /api/v1/badges/verify/{code}` |

- **Autenticación:** las sesiones se preparan **antes** de la corrida — `setup()` loguea el pool de usuarios una sola vez (evita saturar el rate limit de 100 req/min por IP en `/auth/login`) y todas las VUs reutilizan esos tokens. **La autenticación NO forma parte del recorrido medido en los niveles de carga**; si se quiere medir una ráfaga de logins, debe correrse y reportarse aparte (no incluido todavía — pendiente si el equipo decide agregarlo).
- **Cuentas e intentos distintos:** cada VU toma un token al azar de un pool de 300, evitando que todas las VUs compartan una sola cuenta/intento.
- **Comprobación de envío duplicado sin doble calificación:** cada iteración que completa un quiz reenvía el mismo `attempt_id` inmediatamente después. Se espera `409` (`ErrAttemptAlreadySubmitted`) y NO un `200` con score recalculado. Métrica `quiz_double_grading_detected` (debe quedar en 0; threshold `count==0`).
- **Distribución de operaciones constante entre niveles:** los porcentajes de arriba no cambian entre corridas — solo cambia `MAX_VUS`.

### Niveles de carga

Ejecutados con `loadtests/k6/run_escenario1_niveles.ps1` (perfil corto `LEVEL_RUN=true`: rampa de 1 min + 3 min sostenido + 30s de enfriamiento por nivel, en vez de los 23 min del perfil completo de la sección 1 del enunciado).

| Nivel | MAX_VUS | Justificación |
|---|---|---|
| Línea base | 10 | Carga mínima, referencia de latencia sin contención |
| Nivel 1 | 50 | Actividad baja/moderada |
| Nivel 2 | 150 | Actividad moderada/alta |
| Nivel 3 | 400 | Carga alta — punto donde se espera empezar a ver degradación en una VM de 2 vCPU / 2 GiB |
| Repetición | (= Nivel 3 o el nivel donde se observó degradación) | Confirmar estabilidad del punto de degradación |

> Ajustar `$Niveles` en el script según lo observado: si el Nivel 3 ya rompe thresholds, no hace falta subir más — repetirlo alcanza para documentar estabilidad. Si el presupuesto no permite llegar a saturación real, aclarar explícitamente que el máximo reportado no es la capacidad máxima de la plataforma.

### Condiciones fijas durante todas las corridas

_(completar antes de correr: commit/tag del backend, tamaño real de la VM, versión de k6, concurrencia de workers, si hay caché involucrada)_

- Commit/release evaluado: `TODO`
- Configuración de la VM (Web Server): `TODO — 2 vCPU / 2 GiB según el enunciado, o la más cercana disponible, justificar`
- Versión de k6: `TODO (k6 version)`
- Base de datos: Cloud SQL PostgreSQL — `TODO tier/vCPU/RAM`

### Resultados por nivel

_(pegar aquí, por cada nivel: p50/p95/p99 por endpoint, throughput, tasa de error, y enlazar el `.json`/`.log` correspondiente en `loadtests/results/escenario1/`. Cruzar con las métricas de infraestructura de `loadtests/monitoring/monitor_vm.sh` corridas en paralelo — CPU/memoria/red/disco de Web Server, conexiones activas de PostgreSQL)_

| Nivel | MAX_VUS | p50 (ms) | p95 (ms) | p99 (ms) | Throughput (req/s) | Tasa de error | CPU VM | Mem VM | Conexiones PG activas | Resultado |
|---|---|---|---|---|---|---|---|---|---|---|
| Línea base | 10 | | | | | | | | | |
| Nivel 1 | 50 | | | | | | | | | |
| Nivel 2 | 150 | | | | | | | | | |
| Nivel 3 | 400 | | | | | | | | | |
| Repetición | | | | | | | | | | |

### Respuestas exigidas por el enunciado

**¿Qué volumen de actividad sostiene la plataforma dentro de los umbrales definidos y en qué nivel comienza la degradación?**
`TODO — completar con evidencia de la tabla anterior`

**¿Qué operaciones concentran la latencia o los errores y cómo se relacionan con la API, Redis o su cola de mensajería, el pool de conexiones y PostgreSQL?**
`TODO`

**¿Se conservan la integridad de intentos, la calificación y el progreso bajo concurrencia? (incluye la comprobación de envío duplicado sin doble calificación)**
`TODO — reportar el valor final de quiz_double_grading_detected y si el threshold pasó`

**¿Qué cambio permitiría aumentar la capacidad y qué medición respalda esa propuesta?**
`TODO`

### Limitaciones del experimento

`TODO — ej. si no se alcanzó saturación real por presupuesto/tiempo, aclararlo aquí explícitamente`

---

## Escenario 2 — Carga, procesamiento y consumo multimedia

Script listo: `loadtests/k6/media-load-test.js` (separado de `load-test.js` porque el patrón de tráfico —subida directa a almacenamiento + consumo HLS— es muy distinto). Corre dos escenarios en paralelo: pocos VUs subiendo video real y esperando la transcodificación HLS, y muchos VUs reproduciendo el manifiesto firmado + sus segmentos. Ver `loadtests/README.md` sección 6 para variables y ejemplos de invocación.

_Pendiente: correr el run completo (niveles crecientes de `PLAYBACK_VUS`, análogo a `run_escenario1_niveles.ps1`) y documentar los resultados acá._

## Propuesta de evolución

`TODO — a completar con base en los hallazgos de ambos escenarios`
