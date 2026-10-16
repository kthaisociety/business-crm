# 0005. Appoint the first admin once; admins hand over in the app

Status: Accepted

## Context
`ADMIN_EMAILS` (ADR 0003, ADR 0004) made the people it named admins on every sign-in. That made it config
the deployment carries forever: changing who runs the team meant editing env and redeploying, and
someone it named couldn't be demoted for good. The team changes every year, when a new head of business
takes over. The website already handles its board roles this way: seeded once, then transferred by
their holder.

## Decision
- No `ADMIN_EMAILS`. A new environment gets its first admin from a command, run once from the same image:
  `business-crm appoint-admin [-name NAME] EMAIL`. It adds that person to the team as an admin.
- While the team has an admin the command refuses: admins hand over in the app. `-force` appoints one
  anyway, for recovery when every admin has left without handing over.
- On the Team page an admin hands over: they pick a teammate, who becomes an admin, and they step down to
  board or member, in one step. Admins can still change anyone's role there too.
- Locally, `go run ./cmd/seed` makes the dev user an admin.

## Consequences
- Changing who runs the team is a click, not a secret rewrite or a redeploy.
- Recovery needs someone with access to run the command against the database, the same people who deploy.
- Sign-in no longer promotes anyone: only the command and other admins make admins.
