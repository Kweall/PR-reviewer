import prCreate from './pr_create.js';
import prReassign from './pr_reassign.js';
import teamAdd from './team_add.js';
import teamGet from './team_get.js';
import userGetReview from './user_get_review.js';

export const options = {
    scenarios: {
        pr_create: {
            executor: 'shared-iterations',
            vus: 1,
            iterations: 20,
            maxDuration: '1m',
            exec: 'pr_create',
        },
        pr_reassign: {
            executor: 'shared-iterations',
            vus: 1,
            iterations: 20,
            maxDuration: '1m',
            exec: 'pr_reassign',
        },
        team_add: {
            executor: 'shared-iterations',
            vus: 1,
            iterations: 10,
            maxDuration: '1m',
            exec: 'team_add',
        },
        team_get: {
            executor: 'shared-iterations',
            vus: 1,
            iterations: 20,
            maxDuration: '1m',
            exec: 'team_get',
        },
        user_get_review: {
            executor: 'shared-iterations',
            vus: 1,
            iterations: 20,
            maxDuration: '1m',
            exec: 'user_get_review',
        }
    },
};


export function pr_create() {
    prCreate();
}

export function pr_reassign() {
    prReassign();
}

export function team_add() {
    teamAdd();
}

export function team_get() {
    teamGet();
}

export function user_get_review() {
    userGetReview();
}
