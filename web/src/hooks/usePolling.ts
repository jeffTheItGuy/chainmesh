import { useEffect, useRef } from 'react'

/**
 * usePolling — visibility-aware, flicker-free polling hook.
 *
 * Solves three problems with raw `setInterval` polling:
 *
 * 1. Background-tab waste — pauses polling while `document.hidden` is true,
 *    and refetches the moment the tab becomes visible again.
 *
 * 2. Refresh-button flicker — the `background` flag lets the caller skip
 *    `setLoading(true)` on interval polls, so the button only shows
 *    "Refreshing…" for manual clicks and the initial load.
 *
 * 3. React StrictMode double-mount — the `alive` flag is a local closure
 *    variable (not a ref), so each mount/unmount cycle gets its own isolated
 *    flag. Cleanup from the first mount never poisons the second mount.
 *
 * @param load  Async function to run each tick. Receives `true` for
 *              background polls (interval / visibility refetch) and `false`
 *              for the initial load.
 * @param ms    Polling interval in milliseconds.
 * @param deps  Extra dependencies that should restart the poll loop when they
 *              change (e.g. the selected time range).
 */
export function usePolling(
  load: (background: boolean) => Promise<void>,
  ms: number,
  deps: any[] = []
) {
  // Always call the latest `load` without re-triggering the interval effect.
  const savedLoad = useRef(load)
  useEffect(() => {
    savedLoad.current = load
  }, [load])

  useEffect(() => {
    // Local flag, isolated per mount — StrictMode-safe.
    let alive = true

    const tick = async (background: boolean) => {
      if (!alive || document.hidden) return
      try {
        await savedLoad.current(background)
      } catch (err) {
        // `load` is expected to handle its own error state; this is a safety net.
        console.error('Polling error:', err)
      }
    }

    // Initial load — foreground, so the caller can show a loading state.
    tick(false)

    // Recurring polls — background, so no loading flicker.
    const id = setInterval(() => tick(true), ms)

    // Refetch as soon as the tab becomes visible again.
    const onVisibilityChange = () => {
      if (!document.hidden) tick(true)
    }
    document.addEventListener('visibilitychange', onVisibilityChange)

    return () => {
      alive = false
      clearInterval(id)
      document.removeEventListener('visibilitychange', onVisibilityChange)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ms, ...deps])
}