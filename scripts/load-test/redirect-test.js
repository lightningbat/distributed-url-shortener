import http from "k6/http";
import { check } from "k6";

// ======================================
// CONFIG
// ======================================

const BASE_URL = "http://172.31.4.161:8080";

// total existing IDs
const TOTAL_IDS = 300_000;

// 20% hot set
const HOT_SET_SIZE = Math.floor(TOTAL_IDS * 0.2);

// traffic distribution
const HOT_TRAFFIC_RATIO = 0.8;

// base62 charset
const BASE62 =
  "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ";

// ======================================
// BASE62 ENCODER
// ======================================

function toBase62(num) {
  if (num === 0) return "0";

  let result = "";

  while (num > 0) {
    result = BASE62[num % 62] + result;
    num = Math.floor(num / 62);
  }

  return result;
}

// ======================================
// HOT/COLD ID GENERATOR
// ======================================

function getWeightedId() {
  const isHot = Math.random() < HOT_TRAFFIC_RATIO;

  let idNum;

  if (isHot) {
    // hot range: first 20%
    idNum = Math.floor(Math.random() * HOT_SET_SIZE) + 1;
  } else {
    // cold range: remaining 80%
    idNum =
      Math.floor(
        Math.random() * (TOTAL_IDS - HOT_SET_SIZE)
      ) + HOT_SET_SIZE + 1;
  }

  return toBase62(idNum);
}

// ======================================
// OPTIONS
// ======================================

export const options = {
  discardResponseBodies: true,

  scenarios: {
    redirect_load: {
      executor: "constant-arrival-rate",

      rate: 15000,
      timeUnit: "1s",
      duration: "2m",

      preAllocatedVUs: 1000,
      maxVUs: 2000,
    },
  },

  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<50"],
  },
};

// ======================================
// TEST
// ======================================

export default function () {
  const id = getWeightedId();

  const res = http.get(`${BASE_URL}/${id}`, {
    redirects: 0,
  });

  check(res, {
    "redirect works": (r) =>
      r.status === 301 || r.status === 302,
  });
}
