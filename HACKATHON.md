# Hackathon disclosure

This project was built for a hackathon running **27 Oct – 4 Nov**.
This file identifies what existed before the event and what was newly
developed during it.

## What tupaqnin does

A person and an AI agent write a document together, and the person ends up
able to sign it and defend it. The agent works and the human decides: nothing
blocks the agent, and a single gate, the signoff, protects the document.

## What the jury sees

Open the invitation link and press «Try with the demo case»: send Paula's
request, see what the agent understood, watch it write, answer a critical
decision's card with a reason, and sign a section by explaining it without
looking. The steps are in [`README.md`](README.md#what-the-jury-sees), along
with how to run it locally without an API key.

## Before the event (7 – 26 Oct)

Research and design only, no code:

- First drafts of the design for the three blocks — the cycle (block 1),
  the principal's memory (block 2) and platform and files (block 3).
  Each design file carries its version and date in its header.

## During the event (from 27 Oct)

- The design closed during the event: block 1 validated, block 2 reworked,
  block 3 aligned with blocks 1 and 2. The design lives in `docs/diseno/`
  (blocks 1–3 and the glossary); the build backlogs are in
  `docs/construccion/`.
- **All code**, from 28 Oct: the Go server, the React page, the tests, the CI
  and the deploy files. From the first commit on, every commit is event work.
