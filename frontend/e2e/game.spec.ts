import { test, expect, type Page } from '@playwright/test'

async function createGame(page: Page, name: string) {
  await page.goto('/')
  await page.getByTestId('name-input').fill(name)
  await page.getByTestId('create-game-button').click()
  await page.waitForURL(/\/games\/.+\/lobby/)
}

async function getJoinCode(page: Page): Promise<string> {
  const text = await page.getByTestId('join-code').innerText()
  return text.replace(/\s+/g, '')
}

// The shortest round length reachable through the UI (see LobbyView.vue's
// DURATION_MIN — the range input clamps below this). Any test that starts a
// round should dial it down to this first: the backend default is a full 5
// minutes, and nothing in this suite should ever wait that long.
const SHORT_DURATION_SECONDS = 15

// Must be called by the host, from the lobby, before starting.
async function startGame(page: Page) {
  await page.getByTestId('duration-input').fill(String(SHORT_DURATION_SECONDS))
  await page.getByTestId('duration-input').dispatchEvent('change')
  await expect(page.getByTestId('duration-label')).toContainText('15s')

  // A coordinate-based click races the start button's continuous "bob" CSS
  // animation (which also fails Playwright's default actionability
  // stability check) — the button can be a few pixels away from where the
  // click lands by the time it's delivered. Dispatching a synthetic click
  // event instead invokes the handler directly, sidestepping position
  // entirely.
  await page.getByTestId('start-button').dispatchEvent('click')
  await page.waitForURL(/\/games\/.+\/play/)
}

// Clipboard permission grants aren't supported uniformly across
// Chromium/Firefox/WebKit in headless CI, so tests that need to read back
// what was copied stub `writeText` instead of relying on the real OS
// clipboard. Must be called before any navigation happens on the page.
async function stubClipboard(page: Page) {
  await page.addInitScript(() => {
    if (!navigator.clipboard) {
      Object.defineProperty(navigator, 'clipboard', { value: {}, configurable: true })
    }
    ;(window as unknown as { __copiedText: string }).__copiedText = ''
    navigator.clipboard.writeText = (text: string) => {
      ;(window as unknown as { __copiedText: string }).__copiedText = text
      return Promise.resolve()
    }
  })
}

test.describe('creating a game', () => {
  test('creates a game and lands in the lobby as host', async ({ page }) => {
    await createGame(page, 'Alice')

    const code = await getJoinCode(page)
    expect(code).toMatch(/^[A-Z0-9]{6}$/)

    const rows = page.getByTestId('player-row')
    await expect(rows).toHaveCount(1)
    await expect(rows.first()).toContainText('Alice')
    await expect(rows.first()).toContainText('YOU')

    // The host sees the start button and no "waiting for host" message.
    await expect(page.getByTestId('start-button')).toBeVisible()
    await expect(page.getByTestId('waiting-message')).toHaveCount(0)
  })

  test('requires a name before creating a game', async ({ page }) => {
    await page.goto('/')
    await page.getByTestId('create-game-button').click()

    await expect(page.getByTestId('home-error')).toHaveText('Enter your name first')
    await expect(page).toHaveURL('/')
  })
})

test.describe('lobby round length', () => {
  test('defaults to 5 min, is host-editable, and updates live for every player', async ({
    browser,
  }) => {
    const hostContext = await browser.newContext()
    const guestContext = await browser.newContext()
    const hostPage = await hostContext.newPage()
    const guestPage = await guestContext.newPage()

    await createGame(hostPage, 'Alice')
    const code = await getJoinCode(hostPage)

    await guestPage.goto('/')
    await guestPage.getByTestId('name-input').fill('Bob')
    await guestPage.getByTestId('join-code-input').fill(code)
    await guestPage.getByTestId('join-game-button').click()
    await guestPage.waitForURL(/\/games\/.+\/lobby/)

    // Default duration is visible to both, but only the host can edit it.
    await expect(hostPage.getByTestId('duration-label')).toContainText('5 min')
    await expect(guestPage.getByTestId('duration-label')).toContainText('5 min')
    await expect(guestPage.getByTestId('duration-input')).toHaveCount(0)

    await hostPage.getByTestId('duration-input').fill('180')
    await hostPage.getByTestId('duration-input').dispatchEvent('change')
    await expect(hostPage.getByTestId('duration-label')).toContainText('3 min')

    // Guest's lobby updates live (via SSE) to show the new duration, with
    // no reload — the whole point of moving this control to the lobby.
    await expect(guestPage.getByTestId('duration-label')).toContainText('3 min')

    const gameId = hostPage.url().match(/\/games\/([^/]+)\/lobby/)![1]
    const res = await hostPage.request.get(`/api/games/${gameId}`)
    const data = await res.json()
    expect(data.game.durationSeconds).toBe(180)

    await hostContext.close()
    await guestContext.close()
  })
})

