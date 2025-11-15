import http from "k6/http";
import { check } from "k6";
import { BASE_URL, options } from "../config.js";

export { options };

export default function () {
    const res = http.get(`${BASE_URL}/users/getReview?user_id=u2`);

    check(res, {
        "status 200": (r) => r.status === 200,
    });
}
