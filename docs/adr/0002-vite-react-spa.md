# 0002. Vite + React single-page app, not Next.js

Status: Accepted

## Context
The KTHAIS stack is Next.js and Go. Here the Go server owns the API (ADR 0001), and the app needs no SSR,
no Next route handlers and no middleware. A static Next export would drop exactly what Next adds.

## Decision
`frontend/` is a Vite + React + TypeScript single-page app with Tailwind CSS, linted with oxlint and
tested with Vitest and Testing Library.

## Consequences
React, TypeScript and Tailwind carry over from landingpage-frontend. Routing, when the app needs more than
one page, is a client-side router.
