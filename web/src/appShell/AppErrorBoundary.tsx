import { Component, type ErrorInfo, type ReactNode } from 'react'
import { AlertTriangle, RefreshCcw } from 'lucide-react'
import { Brand } from '../components/Brand'
import styles from './AppErrorBoundary.module.css'

type AppErrorBoundaryProps = { children: ReactNode }
type AppErrorBoundaryState = { failed: boolean }

export class AppErrorBoundary extends Component<AppErrorBoundaryProps, AppErrorBoundaryState> {
  state: AppErrorBoundaryState = { failed: false }

  static getDerivedStateFromError(): AppErrorBoundaryState {
    return { failed: true }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('管理台页面渲染失败', { error, componentStack: info.componentStack })
  }

  private reloadCurrentPage = () => {
    window.location.reload()
  }

  private returnToOverview = () => {
    window.location.hash = 'overview'
    window.location.reload()
  }

  render() {
    if (!this.state.failed) return this.props.children
    return (
      <main className={styles.shell}>
        <section className={styles.canvas} role="alert" aria-labelledby="app-recovery-title">
          <header className={styles.header}>
            <Brand size="large" />
            <span>页面恢复</span>
          </header>
          <div className={styles.content}>
            <AlertTriangle aria-hidden="true" />
            <div>
              <h1 id="app-recovery-title">页面加载遇到问题</h1>
              <p>当前操作没有继续执行。可以重试当前页面，或返回运行总览后再进入。</p>
            </div>
          </div>
          <div className={styles.actions}>
            <button type="button" onClick={this.returnToOverview}>返回运行总览</button>
            <button className={styles.primary} type="button" onClick={this.reloadCurrentPage}>
              <RefreshCcw aria-hidden="true" />重试当前页面
            </button>
          </div>
        </section>
      </main>
    )
  }
}
