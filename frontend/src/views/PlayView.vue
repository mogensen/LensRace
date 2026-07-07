<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, defineAsyncComponent } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useGameStore } from '@/stores/game'
import type { Item } from '@/lib/api'
import { itemEmoji } from '@/lib/itemIcons'
import { itemName } from '@/lib/catalogNames'
import { avatarEmoji, avatarColor } from '@/lib/avatar'
import { preloadDetectors } from '@/lib/detector'

// Lazy-loaded: this pulls in TensorFlow.js + COCO-SSD, which is hundreds of
// KB and shouldn't block loading the rest of the Play view for players who
// haven't tapped SNAP yet.
const CameraOverlay = defineAsyncComponent(() => import('@/components/CameraOverlay.vue'))

const props = defineProps<{ id: string }>()
const router = useRouter()
const store = useGameStore()
const { t, te } = useI18n()

const activeItem = ref<Item | null>(null)
const error = ref('')
const now = ref(Date.now())
let tickId: ReturnType<typeof setInterval> | undefined

onMounted(async () => {
  // Redundant with the Home/Lobby calls in the normal flow (loading is
  // cached, so this is a no-op once they've already started) — but a
  // player who refreshes mid-round lands here without ever having passed
  // through either of those, so this is the last safety net before
  // CameraOverlay's own on-demand load.
  preloadDetectors()

  const ok = await store.ensureSession(props.id)
  if (!ok) {
    await router.replace({ name: 'home' })
    return
  }
  const status = store.state.gameState?.game.status
  if (status === 'waiting') {
    await router.replace({ name: 'lobby', params: { id: props.id } })
    return
  }
  if (status === 'finished') {
    await router.replace({ name: 'results', params: { id: props.id } })
    return
  }

  tickId = setInterval(() => {
    now.value = Date.now()
  }, 1000)
})

onUnmounted(() => {
  if (tickId) clearInterval(tickId)
})

watch(
  () => store.state.gameState?.game.status,
  (status) => {
    if (status === 'finished') router.push({ name: 'results', params: { id: props.id } })
  },
)

const timeLeft = computed(() => {
  const game = store.state.gameState?.game
  if (!game?.startedAt) return game?.durationSeconds ?? 0
  const started = new Date(game.startedAt).getTime()
  const elapsed = Math.floor((now.value - started) / 1000)
  return Math.max(0, game.durationSeconds - elapsed)
})

function fmt(seconds: number): string {
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return `${m}:${String(s).padStart(2, '0')}`
}

function isFound(itemId: string): boolean {
  return store.me?.capturedItemIds.includes(itemId) ?? false
}

const foundCount = computed(() => store.me?.capturedItemIds.length ?? 0)
const totalItems = computed(() => store.state.gameState?.items.length ?? 0)
const progressPct = computed(() =>
  totalItems.value ? Math.round((foundCount.value / totalItems.value) * 100) : 0,
)
const low = computed(() => timeLeft.value <= 30)

const otherPlayers = computed(() =>
  (store.state.gameState?.players ?? []).filter((p) => p.id !== store.me?.id),
)

function openCamera(item: Item) {
  if (isFound(item.id)) return
  error.value = ''
  activeItem.value = item
}

function onSnap() {
  const next = store.state.gameState?.items.find((i) => !isFound(i.id))
  if (next) openCamera(next)
}

function onCaptured() {
  activeItem.value = null
}

function onCaptureFailed(message: string) {
  activeItem.value = null
  error.value = message
}
</script>

