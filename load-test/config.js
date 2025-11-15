export const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

export const options = {
    vus: 1,
    iterations: 5,
    thresholds: {
        http_req_duration: ["p(95)<300"],
        http_req_failed: ["rate<0.01"],
    },
};

export function validateResponse(res, expectedStatus = 200) {
    if (res.status !== expectedStatus) {
        throw new Error(`Unexpected status: ${res.status}`);
    }
}
