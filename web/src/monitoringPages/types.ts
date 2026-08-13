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
