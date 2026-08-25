import { render, screen, act } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { ToastProvider, useToast } from './ToastProvider'

function TestButton() {
  const { showToast } = useToast()
  return (
    <button onClick={() => showToast('Saved successfully', 'success')}>
      Show toast
    </button>
  )
}

describe('ToastProvider', () => {
  it('shows a toast and auto-dismisses it', () => {
    vi.useFakeTimers()

    render(
      <ToastProvider>
        <TestButton />
      </ToastProvider>
    )

    act(() => {
      screen.getByRole('button', { name: /show toast/i }).click()
    })

    expect(screen.getByText('Saved successfully')).toBeInTheDocument()

    act(() => {
      vi.advanceTimersByTime(4000)
    })

    expect(screen.queryByText('Saved successfully')).not.toBeInTheDocument()

    vi.useRealTimers()
  })
})