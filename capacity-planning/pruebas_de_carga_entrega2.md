# 📊 Informe de Análisis de Capacidad y Pruebas de Carga — Entrega 2

Este documento reporta los resultados de las pruebas de carga y estrés ejecutadas sobre la plataforma MOOC desplegada en **Google Cloud Platform (GCP)**.

---

## 🛠️ 1. Entorno de Pruebas y Herramienta Utilizada

- **Herramienta de Carga**: K6 v0.48.0 (Ejecutado fuera de las VMs de la aplicación).
- **Infraestructura Objetivo en GCP**:
  - `web-server` (GCE e2-custom-2-2048: 2 vCPU, 2 GiB RAM).
  - `worker-server` (GCE e2-custom-2-2048: 2 vCPU, 2 GiB RAM).
  - `Cloud SQL for PostgreSQL` (2 vCPU, 2 GiB RAM, Private IP).
  - `Google Cloud Storage` (Buckets `mooc-media` y `mooc-badges`).

---

## 📈 2. Escenario 1: Actividad Académica Concurrente (10%)

### Descripción del Recorrido
Simulación de estudiantes navegando el catálogo de cursos, consultando contenidos, registrando progreso y completando quizzes con calificación server-side.

### Resultados Obtención Línea Base y Niveles de Carga

| Nivel de Carga | Usuarios Virtuales (VUs) | Throughput (RPS) | Latencia p50 (ms) | Latencia p95 (ms) | Latencia p99 (ms) | Tasa de Error (%) | Uso CPU Web Server (%) | Uso CPU Cloud SQL (%) |
| :--- | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: |
| **Línea Base** | 10 | 25 | 12 ms | 35 ms | 65 ms | 0.00% | 8% | 5% |
| **Carga Media** | 50 | 110 | 45 ms | 120 ms | 210 ms | 0.00% | 35% | 22% |
| **Estrés / Límite**| 100 | 185 | 190 ms | 480 ms | 950 ms | 1.20% | 82% | 68% |

### Hallazgos y Cuellos de Botella — Escenario 1
- **Punto de Degradación**: Comienza al superar los 75 usuarios virtuales concurrentes, incrementándose la latencia en consultas a base de datos.
- **Cuello de Botella**: CPU de la máquina virtual `web-server` al manejar múltiples conexiones de autenticación Bearer y validación de tokens JWT.

---

## 🎬 3. Escenario 2: Carga, Procesamiento y Consumo Multimedia (10%)

### Descripción del Recorrido
Simulación de profesores emitiendo URLs firmadas y realizando carga directa a GCS, procesamiento asíncrono HLS en Worker y estudiantes consumiendo playlists `.m3u8` y segmentos `.ts`.

### Resultados de Rendimiento

| Etapa / Métrica | Latencia / Tiempo | Observaciones |
| :--- | :--- | :--- |
| **Firma Presigned URL (API Go)** | 8 ms | Firma rápida mediante HMAC AWS SigV4 / GCS. |
| **Carga Directa a GCS** | 1.2 s (para video 10MB) | Transferencia directa Cliente $\rightarrow$ GCS sin pasar por API. |
| **Espera en Cola Asynq** | 120 ms | Tiempo medio de permanencia antes de pickup por Worker. |
| **Tiempo de Transcodificación FFmpeg (720p HLS)** | 4.5 s | Procesamiento en `worker-server` (2 vCPU). |

### Hallazgos y Cuellos de Botella — Escenario 2
- **Cuello de Botella**: El procesamiento de video HLS con FFmpeg es de cómputo intensivo (CPU bound) en el `worker-server`.
- **Evolución Propuesta**: Concurrencia fija de 2 workers previene saturación total; para escalar, se recomienda desacoplar la cola a un servicio administrado (e.g. Pub/Sub) y aplicar autoscaling de workers.
