# Pruebas de capacidad — Entrega 2 (ISIS4426)

> Estado: **Escenario 1 y Escenario 2 completos**, con resultados
> funcionales y de infraestructura (Web Server, Worker Server y Cloud SQL)
> para ambos, sobre el commit `870f259`. Ninguna cifra de este documento es
> estimada; todas salen de las corridas enlazadas en `loadtests/results/`.
> Gaps conocidos y explícitos (no bloquean la entrega, quedan documentados
> en "Limitaciones" de cada escenario): p99 no capturado en la corrida de
> Escenario 1 (el script ya lo agrega para corridas futuras); memoria/red/
> disco no capturadas en ninguna VM (Cloud Monitoring retroactivo solo trae
> CPU y conexiones sin agente adicional), con granularidad de 1 minuto
> (más gruesa que los 5s de `monitor_vm.sh`); no se acotó el punto exacto
> de degradación de Escenario 1 entre 150 y 400 VUs; no se corrió una
> ráfaga de login aparte; Escenario 2 no alcanzó saturación real con los
> niveles probados, reutiliza el mismo archivo de video para los "3
> perfiles", y no mide reproducción con un player real ni profundidad de
> cola de asynq directamente.

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
- Configuración de la VM (Web Server y Worker Server): **`e2-highcpu-2`** (2 vCPU, 2 GiB RAM) — coincide con el objetivo de 2 vCPU/2 GiB del enunciado.
- Base de datos: Cloud SQL PostgreSQL (`mooc-db-instance`) — tier **`db-custom-2-8192`** (2 vCPU, 8 GiB RAM), 20 GiB disco, zonal (sin réplicas de lectura, como pide el enunciado).
- Versión de k6: **`k6.exe v2.3.0` (commit e088784614, go1.26.8, windows/amd64)**
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

**Infraestructura** (Cloud Monitoring, cruzado por ventana de tiempo exacta de cada nivel — `loadtests/results/escenario1/infra/`):

| Nivel | CPU Web Server (avg/max %) | CPU Cloud SQL (avg/max %) | Conexiones activas Cloud SQL (avg/max) |
|---|---|---|---|
| Línea base | 4.1 / 5.4 | 5.7 / 6.1 | 7.8 / 8 |
| Nivel 1 | 8.6 / 13.3 | 7.9 / 11.0 | 8.0 / 8 |
| Nivel 2 | 17.6 / 38.2 | 22.9 / 55.7 | 8.4 / 12 |
| Nivel 3 | 8.9 / 24.5 | 11.7 / 35.0 | 16.0 / 28 |
| Repetición | 8.2 / 15.1 | 12.2 / 24.6 | 21.7 / 28 |

**Hallazgo clave:** la CPU de Cloud SQL nunca supera 56% (Nivel 2, su pico real) y de hecho *baja* en Nivel 3 respecto a Nivel 2 — igual que la CPU de Web Server. Lo que sí crece de forma sostenida y proporcionalmente mucho más que la CPU es el número de conexiones activas (8 → 28, x3.5) entre Nivel 2 y Nivel 3/Repetición. Esto apunta a agotamiento del **pool de conexiones** (de la API hacia Cloud SQL, o el límite de conexiones de la instancia) como el mecanismo de degradación, no a falta de cómputo en la base de datos — la CPU de ambas VMs y de Cloud SQL cae en Nivel 3 porque las requests se quedan esperando una conexión libre en vez de ejecutarse.

Nota sobre `quiz_submit` en Nivel 3/Repetición: su latencia se mantiene baja (no es el cuello de botella) porque muy pocas iteraciones llegan a completarlo — la mayoría de las VUs ya quedan bloqueadas esperando `catalog`/`enroll`/`heartbeat`/`quiz_start` (todas saturadas al límite de 60s, el timeout HTTP por defecto de k6) antes de alcanzar el paso de submit. El error 5.19%/8.71% es prácticamente en su totalidad timeout de esas cuatro operaciones.

### Respuestas exigidas por el enunciado

**¿Qué volumen de actividad sostiene la plataforma dentro de los umbrales definidos y en qué nivel comienza la degradación?**

