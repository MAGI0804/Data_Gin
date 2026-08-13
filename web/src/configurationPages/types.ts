import type { ClientResponse } from '../api/client'

export type SourceDefinition = {
  id: number
  name: string
  code: string
  source_type: string
  enabled: boolean
  auth_type: string
  config_json: string
  has_secret?: boolean
  schema_json: string
  dedupe_keys: string
  source_query_key: string
}

export type SourceDraft = {
  id: number | null
  name: string
  code: string
  sourceType: string
  enabled: boolean
  authType: string
  configJSON: string
  schemaJSON: string
  dedupeKeys: string
  sourceQueryKey: string
  hasSecret: boolean
}

export type DestinationDefinition = {
  id: number
  name: string
  code: string
  destination_type: string
  config_json: string
  has_secret?: boolean
  enabled: boolean
}

export type DestinationDraft = {
  id: number | null
  name: string
  code: string
  destinationType: string
  configJSON: string
  enabled: boolean
  hasSecret: boolean
}

export type DeliveryTask = {
  id: number
  name: string
  source_id: number
  clean_table: string
  destination_id: number
  trigger_type: 'manual' | 'schedule' | 'event'
  cron_expr: string
  filter_json: string
  payload_template: string
  enabled: boolean
}

export type DeliveryTaskDraft = {
  id: number | null
  name: string
  sourceID: string
  cleanTable: string
  destinationID: string
  triggerType: DeliveryTask['trigger_type']
  cronExpr: string
  filterJSON: string
  payloadTemplate: string
  enabled: boolean
}

export type ConfigurationClientOptions = {
  method: 'GET' | 'POST' | 'PUT' | 'PATCH'
  body?: unknown
  signal?: AbortSignal
  showResult?: boolean
  silentLoading?: boolean
}

export type ConfigurationClient = (path: string, options: ConfigurationClientOptions) => Promise<ClientResponse>
