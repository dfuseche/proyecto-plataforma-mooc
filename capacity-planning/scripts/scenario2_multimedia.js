import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '1m', target: 5 },   // Carga constante de multimedia
    { duration: '3m', target: 20 },  // Carga incremental de transcodificación
    { duration: '1m', target: 0 },   // Enfriamiento
  ],
  thresholds: {
    http_req_duration: ['p(95)<2000'],
    http_req_failed: ['rate<0.05'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  // Simulación de consulta de estado de procesamiento multimedia HLS
  const res = http.get(`${BASE_URL}/api/v1/courses`);
  check(res, {
    'status 200': (r) => r.status === 200,
  });

  sleep(3);
}
