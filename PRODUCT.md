# Product

## Register

product

## Users

Small teams of developers sharing a repository. They land in api-man while building or debugging APIs locally: tweaking a request body, switching environments, replaying a known call against `dev` or `staging`. Mixed experience levels. Some teammates know the codebase deeply; others arrive at a new repo and need to understand the API surface from the request collection on disk. Primary context is a local dev session next to a terminal and an editor, not a hosted SaaS workflow.

## Product Purpose

A filesystem-honest alternative to Postman. Requests, environments, and body templates live as plain JSON files inside the repo, so they can be committed, reviewed, branched, and shared like any other code. The web UI is a viewer and runner over that filesystem, not a separate database. Success looks like: a teammate clones the repo, opens the UI, and can immediately see, edit, and replay every API call the team cares about, with no import/export ceremony.

## Brand Personality

Quiet. Utilitarian. Craftsmanlike. The voice of a well-built CLI that happens to have a screen attached. Confident defaults, no marketing voice, no exclamation points, no celebratory toasts. Tone matches the medium: spare, precise, slightly opinionated. The interface should feel like something a senior engineer made for their own daily use and decided to share, not a product built to be sold.

## Anti-references

- **Postman itself**: orange accent, busy panel-stuffed enterprise chrome, modal-heavy flows, account/workspace gating. Even though Postman is the obvious comparison, the look is what api-man rejects.
- **Crypto / AI dark-glow neon**: purple-magenta gradients, glowing borders, glassmorphism, "intelligent" copy. The product is not a chatbot wrapper.
- **Bootstrap / default Tailwind UI kit**: flat blue primary buttons, evenly rounded cards, identical card grids, generic SaaS-cream. Off-the-shelf feel betrays the craftsmanlike promise.

## Design Principles

1. **Filesystem-honest.** The UI reflects the on-disk reality. Paths are visible. A request is a file at a path, not a row in an abstract list. Body templates are named files, not hidden state. Nothing in the UI hides where it lives on disk.
2. **Repo-grade legibility.** Because the audience includes new teammates landing in a shared repo, hierarchy and labels beat density tricks. A first-time viewer should be able to read the collection, identify environments, and recognize the active state without a tour.
3. **Quiet by default, sharp on demand.** Neutral surface, restrained color. Status, method, and verbs carry color only when they encode meaning (HTTP method, response status, environment). Decoration without meaning is removed.
4. **Keyboard before mouse for the inner loop.** Send, switch environment, jump to request, switch body template: all reachable by keyboard for repeat users, without sacrificing pointer usability for first-timers.
5. **No ceremony.** No splash, no onboarding modal, no settings sprawl. The tool opens to the work.

## Accessibility & Inclusion

Best-effort, no formal compliance target. Practical floor: keyboard navigable for primary actions (request list, send, environment switch, body tabs), readable contrast on text and method badges, no information conveyed by color alone (method badges pair color with the verb text, status pairs color with the numeric code), respects `prefers-reduced-motion` for any transitions added.
