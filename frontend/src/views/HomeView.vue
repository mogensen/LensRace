<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useGameStore } from '@/stores/game'
import { ApiError, listCategories, type Category } from '@/lib/api'

const router = useRouter()
const store = useGameStore()

const name = ref('')
const joinCode = ref('')
const loading = ref<'create' | 'join' | null>(null)
const error = ref('')
const categories = ref<Category[]>([])

onMounted(async () => {
  try {
    categories.value = await listCategories()
  } catch {
    error.value = 'Could not reach the server. Is the backend running?'
  }
})

function requireName(): boolean {
  if (!name.value.trim()) {
    error.value = 'Enter your name first'
    return false
  }
  return true
}

async function onCreate() {
  const defaultCategory = categories.value[0]
  if (!requireName() || !defaultCategory) return
  loading.value = 'create'
  error.value = ''
  try {
    const session = await store.createGame(defaultCategory.id, name.value.trim())
    await router.push({ name: 'lobby', params: { id: session.game.id } })
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Could not create game'
  } finally {
    loading.value = null
  }
}

async function onJoin() {
  if (!requireName()) return
  if (!joinCode.value.trim()) {
    error.value = 'Enter a game code'
    return
  }
  loading.value = 'join'
  error.value = ''
  try {
    const session = await store.joinGame(joinCode.value.trim(), name.value.trim())
    await router.push({ name: 'lobby', params: { id: session.game.id } })
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Could not join game'
  } finally {
    loading.value = null
  }
}

function onJoinInput(e: Event) {
  const target = e.target as HTMLInputElement
  joinCode.value = target.value.toUpperCase().replace(/[^A-Z0-9]/g, '')
}

const canCreate = computed(() => categories.value.length > 0 && loading.value === null)
</script>

<template>
  <main class="sh-app flex min-h-screen items-center justify-center p-7">
    <div class="flex w-full max-w-sm flex-col gap-5" style="animation: sh-fade-up 0.4s ease both">
      <div class="flex flex-1 flex-col items-center justify-center gap-4 py-6">
        <div
          class="flex h-[110px] w-[110px] items-center justify-center rounded-[34px] border-[3px] text-5xl"
          style="
            background: var(--sh-orange);
            border-color: var(--sh-ink);
            box-shadow: 5px 6px 0 var(--sh-ink);
            animation: sh-bob 3s ease-in-out infinite;
          "
        >
          📸
        </div>
        <div class="text-center">
          <h1 class="sh-title text-[44px] leading-[0.92] tracking-tight">SNAP<br />HUNT</h1>
          <p class="mt-2 text-base font-bold" style="color: var(--sh-muted)">
            Race to photograph everything!
          </p>
        </div>
      </div>

      <input
        v-model="name"
        data-testid="name-input"
        placeholder="Your name"
        maxlength="20"
        class="sh-input px-4 py-3 text-center text-lg"
      />

      <button
        data-testid="create-game-button"
        class="sh-btn sh-btn-primary py-4 text-xl"
        :disabled="!canCreate"
        @click="onCreate"
      >
        {{ loading === 'create' ? 'Creating…' : '🎮 Create a game' }}
      </button>

      <div class="flex items-center gap-3 text-xs font-extrabold" style="color: #cbb59c">
        <div class="h-[2.5px] flex-1 rounded" style="background: var(--sh-border-light)"></div>
        OR
        <div class="h-[2.5px] flex-1 rounded" style="background: var(--sh-border-light)"></div>
      </div>

      <div class="flex gap-2.5">
        <input
          :value="joinCode"
          data-testid="join-code-input"
          placeholder="CODE"
          maxlength="6"
          class="sh-input min-w-0 flex-1 py-3 text-center text-2xl tracking-[4px] uppercase"
          @input="onJoinInput"
        />
        <button
          data-testid="join-game-button"
          class="sh-btn sh-btn-yellow px-6 text-lg"
          :disabled="loading !== null"
          @click="onJoin"
        >
          {{ loading === 'join' ? '…' : 'Join' }}
        </button>
      </div>

      <p
        v-if="error"
        data-testid="home-error"
        class="text-center text-sm font-bold"
        style="color: var(--sh-orange)"
      >
        {{ error }}
      </p>
    </div>
  </main>
</template>
