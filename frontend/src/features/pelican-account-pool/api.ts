import { apiClient } from '@/api/client'

export interface AccountPool {
  mode: 'automatic' | 'selected'
  account_ids: number[]
}

export interface PoolAccount {
  id: number
  name: string
  platform: string
  status: string
  schedulable: boolean
}

export interface AccountPoolView {
  pool: AccountPool
  accounts: PoolAccount[]
}

const endpoint = (groupId: number) => `/admin/pelican-showcase/groups/${groupId}/account-pool`

export async function getAccountPool(groupId: number): Promise<AccountPoolView> {
  const { data } = await apiClient.get<AccountPoolView>(endpoint(groupId))
  return data
}

export async function saveAccountPool(groupId: number, pool: AccountPool): Promise<AccountPoolView> {
  const { data } = await apiClient.put<AccountPoolView>(endpoint(groupId), pool)
  return data
}
