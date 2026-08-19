import { Component, type ReactNode } from 'react'

interface Props {
  children: ReactNode
  fallback?: ReactNode
}

interface State {
  hasError: boolean
  error?: Error
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error('ErrorBoundary caught:', error, info)
  }

  render() {
    if (this.state.hasError) {
      return (
        this.props.fallback ?? (
          <div className="alert alert-error" style={{ margin: 32 }}>
            <h2 style={{ margin: '0 0 8px', fontSize: 16 }}>Something went wrong</h2>
            <pre
              style={{
                whiteSpace: 'pre-wrap',
                fontSize: 12,
                margin: 0,
                fontFamily: 'var(--font-mono)',
              }}
            >
              {this.state.error?.message}
            </pre>
            <button
              className="btn btn-primary"
              onClick={() => window.location.reload()}
              style={{ marginTop: 16 }}
            >
              Reload page
            </button>
          </div>
        )
      )
    }
    return this.props.children
  }
}
