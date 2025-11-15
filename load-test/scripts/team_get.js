import http from "k6/http";
import { validateResponse, BASE_URL } from "../config.js";

export const options = {
    vus: 5,
    duration: "30s",
};

export default function () {
    const res = http.get(`${BASE_URL}/team/get?team_name=backend`);

    validateResponse(res, 200);
}
