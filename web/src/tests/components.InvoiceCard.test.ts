import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import InvoiceCard from '@/components/billing/InvoiceCard.vue'
import type { Invoice } from '@/types/billing'

const i18n = createI18n({ legacy: false, locale: 'ru', messages: { ru: { invoice_status: { draft: 'Черновик', sent: 'Отправлен', paid: 'Оплачен', overdue: 'Просрочен', voided: 'Аннулирован' } } } })

const invoice: Invoice = {
  id: '550e8400-e29b-41d4-a716-446655440000',
  clinic_id: 'c1',
  patient_id: 'p1',
  service_name: 'Pro план — май 2025',
  amount: '49900',
  currency: 'KZT',
  status: 'sent',
  due_at: '2025-06-01T00:00:00Z',
  paid_at: undefined,
  created_at: '2025-05-01T00:00:00Z',
}

describe('InvoiceCard', () => {
  it('renders service name', () => {
    const wrapper = mount(InvoiceCard, { props: { invoice }, global: { plugins: [i18n] } })
    expect(wrapper.text()).toContain('Pro план — май 2025')
  })

  it('renders formatted amount', () => {
    const wrapper = mount(InvoiceCard, { props: { invoice }, global: { plugins: [i18n] } })
    // formatKZT returns something like "49 900 ₸"
    expect(wrapper.text()).toContain('49')
    expect(wrapper.text()).toContain('900')
  })

  it('emits download event when PDF button clicked', async () => {
    const wrapper = mount(InvoiceCard, { props: { invoice }, global: { plugins: [i18n] } })
    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('download')).toBeTruthy()
    expect(wrapper.emitted('download')![0]).toEqual([invoice.id])
  })
})
