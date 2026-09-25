# Pruebas de capacidad — Entrega 2 (ISIS4426)

> Estado: **Escenario 1 completo** (commit `870f259`, corrida `20260925_131343_*`) —
> faltan métricas de infraestructura (VM/PostgreSQL) y p99, ver limitaciones.
> **Escenario 2 pendiente.** Ninguna cifra de este archivo debe darse por
> buena hasta que la sección correspondiente tenga resultados pegados y
> enlazados (no estimados ni inventados).

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

- **Autenticación:** las sesiones se preparan **antes** de la corrida — `setup()` loguea el pool de usuarios una sola vez (evita saturar el rate limit de 100 req/min por IP en `/auth/login`) y todas las VUs reutilizan esos tokens. **La autenticación NO forma parte del recorrido medido en los niveles de carga**; no se corrió una variante separada de ráfaga de logins en esta entrega (queda como trabajo futuro, ver limitaciones).
- **Cuentas e intentos distintos:** cada VU toma un token al azar de un pool de hasta 300, evitando que todas las VUs compartan una sola cuenta/intento.
- **Comprobación de envío duplicado sin doble calificación:** cada iteración que completa un quiz reenvía el mismo `attempt_id` inmediatamente después. Se espera `409` (`ErrAttemptAlreadySubmitted`) y NO un `200` con score recalculado. Métrica `quiz_double_grading_detected` (threshold `count==0`).
- **Distribución de operaciones constante entre niveles:** los porcentajes de arriba no cambian entre corridas — solo cambia `MAX_VUS`.

### Bug encontrado y corregido durante esta entrega

Las primeras corridas completas (24 y 25 de septiembre, commits previos al fix) tenían `quiz_start`/`quiz_submit` en **0 muestras** de forma silenciosa: la API envuelve todas sus respuestas en `{"success": true, "data": {...}}`, pero `load-test.js` leía `snapshot.questions` y `attempt.status`/`attempt.id` directamente en vez de `snapshot.data.questions` y `attempt.data.*`. Como el status HTTP era 200/201, no había ningún error visible — el flujo de quiz simplemente nunca se ejercitaba, y la comprobación de envío duplicado (exigida por el enunciado) nunca corría de verdad.

Se corrigió en el commit `870f259` (ver `loadtests/k6/load-test.js`), junto con `loadtests/seed/reset_quiz_attempts.sql` (los tokens de menor número agotan sus 3 intentos entre corridas si no se resetean — `max_attempts=3` en el quiz sembrado). **Los resultados de esta sección corresponden a la corrida posterior al fix**, con los intentos reseteados justo antes de arrancar.

### Niveles de carga

Ejecutados con `loadtests/k6/run_escenario1_niveles.ps1` (perfil corto `LEVEL_RUN=true`: rampa de 1 min + 3 min sostenido + 30s de enfriamiento por nivel).

| Nivel | MAX_VUS | Justificación |
|---|---|---|
| Línea base | 10 | Carga mínima, referencia de latencia sin contención |
| Nivel 1 | 50 | Actividad baja/moderada |
| Nivel 2 | 150 | Actividad moderada/alta |
| Nivel 3 | 400 | Carga alta — se observó degradación clara (ver resultados) |
| Repetición | 400 | Confirmar estabilidad del punto de degradación |

### Condiciones fijas durante todas las corridas

- Commit/release evaluado: `870f259` (rama `main`)
- Configuración de la VM (Web Server): `TODO — completar con la especificación real del proveedor (objetivo: 2 vCPU / 2 GiB o la más cercana disponible, justificar si difiere)`
- Base de datos: Cloud SQL PostgreSQL (`mooc-db-instance`) — `TODO tier/vCPU/RAM`
- Versión de k6: `TODO — correr "k6 version" y pegar aquí`
- Generador de carga: laptop del equipo (Intel Core i9-13905H, 14 núcleos/20 hilos, 32 GB RAM) — fuera de las dos VMs de la aplicación, como exige el enunciado. Con esta carga (máx. ~35 req/s, cientos de VUs I/O-bound) el generador no fue el cuello de botella; no se instrumentó CPU/red del generador para esta entrega (ver limitaciones).
- Corrida usada: `loadtests/results/escenario1/20260925_131343_*` (línea base 13:13, repetición finalizó 13:54 — ver `.log`/`.json` de cada nivel)