Hasta 150 VUs concurrentes (Nivel 2) la plataforma sostiene toda la mezcla de operaciones con 0% de errores y p95 por debajo de 300 ms en todos los endpoints (throughput ~56 req/s). Entre 150 y 400 VUs ocurre el colapso: en Nivel 3 el p95 global salta a ~59.9s (contra el umbral de referencia de <500-800ms por endpoint) y la tasa de error sube a 5.19%, con la Repetición confirmando el mismo punto de quiebre (8.71% de error, throughput cayendo de 56 a ~15-21 req/s). El límite sostenible de esta configuración está entre Nivel 2 (150) y Nivel 3 (400); no se acotó más fino dentro del alcance de esta entrega (ver limitaciones).

**¿Qué operaciones concentran la latencia o los errores y cómo se relacionan con la API, Redis o su cola de mensajería, el pool de conexiones y PostgreSQL?**

Los cuatro endpoints que golpean la base de datos en cada request (`catalog`, `enroll`, `heartbeat`, `quiz_start` — todos con lectura/escritura a PostgreSQL) se degradan juntos y de forma pareja en Nivel 3/Repetición (p95 ~58.9-60.0s en los cuatro), lo que apunta a un cuello de botella compartido aguas abajo. Las métricas de infraestructura (tabla arriba) confirman **cuál** de los dos: no es cómputo — la CPU de Cloud SQL nunca pasa de 56% y de hecho cae en Nivel 3 respecto a Nivel 2 (22.9%→11.7% avg), igual que la CPU de Web Server (17.6%→8.9% avg) — es el **pool de conexiones hacia PostgreSQL**, cuyas conexiones activas casi se cuadruplican (8→28) justo cuando la CPU de ambos servidores cae, el patrón clásico de requests haciendo cola por una conexión libre en vez de ejecutarse. Este escenario no usa Redis ni cola de mensajería (esa es la ruta del Escenario 2).

**¿Se conservan la integridad de intentos, la calificación y el progreso bajo concurrencia? (incluye la comprobación de envío duplicado sin doble calificación)**

Sí. `quiz_double_grading_detected` quedó en **0** en los 5 niveles, con tráfico real detrás (a diferencia de las corridas previas al fix): en cada nivel hubo intentos de quiz completados (`quiz_submit` con muestras reales) y cada reenvío del mismo `attempt_id` fue rechazado con 409 sin recalificar, incluso en Nivel 3/Repetición bajo saturación. `quiz_max_attempts_reached` (16 → 169 → 577 → 356 → 240) muestra el límite de `max_attempts=3` funcionando como se espera conforme se agotan los tokens del pool.

**¿Qué cambio permitiría aumentar la capacidad y qué medición respalda esa propuesta?**

Ver "Propuesta de evolución" al final del documento — se completa junto con el Escenario 2 para dar una propuesta conjunta.

### Limitaciones del experimento

- **CPU/memoria de infraestructura obtenidas retroactivamente de Cloud Monitoring** (no se corrió `monitor_vm.sh` en vivo durante esta corrida), cruzadas por ventana de tiempo de cada nivel — ver `loadtests/results/escenario1/infra/`. No se capturaron memoria/red/disco de la VM ni de PostgreSQL (Cloud Monitoring por defecto solo trae CPU y conexiones sin agente adicional), y la granularidad es de 1 minuto, más gruesa que los 5s de `monitor_vm.sh`.
- **p99 no capturado en esta corrida.** El `summaryTrendStats` por defecto de k6 solo exporta avg/min/med/p90/p95/max; se agregó `p(99)` explícitamente al script para corridas futuras, pero esta corrida ya no se repitió solo por esa métrica.
- **No se acotó el punto exacto de degradación entre 150 y 400 VUs** — el salto entre Nivel 2 y Nivel 3 es grande; un nivel intermedio (p. ej. 250) ayudaría a ubicar el límite con más precisión.
- **No se ejecutó una variante separada de ráfaga de login**, tal como permite el enunciado (autenticación fuera del recorrido medido en todos los niveles).
- El máximo probado (400 VUs, ya claramente degradado) no equivale a la capacidad máxima teórica de la plataforma — es el punto donde se decidió detener el escalado para esta entrega.

---

## Escenario 2 — Carga, procesamiento y consumo multimedia

### Definición del escenario

Dos escenarios de k6 corriendo en paralelo en `loadtests/k6/media-load-test.js` (ver comentarios del archivo para el detalle):

- **`subida_multimedia`** (pocos VUs): sube un video real (`loadtests/assets/sample_upload.mp4`, ~90KB, 5s) por PUT directo a almacenamiento vía URL firmada, confirma la carga (`complete-upload`) y espera a que termine la transcodificación HLS asíncrona.
- **`consumo_hls`** (muchos VUs): pide `stream-url`, descarga el manifiesto firmado y hasta 5 segmentos `.ts` por iteración, a la cadencia declarada (no descarga todo el video de una sentada).

