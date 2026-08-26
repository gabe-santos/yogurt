## Agent skills

### Issue tracker

Issues live in GitHub Issues (via `gh` CLI). See `docs/agents/issue-tracker.md`.

### Domain docs

Single-context layout — one `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.

### UI components (`web/`)

When building or modifying a Svelte component in `web/`, check shadcn-svelte first, in this order: 1) reuse an existing primitive already installed in `web/src/lib/components/ui/`; 2) if none fits, check the shadcn-svelte registry and install a matching primitive via `pnpm dlx shadcn-svelte@latest add <component>`; 3) if no shadcn-svelte primitive matches, compose the component from existing installed primitives; 4) only build from scratch if none of the above apply.

Before editing any file, read it first. Before modifying a function, grep for all callers. Research before you edit.

When reporting information to me, be extremely concise and sacrifice grammar for the sake of concision.
