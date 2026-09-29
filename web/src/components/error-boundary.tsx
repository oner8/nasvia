import { Component, type ErrorInfo, type ReactNode } from 'react'

interface Props {
  children: ReactNode
}

interface State {
  error: Error | null
}

/** 全局错误边界：单个组件异常时给出可读提示，而不是整页空白。 */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('[nasvia] 界面渲染出错：', error, info.componentStack)
  }

  render() {
    const { error } = this.state
    if (!error) return this.props.children

    return (
      <div className="flex min-h-dvh items-center justify-center p-6">
        <div className="w-full max-w-lg rounded-2xl border border-[var(--color-border)] bg-[var(--color-card)] p-6">
          <h1 className="text-base font-semibold">页面渲染出错</h1>
          <p className="mt-2 text-xs leading-relaxed text-[var(--color-muted-foreground)]">
            请刷新重试；若持续出现，可在 NAS 上查看服务日志。
          </p>
          <pre className="mt-3 overflow-x-auto rounded-xl bg-[var(--color-muted)] p-3 text-xs leading-relaxed">
            {error.message}
          </pre>
          <button
            type="button"
            onClick={() => window.location.reload()}
            className="mt-4 h-10 rounded-xl bg-[var(--color-primary)] px-4 text-sm font-medium text-[var(--color-primary-foreground)]"
          >
            刷新页面
          </button>
        </div>
      </div>
    )
  }
}
