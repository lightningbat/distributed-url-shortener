import http from "k6/http";
import { check, sleep } from "k6";
import { Trend, Rate } from "k6/metrics";

// ======================
// CONFIG
// ======================

const URL = "http://172.31.4.161:8080/api/short";

const PAYLOAD = JSON.stringify({
  "longurl": "https://github.com/torvalds/linux",
});

const PARAMS = {
  headers: {
    "Content-Type": "application/json",
  },
  timeout: "30s",
};

// ======================
// CUSTOM METRICS
// ======================

const successRate = new Rate("success_rate");
const latencyTrend = new Trend("latency_ms");

// ======================
// TEST PROFILE
// ======================

export const options = {
  discardResponseBodies: true,

  scenarios: {
    stress_test: {
      executor: "ramping-vus",

      stages: [
        { duration: "30s", target: 100 },   // warmup
        { duration: "1m", target: 500 },
        { duration: "1m", target: 1000 },
        { duration: "1m", target: 2000 },
        { duration: "1m", target: 4000 },   // push hard
        { duration: "30s", target: 0 },     // cooldown
      ],

      gracefulRampDown: "10s",
    },
  },

  thresholds: {
    http_req_failed: ["rate<0.05"], // <5% failures
    http_req_duration: ["p(95)<2000"],
    success_rate: ["rate>0.95"],
  },

  // Useful for very high throughput tests
  noConnectionReuse: false,
  userAgent: "k6-stress-test",
};

// ======================
// TEST LOGIC
// ======================

export default function () {
  const res = http.post(URL, PAYLOAD, PARAMS);

  const ok = check(res, {
    "status is 2xx": (r) => r.status >= 200 && r.status < 300,
  });

  successRate.add(ok);
  latencyTrend.add(res.timings.duration);
}
