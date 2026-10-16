# 0003. Team members and their roles live in business-crm

Status: Accepted; who joins on sign-in superseded by 0004; `ADMIN_EMAILS` superseded by 0005

## Context
Deals have an owner, and the team wants to leave a lead unassigned, claim one, or hand it to someone
else. That needs a list of who is on the business team, including people who haven't used the app yet.
Some actions, like changing who is on the team, should be limited to a few people. identity-service
(PCR 0005) will say who someone is, but not what they may do in this app.

## Decision
- `members` is the team: one row per person, matched to the signed-in user by email. Signing in adds
  you as a `member` the first time; an admin can also add people by email before they've signed in.
- Every member has one role:
  - `member`: the default. Sees everything; adds and edits deals, companies and contacts; claims,
    assigns and unassigns any deal.
  - `board`: a member who also sees Reports.
  - `admin`: also sees Reports and manages the team: adds and removes members and changes roles. There
    is always at least one admin.
- `ADMIN_EMAILS` (comma-separated, PCR 0001) makes those people admins when they sign in, so a new
  environment has an admin without a database edit. It only promotes; demoting is done in the app.
- The API enforces team changes (403); the UI hides what a role can't do.
- Reports are hidden in the UI only: they're worked out from the deals every member can already read, so
  hiding the numbers themselves would mean the API leaving fees out for members.

## Consequences
- Assigning works before someone's first sign-in, so a lead can go to anyone on the team.
- Roles are this app's, not the organisation's: SSO decides who you are, this table decides what you
  can do here. When membership comes from the org's SSO, the role stays here.
- A member who owns or supports deals can't be removed; their deals are reassigned first, so history
  keeps a name.