`PLAYBACK_POOL_SIZE=3` en todos los niveles: 3 videos pre-transcodificados al arranque de cada corrida, cubriendo los 3 perfiles que pide el enunciado (mismo archivo de origen reutilizado; ver limitaciones). La concurrencia de workers no se tocó entre niveles.

### Niveles de carga

Ejecutados con `loadtests/k6/run_escenario2_niveles.ps1` (subida y consumo suben juntos, para simular más profesores publicando a la vez que más estudiantes reproduciendo).

| Nivel | UPLOAD_VUS | PLAYBACK_VUS | Duración |
|---|---|---|---|
| Línea base | 1 | 10 | 2m |
| Nivel 1 | 3 | 50 | 3m |
| Nivel 2 | 5 | 150 | 3m |
| Nivel 3 | 10 | 300 | 3m |
| Repetición | 10 | 300 | 3m |

### Condiciones fijas durante todas las corridas

- Commit/release evaluado: `870f259` (rama `main`)
- Corrida usada: `loadtests/results/escenario2/20260925_141917_*` (14:19-14:40 hora Bogotá / 19:19-19:40 UTC)
- Monitoreo de infraestructura: `loadtests/monitoring/monitor_vm.sh` corrido en paralelo por SSH en **Web Server**, cada 5s → `loadtests/results/escenario2/web-server_escenario2.csv`. **Worker Server y Cloud SQL** se cruzaron retroactivamente con Cloud Monitoring (granularidad 1 min) → `loadtests/results/escenario2/infra/`.
- RAM de Web Server confirmada por el propio CSV (`host_mem_total_mb`): **1976 MB (~2 GiB)**, consistente con la configuración objetivo del enunciado.
- Generador de carga: misma laptop del equipo, fuera de las VMs de la aplicación.
- Base de datos: Cloud SQL PostgreSQL (`mooc-db-instance`) — tier **`db-custom-2-8192`** (2 vCPU, 8 GiB RAM), 20 GiB disco, zonal (sin réplicas de lectura, como pide el enunciado).
- Versión de k6: **`k6.exe v2.3.0` (commit e088784614, go1.26.8, windows/amd64)**

### Resultados por nivel

**Tráfico HTTP y negocio** (avg/p95/p99 en ms; `PUT` es la subida directa a almacenamiento, no pasa por la API):

| Nivel | `create_resource` | `upload_put` | `complete_upload` | `manifest` | `segment` | `stream_url` | Error HTTP |
|---|---|---|---|---|---|---|---|
| Línea base | 120/140/173 | 445/547/887 | 164/214/269 | 156/180/212 | 529/689/1197 | 116/125/156 | 0.00% |
| Nivel 1 | 115/121/165 | 421/512/1008 | 144/162/184 | 153/182/255 | 490/646/774 | 117/126/257 | 0.00% |
| Nivel 2 | 112/119/123 | 403/456/530 | 137/153/162 | 144/167/268 | 457/612/743 | 114/120/289 | 0.00% |
| Nivel 3 | 113/121/153 | 385/471/520 | 135/149/157 | 145/170/353 | 457/609/740 | 116/119/355 | 0.00% |
| Repetición | 112/117/144 | 366/462/519 | 135/148/166 | 145/169/366 | 451/609/776 | 116/120/349 | 0.00% |

**Procesamiento asíncrono** (`media_processing_duration`, desde carga completa hasta `available`, en ms):

| Nivel | avg | p95 | p99 | max | Timeouts | Fallos de descarga de segmento |
|---|---|---|---|---|---|---|
| Línea base | 3393 | 3568 | 5794 | 6351 | 0 | 0 |
| Nivel 1 | 3401 | 4550 | 6346 | 6347 | 0 | 0 |
| Nivel 2 | 3656 | 6335 | 6338 | 6340 | 0 | 0 |
| Nivel 3 | 5444 | 6355 | 9459 | 9462 | 0 | 0 |
| Repetición | 4955 | 6347 | 9312 | 9451 | 0 | 0 |

**Infraestructura — Web Server** (`web-server_escenario2.csv`, cruzado por ventana de tiempo de cada nivel):

