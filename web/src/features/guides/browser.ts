import './guides.css'

/** Native image links and anchors work without this optional enhancement. */
const imageDialog = document.querySelector<HTMLDialogElement>(
  '[data-guide-dialog]'
)
if (imageDialog && typeof imageDialog.showModal === 'function') {
  const enlarged = imageDialog.querySelector<HTMLImageElement>('img')
  document.addEventListener('click', (event) => {
    if (!(event.target instanceof Element) || !enlarged) return
    const link = event.target.closest<HTMLAnchorElement>('[data-guide-image]')
    const image = link?.querySelector('img')
    if (!link || !image || event.defaultPrevented) return
    if (
      event instanceof MouseEvent &&
      (event.button !== 0 ||
        event.metaKey ||
        event.ctrlKey ||
        event.shiftKey ||
        event.altKey)
    ) {
      return
    }
    event.preventDefault()
    enlarged.src = link.href
    enlarged.alt = image.alt
    imageDialog.showModal()
  })
  imageDialog.addEventListener('click', (event) => {
    if (event.target === imageDialog) imageDialog.close()
  })
  imageDialog.addEventListener('close', () => {
    enlarged?.removeAttribute('src')
  })
}

const tocLinks = [
  ...document.querySelectorAll<HTMLAnchorElement>('.article-toc nav a'),
]
if (tocLinks.length > 0) {
  const sections = tocLinks.map((link) => ({
    link,
    section: document.querySelector<HTMLElement>(link.hash),
  }))
  const updateToc = () => {
    let active = sections[0]?.link
    for (const item of sections) {
      if (item.section && item.section.getBoundingClientRect().top <= 160) {
        active = item.link
      }
    }
    for (const { link } of sections) {
      link.classList.toggle('active', link === active)
      if (link === active) link.setAttribute('aria-current', 'location')
      else link.removeAttribute('aria-current')
    }
  }
  let pending = false
  window.addEventListener(
    'scroll',
    () => {
      if (pending) return
      pending = true
      requestAnimationFrame(() => {
        updateToc()
        pending = false
      })
    },
    { passive: true }
  )
  updateToc()
}
