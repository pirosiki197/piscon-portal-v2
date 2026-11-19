<script setup lang="ts">
import PageTitle from '@/components/PageTitle.vue'
import TeamEnvironmentManagement from '@/components/TeamEnvironmentManagement.vue'
import { useTeamsData } from '@/lib/useServerData'
import { useUsers } from '@/lib/useUsers'
import { ref } from 'vue'
import { Icon } from '@iconify/vue'

const { data: teams } = useTeamsData()
const { getUserById } = useUsers()

// true -> closed, false -> open
const collapseStates = ref<Record<string, boolean>>({})
</script>

<template>
  <main class="admin-instances-container">
    <PageTitle icon="mdi:database-cog">チーム環境 (管理者)</PageTitle>

    <div class="team-instances">
      <div v-for="team in teams" :key="team.id" class="team-instance">
        <h2 class="team-instance-title" :id="team.id">
          <div class="team-name">{{ team.name }}</div>
          <div class="team-members">
            <UserChip
              v-for="member in team.members"
              :key="member"
              :name="getUserById(member)?.name ?? ''"
              :icon-url="getUserById(member)?.iconUrl"
            />
          </div>
          <button
            class="team-accordion-button"
            :class="{ closed: collapseStates[team.id] }"
            @click="collapseStates[team.id] = !collapseStates[team.id]"
          >
            <Icon icon="mdi:chevron-down" />
          </button>
        </h2>
        <div class="instance-card-list-wrapper" :class="{ closed: collapseStates[team.id] }">
          <TeamEnvironmentManagement :teamId="team.id" />
        </div>
      </div>
    </div>
  </main>
</template>

<style scoped>
.admin-instances-container {
  padding: 1rem;
  display: flex;
  flex-direction: column;
  gap: 2rem;
  container-type: inline-size;
}

.team-instances {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}

.team-instance {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  border: 1px solid var(--ct-slate-300);
  border-radius: 4px;
  padding: 1rem;
}

.team-instance-title {
  font-size: 1.5rem;
  font-weight: bold;
  display: grid;
  grid-template-areas: 'team-name team-members team-accordion-button';
  grid-template-columns: auto 1fr auto;
  gap: 0.5rem;
}

@container (max-width: 480px) {
  .team-instance-title {
    grid-template-areas:
      'team-name team-accordion-button'
      'team-members team-members';
    grid-template-columns: 1fr auto;
  }
}

.team-name {
  grid-area: team-name;
}

.team-members {
  grid-area: team-members;
  display: flex;
  gap: 0.5rem;
}

.team-accordion-button {
  grid-area: team-accordion-button;
  margin-left: auto;
  background: none;
  border: none;
  cursor: pointer;
  color: var(--ct-slate-800);
  display: grid;
  place-content: center;
  border-radius: 4px;
  width: 2rem;
  height: 2rem;
  transition: background-color 0.1s;
  font-size: 1.5rem;
}
.team-accordion-button:hover {
  background-color: var(--ct-slate-100);
}
.team-accordion-button:active {
  background-color: var(--ct-slate-200);
}

.team-accordion-button > svg {
  transition: transform 0.2s;
  transform: rotate(0);
}
.team-accordion-button.closed > svg {
  transform: rotate(-90deg);
}

.instance-card-list-wrapper.closed {
  display: none;
}
</style>