### Resultados por nivel

_(p50/p90/p95 de `http_req_duration` global, en ms; **p99 no quedó capturado en esta corrida** — el `summaryTrendStats` por defecto de k6 no incluye p99, ya corregido en el script para corridas futuras, ver limitaciones)_

| Nivel | MAX_VUS | p50 (ms) | p90 (ms) | p95 (ms) | Throughput (req/s) | Tasa de error | Iteraciones completas | `quiz_max_attempts_reached` |
|---|---|---|---|---|---|---|---|---|
| Línea base | 10 | 119.8 | 126.8 | 129.4 | 7.09 | 0.00% | 1009 | 16 |
| Nivel 1 | 50 | 116.3 | 126.2 | 137.4 | 28.44 | 0.00% | 5021 | 169 |
| Nivel 2 | 150 | 118.1 | 214.1 | 255.4 | 56.03 | 0.00% | 14818 | 577 |
| Nivel 3 | 400 | 157.7 | 56984.0 | 59894.8 | 21.22 | 5.19% | 5826 | 356 |
| Repetición | 400 | 149.4 | 59169.0 | 60000.2 | 14.77 | 8.71% | 3961 | 240 |

Por endpoint (avg / p95, ms) — línea base → nivel 2 (rango sano) vs. nivel 3 (saturado):

| Endpoint | Línea base (avg/p95) | Nivel 1 (avg/p95) | Nivel 2 (avg/p95) | Nivel 3 (avg/p95) | Repetición (avg/p95) |
|---|---|---|---|---|---|
| `catalog` | 119.5 / 127.2 | 117.3 / 134.3 | 138.7 / 246.1 | 6586.3 / 59659.1 | 10345.3 / 60000.3 |
| `enroll` | 119.3 / 125.4 | 117.0 / 123.8 | 132.4 / 219.8 | 7340.9 / 60000.1 | 11451.6 / 60000.2 |
| `heartbeat` | 125.9 / 131.5 | 127.4 / 148.4 | 170.9 / 303.8 | 5838.5 / 59897.7 | 10078.9 / 60000.3 |
| `badge` | 116.6 / 122.7 | 113.5 / 120.7 | 123.2 / 195.6 | 8552.8 / 58939.1 | 11776.4 / 58944.4 |
| `quiz_start` | 123.6 / 130.3 | 122.2 / 134.4 | 156.0 / 275.2 | 4511.2 / 59896.0 | 8708.4 / 59897.3 |
| `quiz_submit` | 123.3 / 127.1 | 124.8 / 133.2 | 157.7 / 290.5 | **199.9 / 306.5** | **178.3 / 265.8** |

Nota sobre `quiz_submit` en Nivel 3/Repetición: su latencia se mantiene baja (no es el cuello de botella) porque muy pocas iteraciones llegan a completarlo — la mayoría de las VUs ya quedan bloqueadas esperando `catalog`/`enroll`/`heartbeat`/`quiz_start` (todas saturadas al límite de 60s, el timeout HTTP por defecto de k6) antes de alcanzar el paso de submit. El error 5.19%/8.71% es prácticamente en su totalidad timeout de esas cuatro operaciones.

### Respuestas exigidas por el enunciado

**¿Qué volumen de actividad sostiene la plataforma dentro de los umbrales definidos y en qué nivel comienza la degradación?**

Hasta 150 VUs concurrentes (Nivel 2) la plataforma sostiene toda la mezcla de operaciones con 0% de errores y p95 por debajo de 300 ms en todos los endpoints (throughput ~56 req/s). Entre 150 y 400 VUs ocurre el colapso: en Nivel 3 el p95 global salta a ~59.9s (contra el umbral de referencia de <500-800ms por endpoint) y la tasa de error sube a 5.19%, con la Repetición confirmando el mismo punto de quiebre (8.71% de error, throughput cayendo de 56 a ~15-21 req/s). El límite sostenible de esta configuración está entre Nivel 2 (150) y Nivel 3 (400); no se acotó más fino dentro del alcance de esta entrega (ver limitaciones).

