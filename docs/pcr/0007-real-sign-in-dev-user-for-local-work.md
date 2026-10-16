# 0007. Real sign-in; the dev user only for local work

Status: Accepted

## Convention
- Deployed environments sign in with OIDC (ADR 0004) and never set `DEV_USER_EMAIL`. The server refuses
  to start with both set.
- The dev user is for local work only: without an OIDC client in `.env`, every request is signed in as
  `DEV_USER_EMAIL`, as before.
- Handlers still get the user only from `auth.Current`.
- Real personal data is allowed only where sign-in is real. Seed and test data stay made up
  (`@example.com`), and a local database with the dev user holds made-up data only.

## Why
PCR 0005 kept real data out until sign-in was real. With OIDC sign-in, a deployed app protects its data,
so it can hold the team's real deals. Local work still shouldn't need a client or hold real data.

## How to apply
Set `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET` and `PUBLIC_URL` to sign in for real (see `.env.example`);
leave them unset and set `DEV_USER_EMAIL` for the dev user. Supersedes PCR 0005.
