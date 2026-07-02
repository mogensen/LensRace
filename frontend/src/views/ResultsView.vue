<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useGameStore } from '@/stores/game'
import type { Player } from '@/lib/api'
import { avatarEmoji, avatarColor } from '@/lib/avatar'

const props = defineProps<{ id: string }>()
const router = useRouter()
const store = useGameStore()
const { t } = useI18n()

interface Confetti {
  left: number
  delay: number
  dur: number
  color: string
  size: number
  rot: number
}
const confetti = ref<Confetti[]>([])

onMounted(async () => {
  const ok = await store.ensureSession(props.id)
  if (!ok) {
    await router.replace({ name: 'home' })
    return
  }

  const colors = ['#ffc02e', '#ff8a4a', '#4aa3ff', '#34b36a', '#9a6bff']
  confetti.value = Array.from({ length: 28 }, (_, i) => ({
    left: Math.random() * 100,
    delay: Math.random() * 0.8,
    dur: 1.8 + Math.random() * 1.6,
    color: colors[i % colors.length]!,
    size: 8 + Math.random() * 8,
    rot: Math.random() * 360,
  }))
})

const ranking = computed(() =>
  [...(store.state.gameState?.players ?? [])].sort((a, b) => b.score - a.score),
)
const totalItems = computed(() => store.state.gameState?.items.length ?? 0)
const medals = ['🥇', '🥈', '🥉']

const heights = [96, 74, 56]

const podium = computed(() => {
  const top = ranking.value.slice(0, 3)
  const desiredOrder =
    top.length >= 3 ? [top[1], top[0], top[2]] : top.length === 2 ? [top[1], top[0]] : top
  return desiredOrder
    .filter((p): p is Player => p !== undefined)
    .map((player) => {
      const place = ranking.value.indexOf(player)
      return { player, place, medal: medals[place], height: heights[place] }
    })
})

const youWon = computed(() => ranking.value[0]?.id === store.me?.id)
const allFound = computed(
  () => ranking.value.some((p) => p.score === totalItems.value) && totalItems.value > 0,
)
const endTitle = computed(() =>
  youWon.value
    ? t('results.wonTitle')
    : allFound.value
      ? t('results.allFoundTitle')
      : t('results.timesUpTitle'),
)
const endSub = computed(() => {
  const found = store.me?.capturedItemIds.length ?? 0
  return t('results.summary', {
    found,
    total: totalItems.value,
    score: store.me?.score ?? 0,
  })
})

async function onPlayAgain() {
  store.reset()
  await router.push({ name: 'home' })
}
</script>

<template>
  <main
    v-if="store.state.gameState"
    class="sh-app relative flex min-h-screen flex-col overflow-hidden p-6"
  >
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <span
        v-for="(cf, i) in confetti"
        :key="i"
        class="absolute -top-5 rounded-sm"
        :style="`left:${cf.left}%;width:${cf.size}px;height:${cf.size * 1.4}px;background:${cf.color};transform:rotate(${cf.rot}deg);animation:sh-confetti-fall ${cf.dur}s linear ${cf.delay}s infinite`"
      ></span>
    </div>

    <div class="z-[2] mt-1 mb-1 text-center">
      <div class="sh-title text-3xl" style="animation: sh-wiggle 1.2s ease-in-out infinite">
        {{ endTitle }}
      </div>
      <div class="text-base font-bold" style="color: var(--sh-muted)">{{ endSub }}</div>
    </div>

    <div class="z-[2] my-4 flex items-end justify-center gap-2">
      <div v-for="p in podium" :key="p.player.id" class="flex flex-col items-center gap-1">
        <span class="text-xs">{{ p.medal }}</span>
        <span
          class="flex h-[38px] w-[38px] items-center justify-center rounded-full border-[2.5px] text-xl"
          :style="`background:${avatarColor(p.player.id)};border-color:var(--sh-ink)`"
        >
          {{ avatarEmoji(p.player.id) }}
        </span>
        <span class="sh-title text-xs" style="color: var(--sh-ink)">{{ p.player.name }}</span>
        <div
          class="sh-title flex justify-center rounded-t-2xl border-[3px] pt-1.5 text-lg"
          :style="`width:62px;height:${p.height}px;border-color:var(--sh-ink);color:var(--sh-ink);background:${['#ffd45e', '#dfe6ee', '#f0c79a'][p.place]}`"
        >
          {{ p.player.score }}
        </div>
      </div>
    </div>

    <div class="z-[2] flex flex-1 flex-col gap-2 overflow-y-auto">
      <div
        v-for="(p, i) in ranking"
        :key="p.id"
        class="flex items-center gap-2.5 rounded-2xl border-[2.5px] px-3 py-2.5"
        :style="
          p.id === store.me?.id
            ? 'background: #fff1d9; border-color: var(--sh-ink); box-shadow: 3px 4px 0 var(--sh-ink)'
            : 'background: #fff; border-color: var(--sh-ink); box-shadow: 2px 3px 0 var(--sh-ink)'
        "
      >
        <span class="sh-title w-[26px] text-center text-lg">{{ i < 3 ? medals[i] : i + 1 }}</span>
        <span
          class="flex h-9 w-9 items-center justify-center rounded-full border-[2.5px] text-lg"
          :style="`background:${avatarColor(p.id)};border-color:var(--sh-ink)`"
        >
          {{ avatarEmoji(p.id) }}
        </span>
        <span class="sh-title flex-1 text-base">{{ p.name }}</span>
        <b class="sh-title text-lg">{{ p.score }}</b>
      </div>
    </div>

    <button class="sh-btn sh-btn-primary z-[2] mt-3.5 py-4 text-xl" @click="onPlayAgain">
      🔄 {{ t('results.playAgain') }}
    </button>
  </main>
  <main v-else class="sh-app flex min-h-screen items-center justify-center">
    <p class="font-bold" style="color: var(--sh-muted)">{{ t('common.loading') }}</p>
  </main>
</template>
