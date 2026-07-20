# Writing and running Playwright tests

Every user-facing feature in `frontend/src` should get e2e coverage in
`frontend/e2e/game.spec.ts` (or a new `frontend/e2e/*.spec.ts` file for an
unrelated feature area) using Playwright, following the existing patterns in
that file. This is the project's only frontend test suite — there's no unit
test layer for Vue components, so e2e is where behavior gets verified.

## Writing tests

- Reuse `createGame(page, name)` and `getJoinCode(page)` from the top of
  `game.spec.ts` instead of re-deriving the create-game or code-parsing
  steps inline.
- Any test that starts a round (i.e. calls `startGame(page)`, or otherwise
  clicks `start-button`) must use a short duration first — `startGame`
  already dials the lobby's round-length slider down to its UI minimum
  (`SHORT_DURATION_SECONDS`, currently 60s) before starting. The backend
  default is a full 5 minutes; nothing in this suite should ever wait that
  long. Only skip this when the test is specifically about the default
  duration itself (see `lobby round length`).
- Use `page.getByTestId(...)` exclusively, never CSS selectors or text
  matching against styled content — every interactive element in the app
  has a `data-testid`. Add one to any new element a test needs to target.
- A host+guest interaction (anything involving two players, e.g. live SSE
  updates) needs two isolated `browser.newContext()`s, not two `page`s in
  the same context — see `lobby round length` or `joining a game` for the
  pattern. Close both contexts at the end of the test.
- The real `navigator.clipboard` isn't readable/grantable uniformly across
  Chromium/Firefox/WebKit in headless CI. For any test that needs to read
  back what got copied, stub `writeText` via `stubClipboard(page)` (defined
  in `game.spec.ts`) instead of requesting clipboard permissions.
- Prefer Playwright's auto-retrying `expect(locator)...` assertions over a
  manual `waitForSelector` + read; the SSE-driven UI in this app updates
  asynchronously and the auto-retry is what makes that reliable.
- Don't add `{ force: true }` to a click to route around a flaky
  actionability check unless you know exactly why (the one existing use is
  documented inline: a continuously CSS-animated button whose bounding box
  never settles).

## Running tests

```sh
cd frontend
pnpm test:e2e                        # all projects
pnpm exec playwright test --project=chromium   # just Chromium, faster iteration
pnpm exec playwright test -g "invite link"     # filter by title
```

`playwright.config.ts` is fully self-contained — it builds and starts the
real Go backend plus the Vite dev server itself, so don't start either by
hand first. It also builds/serves the **dev** server by default (real
`CI=1` runs switch to a `pnpm build` + `pnpm preview`), which is why editing
frontend source and re-running tests immediately picks up the change — no
rebuild step to remember.

### Running under Claude Code

`playwright.config.ts` detects `CLAUDECODE=1` (set automatically in every
Claude Code session) and adapts automatically — no manual setup needed:

- Runs headless (Claude Code's sandboxes have no display).
- Only runs the `chromium` project — the sandboxed remote environments only
  have Chromium pre-installed, not Firefox/WebKit.
- Points Chromium's `executablePath` at the sandbox's pre-installed binary
  (under `PLAYWRIGHT_BROWSERS_PATH`) instead of the revision Playwright's
  own installer would otherwise expect — sandboxes have no egress to fetch
  one. **Do not run `playwright install`.**

So inside a Claude Code session, plain `pnpm test:e2e` (or
`npx playwright test`) just works. Outside Claude Code — a human's machine,
real CI — none of this activates and behavior is unchanged (all three
browsers, headed unless `CI=1`).

If you add coverage for a feature and the sandbox's `dist/` happens to be
stale (only matters if you force `CI=1` locally, which serves the built
`dist/` via `pnpm preview` instead of the dev server), run `pnpm build`
first — this bit us once already.
