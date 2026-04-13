import type { APIRequestContext, Page } from '@playwright/test'
import { expect, test } from '@playwright/test'

const apiURL = process.env.PLAYWRIGHT_API_URL || 'http://localhost:8080'

type SmokeUser = {
  clinicId: string
  email: string
  password: string
}

async function createAdminUser(request: APIRequestContext): Promise<SmokeUser> {
  const stamp = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
  const clinicId = '309c1dd1-2759-4e08-82df-b163d776ab72'
  const email = `playwright.admin.${stamp}@clinic.kz`
  const password = 'SecurePass123!'

  const response = await request.post(`${apiURL}/api/v1/auth/register`, {
    data: {
      clinic_id: clinicId,
      email,
      password,
      first_name: 'Playwright',
      last_name: 'Admin',
      iin: '990101300001',
      phone: '+77010000001',
      role: 'admin',
    },
  })

  expect(response.ok()).toBeTruthy()

  return { clinicId, email, password }
}

async function loginThroughUI(page: Page, user: SmokeUser) {
  await page.goto('/login')
  await page.getByLabel(/email/i).fill(user.email)
  await page.getByLabel(/пароль|password/i).fill(user.password)
  await page.getByRole('button', { name: /войти|sign in/i }).click()
  await expect(page).toHaveURL(/\/dashboard$/)
}

test('redirects protected route to login when unauthenticated', async ({ page }) => {
  await page.goto('/users')

  await expect(page).toHaveURL(/\/login$/)
  await expect(page.getByRole('button', { name: /войти|sign in/i })).toBeVisible()
})

test('admin can sign in and open users page', async ({ page, request }) => {
  const user = await createAdminUser(request)

  await loginThroughUI(page, user)
  await page.goto('/users')

  await expect(page).toHaveURL(/\/users$/)
  await expect(page.getByPlaceholder(/поиск по имени или email/i)).toBeVisible()
  await expect(page.getByText(user.email)).toBeVisible()
})

test('gateway smoke covers auth, billing, and integration endpoints', async ({ request }) => {
  const user = await createAdminUser(request)

  const loginResponse = await request.post(`${apiURL}/api/v1/auth/login`, {
    data: { email: user.email, password: user.password },
  })
  expect(loginResponse.ok()).toBeTruthy()

  const loginData = await loginResponse.json()
  const accessToken = loginData.access_token as string

  const authMe = await request.get(`${apiURL}/api/v1/auth/me`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  })
  expect(authMe.ok()).toBeTruthy()

  const users = await request.get(`${apiURL}/api/v1/users?limit=10&offset=0`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  })
  expect(users.ok()).toBeTruthy()

  const plans = await request.get(`${apiURL}/api/v1/plans`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  })
  expect(plans.ok()).toBeTruthy()

  const integrations = await request.get(`${apiURL}/api/v1/integrations/${user.clinicId}`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  })
  expect(integrations.ok()).toBeTruthy()
})
