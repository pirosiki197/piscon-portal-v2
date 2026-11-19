<script setup lang="ts">
import { apiBaseUrl } from '@/api'
import MainLayout from '@/layouts/MainLayout.vue'
import { useAuthProviders, useMe } from '@/lib/useServerData'
import { computed, effect } from 'vue'
import { RouterView, useRouter } from 'vue-router'

const { data: me, isLoading } = useMe()
const { data: authProviders } = useAuthProviders()
const router = useRouter()

const SESSION_STORAGE_KEY_REDIRECT = 'redirect'

const status = computed(() => {
  if (isLoading.value) return 'loading'
  return me.value ? 'authenticated' : 'unauthenticated'
})

const loginHandler = (providerId: string) => {
  sessionStorage.setItem(SESSION_STORAGE_KEY_REDIRECT, window.location.pathname)
  window.location.href = `${apiBaseUrl}/auth/${encodeURIComponent(providerId)}/login`
}

effect(() => {
  const redirect = sessionStorage.getItem(SESSION_STORAGE_KEY_REDIRECT)
  if (redirect && status.value === 'authenticated') {
    sessionStorage.removeItem(SESSION_STORAGE_KEY_REDIRECT)
    if (!redirect.startsWith('/')) return
    void router.push(redirect)
  }
})
</script>

<template>
  <MainLayout>
    <div v-if="status === 'loading'" class="loading-message">
      <LoaderAnimation />
      <div>読み込み中...</div>
    </div>
    <div v-if="status === 'unauthenticated'" class="unauthenticated-message">
      <div class="portal-title">PISCON Portal v2</div>
      <div class="login-buttons">
        <MainButton
          v-for="provider in authProviders"
          :key="provider.id"
          @click="loginHandler(provider.id)"
        >
          {{ provider.name }}でログイン
        </MainButton>
      </div>
      <div class="caption-message">利用するにはログインが必要です</div>
    </div>
    <RouterView v-if="status === 'authenticated'" :key="$route.fullPath" />
  </MainLayout>
</template>

<style scoped>
.loading-message {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: calc(100vh - 11rem);
  gap: 1rem;
}
.unauthenticated-message {
  display: grid;
  place-content: center;
  height: calc(100vh - 11rem);
  gap: 1rem;
}

.login-buttons {
  display: grid;
  gap: 0.5rem;
}

.portal-title {
  font-size: 2rem;
  font-weight: bold;
}

.caption-message {
  color: var(--ct-slate-500);
  font-size: 0.875rem;
  text-align: center;
  margin-top: -0.5rem;
}
</style>
