import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  vus: 3,
  duration: "15s",
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<500"],
  },
};

const base = __ENV.BASE_URL || "http://127.0.0.1:8080";

export default function () {
  const res = http.get(base);
  check(res, { "status is 200": (r) => r.status === 200 });
  sleep(0.05);
}