<template>
  <main v-if="store.state.gameState" class="sh-app relative flex min-h-screen flex-col">
    <div class="flex flex-none flex-col gap-2.5 px-5 pt-3 pb-3">
      <div class="flex items-center gap-2.5">
        <div
          class="sh-pill px-3.5 py-1.5 text-lg"
          :style="
            low
              ? 'background: var(--sh-orange); color: #fff; animation: sh-wiggle .5s ease-in-out infinite'
              : 'background: #fff; color: var(--sh-ink)'
          "
        >
          ⏱ {{ fmt(timeLeft) }}
        </div>
        <div class="flex-1"></div>
        <div
          class="flex items-center gap-1.5 rounded-full border-[2.5px] py-1 pr-3 pl-1.5"
          style="background: #fff; border-color: var(--sh-ink); box-shadow: 2px 3px 0 var(--sh-ink)"
        >
          <span
            class="flex h-[26px] w-[26px] items-center justify-center rounded-full border-2 text-sm"
            :style="`background:${avatarColor(store.me?.id ?? '')};border-color:var(--sh-ink)`"
          >
            {{ avatarEmoji(store.me?.id ?? '') }}
          </span>
          <b class="sh-title text-base"
            >{{ store.me?.score ?? 0
            }}<span class="text-xs font-bold" style="color: var(--sh-muted)">
              {{ t('play.points') }}</span
            ></b
          >
        </div>
      </div>

      <div>
        <div class="mb-1 flex items-baseline justify-between">
          <span class="sh-title text-lg">{{ t('play.findThese', { count: totalItems }) }}</span>
          <span class="text-sm font-extrabold" style="color: var(--sh-green)">{{
            t('play.foundCount', { found: foundCount, total: totalItems })
          }}</span>
        </div>
        <div
          class="h-3 overflow-hidden rounded-lg border-2"
          style="background: #f0e2cf; border-color: var(--sh-ink)"
        >
          <div
            class="h-full rounded-lg transition-all duration-500"
            :style="`width:${progressPct}%;background:var(--sh-green)`"
          ></div>
        </div>
      </div>

      <div
        v-if="otherPlayers.length"
        class="flex items-center gap-2 rounded-2xl border border-dashed px-2.5 py-1.5"
        style="background: #fff4e6; border-color: #e8c79a"
      >
        <span class="text-xs font-extrabold" style="color: #c79a5e">{{ t('play.live') }}</span>
        <span
          v-for="p in otherPlayers"
          :key="p.id"
          class="flex items-center gap-1 text-sm font-extrabold"
        >
          <span class="text-sm">{{ avatarEmoji(p.id) }}</span
          >{{ p.score }}
        </span>
      </div>
    </div>

    <div class="flex flex-1 flex-col gap-2.5 overflow-y-auto px-5 pb-24">
      <button
        v-for="it in store.state.gameState.items"
        :key="it.id"
        class="flex w-full items-center gap-3 rounded-2xl border-[2.5px] py-2.5 pr-3 pl-2.5 text-left"
        :disabled="isFound(it.id)"
        :style="
          isFound(it.id)
            ? 'background: #f3f7ee; border-color: var(--sh-ink); opacity: .92'
            : 'background: #fff; border-color: var(--sh-ink); box-shadow: 3px 4px 0 var(--sh-ink)'
        "
        @click="openCamera(it)"
      >
        <span
          class="flex h-11 w-11 flex-none items-center justify-center rounded-[13px] border-[2.5px] text-xl"
          :style="
            isFound(it.id)
              ? 'background: var(--sh-green); color: #fff; border-color: var(--sh-ink)'
              : 'background: #fdeebb; border-color: var(--sh-ink)'
          "
        >
          {{ isFound(it.id) ? '✓' : itemEmoji(it.label) }}
        </span>
        <span
          class="sh-title flex-1 text-base"
          :style="
            isFound(it.id)
              ? 'color: #c2b39f; text-decoration: line-through'
              : 'color: var(--sh-ink)'
          "
        >
          {{ itemName(t, te, it.id, it.displayName) }}
        </span>
        <span
          v-if="isFound(it.id)"
          class="sh-title rounded-xl border-2 px-2 py-0.5 text-xs"
          style="background: var(--sh-yellow); border-color: var(--sh-ink); color: var(--sh-ink)"
        >
          +1
        </span>
        <span v-else class="text-2xl font-extrabold" style="color: #cdbba4">›</span>
      </button>
    </div>

    <button
      class="absolute bottom-5 left-1/2 flex h-[78px] w-[78px] -translate-x-1/2 flex-col items-center justify-center gap-0.5 rounded-full text-white"
      style="
        background: var(--sh-orange);
        border: 3.5px solid var(--sh-ink);
        box-shadow:
          0 8px 0 var(--sh-ink),
          0 14px 22px -6px rgba(42, 35, 27, 0.5);
      "
      :disabled="foundCount >= totalItems"
      @click="onSnap"
    >
      <span class="text-2xl leading-none">📷</span>
      <span class="sh-title text-xs tracking-wide">SNAP</span>
    </button>

    <p
      v-if="error"
      class="absolute bottom-28 left-1/2 -translate-x-1/2 rounded-xl border-2 px-3 py-1.5 text-sm font-bold"
      style="background: #fff; border-color: var(--sh-orange); color: var(--sh-orange)"
    >
      {{ error }}
    </p>

    <CameraOverlay
      v-if="activeItem"
      :item="activeItem"
      :time-str="fmt(timeLeft)"
      @close="activeItem = null"
      @captured="onCaptured"
      @failed="onCaptureFailed"
    />
  </main>
  <main v-else class="sh-app flex min-h-screen items-center justify-center">
    <p class="font-bold" style="color: var(--sh-muted)">{{ t('common.loading') }}</p>
  </main>
</template>
