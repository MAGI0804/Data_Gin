import { Plus, Trash2 } from 'lucide-react'
import type { OfficeQueryParameter } from './types'
import styles from './OfficeMessage.module.css'

type Props = {
  value: OfficeQueryParameter[]
  testValues: Record<string, string>
  disabled?: boolean
  onChange: (value: OfficeQueryParameter[]) => void
  onTestValuesChange: (value: Record<string, string>) => void
}
export function QueryParameterEditor({ value, testValues, disabled = false, onChange, onTestValuesChange }: Props) {
  function update(index: number, patch: Partial<OfficeQueryParameter>) {
    onChange(value.map((item, current) => current === index ? { ...item, ...patch } : item))
  }
  return <div className={styles.editorBlock}>
    <div className={styles.editorHeading}><div><h3>SELECT 形参</h3><p>SQL 使用命名绑定，例如 <code>:bill_date</code>。日期会解析后以 Oracle 日期参数绑定。</p></div><button type="button" disabled={disabled} onClick={() => onChange([...value, { code: '', label: '', valueType: 'string', required: true }])}><Plus aria-hidden="true" />新增参数</button></div>
    {value.length === 0 ? <p className={styles.inlineEmpty}>当前 SELECT 无参数。</p> : <div className={styles.parameterList}>
      {value.map((parameter, index) => {
        const key = parameter.code.trim().toLowerCase()
        return <div className={styles.parameterRow} key={`${index}-${parameter.code}`}>
          <label>参数名<input className={styles.mono} placeholder="bill_date" value={parameter.code} disabled={disabled} onChange={(event) => update(index, { code: event.currentTarget.value.toLowerCase() })} /></label>
          <label>显示名称<input placeholder="业务日期" value={parameter.label} disabled={disabled} onChange={(event) => update(index, { label: event.currentTarget.value })} /></label>
          <label>类型<select value={parameter.valueType} disabled={disabled} onChange={(event) => { const valueType = event.currentTarget.value as OfficeQueryParameter['valueType']; update(index, { valueType, format: valueType === 'date' ? 'yyyyMMdd' : undefined }) }}><option value="string">文本</option><option value="integer">整数</option><option value="decimal">小数</option><option value="date">日期</option></select></label>
          {parameter.valueType === 'date' ? <label>输入格式<select value={parameter.format ?? 'yyyyMMdd'} disabled={disabled} onChange={(event) => update(index, { format: event.currentTarget.value as OfficeQueryParameter['format'] })}><option value="yyyyMMdd">yyyyMMdd</option><option value="yyyy-MM-dd">yyyy-MM-dd</option><option value="yyyy-MM-dd HH:mm:ss">yyyy-MM-dd HH:mm:ss</option></select></label> : null}
          <label>测试值<input className={parameter.valueType !== 'string' ? styles.mono : undefined} placeholder={parameter.valueType === 'date' ? parameter.format : ''} value={testValues[key] ?? ''} disabled={disabled || !key} onChange={(event) => onTestValuesChange({ ...testValues, [key]: event.currentTarget.value })} /></label>
          <label className={styles.checkbox}><input type="checkbox" checked={parameter.required} disabled={disabled} onChange={(event) => update(index, { required: event.currentTarget.checked })} /><span>必填</span></label>
          <button className={styles.iconButton} type="button" aria-label="删除参数" disabled={disabled} onClick={() => onChange(value.filter((_, current) => current !== index))}><Trash2 aria-hidden="true" /></button>
        </div>
      })}
    </div>}
  </div>
}
