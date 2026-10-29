<template>
  <button type="button" class="inline-flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-xs text-gray-600 transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50 dark:text-gray-300 dark:hover:bg-dark-800" :disabled="disabled" data-testid="pelican-account-pool-open" @click="open">
    {{ t('pelicanAccountPool.button') }}
  </button>
  <BaseDialog :show="show" :title="t('pelicanAccountPool.title', { group: groupName })" width="wide" :show-close-button="!saving" :close-on-escape="!saving" @close="close">
    <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">{{ t('pelicanAccountPool.scope') }}</p>
    <p v-if="loading" role="status">{{ t('pelicanAccountPool.loading') }}</p>
    <div v-else-if="loadError" role="alert" class="space-y-3 text-red-600">
      <p>{{ t('pelicanAccountPool.loadError') }}</p>
      <button type="button" class="btn btn-secondary" data-testid="pool-reload" @click="load">{{ t('pelicanAccountPool.retry') }}</button>
    </div>
    <div v-else-if="draft" class="space-y-4">
      <fieldset :disabled="saving" class="space-y-4">
        <div class="flex flex-wrap gap-4">
          <label class="flex items-center gap-2"><input v-model="draft.mode" type="radio" value="automatic" data-testid="pool-automatic" />{{ t('pelicanAccountPool.automatic') }}</label>
          <label class="flex items-center gap-2"><input v-model="draft.mode" type="radio" value="selected" data-testid="pool-selected" />{{ t('pelicanAccountPool.selected') }}</label>
        </div>
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t(draft.mode === 'selected' ? 'pelicanAccountPool.selectedHint' : 'pelicanAccountPool.automaticHint') }}</p>
        <template v-if="draft.mode === 'selected'">
          <input v-model="search" type="search" class="input w-full" :placeholder="t('pelicanAccountPool.search')" :aria-label="t('pelicanAccountPool.search')" data-testid="pool-search" />
          <p class="text-sm text-gray-500" data-testid="pool-count">{{ t('pelicanAccountPool.count', { count: draft.account_ids.length }) }}</p>
          <div class="max-h-72 space-y-2 overflow-y-auto rounded-xl border border-gray-200 p-3 dark:border-dark-600">
            <label v-for="id in missingIds" :key="`missing-${id}`" class="flex items-start gap-2 text-sm text-amber-600 dark:text-amber-400">
              <input v-model="draft.account_ids" type="checkbox" :value="id" :data-testid="`pool-account-${id}`" class="mt-1" />
              <span>{{ t('pelicanAccountPool.missing', { id }) }}</span>
            </label>
            <label v-for="account in filteredAccounts" :key="account.id" class="flex items-start gap-2 rounded-lg p-2 hover:bg-gray-50 dark:hover:bg-dark-700">
              <input v-model="draft.account_ids" type="checkbox" :value="account.id" :disabled="draft.account_ids.length >= 100 && !draft.account_ids.includes(account.id)" :data-testid="`pool-account-${account.id}`" class="mt-1" />
              <span class="min-w-0 break-words text-sm">
                {{ account.name }} <span class="text-gray-500">#{{ account.id }} · {{ account.platform }}</span>
                <small v-if="account.status !== 'active' || !account.schedulable" class="block text-amber-600 dark:text-amber-400">{{ t('pelicanAccountPool.unavailable') }}</small>
              </span>
            </label>
            <p v-if="!filteredAccounts.length && !missingIds.length" class="p-2 text-sm text-gray-500">{{ t('pelicanAccountPool.empty') }}</p>
          </div>
          <p v-if="!valid" class="text-sm text-amber-600 dark:text-amber-400">{{ t('pelicanAccountPool.required') }}</p>
        </template>
      </fieldset>
      <p v-if="saveError" role="alert" class="text-sm text-red-600">{{ t('pelicanAccountPool.saveError') }}</p>
      <p v-if="saved" role="status" class="text-sm text-green-600">{{ t('pelicanAccountPool.saved') }}</p>
    </div>
    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="saving" @click="close">{{ t('pelicanAccountPool.close') }}</button>
      <button type="button" class="btn btn-primary" :disabled="loading || loadError || saving || !valid" data-testid="pool-save" @click="save">{{ t(saving ? 'pelicanAccountPool.saving' : 'pelicanAccountPool.save') }}</button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { getAccountPool, saveAccountPool, type AccountPool, type PoolAccount } from './api'

const props = defineProps<{ groupId: number; groupName: string; disabled?: boolean }>()
const { t } = useI18n()
const show = ref(false)
const loading = ref(false)
const saving = ref(false)
const loadError = ref(false)
const saveError = ref(false)
const saved = ref(false)
const search = ref('')
const draft = ref<AccountPool | null>(null)
const accounts = ref<PoolAccount[]>([])
let requestId = 0

const missingIds = computed(() => draft.value?.account_ids.filter(id => !accounts.value.some(account => account.id === id)) ?? [])
const valid = computed(() => !!draft.value && (draft.value.mode === 'automatic' || (
  draft.value.account_ids.length > 0 && draft.value.account_ids.length <= 100 && !missingIds.value.length
)))
const filteredAccounts = computed(() => {
  const query = search.value.trim().toLocaleLowerCase()
  return accounts.value.filter(account => `${account.name} ${account.id}`.toLocaleLowerCase().includes(query))
})
watch(draft, () => { saved.value = false; saveError.value = false }, { deep: true })

function open() {
  show.value = true
  search.value = ''
  void load()
}

function close() {
  if (saving.value) return
  show.value = false
  requestId++
}

async function load() {
  const currentRequest = ++requestId
  loading.value = true
  loadError.value = false
  saved.value = false
  saveError.value = false
  draft.value = null
  try {
    const view = await getAccountPool(props.groupId)
    if (currentRequest !== requestId) return
    draft.value = { ...view.pool, account_ids: [...view.pool.account_ids] }
    accounts.value = view.accounts
  } catch {
    if (currentRequest === requestId) loadError.value = true
  } finally {
    if (currentRequest === requestId) loading.value = false
  }
}

async function save() {
  if (!draft.value || !valid.value || saving.value) return
  saving.value = true
  saveError.value = false
  saved.value = false
  try {
    await saveAccountPool(props.groupId, {
      mode: draft.value.mode,
      account_ids: draft.value.mode === 'selected' ? [...draft.value.account_ids] : [],
    })
    saved.value = true
  } catch {
    saveError.value = true
  } finally {
    saving.value = false
  }
}
</script>
