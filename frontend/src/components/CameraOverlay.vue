<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useGameStore } from '@/stores/game'
import type { Item } from '@/lib/api'
import { itemEmoji } from '@/lib/itemIcons'
import { detectObjects } from '@/lib/detector'

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

type Stage = 'aim' | 'scan' | 'done' | 'error'
const stage = ref<Stage>('aim')
const cameraError = ref('')
const videoEl = ref<HTMLVideoElement | null>(null)

let mediaStream: MediaStream | null = null
let detectionTimer: ReturnType<typeof setTimeout> | undefined
let detectionCancelled = false
let consecutiveMatches = 0

// Every capture is worth exactly 1 point (score is a derived count of
// captures server-side), unlike the design prototype's arbitrary +10.
const POINTS_PER_CAPTURE = 1

// A match needs this many consecutive detection ticks above the score
// threshold before it auto-triggers a capture, so a single flickery frame
// doesn't fire a false positive.
const MATCH_THRESHOLD_TICKS = 2
const MIN_SCORE = 0.6
const DETECTION_INTERVAL_MS = 400

onMounted(async () => {
  try {
    mediaStream = await navigator.mediaDevices.getUserMedia({
      video: { facingMode: { ideal: 'environment' } },
      audio: false,
    })
    if (videoEl.value) {
      videoEl.value.srcObject = mediaStream
      await videoEl.value.play()
    }
    stage.value = 'aim'
    startDetectionLoop()
  } catch (e) {
    cameraError.value = e instanceof Error ? e.message : 'Could not access the camera'
    stage.value = 'error'
  }
})

onUnmounted(() => {
  stopDetectionLoop()
  mediaStream?.getTracks().forEach((track) => track.stop())
})

// A self-scheduling setTimeout, not setInterval: COCO-SSD inference can
// easily take longer than DETECTION_INTERVAL_MS (especially without GPU
// acceleration), and setInterval would let overlapping calls pile up and
// starve the main thread. Each tick waits for the previous one to finish.
function startDetectionLoop() {
  detectionCancelled = false

  async function tick() {
    if (detectionCancelled || stage.value !== 'aim' || !videoEl.value) return
    try {
      const detections = await detectObjects(videoEl.value)
      const matched = detections.some(
        (d) => d.class.toLowerCase() === props.item.label.toLowerCase() && d.score >= MIN_SCORE,
      )
      consecutiveMatches = matched ? consecutiveMatches + 1 : 0
      if (consecutiveMatches >= MATCH_THRESHOLD_TICKS) {
        onCapture()
        return
      }
    } catch {
      // Transient detection errors (model still warming up on the first
      // few ticks) are fine to ignore; the loop just tries again next tick.
    }
    if (!detectionCancelled && stage.value === 'aim') {
      detectionTimer = setTimeout(tick, DETECTION_INTERVAL_MS)
    }
  }

  tick()
}

function stopDetectionLoop() {
  detectionCancelled = true
  if (detectionTimer) {
    clearTimeout(detectionTimer)
    detectionTimer = undefined
  }
}

function onCapture() {
  if (stage.value !== 'aim') return
  stopDetectionLoop()
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
  }, 900)
}
</script>

<template>
  <div class="fixed inset-0 z-40 flex flex-col overflow-hidden bg-black">
    <video
      ref="videoEl"
      class="absolute inset-0 h-full w-full object-cover"
      muted
      playsinline
      autoplay
    ></video>
    <div
      class="pointer-events-none absolute inset-0"
      style="
        background: linear-gradient(
          to bottom,
          rgba(0, 0, 0, 0.35),
          transparent 20%,
          transparent 70%,
          rgba(0, 0, 0, 0.55)
        );
      "
    ></div>

    <span
      class="pointer-events-none absolute top-[84px] left-6 h-[30px] w-[30px] rounded-tl-lg border-4 border-r-0 border-b-0"
      style="border-color: #fff6ea"
    ></span>
    <span
      class="pointer-events-none absolute top-[84px] right-6 h-[30px] w-[30px] rounded-tr-lg border-4 border-b-0 border-l-0"
      style="border-color: #fff6ea"
    ></span>
    <span
      class="pointer-events-none absolute bottom-[188px] left-6 h-[30px] w-[30px] rounded-bl-lg border-4 border-t-0 border-r-0"
      style="border-color: #fff6ea"
    ></span>
    <span
      class="pointer-events-none absolute right-6 bottom-[188px] h-[30px] w-[30px] rounded-br-lg border-4 border-t-0 border-l-0"
      style="border-color: #fff6ea"
    ></span>

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
        <div
          class="rounded-full px-3 py-1 text-sm font-bold"
          style="background: rgba(0, 0, 0, 0.4); color: rgba(255, 246, 234, 0.9)"
        >
          hold steady on the {{ item.displayName.toLowerCase() }}
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
        v-else-if="stage === 'done'"
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

      <div v-else class="flex flex-col items-center gap-4 px-8 text-center">
        <div class="text-5xl">📵</div>
        <div class="sh-title text-xl" style="color: #fff6ea">Camera unavailable</div>
        <div class="text-sm font-bold" style="color: rgba(255, 246, 234, 0.75)">
          {{ cameraError }}
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
