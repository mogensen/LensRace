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
    // force: true because the start button has a continuous "bob" CSS
    // animation, which fails Playwright's default actionability stability
    // check (the button's bounding box never settles).
    await hostPage.getByTestId('start-button').click({ force: true })
    await hostPage.waitForURL(/\/games\/.+\/play/)

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
