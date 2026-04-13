import { billingApi } from '@/api/billing'
import { analyticsApi } from '@/api/analytics'

function triggerDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

export function useExport() {
  async function downloadInvoicePDF(invoiceId: string): Promise<void> {
    const { data } = await billingApi.downloadInvoicePDF(invoiceId)
    triggerDownload(data as Blob, `invoice-${invoiceId}.pdf`)
  }

  async function downloadAnalyticsExcel(clinicId: string, period: string): Promise<void> {
    const { data } = await analyticsApi.exportExcel({ clinic_id: clinicId, period })
    triggerDownload(data as Blob, `analytics-${period}.xlsx`)
  }

  return { downloadInvoicePDF, downloadAnalyticsExcel }
}
