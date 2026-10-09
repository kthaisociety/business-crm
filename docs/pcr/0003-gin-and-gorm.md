# 0003. Gin and GORM, like every KTHAIS Go service

Status: Accepted

## Convention
The Go side serves HTTP with [Gin](https://gin-gonic.com) and accesses Postgres with
[GORM](https://gorm.io).

## Why
landingpage-backend, onboarding-service, luma-service, growth-portal, identity-service and business-crm use the same
two. Members change every year, and one stack makes it easier for the same people to work on several
services.

## How to apply
Build on Gin and GORM. Other libraries sit on top of them; replacing either needs a new PCR.
