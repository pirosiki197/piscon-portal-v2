<script setup lang="ts">
import { Icon } from '@iconify/vue'
import type { components } from '@/api/openapi'
import MainButton from '@/components/MainButton.vue'
import { useEnqueueBenchmark } from '@/lib/useServerData'
import { computed, ref, useId, watch } from 'vue'

const id = useId()

type Bench = components['schemas']['BenchmarkListItem']
type Environment = components['schemas']['Environment']
const props = defineProps<{
  teamId: string
  benches: Bench[]
  environment: Environment | null
}>()

const { mutate: enqueueRequest, isPending } = useEnqueueBenchmark()

// 問題が対象サーバーを固定しているときは、そのサーバーだけを選べる。
const fixedTargetId = computed(() => props.environment?.benchTargetInstanceId)
const instances = computed(() =>
  (props.environment?.instances ?? [])
    .filter((i) => i.status === 'running')
    .filter((i) => fixedTargetId.value === undefined || i.id === fixedTargetId.value)
    .map((i) => ({
      label: `${i.name} (${i.privateIPAddress})`,
      value: i.id,
    })),
)
const canEnqueue = computed(() => props.benches.every((b) => b.status === 'finished'))

const targetInstanceId = ref<string | null>(null)

watch(
  instances,
  (options) => {
    if (fixedTargetId.value !== undefined) {
      targetInstanceId.value = options[0]?.value ?? null
    } else if (!options.some((i) => i.value === targetInstanceId.value)) {
      targetInstanceId.value = null
    }
  },
  { immediate: true },
)

const enqueueBenchmark = (instanceId: string | null) => {
  if (
    !instances.value.some((i) => i.value === instanceId) ||
    isPending.value ||
    !canEnqueue.value
  ) {
    return
  }
  if (instanceId === null) return
  enqueueRequest({ teamId: props.teamId, instanceId })
}
</script>

<template>
  <div class="benchmark-runner-container">
    <div class="benchmark-runner-title">新しくベンチマークを実行する</div>
    <div class="benchmark-runner-content">
      <div v-if="fixedTargetId !== undefined" class="benchmark-runner-fixed-target">
        {{ instances[0]?.label ?? '対象サーバーが起動していません' }}
      </div>
      <div v-else>
        <Select
          v-model="targetInstanceId"
          :options="instances"
          option-label="label"
          option-value="value"
          :placeholder="'対象サーバーを選択'"
          :id="id"
          class="benchmark-runner-instance-selector"
        />
      </div>
      <MainButton
        :disabled="targetInstanceId === null || isPending || !canEnqueue"
        variant="primary"
        @click="enqueueBenchmark(targetInstanceId)"
        class="benchmark-runner-button"
      >
        <Icon icon="mdi:thunder" width="24" height="24" />
        <span>実行</span>
      </MainButton>
    </div>
    <div class="benchmark-runner-messages">
      <div v-if="!canEnqueue" class="benchmark-runner-no-available-slot-message">
        <Icon icon="mdi:alert" width="20" height="20" />
        <span>現在実行中のベンチマークがあるため、新しいベンチマークを実行できません。</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.benchmark-runner-container {
  padding: 1rem;
  border: 1px solid var(--ct-slate-300);
  border-radius: 4px;
}

.benchmark-runner-title {
  font-weight: 600;
  margin-bottom: 0.5rem;
}

.benchmark-runner-content {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 0.5rem;
}

@media screen and (max-width: 480px) {
  .benchmark-runner-content {
    grid-template-columns: 1fr;
  }
}

.benchmark-runner-instance-selector-label {
  display: block;
  font-size: 0.9rem;
  font-weight: 600;
  margin-bottom: 0.25rem;
}

.benchmark-runner-instance-selector {
  width: 100%;
}

.benchmark-runner-fixed-target {
  align-self: center;
  font-weight: 600;
}

.benchmark-runner-button {
  align-self: center;
}

.benchmark-runner-messages {
  margin-top: 0.5rem;
}

.benchmark-runner-no-available-slot-message {
  color: var(--ct-red-600);
  font-size: 0.9rem;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 0.25rem;
}
</style>
