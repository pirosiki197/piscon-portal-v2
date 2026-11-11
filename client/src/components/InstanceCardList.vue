<script setup lang="ts">
import type { components } from '@/api/openapi'
import CopyToClipboardButton from '@/components/CopyToClipboardButton.vue'
import InstanceStatusChip from '@/components/InstanceStatusChip.vue'
import MainButton from '@/components/MainButton.vue'
import { useEnqueueBenchmark } from '@/lib/useServerData'
import { Icon } from '@iconify/vue'
import notfoundImage from '@/assets/not-found.png'

type Instance = components['schemas']['Instance']
const { teamId, instances, benchTargetInstanceId } = defineProps<{
  teamId: string
  instances: Instance[]
  // 問題が対象サーバーを固定しているときの、対象サーバーの ID
  benchTargetInstanceId?: string
}>()

const { mutate: enqueueBenchmark } = useEnqueueBenchmark({
  redirect: true,
})

const enqueueBenchmarkHandler = (instanceId: string) => {
  enqueueBenchmark({ teamId, instanceId })
}

const canBenchmark = (instance: Instance) =>
  instance.status === 'running' &&
  (benchTargetInstanceId === undefined || instance.id === benchTargetInstanceId)

const getMissingIPAddressMessage = (status: Instance['status'], kind: 'public' | 'private') => {
  if (status === 'deleting' || status === 'deleted') return '削除済みです'
  if (kind === 'public' && (status === 'stopped' || status === 'stopping')) {
    return '起動時に割り振られます'
  }
  if (status === 'building' || status === 'starting') return '割り当て待ちです'
  return '取得できません'
}
</script>

<template>
  <div class="instance-card-list-container">
    <div v-if="instances?.length === 0" class="no-instances">
      <img :src="notfoundImage" alt="" width="192" height="192" />
      <div>サーバーはまだありません</div>
    </div>
    <div class="instance-card-list">
      <div v-for="instance in instances" :key="instance.id" class="instance-card">
        <div class="instance-info">
          <div class="card-title">
            <Icon icon="mdi:server-network" width="24" height="24" />
            <div>{{ instance.name }}</div>
            <InstanceStatusChip :status="instance.status" />
          </div>
          <div class="info-elements">
            <div class="info-element">
              <div class="info-label">プライベートIPアドレス</div>
              <div class="info-value">
                <span>
                  {{
                    instance.privateIPAddress ||
                    getMissingIPAddressMessage(instance.status, 'private')
                  }}
                </span>
                <CopyToClipboardButton
                  v-if="instance.privateIPAddress"
                  :text="instance.privateIPAddress"
                />
              </div>
            </div>
            <div class="info-element">
              <div class="info-label">パブリックIPアドレス</div>
              <div class="info-value">
                <span>
                  {{
                    instance.publicIPAddress ||
                    getMissingIPAddressMessage(instance.status, 'public')
                  }}
                </span>
                <CopyToClipboardButton
                  v-if="instance.publicIPAddress"
                  :text="instance.publicIPAddress"
                />
              </div>
            </div>
          </div>
          <div class="info-actions">
            <MainButton
              @click="enqueueBenchmarkHandler(instance.id)"
              class="action-button"
              :disabled="!canBenchmark(instance)"
            >
              <Icon icon="mdi:thunder" width="20" height="20" />
              <span>ベンチマーク実行</span>
            </MainButton>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.instance-card-list-container {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.no-instances {
  padding: 1rem;
  border: 1px dashed var(--ct-slate-300);
  border-radius: 4px;
  text-align: center;
  color: var(--ct-slate-400);
  font-weight: 600;
}

.instance-card-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(600px, 100%), 1fr));
  gap: 1rem;
  container-type: inline-size;
}

.instance-card {
  padding: 1rem;
  border: 1px solid var(--ct-slate-300);
  border-radius: 4px;
  display: flex;
  gap: 2rem;
}

.instance-info {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  flex: 1;
}

.card-title {
  font-weight: 600;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.info-elements {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 0.5rem;
}

.info-label {
  font-size: 0.8rem;
  font-weight: 600;
}

.info-value {
  display: flex;
  align-items: center;
  gap: 0.25rem;
}

.info-actions {
  display: flex;
}

.action-button {
  flex: 1;
}

@container (max-width: 600px) {
  .info-elements {
    grid-template-columns: 1fr;
  }
}
</style>
