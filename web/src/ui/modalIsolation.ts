export function isolateModalLayer(layer: HTMLElement) {
  const siblings = Array.from(document.body.children).filter(
    (element): element is HTMLElement => element instanceof HTMLElement && element !== layer,
  )
  const previous = siblings.map((element) => ({
    element,
    inert: element.hasAttribute('inert'),
    ariaHidden: element.getAttribute('aria-hidden'),
  }))

  for (const { element } of previous) {
    element.setAttribute('inert', '')
    element.setAttribute('aria-hidden', 'true')
  }

  return () => {
    for (const { element, inert, ariaHidden } of previous) {
      if (!inert) element.removeAttribute('inert')
      if (ariaHidden === null) element.removeAttribute('aria-hidden')
      else element.setAttribute('aria-hidden', ariaHidden)
    }
  }
}