test.describe('joining a game', () => {
  test('joins with a valid code and both players see each other live', async ({ browser }) => {
    const hostContext = await browser.newContext()
    const guestContext = await browser.newContext()
    const hostPage = await hostContext.newPage()
    const guestPage = await guestContext.newPage()

    await createGame(hostPage, 'Alice')
    const code = await getJoinCode(hostPage)

    await guestPage.goto('/')
    await guestPage.getByTestId('name-input').fill('Bob')
    await guestPage.getByTestId('join-code-input').fill(code)
    await guestPage.getByTestId('join-game-button').click()
    await guestPage.waitForURL(/\/games\/.+\/lobby/)

    // Guest sees both players and the "waiting for host" message, not the
    // start button (only the host can start).
    await expect(guestPage.getByTestId('player-row')).toHaveCount(2)
    await expect(guestPage.getByTestId('waiting-message')).toBeVisible()
    await expect(guestPage.getByTestId('start-button')).toHaveCount(0)

    // Host's lobby updates live (via SSE) to show the guest, with no reload.
    await expect(hostPage.getByTestId('player-row')).toHaveCount(2)
    await expect(hostPage.getByTestId('player-row').filter({ hasText: 'Bob' })).toBeVisible()

    await hostContext.close()
    await guestContext.close()
  })

  test('lets a player join by lowercase code', async ({ browser }) => {
    const hostContext = await browser.newContext()
    const guestContext = await browser.newContext()
    const hostPage = await hostContext.newPage()
    const guestPage = await guestContext.newPage()

    await createGame(hostPage, 'Alice')
    const code = await getJoinCode(hostPage)

    await guestPage.goto('/')
    await guestPage.getByTestId('name-input').fill('Bob')
    // The join-code input itself uppercases keystrokes, so type lowercase
    // to exercise that behavior rather than pre-filling an already-cased value.
    await guestPage.getByTestId('join-code-input').pressSequentially(code.toLowerCase())
    await guestPage.getByTestId('join-game-button').click()

    await guestPage.waitForURL(/\/games\/.+\/lobby/)
    await expect(guestPage.getByTestId('player-row')).toHaveCount(2)

    await hostContext.close()
    await guestContext.close()
  })

  test('requires a name before joining', async ({ page }) => {
    await page.goto('/')
    await page.getByTestId('join-code-input').fill('ABCDEF')
    await page.getByTestId('join-game-button').click()

    await expect(page.getByTestId('home-error')).toHaveText('Enter your name first')
    await expect(page).toHaveURL('/')
  })

  test('requires a code before joining', async ({ page }) => {
    await page.goto('/')
    await page.getByTestId('name-input').fill('Bob')
    await page.getByTestId('join-game-button').click()

    await expect(page.getByTestId('home-error')).toHaveText('Enter a game code')
    await expect(page).toHaveURL('/')
  })

  test('shows an error when the code does not match any game', async ({ page }) => {
    await page.goto('/')
    await page.getByTestId('name-input').fill('Bob')
    await page.getByTestId('join-code-input').fill('ZZZZZZ')
    await page.getByTestId('join-game-button').click()

    await expect(page.getByTestId('home-error')).toContainText('not found')
    await expect(page).toHaveURL('/')
  })

  test('shows an error when the game has already started', async ({ browser }) => {
    const hostContext = await browser.newContext()
    const hostPage = await hostContext.newPage()

    await createGame(hostPage, 'Alice')
    const code = await getJoinCode(hostPage)
    await startGame(hostPage)

    const guestContext = await browser.newContext()
    const guestPage = await guestContext.newPage()
    await guestPage.goto('/')
    await guestPage.getByTestId('name-input').fill('Bob')
    await guestPage.getByTestId('join-code-input').fill(code)
    await guestPage.getByTestId('join-game-button').click()

    await expect(guestPage.getByTestId('home-error')).toContainText('already started')
    await expect(guestPage).toHaveURL('/')

    await hostContext.close()
    await guestContext.close()
  })
})