| Nivel | CPU contenedor API (avg/max %) | CPU host (avg/max %) | Memoria host (avg/max MB de 1976) | Conexiones PG activas (avg/max) |
|---|---|---|---|---|
| Línea base | 1.6 / 11.5 | 1.9 / 6.0 | 636 / 653 | 1.0 / 1 |
| Nivel 1 | 3.5 / 7.1 | 4.0 / 22.0 | 642 / 662 | 1.0 / 1 |
| Nivel 2 | 8.6 / 14.0 | 8.2 / 28.0 | 646 / 667 | 1.0 / 1 |
| Nivel 3 | 17.3 / 29.2 | 13.2 / 24.0 | 646 / 676 | 1.2 / 3 |
| Repetición | 21.2 / 117.1* | 12.4 / 22.0 | 647 / 667 | 1.0 / 1 |

\* Pico puntual de una sola muestra de 5s (probablemente una ráfaga de requests concurrentes atendidas en más de un core); el resto de la corrida se mantiene por debajo del 30%.

**Infraestructura — Worker Server y Cloud SQL** (Cloud Monitoring, retroactivo, cruzado por ventana de tiempo de cada nivel — `loadtests/results/escenario2/infra/`):

| Nivel | CPU Worker Server (avg/max %) | CPU Cloud SQL (avg/max %) | Conexiones activas Cloud SQL (avg/max) |
|---|---|---|---|
| Línea base | 11.7 / 13.9 | 5.6 / 5.6 | 8.0 / 8 |
| Nivel 1 | 26.3 / 31.9 | 5.9 / 6.2 | 8.0 / 8 |
| Nivel 2 | 40.3 / 48.3 | 7.4 / 7.9 | 9.0 / 9 |
| Nivel 3 | 71.8 / 81.7 | 9.1 / 9.8 | 9.3 / 10 |
| Repetición | 43.6 / 84.1 | 7.1 / 10.3 | 9.0 / 9 |

**Hallazgo clave:** a diferencia de Escenario 1, acá **sí hay un componente que crece claramente con la carga hasta niveles altos de uso real**: Worker Server pasa de 11.7% a 71.8-84.1% de CPU (línea base → Nivel 3/Repetición), mientras Web Server (<30%) y Cloud SQL (<10%) se mantienen holgados en todo momento. Esto confirma con medición directa —no solo por inferencia de `media_processing_duration`— que el pipeline de transcodificación en Worker Server es el primer componente en acercarse a su límite.

**Throughput y checks:** 0 fallas HTTP y 0 checks fallidos en los 5 niveles (Línea base: 1040 reqs/7.1 req/s → Repetición: 46383 reqs/228 req/s). `media_processing_timeouts=0` y `media_segment_download_failures=0` en todos los niveles — ningún job quedó atascado ni ningún segmento HLS falló al descargar.

### Análisis por punto exigido por el enunciado

**Carga directa (URLs firmadas):** `create_resource` (autorización + URL firmada) se mantiene 112-120ms avg en todos los niveles — la API nunca es el cuello de botella de la carga. `upload_put` (transferencia directa a almacenamiento, no pasa por la API) es el paso más lento del flujo de subida (366-445ms avg), consistente con ser transferencia de archivo real contra el object storage, no cómputo de la API.

**Procesamiento asíncrono:** el tiempo desde carga completa hasta `available` crece con la carga — de ~3.4s (línea base) a ~5.4s avg / 9.5s p99 (Nivel 3) — pero se mantiene muy por debajo del umbral de referencia (45s) en todos los niveles. El crecimiento no es proporcional al de `PLAYBACK_VUS`/`UPLOAD_VUS` (que se multiplica x30), lo que sugiere que la concurrencia de workers (mantenida fija a propósito) empieza a ser el limitante del pipeline de transcodificación antes que la API o la base de datos — confirmado con CPU real de Worker Server (11.7% → 71.8-84.1% entre línea base y Nivel 3/Repetición, ver tabla de infraestructura arriba).

**Trabajos completados, reintentos y cola:** 0 timeouts y 0 rechazos de encolado (`media_enqueue_rejected`, sin muestras) en los 5 niveles — todo lo que se aceptó a la API terminó en `available`. No se observó profundidad ni antigüedad de la cola de asynq directamente (no hay panel de asynq monitoreado en esta entrega); la métrica usada como proxy es `media_processing_duration`.

