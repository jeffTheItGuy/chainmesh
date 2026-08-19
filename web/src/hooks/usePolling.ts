import { useEffect, useRef } from 'react'

/**
 * usePolling — visibility-aware, abort-safe polling hook.
 *
 * Improvements over previous version:
 * - Uses AbortController for proper cancellation
 * - Cleaner API: load function receives signal instead of background flag
 * - Caller handles loading state via the signal or separate state
 */
export function usePolling(
  load: (signal: AbortSignal, background: boolean) => Promise<void>,
  ms: number,
  deps: React.DependencyList = []
) {
  const savedLoad = useRef(load)
  useEffect(() => {
    savedLoad.current = load
  }, [load])

  useEffect(() => {
    const controller = new AbortController()
    let intervalId: ReturnType<typeof setInterval>

    const tick = async (background: boolean) => {
      if (controller.signal.aborted || document.hidden) return
      try {
        await savedLoad.current(controller.signal, background)
      } catch (err) {
        if (!controller.signal.aborted) {
          console.error('Polling error:', err)
        }
      }
    }

    // Initial load
    tick(false)

    // Recurring polls
    intervalId = setInterval(() => tick(true), ms)

    // Refetch when tab becomes visible
    const onVisibilityChange = () => {
      if (!document.hidden) tick(true)
    }
    document.addEventListener('visibilitychange', onVisibilityChange)

    return () => {
      controller.abort()
      clearInterval(intervalId)
      document.removeEventListener('visibilitychange', onVisibilityChange)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ms, ...deps])
}
