# Vocabulary

Words as this project uses them.

- **Business CRM**: this app. The business team's CRM, for KTHAIS's relations with companies.
- **Business team**: the KTHAIS team that runs company relations and partnerships.
- **User**: the person signed in.
- **Dev user**: the simulated user every request is signed in as until identity-service is live
  ([PCR 0005](docs/pcr/0005-simulated-sign-in-until-identity-service.md)).
- **Deal**: one piece of work with a company in one semester: an event, a partnership or a job post. A
  company can have many deals (one per semester, or several at once). Replaces a row in the team's deal
  log spreadsheet.
- **Status**: where a deal is: Lead → Discussing → Agreement → Signed → Finished, or Canceled. The board
  has one column per status.
- **Semester**: HT (autumn) or VT (spring) plus the year, e.g. "HT 2026". The app shows one semester at a
  time.
- **Owner** and **Support**: the business-team member who runs a deal, and the members who help.
- **Next step**: the one thing to do next on a deal, with an optional due date.
- **Touchpoint**: an entry on a deal's timeline: an email, a meeting or a note. Email sync and Fathom
  meeting notes will add touchpoints on their own.
- **Follow-up nudge**: the warning on an open deal whose next step is overdue, or that has had no
  touchpoint for 4 days.
- **Fee** and **Invoice**: what the company pays (0 for a free event), and whether that has been invoiced
  and paid.
