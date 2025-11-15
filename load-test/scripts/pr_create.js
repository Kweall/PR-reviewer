import http from "k6/http";
import { check } from "k6";
import { BASE_URL, options } from "../config.js";

export { options };

export default function () {
    const id = `pr-${Math.floor(Math.random() * 100000)}`;

    const payload = JSON.stringify({
        pull_request_id: id,
        pull_request_name: "Load Test PR",
        author_id: "u1",
    });

    const res = http.post(`${BASE_URL}/pullRequest/create`, payload, {
        headers: { "Content-Type": "application/json" },
    });

    check(res, {
        "PR created or already exists": (r) =>
            r.status === 201 || r.status === 409,
    });
}
