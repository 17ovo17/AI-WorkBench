// Ultimate user-journey UI baseline for AI WorkBench.
// Run with: npx playwright test tests/whitebox-ui.spec.js
import { test, expect } from '@playwright/test'

const routes = [
  { path: '/workbench', labels: [/智能对话|会话|模型|发送/i] },
  { path: '/diagnose', labels: [/智能诊断|诊断|报告|IP/i] },
  { path: '/alerts', labels: [/告警中心|告警|AI 诊断|恢复|筛选/i] },
  { path: '/topology', labels: [/业务拓扑|新增业务端口|业务巡检|重新发现|业务列表/i] },
  { path: '/catpaw', labels: [/探针管理|凭证管理|远程安装|探针/i] },
  { path: '/settings/ai', labels: [/AI 配置|Provider|模型|保存/i] },
  { path: '/settings/datasource', labels: [/数据源|Prometheus|MySQL|Redis|保存/i] },
]

const mojibakePatterns = [
  '锟斤拷',
  '����',
  '鎺',
  '杩',
  '鏅',
  '涓?',
  '???',
]

async function expectReadable(page) {
  const body = page.locator('body')
  for (const pattern of mojibakePatterns) {
    await expect(body).not.toContainText(pattern)
  }
  await expect(body).not.toContainText('${AI_WORKBENCH_API_KEY}')
  await expect(body).not.toContainText(/sk-[A-Za-z0-9]{12,}/)
}

async function expectNoUnexpectedConsole(page) {
  const messages = []
  page.on('console', msg => {
    if (msg.type() === 'error' && !msg.text().includes('favicon.ico')) {
      messages.push(msg.text())
    }
  })
  return messages
}

test.describe('AI WorkBench ultimate UI baseline', () => {
  test.setTimeout(120000)
  for (const route of routes) {
    test(`${route.path} loads readable user controls`, async ({ page }) => {
      const consoleErrors = await expectNoUnexpectedConsole(page)
      await page.goto(route.path)
      await expect(page).toHaveTitle(/AI|WorkBench/i)
      await expectReadable(page)
      for (const label of route.labels) {
        await expect(page.locator('body')).toContainText(label)
      }
      expect(consoleErrors).toEqual([])
    })
  }

  test('topology shows scoped semantic layers without agent nodes', async ({ page }) => {
    const consoleErrors = await expectNoUnexpectedConsole(page)
    await page.goto('/topology')
    await expectReadable(page)
    await expect(page.getByText('业务主机清单', { exact: true }).first()).toBeVisible()
    await expect(page.locator('body')).toContainText('点击即可定位到对应业务组件')
    await expect(page.locator('.summary')).toContainText('组件')
    await expect(page.locator('.topo-node').filter({ hasText: '业务主机 ·' })).toHaveCount(0)
    await expect(page.getByText('入口层', { exact: true }).first()).toBeVisible()
    await expect(page.getByText('应用层', { exact: true }).first()).toBeVisible()
    await expect(page.getByText('中间件层', { exact: true }).first()).toBeVisible()
    await expect(page.getByText('数据库层', { exact: true }).first()).toBeVisible()
    await expect(page.getByText('观测层', { exact: true }).first()).toBeVisible()
    await expect(page.getByText(/Main Agent|主 Agent|Catpaw Sub Agent|Catpaw 子 Agent/)).toHaveCount(0)
    const firstHost = page.locator('.host-scope-item').first()
    if (await firstHost.count()) {
      await firstHost.click()
      await expect(firstHost).toHaveClass(/active/)
      await expect(page.locator('.topo-node.selected')).toHaveCount(1)
    }
    await expect(page.locator('body')).toContainText(/入口层 · .*:80/i)
    await expect(page.locator('body')).toContainText(/应用层 · .*8081/i)
    await expect(page.locator('body')).toContainText(/数据库层 · .*1521/i)
    expect(consoleErrors).toEqual([])
  })

  test('topology business inspection covers monitoring tabs', async ({ page }) => {
    const consoleErrors = await expectNoUnexpectedConsole(page)
    await page.goto('/topology')
    await expectReadable(page)
    await page.getByRole('button', { name: '业务巡检' }).click()
    await expect(page.getByRole('dialog', { name: '业务巡检与业务监控' })).toBeVisible({ timeout: 90000 })
    await expect(page.getByText(/AI business inspection completed|业务巡检完成/).first()).toBeVisible({ timeout: 90000 })
    for (const tabName of ['业务巡检', '业务分析', '业务进程', '业务属性', '业务资源', '告警情况']) {
      await expect(page.getByRole('tab', { name: tabName })).toBeVisible()
      await page.getByRole('tab', { name: tabName }).click()
      await expectReadable(page)
    }
    await expect(page.getByRole('button', { name: '重新巡检' })).toBeVisible()
    expect(consoleErrors).toEqual([])
  })

  test('catpaw chat panel is readable and disabled states are clear', async ({ page }) => {
    const consoleErrors = await expectNoUnexpectedConsole(page)
    await page.goto('/catpaw')
    await expectReadable(page)
    await page.getByRole('tab', { name: '探针对话' }).click()
    const chatText = page.getByText('探针交互诊断')
    if (await chatText.count()) {
      await expect(chatText.first()).toBeVisible()
      await expect(page.getByText('选择在线探针', { exact: true }).first()).toBeVisible()
      await expect(page.getByText('发送', { exact: true }).first()).toBeVisible()
    }
    expect(consoleErrors).toEqual([])
  })

  test('browser back/forward retains route render', async ({ page }) => {
    const consoleErrors = await expectNoUnexpectedConsole(page)
    await page.goto('/workbench')
    await page.goto('/topology')
    await page.goBack()
    await expect(page).toHaveURL(/\/workbench$/)
    await expectReadable(page)
    await page.goForward()
    await expect(page).toHaveURL(/\/topology$/)
    await expectReadable(page)
    expect(consoleErrors).toEqual([])
  })

  test('topology create and discover gives visible result', async ({ page }) => {
    const consoleErrors = await expectNoUnexpectedConsole(page)
    const name = `mcp-create-regression-${Date.now()}`
    await page.goto('/topology')
    await page.locator('.actions .el-button').first().click()
    const dialog = page.locator('.el-dialog:visible').last()
    await dialog.locator('input').first().fill(name)
    await dialog.locator('textarea').nth(1).fill('198.18.20.11')
    await dialog.locator('textarea').nth(2).fill('198.18.20.11:8081 jvm TCP')
    await dialog.getByRole('button').last().click()
    await expect(page.getByText(name).first()).toBeVisible({ timeout: 90000 })
    await expect(page.locator('.summary')).toContainText(/1/, { timeout: 90000 })
    await expect(page.locator('.topo-node')).toContainText('198.18.20.11:8081 jvm')
    
    await expect(page.getByText('Request failed')).toHaveCount(0)
    await expect(page.getByText('Internal Server Error')).toHaveCount(0)
    expect(consoleErrors.filter(item => !item.includes('favicon.ico'))).toEqual([])
  })

})





