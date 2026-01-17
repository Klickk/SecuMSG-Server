# GDPR delete summary

This project exposes user-owned data across distributed services (auth, keys, messages).
The Profile page documents which fields live in which service, and the client can
trigger GDPR-style deletions per service or as a coordinated wipe (auth deleted last).
After deletion completes, the client clears encrypted IndexedDB stores, removes local
session tokens, and redirects back to onboarding.
