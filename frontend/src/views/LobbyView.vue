<script setup lang="ts">
import { onMounted, watch, ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import QRCode from 'qrcode'
import { useGameStore } from '@/stores/game'
import { listCategories, ApiError, type Category } from '@/lib/api'
import { categoryName } from '@/lib/catalogNames'
import { avatarEmoji, avatarColor } from '@/lib/avatar'
import { preloadDetectors } from '@/lib/detector'

const props = defineProps<{ id: string }>()
const router = useRouter()
const store = useGameStore()
const { t, te } = useI18n()

const categories = ref<Category[]>([])
const error = ref('')
const linkCopied = ref(false)
const qrDataUrl = ref('')
const showQr = ref(false)

// The backend accepts any duration from 15s to 3600s, but a scavenger hunt
// round realistically wants to be short — this range (and the 300s
// default) come from the original design's own roundSeconds prop.
const DURATION_MIN = 15
const DURATION_MAX = 600
// Must evenly divide (value - DURATION_MIN) for every duration the slider
// needs to land on exactly — a range input's programmatic value is
// otherwise snapped/rejected against its step, e.g. 30 didn't divide
// evenly from a 15s minimum (180 - 15 = 165, not a multiple of 30).
const DURATION_STEP = 15
const localDuration = ref(300)

onMounted(async () => {
  // Both on-device models start downloading now, well before the round
  // starts (see preloadDetectors: unconditional, not just whichever model
  // this round's items happen to need, since a player might lose their
  // connection between now and when they actually open the camera).
  preloadDetectors()

  const ok = await store.ensureSession(props.id)
  if (!ok) {
    await router.replace({ name: 'home' })
    return
  }
  localDuration.value = store.state.gameState?.game.durationSeconds ?? 300
  try {
    categories.value = await listCategories()
  } catch {
    // Non-fatal: the current category name still comes through in
    // gameState, the picker options just won't render.
  }
})

// The host drags the slider freely (local, no network calls per tick);
// everyone else just reflects the server's current value. Only committing
// on "change" (drag release) rather than every "input" tick avoids
// spamming the API — and every player, not just the host, sees the
// broadcasted result live via SSE.
const durationLabel = computed(() => {
  const seconds = store.isHost
    ? localDuration.value
    : (store.state.gameState?.game.durationSeconds ?? 300)
  const mins = Math.floor(seconds / 60)
  const secs = seconds % 60
  return secs === 0
    ? t('lobby.durationMinutes', { mins })
    : t('lobby.durationMinutesSeconds', { mins, secs })
})

async function onDurationChange() {
  error.value = ''
  try {
    await store.changeDuration(localDuration.value)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : t('lobby.errors.durationFailed')
  }
}

watch(
  () => store.state.gameState?.game.status,
  (status) => {
    if (status === 'playing') router.push({ name: 'play', params: { id: props.id } })
  },
)

async function pickCategory(categoryId: string) {
  error.value = ''
  try {
    await store.changeCategory(categoryId)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : t('lobby.errors.categoryFailed')
  }
}

async function onStart() {
  error.value = ''
  try {
    await store.start()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : t('lobby.errors.startFailed')
  }
}

const inviteLink = computed(() => {
  const code = store.state.gameState?.game.joinCode
  return code ? `${window.location.origin}/join/${code}` : ''
})

async function onCopyLink() {
  if (!inviteLink.value) return
  try {
    await navigator.clipboard.writeText(inviteLink.value)
    linkCopied.value = true
    setTimeout(() => (linkCopied.value = false), 2000)
  } catch {
    error.value = t('lobby.errors.copyLinkFailed')
  }
}

async function onShowQr() {
  if (!inviteLink.value) return
  error.value = ''
  try {
    qrDataUrl.value = await QRCode.toDataURL(inviteLink.value, { width: 240, margin: 1 })
    showQr.value = true
  } catch {
    error.value = t('lobby.errors.qrFailed')
  }
}
</script>

<template>
  <main v-if="store.state.gameState" class="sh-app flex min-h-screen flex-col p-6">
    <div class="mb-4 text-center">
      <div class="text-xs font-extrabold tracking-wide" style="color: var(--sh-muted)">
        {{ t('lobby.shareCode') }}
      </div>
      <div data-testid="join-code" class="mt-2 inline-flex gap-1.5">
        <span
          v-for="(ch, i) in store.state.gameState.game.joinCode.split('')"
          :key="i"
          class="sh-title flex h-12 w-9 items-center justify-center rounded-xl border-[3px] text-2xl"
          style="
            background: #fff;
            border-color: var(--sh-ink);
            box-shadow: 3px 4px 0 var(--sh-ink);
            color: var(--sh-orange);
          "
        >
          {{ ch }}
        </span>
      </div>
      <div class="mt-3 flex justify-center gap-2">
        <button
          data-testid="copy-link-button"
          class="sh-btn sh-btn-yellow px-4 py-2 text-sm"
          @click="onCopyLink"
        >
          {{ linkCopied ? `✅ ${t('lobby.linkCopied')}` : `🔗 ${t('lobby.copyLink')}` }}
        </button>
        <button
          data-testid="show-qr-button"
          class="sh-btn sh-btn-yellow px-4 py-2 text-sm"
          @click="onShowQr"
        >
          📱 {{ t('lobby.showQr') }}
        </button>
      </div>
    </div>

    <div
      v-if="showQr"
      data-testid="qr-modal"
      class="fixed inset-0 z-40 flex items-center justify-center p-6"
      style="background: rgba(0, 0, 0, 0.5)"
      @click.self="showQr = false"
    >
      <div class="sh-card flex flex-col items-center gap-4 p-6" style="background: #fff">
        <div class="sh-title text-base">{{ t('lobby.scanToJoin') }}</div>
        <img :src="qrDataUrl" :alt="t('lobby.showQr')" data-testid="qr-code-image" class="h-60 w-60" />
        <button
          data-testid="close-qr-button"
          class="sh-btn sh-btn-yellow px-4 py-2 text-sm"
          @click="showQr = false"
        >
          {{ t('lobby.close') }}
        </button>
      </div>
    </div>

    <div class="sh-title mb-2 text-base">{{ t('lobby.category') }}</div>
    <div class="mb-4 flex flex-col gap-2">
      <button
        v-for="c in categories"
        :key="c.id"
        class="sh-card flex items-center gap-3 px-3.5 py-2.5"
        :class="{ 'opacity-70': !store.isHost }"
        :disabled="!store.isHost"
        :style="c.id === store.state.gameState.game.categoryId ? 'background: #fff1d9' : undefined"
        @click="pickCategory(c.id)"
      >
        <span class="text-2xl">{{ c.icon }}</span>
        <span class="sh-title flex-1 text-left text-base">{{ categoryName(t, te, c.id, c.name) }}</span>
        <span
          class="flex h-6 w-6 items-center justify-center rounded-full border-2 text-xs font-extrabold"
          :style="
            c.id === store.state.gameState.game.categoryId
              ? 'background: var(--sh-green); color: #fff; border-color: var(--sh-ink)'
              : 'background: #f0e2cf; border-color: var(--sh-ink); color: #f0e2cf'
          "
        >
          {{ c.id === store.state.gameState.game.categoryId ? '✓' : '' }}
        </span>
      </button>
    </div>

    <div class="sh-title mb-2 text-base">{{ t('lobby.roundLength') }}</div>
    <div class="sh-card mb-4 flex flex-col gap-1.5 px-3.5 py-2.5">
      <div class="flex items-center justify-between text-sm font-extrabold">
        <span data-testid="duration-label">⏱ {{ durationLabel }}</span>
        <span v-if="!store.isHost" class="text-xs font-bold" style="color: var(--sh-muted)">
          {{ t('lobby.setByHost') }}
        </span>
      </div>
      <input
        v-if="store.isHost"
        v-model.number="localDuration"
        data-testid="duration-input"
        type="range"
        :min="DURATION_MIN"
        :max="DURATION_MAX"
        :step="DURATION_STEP"
        class="w-full"
        style="accent-color: var(--sh-orange)"
        @change="onDurationChange"
      />
    </div>

    <div class="sh-title mb-2 flex items-center gap-2 text-base">
      {{ t('lobby.players') }}
      <span
        class="rounded-full border-2 px-2 py-0.5 text-xs font-bold text-white"
        style="background: var(--sh-green); border-color: var(--sh-ink)"
      >
        {{ store.state.gameState.players.length }}
      </span>
    </div>
    <div data-testid="player-list" class="mb-4 grid grid-cols-2 gap-2.5">
      <div
        v-for="p in store.state.gameState.players"
        :key="p.id"
        data-testid="player-row"
        class="sh-card flex items-center gap-2 p-2.5"
      >
        <span
          class="flex h-8 w-8 items-center justify-center rounded-full border-2 text-lg"
          :style="`background:${avatarColor(p.id)};border-color:var(--sh-ink)`"
        >
          {{ avatarEmoji(p.id) }}
        </span>
        <span class="sh-title text-sm">{{ p.name }}</span>
        <span
          class="ml-auto rounded-lg px-1.5 py-0.5 text-[10px] font-extrabold"
          :style="
            p.id === store.me?.id
              ? 'background: var(--sh-orange); color: #fff'
              : 'background: #f0e2cf; color: var(--sh-muted)'
          "
        >
          {{
            p.id === store.me?.id ? t('lobby.you') : p.isHost ? t('lobby.host') : t('lobby.ready')
          }}
        </span>
      </div>
    </div>

    <div class="flex-1"></div>

    <button
      v-if="store.isHost"
      data-testid="start-button"
      class="sh-btn sh-btn-green py-4 text-xl"
      style="animation: sh-bob 2.6s ease-in-out infinite"
      @click="onStart"
    >
      🚀 {{ t('lobby.startButton') }}
    </button>
    <div
      v-else
      data-testid="waiting-message"
      class="rounded-2xl border-[3px] border-dashed p-4 text-center text-lg font-bold"
      style="border-color: var(--sh-border-dashed); color: var(--sh-muted)"
    >
      ⏳ {{ t('lobby.waitingMessage') }}
    </div>

    <p
      v-if="error"
      data-testid="lobby-error"
      class="mt-3 text-center text-sm font-bold"
      style="color: var(--sh-orange)"
    >
      {{ error }}
    </p>
  </main>
  <main v-else class="sh-app flex min-h-screen items-center justify-center">
    <p class="font-bold" style="color: var(--sh-muted)">{{ t('common.loading') }}</p>
  </main>
</template>
