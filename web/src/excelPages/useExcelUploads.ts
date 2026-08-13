import { useCallback, useState } from 'react'
import type { ApiRequestOptions, ClientResponse, HTTPMethod } from '../api/client'
import { excelChunkSize, readObject, sameExcelFile, type ExcelUploadRef, type ExcelUploadSession, type ExcelUploadSlot } from './excelPageSupport'

export type ExcelPageClientOptions = Omit<ApiRequestOptions, 'method'> & { method?: HTTPMethod; showResult?: boolean; silentLoading?: boolean }
export type ExcelPageClient = (path: string, options?: ExcelPageClientOptions) => Promise<ClientResponse>

function requestErrorMessage(response: ClientResponse, fallback: string) {
  return response.error?.message || fallback
}

export function buildExcelUploadPayload(uploadId: string, config: unknown) {
  const payload = new FormData()
  payload.append('uploadId', uploadId)
  payload.append('config', JSON.stringify(config))
  return payload
}

export function useExcelUploads(client: ExcelPageClient) {
  const [uploadRefs, setUploadRefs] = useState<Partial<Record<ExcelUploadSlot, ExcelUploadRef>>>({})
  const [uploadProgress, setUploadProgress] = useState('')

  function clearUploadRef(slot: ExcelUploadSlot) {
    setUploadRefs((current) => ({ ...current, [slot]: undefined }))
    setUploadProgress('')
  }

  const resetUploads = useCallback(() => {
    setUploadRefs({})
    setUploadProgress('')
  }, [])

  async function ensureExcelUpload(slot: ExcelUploadSlot, file: File) {
    const existing = uploadRefs[slot]
    if (existing && sameExcelFile(file, existing)) return existing.uploadId

    const totalChunks = Math.ceil(file.size / excelChunkSize)
    setUploadProgress(`准备上传 ${file.name}，共 ${totalChunks} 个分片`)
    const createResult = await client('/v1/excel-match-jobs/uploads', {
      method: 'POST',
      body: { fileName: file.name, totalChunks },
      showResult: false,
      silentLoading: true,
      retry: false,
    })
    if (!createResult.ok) throw new Error(requestErrorMessage(createResult, '创建分片上传会话失败'))
    const session = readObject<ExcelUploadSession>(createResult, 'upload')
    if (!session?.uploadId) throw new Error('上传会话返回缺少 uploadId')

    for (let index = 0; index < totalChunks; index++) {
      const start = index * excelChunkSize
      const end = Math.min(file.size, start + excelChunkSize)
      const chunkForm = new FormData()
      chunkForm.append('index', String(index))
      chunkForm.append('totalChunks', String(totalChunks))
      chunkForm.append('chunk', file.slice(start, end), `${file.name}.part${index}`)
      setUploadProgress(`上传分片 ${index + 1}/${totalChunks}`)
      const chunkResult = await client(`/v1/excel-match-jobs/uploads/${encodeURIComponent(session.uploadId)}/chunks`, {
        method: 'POST',
        body: chunkForm,
        showResult: false,
        silentLoading: true,
        retry: false,
        timeoutMs: 120_000,
      })
      if (!chunkResult.ok) throw new Error(requestErrorMessage(chunkResult, `上传分片 ${index + 1} 失败`))
    }

    setUploadProgress('合并 Excel 分片')
    const completeResult = await client(`/v1/excel-match-jobs/uploads/${encodeURIComponent(session.uploadId)}/complete`, {
      method: 'POST',
      body: { totalChunks },
      showResult: false,
      silentLoading: true,
      retry: false,
    })
    if (!completeResult.ok) throw new Error(requestErrorMessage(completeResult, '合并 Excel 分片失败'))

    const nextRef: ExcelUploadRef = {
      uploadId: session.uploadId,
      fileName: file.name,
      size: file.size,
      lastModified: file.lastModified,
      totalChunks,
    }
    setUploadRefs((current) => ({ ...current, [slot]: nextRef }))
    setUploadProgress(`上传完成：${file.name}`)
    return session.uploadId
  }

  return { uploadProgress, clearUploadRef, ensureExcelUpload, resetUploads }
}
