import http from "k6/http";
import { check } from "k6";
import { BASE_URL, options } from "../config.js";

export { options };

export default function () {
    const payload = JSON.stringify({
        pull_request_id: "pr-1001",
        old_user_id: "u2",
    });

    const res = http.post(`${BASE_URL}/pullRequest/reassign`, payload, {
        headers: { "Content-Type": "application/json" },
    });

    check(res, {
        "valid status": (r) =>
            r.status === 200 || r.status === 404 || r.status === 409,
    });
}
