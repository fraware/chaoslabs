import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  stages: [
    { duration: "10s", target: 5 },
    { duration: "20s", target: 10 },
    { duration: "10s", target: 0 },
  ],
  thresholds: {
    http_req_failed: ["rate<0.05"],
  },
};

const base = __ENV.BASE_URL || "http://127.0.0.1:8080";

export default function () {
  const res = http.get(base);
  check(res, { "status is 200": (r) => r.status === 200 });
  sleep(0.02);
}
