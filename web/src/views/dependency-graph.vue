<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()

onMounted(() => {
  router.replace({ path: '/services', query: { view: 'topology' } })
})
</script>

<template>
  <div class="dependency-redirect"></div>
</template>
