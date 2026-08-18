import { useEffect, useRef } from 'react'

export function usePolling(
  load: (background: boolean) => Promise<void>,
  ms: number,
  deps: any[] = []
) {
  const savedLoad = useRef(load)
  
  useEffect(() => {
    savedLoad.current = load
  }, [load])

  useEffect(() => {
    let alive = true
    
    const tick = async (background: boolean) => {
      if (!alive || document.hidden) return
      try {
        await savedLoad.current(background)
      } catch (e) {
        console.error('Polling error', e)
      }
    }
    
    tick(false) // Initial load
    const id = setInterval(() => tick(true), ms)
    
    const onVis = () => {
      if (!document.hidden) tick(true)
    }
    document.addEventListener('visibilitychange', onVis)
    
    return () => {
      alive = false
      clearInterval(id)
      document.removeEventListener('visibilitychange', onVis)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ms, ...deps])
}