**Consumo HLS:** `manifest` y `segment` se mantienen estables y dentro de umbral en los 5 niveles (segment p95 608-689ms contra umbral de referencia 1000ms), incluso con `PLAYBACK_VUS` en 300. `stream_url` p99 sube de 156ms a ~350-355ms desde Nivel 1 en adelante (polling mientras el video termina de procesarse), pero su p95 se mantiene bajo 130ms — el p99 alto es exactamente el patrón esperado del pequeño porcentaje de reproducciones que arrancan justo cuando el video todavía se está transcodificando.

**Componente que limita el flujo:** con los niveles probados (hasta `UPLOAD_VUS=10`/`PLAYBACK_VUS=300`), **no se alcanzó un punto de saturación real** — 0% de error HTTP en todos los niveles. Pero ya hay un componente claramente más cargado que el resto: Worker Server llega a 71.8-84.1% de CPU en Nivel 3/Repetición (medido directamente, ver tabla de infraestructura), mientras Web Server se mantiene bajo 30% y Cloud SQL bajo 10%. Con un nivel más de carga (más VUs de subida, que es lo que fuerza más transcodificaciones concurrentes) es esperable que Worker Server sea el primero en saturar. Con una CDN delante del almacenamiento de objetos, el tráfico de `manifest`/`segment` (ya el 90%+ de las requests en los niveles altos) dejaría de pasar por la API/almacenamiento directo, liberando esa capacidad para más subida y procesamiento concurrente; más capacidad de procesamiento (más workers o más CPU en Worker Server) atacaría directamente el único componente que mostró crecimiento real con la carga.

### Limitaciones del experimento

- **No se alcanzó saturación real** en ningún nivel probado — el máximo reportado (`UPLOAD_VUS=10`, `PLAYBACK_VUS=300`) no corresponde a la capacidad máxima de la plataforma, solo al techo probado en esta entrega.
- **Los 3 perfiles de video son el mismo archivo de origen** (`sample_upload.mp4`, ~90KB/5s) reutilizado 3 veces vía `PLAYBACK_POOL_SIZE=3`, no 3 archivos con duración/tamaño/resolución realmente distintos como pide el enunciado.
- **No se midió tiempo hasta el primer cuadro ni interrupciones de reproducción con un reproductor real** — las métricas de manifiesto/segmento son peticiones HTTP, no reproducción real.
- **No se instrumentó la profundidad/antigüedad de la cola de asynq directamente**; se usó `media_processing_duration` como proxy.


## Propuesta de evolución

Los dos escenarios apuntan a componentes distintos, así que la evolución tiene dos frentes:

1. **Escenario 1 (académico): el cuello de botella está aguas abajo de la API** — `catalog`/`enroll`/`heartbeat`/`quiz_start` se degradan juntos y a la par entre Nivel 2 (150 VUs, sano) y Nivel 3 (400 VUs, p95~60s, 5-9% error), lo que apunta al pool de conexiones de la API hacia PostgreSQL o a la propia instancia de Cloud SQL (VM de 2 vCPU) saturándose bajo escritura+lectura concurrente. La medición que respalda esto: los cuatro endpoints comparten el mismo patrón de degradación pese a tener costos de negocio muy distintos (una lectura de catálogo vs. una escritura de heartbeat), lo cual descarta que sea un endpoint particular con una consulta cara. Propuesta: subir el tier de Cloud SQL (más vCPU/conexiones máximas) y/o aumentar el `max_open_conns` del pool de la API, y repetir el Nivel 3 para confirmar si el punto de quiebre se corre hacia arriba.
2. **Escenario 2 (multimedia): el cuello de botella es el pipeline de transcodificación, no la API ni el consumo HLS** — `media_processing_duration` crece de ~3.4s a ~5.4s avg (9.5s p99) entre línea base y Nivel 3, mientras la API (CPU Web Server <30% en el peor caso) y el consumo HLS (segment p95 estable en ~610-690ms) se mantienen sanos con 30x más carga. La medición que respalda esto: el único número que crece con la carga es justamente el que depende de la concurrencia fija de Worker Server. Propuesta: aumentar la concurrencia de workers/CPU de Worker Server y, para el consumo (ya el grueso del tráfico en los niveles altos), poner una CDN delante del almacenamiento de objetos — libera esa capacidad de la API/almacenamiento directo para más subida y procesamiento concurrente.

Ambas propuestas quedan pendientes de validar con una corrida de confirmación (fuera del alcance de esta entrega); ver limitaciones de cada escenario para lo que falta medir antes de tomarlas como definitivas (sobre todo memoria/disco/red, y granularidad más fina que el minuto de Cloud Monitoring).
