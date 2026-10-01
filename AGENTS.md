# AGENTS.md — Guidelines for AI Coding Assistants

This document defines core behavioral rules and coding guidelines that all AI assistants (LLMs) must strictly follow when working on this codebase.

---

## 1. Always Use Default shadcn/ui Styling First
* **Default to standard design tokens:** Always use the official shadcn/ui components, CSS variables, and design tokens (`bg-background`, `text-foreground`, `bg-card`, `border-border`, `text-muted-foreground`, etc.).
* **Minimal Overrides:** DO NOT invent custom CSS classes, arbitrary ad-hoc margins, or hacky styling. Use the components as provided by shadcn/ui and only apply Tailwind utility classes when strictly necessary for layout alignment.

---

## 2. Mandatory Component Verification Rule
* **Verify Docs & Code Before Using Components:** shadcn/ui and its underlying primitives have evolved significantly (e.g. migrating to `@base-ui/react`, native Toast primitives, updated prop names).
* **No Outdated Memory Assumptions:** You must NEVER assume component props, component names, or API signatures from training memory. **Always inspect the actual generated component source files in `web/src/components/ui/` or check official documentation** before using the components.

---

## 3. Strict Git Policy — NEVER Touch Git State
* **NEVER run mutating git commands:** DO NOT execute `git add`, `git reset`, `git commit`, `git restore`, `git checkout`, `git stash`, or any other command that modifies the git index, working tree state, or commit history.
* **Respect the user's staging baseline:** The user explicitly stages files as a review baseline to inspect changes via `git diff`. Staging files (`git add`) or unstaging files (`git reset`) obliterates their review setup.
* **Git state belongs strictly to the user:** Never touch the staging area or git index under any circumstances.
