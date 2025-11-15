import http from "k6/http";
import { check } from "k6";
import { BASE_URL, options } from "../config.js";

export { options };

export default function () {
    const payload = JSON.stringify({
        team_name: `team-${Math.floor(Math.random() * 10000)}`,
        members: [
            { user_id: "u1", username: "User1", is_active: true },
            { user_id: "u2", username: "User2", is_active: true },
        ],
    });

    const res = http.post(`${BASE_URL}/team/add`, payload, {
        headers: { "Content-Type": "application/json" },
    });

    check(res, {
        "team created or already exists": (r) =>
            r.status === 201 || r.status === 400,
    });
}