test.describe('lobby invite link', () => {
  test('copies an invite link containing the join code', async ({ page }) => {
    await stubClipboard(page)
    await createGame(page, 'Alice')
    const code = await getJoinCode(page)

    await expect(page.getByTestId('copy-link-button')).toHaveText('🔗 Copy invite link')
    await page.getByTestId('copy-link-button').click()

    const copiedText = await page.evaluate(
      () => (window as unknown as { __copiedText: string }).__copiedText,
    )
    expect(copiedText).toMatch(new RegExp(`/join/${code}$`))
    await expect(page.getByTestId('copy-link-button')).toHaveText('✅ Link copied!')
  })

  test('shows and dismisses a QR code for the invite link', async ({ page }) => {
    await createGame(page, 'Alice')

    await expect(page.getByTestId('qr-modal')).toHaveCount(0)
    await page.getByTestId('show-qr-button').click()

    const modal = page.getByTestId('qr-modal')
    await expect(modal).toBeVisible()
    const qrSrc = await page.getByTestId('qr-code-image').getAttribute('src')
    expect(qrSrc).toMatch(/^data:image\/png;base64,/)

    await page.getByTestId('close-qr-button').click()
    await expect(modal).toHaveCount(0)
  })

  test('dismisses the QR modal by clicking the backdrop', async ({ page }) => {
    await createGame(page, 'Alice')
    await page.getByTestId('show-qr-button').click()

    const modal = page.getByTestId('qr-modal')
    await expect(modal).toBeVisible()
    // Click the backdrop itself (top-left corner), not the card inside it.
    await modal.click({ position: { x: 5, y: 5 } })
    await expect(modal).toHaveCount(0)
  })
})

test.describe('joining via invite link', () => {
  test('pre-fills the code, focuses the name field, and hides the create-game option', async ({
    browser,
  }) => {
    const hostContext = await browser.newContext()
    const hostPage = await hostContext.newPage()
    await createGame(hostPage, 'Alice')
    const code = await getJoinCode(hostPage)

    const guestContext = await browser.newContext()
    const guestPage = await guestContext.newPage()
    await guestPage.goto(`/join/${code}`)

    await expect(guestPage.getByTestId('join-code-input')).toHaveValue(code)
    await expect(guestPage.getByTestId('create-game-button')).toHaveCount(0)
    await expect(guestPage.getByText('OR')).toHaveCount(0)
    await expect(guestPage.getByTestId('name-input')).toBeFocused()

    await hostContext.close()
    await guestContext.close()
  })

  test('joins the game from the invite-link screen', async ({ browser }) => {
    const hostContext = await browser.newContext()
    const hostPage = await hostContext.newPage()
    await createGame(hostPage, 'Alice')
    const code = await getJoinCode(hostPage)

    const guestContext = await browser.newContext()
    const guestPage = await guestContext.newPage()
    await guestPage.goto(`/join/${code}`)
    await guestPage.getByTestId('name-input').fill('Bob')
    await guestPage.getByTestId('join-game-button').click()

    await guestPage.waitForURL(/\/games\/.+\/lobby/)
    await expect(guestPage.getByTestId('player-row')).toHaveCount(2)

    await hostContext.close()
    await guestContext.close()
  })
})
