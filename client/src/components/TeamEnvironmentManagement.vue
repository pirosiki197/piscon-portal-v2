<script setup lang="ts">
import type { components } from '@/api/openapi'
import InstanceCardList from '@/components/InstanceCardList.vue'
import MainButton from '@/components/MainButton.vue'
import { useInterval } from '@/lib/useInterval'
import {
  useCreateTeamEnvironment,
  useDeleteTeamEnvironment,
  useStartTeamEnvironment,
  useStopTeamEnvironment,
  useTeamEnvironment,
} from '@/lib/useServerData'
import { Icon } from '@iconify/vue'
import { computed } from 'vue'

type EnvironmentStatus = components['schemas']['EnvironmentStatus']

const { teamId } = defineProps<{ teamId: string }>()

const { data: environment, refetch } = useTeamEnvironment(teamId)
const create = useCreateTeamEnvironment()
const remove = useDeleteTeamEnvironment()
const start = useStartTeamEnvironment()
const stop = useStopTeamEnvironment()

const statusLabels: Record<EnvironmentStatus, string> = {
  creating: '作成中',
  ready: '作成済み',
  failed: '作成に失敗しました',
  deleting: '削除中',
  deleteFailed: '削除に失敗しました',
}

const instances = computed(() => environment.value?.instances ?? [])
const isReady = computed(() => environment.value?.status === 'ready')
// 作成中のまま止まったチーム環境も作り直せるように、作成中でも削除できる。
const canDelete = computed(() => !!environment.value && environment.value.status !== 'deleting')
const mutationError = computed(
  () => create.error.value ?? remove.error.value ?? start.error.value ?? stop.error.value,
)

const deleteEnvironment = () => {
  if (!window.confirm('チーム環境のサーバーをすべて削除します。よろしいですか？')) return
  remove.mutate({ teamId })
}

useInterval(() => {
  const loadingInstanceStatuses = ['building', 'starting', 'stopping', 'deleting']
  if (
    environment.value?.status === 'creating' ||
    environment.value?.status === 'deleting' ||
    instances.value.some((i) => loadingInstanceStatuses.includes(i.status))
  ) {
    void refetch()
  }
}, 1000)
</script>

<template>
  <div class="team-environment-container">
    <div v-if="environment === null" class="environment-panel">
      <div>チーム環境はまだありません</div>
      <MainButton :loading="create.isPending.value" @click="create.mutate({ teamId })">
        <Icon icon="mdi:tools" width="20" height="20" />
        <span>チーム環境を作成</span>
      </MainButton>
    </div>
    <div v-else-if="environment !== undefined" class="environment-panel">
      <div class="environment-status">
        <Icon icon="mdi:server-network" width="24" height="24" />
        <span>チーム環境: {{ statusLabels[environment.status] }}</span>
        <Icon
          v-if="environment.status === 'creating' || environment.status === 'deleting'"
          icon="mdi:loading"
          width="20"
          height="20"
          class="loading-spinner"
        />
      </div>
      <div v-if="environment.failureReason" class="environment-failure">
        {{ environment.failureReason }}
      </div>
      <div class="environment-actions">
        <MainButton
          :disabled="!isReady || !instances.some((i) => i.status === 'stopped')"
          :loading="start.isPending.value"
          @click="start.mutate({ teamId })"
        >
          <Icon icon="mdi:play-circle" width="20" height="20" />
          <span>すべて起動</span>
        </MainButton>
        <MainButton
          :disabled="!isReady || !instances.some((i) => i.status === 'running')"
          :loading="stop.isPending.value"
          @click="stop.mutate({ teamId })"
        >
          <Icon icon="mdi:stop-pause" width="20" height="20" />
          <span>すべて停止</span>
        </MainButton>
        <MainButton
          variant="destructive"
          :disabled="!canDelete"
          :loading="remove.isPending.value"
          @click="deleteEnvironment"
        >
          <Icon icon="mdi:trash-can" width="20" height="20" />
          <span>チーム環境を削除</span>
        </MainButton>
      </div>
    </div>
    <div v-if="mutationError" class="environment-failure">
      {{ mutationError.message }}
    </div>
    <InstanceCardList
      v-if="environment"
      :teamId="teamId"
      :instances="instances"
      :benchTargetInstanceId="environment.benchTargetInstanceId"
    />
  </div>
</template>

<style scoped>
.team-environment-container {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.environment-panel {
  padding: 1rem;
  border: 1px solid var(--ct-slate-300);
  border-radius: 4px;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.environment-status {
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.environment-failure {
  color: var(--ct-red-600);
  font-size: 0.9rem;
  font-weight: 600;
}

.environment-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.loading-spinner {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}
</style>
