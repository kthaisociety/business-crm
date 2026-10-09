# 0005. Simulate a signed-in user until identity-service is live

Status: Accepted

## Convention
- Until identity-service is live, the app simulates a signed-in user: `internal/auth` signs every request
  in as the dev user from `DEV_USER_EMAIL`. Unset, every `/api` request but health gets a 401.
- Handlers get the user only from `auth.Current`, so features are built for a signed-in user from the
  start.
- Once identity-service is live, the app signs in through it for real, against a local identity-service
  or staging, and the dev user goes.
- Nothing goes to production without real sign-in. While the user is simulated, the app holds no real
  personal data, in any environment: seed and test data are made up (`@example.com`).

## Why
Most features depend on who is signed in, and identity-service isn't live yet. The simulated user
protects nothing: anyone who can reach the app is signed in, and a CRM is a list of named people.

## How to apply
Set `DEV_USER_EMAIL` in `.env`. Read the user with `auth.Current(c)` in handlers behind `auth.Middleware`.
