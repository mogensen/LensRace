<script setup lang="ts">
import { ref } from 'vue'
import { useGameStore } from '@/stores/game'
import type { Item } from '@/lib/api'
import { itemEmoji } from '@/lib/itemIcons'

const props = defineProps<{
  item: Item
  timeStr: string
}>()

const emit = defineEmits<{
  close: []
  captured: [itemId: string]
  failed: [message: string]
}>()

const store = useGameStore()

type Stage = 'aim' | 'scan' | 'done'
const stage = ref<Stage>('aim')

// Every capture is worth exactly 1 point (score is a derived count of
// captures server-side), unlike the design prototype's arbitrary +10.
const POINTS_PER_CAPTURE = 1

// Real ML-based on-device detection lands in a later milestone; for now the
// "scan" is a directed capture of the item the player tapped in the list.
function onCapture() {
  if (stage.value !== 'aim') return
  stage.value = 'scan'

  setTimeout(() => {
    store
      .capture(props.item.id)
      .then(() => {
        stage.value = 'done'
        setTimeout(() => emit('captured', props.item.id), 1300)
      })
      .catch((e: unknown) => {
        const message = e instanceof Error ? e.message : 'Could not record capture'
        emit('failed', message)
      })
  }, 1400)
}
</script>

<template>
  <div
    class="fixed inset-0 z-40 flex flex-col"
    style="
      background: radial-gradient(120% 80% at 50% 35%, #4a4236 0%, #2a2419 55%, #141009 100%);
      animation: sh-fade-up 0.25s ease both;
    "
  >
    <div
      class="pointer-events-none absolute inset-0"
      style="
        background-image:
          linear-gradient(rgba(255, 255, 255, 0.05) 1px, transparent 1px),
          linear-gradient(90deg, rgba(255, 255, 255, 0.05) 1px, transparent 1px);
        background-size: 34px 34px;
      "
    ></div>

    <div class="relative z-[2] flex items-center gap-2.5 px-5 pt-4">
      <button
        class="flex h-10 w-10 flex-none items-center justify-center rounded-full text-lg font-extrabold text-white"
        style="background: rgba(255, 255, 255, 0.15); border: 2px solid rgba(255, 255, 255, 0.5)"
        @click="emit('close')"
      >
        ✕
      </button>
      <div
        class="flex flex-1 items-center gap-2.5 rounded-2xl border-[2.5px] px-3 py-1.5"
        style="
          background: rgba(255, 246, 234, 0.95);
          border-color: var(--sh-ink);
          box-shadow: 3px 4px 0 rgba(0, 0, 0, 0.4);
        "
      >
        <span class="text-xs font-extrabold" style="color: var(--sh-muted)">FIND</span>
        <span class="text-2xl">{{ itemEmoji(item.label) }}</span>
        <span class="sh-title text-lg">{{ item.displayName }}</span>
      </div>
      <div
        class="rounded-2xl border-2 px-2.5 py-1.5 font-mono text-sm font-extrabold text-white"
        style="background: rgba(0, 0, 0, 0.5); border-color: rgba(255, 255, 255, 0.4)"
      >
        {{ timeStr }}
      </div>
    </div>

    <div class="relative z-[2] flex flex-1 items-center justify-center">
      <div v-if="stage === 'aim'" class="flex flex-col items-center gap-3.5">
        <div
          class="text-8xl opacity-30"
          style="filter: grayscale(0.3); animation: sh-bob 2.4s ease-in-out infinite"
        >
          {{ itemEmoji(item.label) }}
        </div>
        <div class="relative">
          <span
            class="absolute -inset-[18px] rounded-full border-[3px]"
            style="
              border-color: rgba(255, 246, 234, 0.6);
              animation: sh-pulse-ring 1.8s ease-out infinite;
            "
          ></span>
          <span
            class="block h-[70px] w-[70px] rounded-[18px] border-[3px] border-dashed"
            style="border-color: rgba(255, 246, 234, 0.85)"
          ></span>
        </div>
        <div class="text-sm font-bold" style="color: rgba(255, 246, 234, 0.8)">
          point at the {{ item.displayName.toLowerCase() }}
        </div>
      </div>

      <div v-else-if="stage === 'scan'" class="relative flex flex-col items-center gap-4.5">
        <div
          class="h-16 w-16 rounded-full border-[5px]"
          style="
            border-color: rgba(255, 246, 234, 0.25);
            border-top-color: var(--sh-orange);
            animation: sh-spin-slow 0.8s linear infinite;
          "
        ></div>
        <div class="sh-title text-xl" style="color: #fff6ea">Scanning…</div>
        <span
          class="absolute right-6 left-6 h-1 rounded-full"
          style="
            background: linear-gradient(90deg, transparent, var(--sh-orange), transparent);
            box-shadow: 0 0 16px 3px var(--sh-orange);
            animation: sh-scan-move 1.1s ease-in-out infinite alternate;
          "
        ></span>
      </div>

      <div
        v-else
        class="flex flex-col items-center gap-3.5"
        style="animation: sh-pop-in 0.45s ease both"
      >
        <div
          class="flex h-[120px] w-[120px] items-center justify-center rounded-[34px] border-4 text-6xl"
          style="
            background: var(--sh-green);
            border-color: #fff6ea;
            box-shadow: 0 14px 30px -6px rgba(0, 0, 0, 0.6);
            animation: sh-check-pop 0.5s ease both;
          "
        >
          ✓
        </div>
        <div class="sh-title text-2xl" style="color: #fff6ea">Got it!</div>
        <div
          class="sh-title rounded-2xl border-[3px] px-4 py-1.5 text-xl"
          style="
            background: var(--sh-yellow);
            border-color: var(--sh-ink);
            color: var(--sh-ink);
            box-shadow: 3px 4px 0 var(--sh-ink);
          "
        >
          +{{ POINTS_PER_CAPTURE }} point
        </div>
      </div>
    </div>

    <div class="relative z-[2] flex justify-center pb-10">
      <button
        v-if="stage === 'aim'"
        class="relative h-[84px] w-[84px] rounded-full"
        style="background: #fff6ea; border: 6px solid rgba(255, 255, 255, 0.45)"
        @click="onCapture"
      >
        <span
          class="absolute inset-[9px] rounded-full border-[3px]"
          style="background: var(--sh-orange); border-color: var(--sh-ink)"
        ></span>
      </button>
    </div>
  </div>
</template>
