<script setup lang="ts">
import { Icon } from '@iconify/vue'

defineProps<{ iconUrl?: string }>()
</script>

<template>
  <img v-if="iconUrl" :src="iconUrl" alt="" width="24" height="24" />
  <Icon v-else icon="mdi:account-circle" width="24" height="24" />
</template>
