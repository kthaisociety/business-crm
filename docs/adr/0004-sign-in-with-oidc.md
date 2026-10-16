# 0004. Sign in with OIDC, through Google until identity-service is live

Status: Accepted

## Context
The app has simulated sign-in (PCR 0005), so it can't hold the team's real deals. identity-service will
be every KTHAIS app's OpenID Connect provider, but it isn't live yet. Google is an OIDC provider too. The
CRM is for the business team and the board, not all of KTHAIS.

## Decision
- The app is an OIDC client (authorization code flow with PKCE, from its backend): `/auth/login` sends
  you to the provider, `/auth/callback` takes the code back, and `/auth/logout` signs you out. The
  provider is config (`OIDC_ISSUER`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`), so moving to
  identity-service later changes env, not code.
- Until identity-service is live, the provider is Google, with this app's own OAuth client in its own
  Google Cloud project in the kthais.com organisation. The client is **Internal**, so Google itself only
  lets kthais.com accounts sign in, and no other app's secret is shared. This is a stopgap that departs
  from identity-service's ADR 0002 (only it holds Google credentials): once identity-service signs apps
  in, this app moves to it and the client is deleted.
- The ID token is verified (signature, issuer, audience, expiry, nonce), and the email must be verified
  and in `SIGN_IN_DOMAIN` (`kthais.com`). With Google, the token's `hd` claim must match it too.
- Only people already on the team sign in: an admin adds them on the Team page first, by email. People in
  `ADMIN_EMAILS` are let in and made admins, so a new environment has an admin. Anyone else gets "ask an
  admin to add you". This replaces ADR 0003's "signing in adds you as a member the first time".
- A session is a random token in an HttpOnly, SameSite=Lax cookie, stored hashed in Postgres for 7 days.
  Removing a member deletes their sessions.
- The dev user (`DEV_USER_EMAIL`) stays for local work without a client; the server refuses to start with
  both set.

## Consequences
- An invitation is just adding someone on the Team page; there's no shared passphrase to leak or rotate.
- Signing out is per browser; there is no single sign-out across KTHAIS apps until identity-service.
- Members are matched by email. identity-service will issue its own subjects, so storing Google's now
  would only have to be redone.
- Reading Gmail needs more than signing in: a scope each member consents to. Because the client is
  Internal, Google doesn't need to verify the app for it. Whether to do it is email sync's decision, not
  this one.
- Each environment gets its own client (local, staging, production), each with only its own callback.
