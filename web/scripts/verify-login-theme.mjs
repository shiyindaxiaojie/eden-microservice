import { spawn } from 'node:child_process'
import { existsSync } from 'node:fs'
import { mkdtemp, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

const baseURL = process.env.UI_BASE_URL || 'http://localhost:2019'
const chromePath = process.env.CHROME_PATH || findChromePath()
const viewport = { width: 1440, height: 900 }
const remotePort = Number(process.env.CHROME_DEBUG_PORT || 9227)
const expectedDocumentTitle = '微服务平台'

function assert(condition, message, details) {
  if (!condition) {
    const suffix = details ? `\n${JSON.stringify(details, null, 2)}` : ''
    throw new Error(`${message}${suffix}`)
  }
}

function findChromePath() {
  const candidates = [
    'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
    'C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe',
    'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
  ]

  return candidates.find((path) => {
    return existsSync(path)
  })
}

async function wait(ms) {
  await new Promise((resolve) => setTimeout(resolve, ms))
}

async function fetchJson(url, options) {
  const res = await fetch(url, options)
  if (!res.ok) {
    throw new Error(`GET ${url} failed with ${res.status}`)
  }
  return res.json()
}

async function waitForDebugger() {
  const started = Date.now()
  let lastError
  while (Date.now() - started < 10000) {
    try {
      return await fetchJson(`http://127.0.0.1:${remotePort}/json/version`)
    } catch (error) {
      lastError = error
      await wait(120)
    }
  }
  throw lastError || new Error('Chrome debugger did not start')
}

function createCDPClient(socketUrl) {
  const ws = new WebSocket(socketUrl)
  let id = 0
  const pending = new Map()

  ws.addEventListener('message', (event) => {
    const payload = JSON.parse(event.data)
    if (payload.id && pending.has(payload.id)) {
      const { resolve, reject } = pending.get(payload.id)
      pending.delete(payload.id)
      if (payload.error) {
        reject(new Error(payload.error.message || JSON.stringify(payload.error)))
      } else {
        resolve(payload.result)
      }
      return
    }
  })

  function send(method, params = {}) {
    const messageId = ++id
    const message = JSON.stringify({ id: messageId, method, params })
    return new Promise((resolve, reject) => {
      pending.set(messageId, { resolve, reject })
      ws.send(message)
    })
  }

  async function ready() {
    if (ws.readyState === WebSocket.OPEN) return
    await new Promise((resolve, reject) => {
      ws.addEventListener('open', resolve, { once: true })
      ws.addEventListener('error', reject, { once: true })
    })
  }

  async function close() {
    ws.close()
    await wait(60)
  }

  return { ready, send, close }
}

async function evaluate(client, expression) {
  const result = await client.send('Runtime.evaluate', {
    expression,
    awaitPromise: true,
    returnByValue: true,
  })

  if (result.exceptionDetails) {
    throw new Error(
      result.exceptionDetails.exception?.description
        || result.exceptionDetails.text
        || 'Runtime.evaluate failed',
    )
  }

  return result.result.value
}

async function navigate(client, url) {
  await client.send('Page.navigate', { url })
  await wait(900)
  await evaluate(client, `new Promise((resolve) => {
    if (document.readyState === 'complete') {
      requestAnimationFrame(() => requestAnimationFrame(resolve))
      return
    }
    window.addEventListener('load', () => requestAnimationFrame(() => requestAnimationFrame(resolve)), { once: true })
  })`)
}

async function main() {
  assert(chromePath, 'Chrome or Edge executable was not found')

  const userDataDir = await mkdtemp(join(tmpdir(), 'eden-ui-verify-'))
  let client
  const chrome = spawn(chromePath, [
    '--headless=new',
    `--remote-debugging-port=${remotePort}`,
    `--user-data-dir=${userDataDir}`,
    `--window-size=${viewport.width},${viewport.height}`,
    '--disable-gpu',
    '--no-first-run',
    'about:blank',
  ], { stdio: 'ignore' })

  try {
    await waitForDebugger()
    const pageTarget = await fetchJson(`http://127.0.0.1:${remotePort}/json/new?about:blank`, { method: 'PUT' })
    assert(pageTarget, 'Chrome page target was not found')
    client = createCDPClient(pageTarget.webSocketDebuggerUrl)
    await client.ready()
    await client.send('Page.enable')
    await client.send('Runtime.enable')
    await client.send('Network.enable')
    await client.send('Emulation.setDeviceMetricsOverride', {
      width: viewport.width,
      height: viewport.height,
      deviceScaleFactor: 1,
      mobile: false,
    })

    await navigate(client, `${baseURL}/login`)
    await wait(700)
    const loginBackground = await evaluate(client, `(() => {
      const layer = document.querySelector('.background-layer')
      const rect = layer.getBoundingClientRect()
      const image = layer.querySelector('.bg-image')
      const overlay = layer.querySelector('.bg-overlay')
      const nav = document.querySelector('.top-nav')
      const main = document.querySelector('.login-main-content, .main-content')
      const startButton = document.querySelector('.start-game-btn')
      const modal = document.querySelector('.login-modal, .modal-content')
      const panel = document.querySelector('.hero-login-panel')
      const loginForm = panel?.querySelector('.hero-login-form')
      const usernameInput = panel?.querySelector('input[autocomplete="username"]')
      const passwordInput = panel?.querySelector('input[autocomplete="current-password"]')
      const submitButton = panel?.querySelector('.login-btn')
      const inputWrapper = panel?.querySelector('.input-wrapper')
      const panelRect = panel?.getBoundingClientRect()
      const subtitle = document.querySelector('.game-subtitle')
      const bubbles = Array.from(document.querySelectorAll('.bubble'))
      const bubblesInLoginBand = bubbles.filter((bubble) => {
        const left = Number.parseFloat(bubble.style.left || '0')
        return left > 72 && left < 92
      })
      const oldParticles = Array.from(document.querySelectorAll('.particle'))
      const firstBubbleStyle = bubbles[0] ? window.getComputedStyle(bubbles[0]) : null
      const loginTitle = panel?.querySelector('.hero-login-title')
      const loginTitleAfter = loginTitle ? window.getComputedStyle(loginTitle, '::after') : null
      const brandDivider = document.querySelector('.brand-divider')
      const infoSection = document.querySelector('.game-info-section')
      const infoBefore = infoSection ? window.getComputedStyle(infoSection, '::before') : null
      const infoAfter = infoSection ? window.getComputedStyle(infoSection, '::after') : null
      const isVisible = (el) => {
        if (!el) return false
        const elStyle = window.getComputedStyle(el)
        const elRect = el.getBoundingClientRect()
        return elStyle.display !== 'none'
          && elStyle.visibility !== 'hidden'
          && elStyle.opacity !== '0'
          && elRect.width > 0
          && elRect.height > 0
      }
      const style = window.getComputedStyle(layer)
      const imageStyle = image ? window.getComputedStyle(image) : null
      const overlayStyle = overlay ? window.getComputedStyle(overlay) : null
      const navStyle = nav ? window.getComputedStyle(nav) : null
      const mainStyle = main ? window.getComputedStyle(main) : null
      const panelStyle = panel ? window.getComputedStyle(panel) : null
      const formStyle = loginForm ? window.getComputedStyle(loginForm) : null
      const submitButtonStyle = submitButton ? window.getComputedStyle(submitButton) : null
      const inputWrapperStyle = inputWrapper ? window.getComputedStyle(inputWrapper) : null
      return {
        width: rect.width,
        height: rect.height,
        layerBackground: style.backgroundImage,
        layerBackgroundColor: style.backgroundColor,
        documentTitle: document.title,
        overlayBackground: overlayStyle?.backgroundImage || '',
        imageLoaded: image instanceof HTMLImageElement && image.complete && image.naturalWidth > 0,
        imageOpacity: imageStyle?.opacity || '',
        imageTransform: imageStyle?.transform || '',
        navBackground: navStyle?.backgroundColor || '',
        mainBackground: mainStyle?.backgroundColor || '',
        startButtonExists: Boolean(startButton),
        modalExists: Boolean(modal),
        panelVisible: isVisible(panel),
        panelLeft: panelRect?.left || 0,
        panelRight: panelRect?.right || 0,
        panelWidth: panelRect?.width || 0,
        panelTitle: panel?.querySelector('.hero-login-title, .tab-btn.active')?.textContent?.trim() || '',
        subtitleText: subtitle?.textContent?.trim() || '',
        pageText: document.body.textContent || '',
        panelBackground: panelStyle?.backgroundColor || '',
        formBackgroundImage: formStyle?.backgroundImage || '',
        formBackgroundColor: formStyle?.backgroundColor || '',
        formBorderTopWidth: formStyle?.borderTopWidth || '',
        formBoxShadow: formStyle?.boxShadow || '',
        formBorderRadius: formStyle?.borderRadius || '',
        submitButtonBackgroundImage: submitButtonStyle?.backgroundImage || '',
        submitButtonBackgroundColor: submitButtonStyle?.backgroundColor || '',
        submitButtonBoxShadow: submitButtonStyle?.boxShadow || '',
        inputWrapperBoxShadow: inputWrapperStyle?.boxShadow || '',
        firstBubbleBackgroundImage: firstBubbleStyle?.backgroundImage || '',
        firstBubbleBoxShadow: firstBubbleStyle?.boxShadow || '',
        bubbleCount: bubbles.length,
        bubblesInLoginBand: bubblesInLoginBand.length,
        oldParticleCount: oldParticles.length,
        firstBubbleAnimation: firstBubbleStyle?.animationName || '',
        loginTitleAfterContent: loginTitleAfter?.content || '',
        brandDividerExists: Boolean(brandDivider),
        infoBeforeContent: infoBefore?.content || '',
        infoAfterContent: infoAfter?.content || '',
        usernameVisible: isVisible(usernameInput),
        passwordVisible: isVisible(passwordInput),
        submitButtonVisible: isVisible(submitButton),
      }
    })()`)

    assert(loginBackground.width >= viewport.width, 'login background does not cover viewport width', loginBackground)
    assert(loginBackground.height >= viewport.height, 'login background does not cover viewport height', loginBackground)
    assert(loginBackground.documentTitle === expectedDocumentTitle, 'browser title should stay fixed on the login page', loginBackground)
    assert(loginBackground.imageLoaded, 'login background image did not load', loginBackground)
    assert(loginBackground.layerBackground !== 'none', 'login background has no fallback background', loginBackground)
    assert(loginBackground.overlayBackground.includes('gradient'), 'login background overlay is missing gradient depth', loginBackground)
    assert(loginBackground.imageOpacity !== '0', 'login background image is hidden', loginBackground)
    assert(loginBackground.imageTransform !== 'none', 'login background image should be shifted left with transform', loginBackground)
    assert(loginBackground.navBackground.includes('rgba(255, 255, 255'), 'light login banner should be translucent white', loginBackground)
    assert(!loginBackground.startButtonExists, 'login page should not require a button before username/password input', loginBackground)
    assert(!loginBackground.modalExists, 'login page should not render a modal-based username/password flow', loginBackground)
    assert(loginBackground.panelVisible, 'right side username/password login panel should be visible inline', loginBackground)
    assert(loginBackground.usernameVisible, 'inline login panel should show the username input', loginBackground)
    assert(loginBackground.passwordVisible, 'inline login panel should show the password input', loginBackground)
    assert(loginBackground.submitButtonVisible, 'inline login panel should show the login submit button', loginBackground)
    assert(loginBackground.panelLeft > viewport.width * 0.52, 'inline login panel should stay on the right side of the hero', loginBackground)
    assert(
      loginBackground.panelBackground === 'rgba(0, 0, 0, 0)' || loginBackground.panelBackground === 'transparent',
      'inline login panel shell should stay transparent instead of becoming a heavy card',
      loginBackground,
    )
    assert(
      loginBackground.formBackgroundImage === 'none'
        && (loginBackground.formBackgroundColor === 'rgba(0, 0, 0, 0)' || loginBackground.formBackgroundColor === 'transparent')
        && loginBackground.formBorderTopWidth === '0px'
        && loginBackground.formBoxShadow === 'none',
      'inline login form should not have an outer panel background, border, or shadow',
      loginBackground,
    )
    assert(loginBackground.submitButtonBackgroundImage === 'none', 'login button should use a flat background without gradient', loginBackground)
    assert(loginBackground.submitButtonBoxShadow === 'none', 'login button should not use a 3D shadow', loginBackground)
    assert(loginBackground.inputWrapperBoxShadow === 'none', 'login inputs should not use inner or glow shadows', loginBackground)
    assert(loginBackground.firstBubbleBackgroundImage === 'none', 'login bubbles should be flat circles without highlight gradients', loginBackground)
    assert(loginBackground.firstBubbleBoxShadow === 'none', 'login bubbles should not use 3D shadows', loginBackground)
    assert(loginBackground.oldParticleCount === 0, 'login background should not use the old falling snowflake particles', loginBackground)
    assert(loginBackground.bubbleCount >= 28, 'login background should render dense floating bubbles', loginBackground)
    assert(loginBackground.bubblesInLoginBand === 0, 'login bubbles should stay out of the right-side login content band', loginBackground)
    assert(loginBackground.firstBubbleAnimation.includes('bubble'), 'login bubbles should use the bubble rise animation', loginBackground)
    assert(!loginBackground.brandDividerExists, 'login banner should not keep the brand divider line', loginBackground)
    assert(
      (loginBackground.infoBeforeContent === 'none' || loginBackground.infoBeforeContent === 'normal')
        && (loginBackground.infoAfterContent === 'none' || loginBackground.infoAfterContent === 'normal'),
      'right hero content should not keep decorative pseudo-element residue',
      loginBackground,
    )
    assert(
      loginBackground.loginTitleAfterContent === ''
        || loginBackground.loginTitleAfterContent === 'none'
        || loginBackground.loginTitleAfterContent === 'normal',
      'username/password label should not keep the decorative trailing line',
      loginBackground,
    )
    assert(loginBackground.panelTitle === '', 'inline login panel should not render the username/password label copy', loginBackground)
    assert(loginBackground.subtitleText === '', 'login hero should not render the lightweight control-plane subtitle copy', loginBackground)
    assert(!loginBackground.pageText.includes('用户密码输入'), 'login page should not render 用户密码输入 copy', loginBackground)
    assert(!loginBackground.pageText.includes('轻量级微服务控制面'), 'login page should not render lightweight control-plane subtitle copy', loginBackground)
    assert(!loginBackground.pageText.includes('Username / Password'), 'login page should not render username/password label copy', loginBackground)
    assert(!loginBackground.pageText.includes('Lightweight microservice control plane'), 'login page should not render English subtitle copy', loginBackground)
    assert(
      loginBackground.mainBackground === 'rgba(0, 0, 0, 0)' || loginBackground.mainBackground === 'transparent',
      `login main content should not paint over the background, got ${loginBackground.mainBackground}`,
      loginBackground,
    )

    await evaluate(client, `document.querySelector('.theme-toggle-btn').click()`)
    await wait(120)
    const darkLoginBanner = await evaluate(client, `(() => ({
      dataTheme: document.documentElement.getAttribute('data-theme'),
      documentTitle: document.title,
      navBackground: window.getComputedStyle(document.querySelector('.top-nav')).backgroundColor,
      imageFilter: window.getComputedStyle(document.querySelector('.bg-image')).filter,
      overlayBackground: window.getComputedStyle(document.querySelector('.bg-overlay')).backgroundImage,
    }))()`)

    assert(darkLoginBanner.dataTheme === 'dark', 'login theme toggle should set dark theme', darkLoginBanner)
    assert(darkLoginBanner.documentTitle === expectedDocumentTitle, 'browser title should not change after login theme toggle', darkLoginBanner)
    assert(!darkLoginBanner.navBackground.includes('rgba(255, 255, 255'), 'dark login banner should not use translucent white', darkLoginBanner)
    assert(darkLoginBanner.navBackground.includes('rgba(') && !darkLoginBanner.navBackground.includes('rgb('), 'dark login banner should stay translucent', darkLoginBanner)
    assert(!darkLoginBanner.imageFilter.includes('brightness(0.5'), 'dark login background image should not be overly dimmed', darkLoginBanner)

    const storedAuth = await evaluate(client, `(() => {
      localStorage.setItem('token', 'verify-token')
      localStorage.setItem('username', 'verify-user')
      localStorage.setItem('nickname', 'verify-user')
      localStorage.setItem('user_role', 'admin')
      localStorage.setItem('theme', 'light')
      document.cookie = 'theme=light;path=/;max-age=31536000'
      return {
        href: window.location.href,
        token: localStorage.getItem('token'),
        theme: localStorage.getItem('theme'),
        cookie: document.cookie,
      }
    })()`)
    assert(storedAuth.token === 'verify-token', 'verification token was not stored before navigating home', storedAuth)

    await client.send('Network.setBlockedURLs', { urls: ['*/v1/*'] })
    await navigate(client, `${baseURL}/`)
    const beforeToggle = await evaluate(client, `(() => ({
      documentTitle: document.title,
      dataTheme: document.documentElement.getAttribute('data-theme'),
      classList: Array.from(document.documentElement.classList),
      localTheme: localStorage.getItem('theme'),
      cookieTheme: document.cookie.match(/(?:^|; )theme=([^;]+)/)?.[1] || '',
    }))()`)

    assert(beforeToggle.dataTheme === 'light', 'home should start in light theme from persisted preference', beforeToggle)
    assert(beforeToggle.documentTitle === expectedDocumentTitle, 'browser title should stay fixed on the home page', beforeToggle)
    assert(beforeToggle.localTheme === 'light', 'home should preserve light theme in localStorage', beforeToggle)
    assert(!beforeToggle.classList.includes('dark'), 'light theme should not keep Element Plus dark class', beforeToggle)

    const homeState = await evaluate(client, `(() => ({
      href: window.location.href,
      appLayout: Boolean(document.querySelector('.app-layout')),
      publicLayout: Boolean(document.querySelector('.public-layout')),
      themeToggle: Boolean(document.querySelector('[data-theme-toggle]')),
      headerButtonCount: document.querySelectorAll('.header-actions .header-btn').length,
      token: localStorage.getItem('token'),
      storageKeys: Object.keys(localStorage),
    }))()`)
    assert(homeState.themeToggle, 'home theme toggle button was not rendered', homeState)

    await evaluate(client, `document.querySelector('[data-theme-toggle]').click()`)
    await wait(120)
    const afterDarkToggle = await evaluate(client, `(() => ({
      documentTitle: document.title,
      dataTheme: document.documentElement.getAttribute('data-theme'),
      classList: Array.from(document.documentElement.classList),
      localTheme: localStorage.getItem('theme'),
      cookieTheme: document.cookie.match(/(?:^|; )theme=([^;]+)/)?.[1] || '',
    }))()`)

    assert(afterDarkToggle.dataTheme === 'dark', 'home theme toggle should set html data-theme to dark')
    assert(afterDarkToggle.documentTitle === expectedDocumentTitle, 'browser title should not change after home theme toggle', afterDarkToggle)
    assert(afterDarkToggle.classList.includes('dark'), 'home theme toggle should set Element Plus dark class')
    assert(afterDarkToggle.localTheme === 'dark', 'home theme toggle should persist dark theme to localStorage')
    assert(afterDarkToggle.cookieTheme === 'dark', 'home theme toggle should persist dark theme to cookie')

    const darkHomeStyles = await evaluate(client, `(() => {
      const activityHead = document.querySelector('.activity-head')
      const shell = document.querySelector('.dashboard-shell')
      const style = activityHead ? window.getComputedStyle(activityHead) : null
      const shellStyle = shell ? window.getComputedStyle(shell) : null
      return {
        dataTheme: document.documentElement.getAttribute('data-theme'),
        classList: Array.from(document.documentElement.classList),
        shellClass: shell?.className || '',
        shellActivityHeadVar: shellStyle?.getPropertyValue('--dashboard-activity-head-bg') || '',
        activityHeadBackground: style?.backgroundImage || '',
        activityHeadBackgroundColor: style?.backgroundColor || '',
      }
    })()`)

    assert(
      !darkHomeStyles.activityHeadBackground.includes('rgba(255, 255, 255, 0.68)'),
      'home dark theme should not use the light activity header gradient',
      darkHomeStyles,
    )

    await evaluate(client, `document.querySelector('[data-theme-toggle]').click()`)
    await wait(120)
    const afterLightToggle = await evaluate(client, `(() => ({
      documentTitle: document.title,
      dataTheme: document.documentElement.getAttribute('data-theme'),
      classList: Array.from(document.documentElement.classList),
      localTheme: localStorage.getItem('theme'),
      cookieTheme: document.cookie.match(/(?:^|; )theme=([^;]+)/)?.[1] || '',
    }))()`)

    assert(afterLightToggle.dataTheme === 'light', 'home theme toggle should return html data-theme to light')
    assert(afterLightToggle.documentTitle === expectedDocumentTitle, 'browser title should remain fixed after returning to light theme', afterLightToggle)
    assert(!afterLightToggle.classList.includes('dark'), 'light theme should remove Element Plus dark class')
    assert(afterLightToggle.localTheme === 'light', 'home theme toggle should persist light theme to localStorage')
    assert(afterLightToggle.cookieTheme === 'light', 'home theme toggle should persist light theme to cookie')

  } finally {
    if (client) {
      await client.close()
    }
    chrome.kill()
    await new Promise((resolve) => {
      chrome.once('exit', resolve)
      setTimeout(resolve, 1500)
    })
    await rm(userDataDir, { recursive: true, force: true }).catch(() => {})
  }
}

main().catch((error) => {
  console.error(error)
  process.exit(1)
})