**¿Qué operaciones concentran la latencia o los errores y cómo se relacionan con la API, Redis o su cola de mensajería, el pool de conexiones y PostgreSQL?**

Los cuatro endpoints que golpean la base de datos en cada request (`catalog`, `enroll`, `heartbeat`, `quiz_start` — todos con lectura/escritura a PostgreSQL) se degradan juntos y de forma pareja en Nivel 3/Repetición (p95 ~58.9-60.0s en los cuatro), lo que apunta a un cuello de botella compartido aguas abajo — el candidato más probable es el pool de conexiones de la API hacia PostgreSQL (o el propio PostgreSQL con la VM de 2 vCPU) saturándose, no un endpoint individual con una consulta particularmente cara. No se capturaron métricas de infraestructura (CPU/memoria de la VM, conexiones activas de PostgreSQL) durante esta corrida para confirmar cuál de los dos es el límite exacto — es la limitación más importante de este resultado (ver abajo). Este escenario no usa Redis ni cola de mensajería (esa es la ruta del Escenario 2).

**¿Se conservan la integridad de intentos, la calificación y el progreso bajo concurrencia? (incluye la comprobación de envío duplicado sin doble calificación)**

Sí. `quiz_double_grading_detected` quedó en **0** en los 5 niveles, con tráfico real detrás (a diferencia de las corridas previas al fix): en cada nivel hubo intentos de quiz completados (`quiz_submit` con muestras reales) y cada reenvío del mismo `attempt_id` fue rechazado con 409 sin recalificar, incluso en Nivel 3/Repetición bajo saturación. `quiz_max_attempts_reached` (16 → 169 → 577 → 356 → 240) muestra el límite de `max_attempts=3` funcionando como se espera conforme se agotan los tokens del pool.

**¿Qué cambio permitiría aumentar la capacidad y qué medición respalda esa propuesta?**

Ver "Propuesta de evolución" al final del documento — se completa junto con el Escenario 2 para dar una propuesta conjunta.

### Limitaciones del experimento

- **No se instrumentó infraestructura durante esta corrida** (`loadtests/monitoring/monitor_vm.sh` no se corrió en paralelo vía SSH a Web Server). La tabla de resultados no tiene columnas de CPU/memoria/red/disco de la VM ni conexiones activas de PostgreSQL, que el enunciado pide explícitamente. La conclusión sobre el pool de conexiones/PostgreSQL como cuello de botella es una hipótesis razonable a partir del patrón de latencias, no una medición directa.
- **p99 no capturado en esta corrida.** El `summaryTrendStats` por defecto de k6 solo exporta avg/min/med/p90/p95/max; se agregó `p(99)` explícitamente al script para corridas futuras, pero esta corrida ya no se repitió solo por esa métrica.
- **No se acotó el punto exacto de degradación entre 150 y 400 VUs** — el salto entre Nivel 2 y Nivel 3 es grande; un nivel intermedio (p. ej. 250) ayudaría a ubicar el límite con más precisión.
- **No se ejecutó una variante separada de ráfaga de login**, tal como permite el enunciado (autenticación fuera del recorrido medido en todos los niveles).
- El máximo probado (400 VUs, ya claramente degradado) no equivale a la capacidad máxima teórica de la plataforma — es el punto donde se decidió detener el escalado para esta entrega.

---

## Escenario 2 — Carga, procesamiento y consumo multimedia

Script listo: `loadtests/k6/media-load-test.js` (separado de `load-test.js` porque el patrón de tráfico —subida directa a almacenamiento + consumo HLS— es muy distinto). Corre dos escenarios en paralelo: pocos VUs subiendo video real y esperando la transcodificación HLS, y muchos VUs reproduciendo el manifiesto firmado + sus segmentos. Ver `loadtests/README.md` sección 6 para variables y ejemplos de invocación.

_Pendiente: correr el run completo (niveles crecientes de `PLAYBACK_VUS`, análogo a `run_escenario1_niveles.ps1`) y documentar los resultados acá._

## Propuesta de evolución

`TODO — a completar con base en los hallazgos de ambos escenarios`
