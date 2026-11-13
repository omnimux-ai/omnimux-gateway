package router

import "strings"

// fork: the desktop landing page is the omnimux.ai homepage (2026-10-27). The
// shell therefore carries the desktop surface marker and the desktop pricing
// group for every visitor. `d.omnimux.ai` is retired and 301-redirects here, so
// no request host participates in this decision any more.
//
// `desktopSurfaceShell` rewrites a rendered public page into that surface. The
// frontend reads `data-surface` and `data-pricing-group`. It deliberately does
// not touch robots metadata: the homepage stays indexable, and the private
// shell declares its own `noindex`.
func desktopSurfaceShell(page []byte) []byte {
	if len(page) == 0 {
		return page
	}
	shell := string(page)

	// Attribute injection is independent of the shell's exact <html> attributes
	// so preview fixtures and production markup behave the same.
	if idx := strings.Index(shell, "<html"); idx >= 0 {
		if end := strings.Index(shell[idx:], ">"); end >= 0 {
			closeAt := idx + end
			if !strings.Contains(shell[idx:closeAt], "data-surface=") {
				shell = shell[:closeAt] + ` data-surface="desktop" data-pricing-group="desktop-720p"` + shell[closeAt:]
			}
		}
	}

	return []byte(shell)
}
