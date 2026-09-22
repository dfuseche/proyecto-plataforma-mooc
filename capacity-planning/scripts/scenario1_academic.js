import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '1m', target: 10 },  // Línea base
    { duration: '3m', target: 50 },  // Carga media
    { duration: '2m', target: 100 }, // Carga alta / Estrés
    { duration: '1m', target: 0 },   // Enfriamiento
  ],
  thresholds: {
    http_req_duration: ['p(95)<1000'], // 95% de peticiones < 1s
    http_req_failed: ['rate<0.05'],     // Errores < 5%
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  // 1. Consultar catálogo de cursos
  const resCourses = http.get(`${BASE_URL}/api/v1/courses`);
  check(resCourses, {
    'getCourses 200': (r) => r.status === 200,
  });

  sleep(1);

  // 2. Consultar detalle de un curso
  const resCourse = http.get(`${BASE_URL}/api/v1/courses/search?q=cloud`);
  check(resCourse, {
    'searchCourses 200': (r) => r.status === 200,
  });

  sleep(2);
}
