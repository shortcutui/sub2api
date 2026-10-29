import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import PelicanAccountPoolButton from '../PelicanAccountPoolButton.vue'
import { getAccountPool, saveAccountPool, type AccountPoolView } from '../api'
import { messages } from '../messages'

vi.mock('../api', () => ({ getAccountPool: vi.fn(), saveAccountPool: vi.fn() }))

const view = (): AccountPoolView => ({
  pool: { mode: 'automatic', account_ids: [] },
  accounts: [
    { id: 11, name: '精选 A', platform: 'openai', status: 'active', schedulable: true },
    { id: 12, name: '精选 B', platform: 'openai', status: 'disabled', schedulable: false },
  ],
})
const createWrapper = () => mount(PelicanAccountPoolButton, {
  props: { groupId: 4, groupName: '测试分组' },
  global: {
    plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh: { pelicanAccountPool: messages.zh } } })],
    stubs: { BaseDialog: { props: ['show', 'title'], template: '<div v-if="show"><h3>{{ title }}</h3><slot /><slot name="footer" /></div>' } },
  },
})

beforeEach(() => {
  vi.mocked(getAccountPool).mockReset().mockResolvedValue(view())
  vi.mocked(saveAccountPool).mockReset().mockResolvedValue(view())
})

describe('展示账号池', () => {
  it('打开时读取分组配置，指定模式禁止空名单，搜索后仍保留全部选择', async () => {
    const wrapper = createWrapper()
    expect(getAccountPool).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="pelican-account-pool-open"]').trigger('click')
    await flushPromises()
    expect(getAccountPool).toHaveBeenCalledWith(4)
    await wrapper.get('[data-testid="pool-selected"]').setValue()
    expect(wrapper.get('[data-testid="pool-save"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="pool-account-11"]').setValue(true)
    await wrapper.get('[data-testid="pool-search"]').setValue('精选 B')
    await wrapper.get('[data-testid="pool-account-12"]').setValue(true)
    await wrapper.get('[data-testid="pool-save"]').trigger('click')
    await flushPromises()
    expect(saveAccountPool).toHaveBeenCalledWith(4, { mode: 'selected', account_ids: [11, 12] })
    expect(wrapper.text()).toContain('从下一轮测试开始生效')
    wrapper.unmount()
  })

  it('显示已移组的选择，要求移除后才能保存指定模式', async () => {
    vi.mocked(getAccountPool).mockResolvedValue({ ...view(), pool: { mode: 'selected', account_ids: [11, 99] } })
    const wrapper = createWrapper()
    await wrapper.get('[data-testid="pelican-account-pool-open"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('账号 #99 已移出分组或删除')
    expect(wrapper.get('[data-testid="pool-save"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="pool-account-99"]').setValue(false)
    await wrapper.get('[data-testid="pool-save"]').trigger('click')
    await flushPromises()
    expect(saveAccountPool).toHaveBeenCalledWith(4, { mode: 'selected', account_ids: [11] })
    wrapper.unmount()
  })

  it('切回自动模式显式清空名单', async () => {
    vi.mocked(getAccountPool).mockResolvedValue({ ...view(), pool: { mode: 'selected', account_ids: [11] } })
    const wrapper = createWrapper()
    await wrapper.get('[data-testid="pelican-account-pool-open"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="pool-automatic"]').setValue()
    await wrapper.get('[data-testid="pool-save"]').trigger('click')
    await flushPromises()
    expect(saveAccountPool).toHaveBeenCalledWith(4, { mode: 'automatic', account_ids: [] })
    wrapper.unmount()
  })

  it('加载失败时禁止保存，重试成功后可编辑', async () => {
    vi.mocked(getAccountPool).mockRejectedValueOnce(new Error('offline'))
    const wrapper = createWrapper()
    await wrapper.get('[data-testid="pelican-account-pool-open"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="pool-save"]').attributes('disabled')).toBeDefined()
    expect(saveAccountPool).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="pool-reload"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="pool-automatic"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('同组不同计划重新打开时读取最新共享配置，保存失败保留选择', async () => {
    const wrapper = createWrapper()
    await wrapper.get('[data-testid="pelican-account-pool-open"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="pool-selected"]').setValue()
    await wrapper.get('[data-testid="pool-account-11"]').setValue(true)
    vi.mocked(saveAccountPool).mockRejectedValueOnce(new Error('membership changed'))
    await wrapper.get('[data-testid="pool-save"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('保存失败')
    expect((wrapper.get('[data-testid="pool-account-11"]').element as HTMLInputElement).checked).toBe(true)
    const other = createWrapper()
    vi.mocked(getAccountPool).mockResolvedValue({ ...view(), pool: { mode: 'selected', account_ids: [12] } })
    await other.get('[data-testid="pelican-account-pool-open"]').trigger('click')
    await flushPromises()
    expect((other.get('[data-testid="pool-account-12"]').element as HTMLInputElement).checked).toBe(true)
    wrapper.unmount()
    other.unmount()
  })
})
