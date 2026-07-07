<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useGameStore } from '@/stores/game'
import type { Item } from '@/lib/api'
import { itemEmoji } from '@/lib/itemIcons'
import { itemName } from '@/lib/catalogNames'
import { detectObjects, classMatchesLabel, pickDetector } from '@/lib/detector'

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
const { t, te } = useI18n()

const localizedItemName = computed(() => itemName(t, te, props.item.id, props.item.displayName))

type Stage = 'aim' | 'scan' | 'done' | 'error'
const stage = ref<Stage>('aim')
const cameraError = ref('')
const videoEl = ref<HTMLVideoElement | null>(null)
const detectionTrouble = ref(false)

let mediaStream: MediaStream | null = null
let detectionTimer: ReturnType<typeof setTimeout> | undefined
let detectionCancelled = false
let consecutiveMatches = 0
let consecutiveErrors = 0

// Every capture is worth exactly 1 point (score is a derived count of
// captures server-side), unlike the design prototype's arbitrary +10.
const POINTS_PER_CAPTURE = 1

// A match needs this many consecutive detection ticks above the score
// threshold before it auto-triggers a capture, so a single flickery frame
// doesn't fire a false positive.
const MATCH_THRESHOLD_TICKS = 2
const MIN_SCORE = 0.4
const DETECTION_INTERVAL_MS = 400

// If detection keeps throwing (as opposed to just not finding a match)
// for this many consecutive ticks (~2.4s), surface it — otherwise a
// genuinely broken model (blocked script, no WebGL, etc.) looks
// identical to "just hasn't found it yet" and is impossible to diagnose.
const ERROR_WARNING_TICKS = 6

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
    cameraError.value = e instanceof Error ? e.message : t('play.cameraAccessFailed')
    stage.value = 'error'
  }
})

onUnmounted(() => {
  stopDetectionLoop()
  mediaStream?.getTracks().forEach((track) => track.stop())
})

// Whether a set of detections includes the item we're hunting, above the
// score threshold.
function matchesItem(detections: Awaited<ReturnType<typeof detectObjects>>): boolean {
  return detections.some(
    (d) => classMatchesLabel(d.class, props.item.label) && d.score >= MIN_SCORE,
  )
}

// Logs the model's raw output every tick — what it actually saw, not just
// whether it happened to match — so a "why won't this detect" report can
// be diagnosed from the browser console instead of guessing (e.g. seeing
// the right class at 0.32 confidence explains a miss against MIN_SCORE=0.4
// far better than silence does). Also logs which of the two on-device
// models (COCO-SSD or MobileNet) is running for this item, since which one
// gets used is picked automatically per label.
function logDetections(detections: Awaited<ReturnType<typeof detectObjects>>): void {
  const seen = detections.length
    ? detections.map((d) => `${d.class} (${Math.round(d.score * 100)}%)`).join(', ')
    : '(nothing)'
  console.log(
    `[detector:${pickDetector(props.item.label)}] looking for "${props.item.label}" — model saw: ${seen}`,
  )
}

// A self-scheduling setTimeout, not setInterval: COCO-SSD inference can
// easily take longer than DETECTION_INTERVAL_MS (especially without GPU
// acceleration), and setInterval would let overlapping calls pile up and
// starve the main thread. Each tick waits for the previous one to finish.
function startDetectionLoop() {
  detectionCancelled = false

  async function tick() {
    if (detectionCancelled || stage.value !== 'aim' || !videoEl.value) return
    try {
      const detections = await detectObjects(videoEl.value, props.item.label)
      logDetections(detections)
      consecutiveErrors = 0
      detectionTrouble.value = false
      consecutiveMatches = matchesItem(detections) ? consecutiveMatches + 1 : 0
      if (consecutiveMatches >= MATCH_THRESHOLD_TICKS) {
        beginCapture()
        return
      }
    } catch (e) {
      // Transient detection errors (model still warming up on the first
      // few ticks) are fine — the loop just tries again next tick. But if
      // it's still failing after ERROR_WARNING_TICKS in a row, that's not
      // "hasn't found it yet", it's genuinely broken, so surface it.
      consecutiveErrors += 1
      if (consecutiveErrors === 1) {
        console.error('[detector] detection failed:', e)
      }
      if (consecutiveErrors >= ERROR_WARNING_TICKS) {
        detectionTrouble.value = true
      }
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

function beginCapture() {
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
        const message = e instanceof Error ? e.message : t('play.captureFailed')
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
      class="pointer-events-none absolute bottom-6 left-6 h-[30px] w-[30px] rounded-bl-lg border-4 border-t-0 border-r-0"
      style="border-color: #fff6ea"
    ></span>
    <span
      class="pointer-events-none absolute right-6 bottom-6 h-[30px] w-[30px] rounded-br-lg border-4 border-t-0 border-l-0"
      style="border-color: #fff6ea"
    ></span>

    <!-- The item needs to fill nearly this whole frame for the on-device
         model to recognize it — a small centered reticle previously implied
         a tiny, distant object was enough, so this spans the same area as
         the corner brackets above to make "get close, fill the frame" the
         obvious reading. -->
    <div
      v-if="stage === 'aim'"
      class="pointer-events-none absolute rounded-[28px] border-[3px] border-dashed"
      style="
        top: 84px;
        right: 24px;
        bottom: 24px;
        left: 24px;
        border-color: rgba(255, 246, 234, 0.75);
        animation: sh-frame-pulse 1.8s ease-in-out infinite;
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
        <span class="text-xs font-extrabold" style="color: var(--sh-muted)">{{
          t('camera.find')
        }}</span>
        <span class="text-2xl">{{ itemEmoji(item.label) }}</span>
        <span class="sh-title text-lg">{{ localizedItemName }}</span>
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
          class="rounded-full px-3 py-1 text-sm font-bold"
          style="background: rgba(0, 0, 0, 0.4); color: rgba(255, 246, 234, 0.9)"
        >
          {{ t('camera.holdSteady', { item: localizedItemName.toLowerCase() }) }}
        </div>
        <div
          v-if="detectionTrouble"
          class="rounded-full px-3 py-1 text-xs font-bold"
          style="background: rgba(255, 107, 61, 0.9); color: #fff"
        >
          ⚠ {{ t('camera.detectionTrouble') }}
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
        <div class="sh-title text-xl" style="color: #fff6ea">{{ t('camera.scanning') }}</div>
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
        <div class="sh-title text-2xl" style="color: #fff6ea">{{ t('camera.gotIt') }}</div>
        <div
          class="sh-title rounded-2xl border-[3px] px-4 py-1.5 text-xl"
          style="
            background: var(--sh-yellow);
            border-color: var(--sh-ink);
            color: var(--sh-ink);
            box-shadow: 3px 4px 0 var(--sh-ink);
          "
        >
          {{ t('camera.point', { n: POINTS_PER_CAPTURE }) }}
        </div>
      </div>

      <div v-else class="flex flex-col items-center gap-4 px-8 text-center">
        <div class="text-5xl">📵</div>
        <div class="sh-title text-xl" style="color: #fff6ea">
          {{ t('camera.cameraUnavailable') }}
        </div>
        <div class="text-sm font-bold" style="color: rgba(255, 246, 234, 0.75)">
          {{ cameraError }}
        </div>
      </div>
    </div>
  </div>
</template>
