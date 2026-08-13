import { FileJson } from 'lucide-react'
import type { ClientResponse } from '../api/client'
import { redactMonitoringJSON } from '../monitoring'
import { Drawer } from '../ui'
import styles from './ResultDrawer.module.css'

export function ResultDrawer({ result, onClose }: { result: ClientResponse | null; onClose: () => void }) {
  return (
    <Drawer
      open={Boolean(result)}
      size="narrow"
      title={<span className={styles.title}><FileJson aria-hidden="true" />接口结果</span>}
      description={result ? `HTTP 状态 ${result.status}` : undefined}
      onClose={onClose}
    >
      {result ? <pre className={styles.json} aria-label="只读 JSON">{jsonText(redactMonitoringJSON(result.data))}</pre> : null}
    </Drawer>
  )
}

function jsonText(value: unknown) {
  if (typeof value === 'string') {
    try {
      return JSON.stringify(JSON.parse(value), null, 2)
    } catch {
      return value
    }
  }
  return JSON.stringify(value ?? {}, null, 2)
}
