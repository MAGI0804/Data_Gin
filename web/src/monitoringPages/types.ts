export interface PipelineRun {
  id: number
  trace_id: string
  run_type: string
  trigger_type: string
  status: string
  total_count: number
  success_count: number
  failed_count: number
  source_id: number
  destination_id: number
  started_at: string | null
  finished_at: string | null
}

export interface StepRun {
  id: number
  run_id: number
  pipeline_id: number
  step_id: number
  step_code: string
  method_type: string
  status: string
  input_json: string
  output_json: string
  generated_config_json: string
  error_message: string
  started_at: string | null
  finished_at: string | null
}

export interface DeliveryLog {
  id: number
  trace_id: string
  run_id: number
  source_code: string
  destination_code: string
  destination_name: string
  destination_id: number
  clean_record_id: number
  business_key: string
  response_summary: string
  http_status: number
  success: boolean
  error_message: string
  retry_count: number
  sent_at: string | null
}

export interface MonitoringClientResult {
  ok: boolean
  status: number
  data: unknown
  error?: { message?: string }
}

export interface MonitoringClientOptions {
  method: 'GET' | 'POST'
  signal?: AbortSignal
  showResult: false
  silentLoading: true
}

export type MonitoringClient = (path: string, options: MonitoringClientOptions) => Promise<MonitoringClientResult>
