<script setup lang="ts">
import BenchmarkList from '@/components/BenchmarkList.vue'
import { useInterval } from '@/lib/useInterval'
import { useTeamBenches, useTeamEnvironment, useTeamInstances } from '@/lib/useServerData'

const { teamId } = defineProps<{ teamId: string }>()

const { data: benches, error: benchesError, refetch } = useTeamBenches(teamId)
const { data: instances } = useTeamInstances(teamId)
const { data: environment } = useTeamEnvironment(teamId)

useInterval(() => {
  const loadingStatuses = ['waiting', 'running', 'readying']
  if (benches.value?.some((b) => loadingStatuses.includes(b.status))) {
    void refetch()
  }
}, 500)
</script>

<template>
  <div class="team-benchmark-list-container">
    <div class="team-benchmark-list-actions">
      <BenchmarkRunner
        :teamId="teamId"
        :benches="benches ?? []"
        :environment="environment ?? null"
      />
      <SingleTeamScoreChart :benches="benches ?? []" />
      <div />
    </div>
    <ErrorMessage v-if="benchesError" :error="benchesError" />
    <BenchmarkList v-else :benches="benches ?? []" :instances="instances ?? []" />
  </div>
</template>

<style scoped>
.team-benchmark-list-container {
  container-type: inline-size;
}

.team-benchmark-list-actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1rem;
  margin-bottom: 1.5rem;
}

@container (max-width: 780px) {
  .team-benchmark-list-actions {
    grid-template-columns: 1fr;
  }
}
</style>
