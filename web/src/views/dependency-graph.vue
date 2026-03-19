<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useI18n } from '../utils/i18n'

const router = useRouter()
const { t } = useI18n()
</script>

<template>
  <div class="dependency-fallback">
    <el-empty :description="t.services.dependencyHint">
      <el-button type="primary" @click="router.push('/services')">{{ t.nav.services }}</el-button>
    </el-empty>
  </div>
</template>

<style scoped>
.dependency-fallback {
  min-height: 60vh;
  display: grid;
  place-items: center;
}
</style>
