export type ReportDefinitionStatus = 'DRAFT' | 'ACTIVE' | 'DISABLED'

export type ReportSummary = {
  id: number
  code: string
  name: string
  category: string
  description: string
  datasourceId: number
  status: ReportDefinitionStatus
  currentDraftVersionId: number
  currentPublishedVersionId: number
  lockVersion: number
  updatedAt: string | null
}

export type ReportCatalogPage = {
  items: ReportSummary[]
  hasMore: boolean
  nextAfterId: number
}

export type ReportCatalogQuery = {
  search?: string
  category?: string
  afterId?: number
  limit?: number
}

export type ReportCenterSection = 'catalog' | 'configuration' | 'query' | 'exports'
