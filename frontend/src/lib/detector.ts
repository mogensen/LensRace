// Loads TensorFlow.js plus one of two on-device models from
// public/vendor/ via a plain <script> tag rather than an ES import:
// COCO-SSD (a fixed set of 80 object classes, used for most items) or
// MobileNet (an ImageNet-1000 image classifier, used as a fallback for
// items COCO-SSD's vocabulary can't recognize at all, like "mountain
// tent" or "daisy" — see pickDetector).
//
// Why <script> tags: tfjs-converter defines a class method literally named
// `import` (`async import(keys, values) {}`). Vite's lightweight
// import-scanner misreads `import(` there as a dynamic import() call and
// corrupts the file at parse time — this happens regardless of
// optimizeDeps include/exclude settings, since it's Vite's transform
// pipeline itself, not the pre-bundler, that's doing the (mis)rewriting.
// Loading the UMD build via <script> (see scripts/copy-vendor.mjs)
// sidesteps Vite's JS transform entirely for these files. Each model's
// script + weights are only fetched the first time that model is actually
// used by default, but callers can (and should — see preloadDetectors)
// kick off both models' downloads early, before a round starts, rather
// than leaving it to chance whether a player still has a connection by
// the time the camera actually needs one.

interface CocoSsdPrediction {
  class: string
  score: number
}

interface CocoSsdModel {
  detect(video: HTMLVideoElement): Promise<CocoSsdPrediction[]>
}

interface MobileNetPrediction {
  className: string
  probability: number
}

interface MobileNetModel {
  classify(video: HTMLVideoElement, topk?: number): Promise<MobileNetPrediction[]>
}

declare global {
  interface Window {
    cocoSsd?: { load(): Promise<CocoSsdModel> }
    mobilenet?: { load(config?: { version: 1 | 2; alpha: number }): Promise<MobileNetModel> }
  }
}

// Keyed by src so concurrent callers (see preloadDetectors, which kicks off
// coco-ssd and mobilenet together — both depend on tf.min.js) await the same
// in-flight load instead of racing: appendChild below adds the <script> tag
// to the DOM synchronously, so a naive "does this tag already exist" check
// would let a second caller resolve immediately, before tf.min.js has
// actually finished loading and defined the global it needs.
const scriptLoads = new Map<string, Promise<void>>()

function loadScript(src: string): Promise<void> {
  const inFlight = scriptLoads.get(src)
  if (inFlight) return inFlight

  const promise = new Promise<void>((resolve, reject) => {
    const script = document.createElement('script')
    script.src = src
    script.onload = () => resolve()
    script.onerror = () => reject(new Error(`Failed to load ${src}`))
    document.head.appendChild(script)
  }).catch((err: unknown) => {
    // Don't cache a permanently-failed load — see loadCocoSsd/loadMobilenet's
    // matching comment.
    scriptLoads.delete(src)
    throw err
  })

  scriptLoads.set(src, promise)
  return promise
}

// The fixed, closed set of 80 object classes TensorFlow.js COCO-SSD can
// ever recognize. An item whose label isn't in this set is routed to the
// secondary MobileNet model instead (see pickDetector) — kept in sync with
// internal/db/db_test.go's cocoSsdClasses, which guards every seeded item
// against exactly this list.
const COCO_SSD_CLASSES = new Set([
  'person',
  'bicycle',
  'car',
  'motorcycle',
  'airplane',
  'bus',
  'train',
  'truck',
  'boat',
  'traffic light',
  'fire hydrant',
  'stop sign',
  'parking meter',
  'bench',
  'bird',
  'cat',
  'dog',
  'horse',
  'sheep',
  'cow',
  'elephant',
  'bear',
  'zebra',
  'giraffe',
  'backpack',
  'umbrella',
  'handbag',
  'tie',
  'suitcase',
  'frisbee',
  'skis',
  'snowboard',
  'sports ball',
  'kite',
  'baseball bat',
  'baseball glove',
  'skateboard',
  'surfboard',
  'tennis racket',
  'bottle',
  'wine glass',
  'cup',
  'fork',
  'knife',
  'spoon',
  'bowl',
  'banana',
  'apple',
  'sandwich',
  'orange',
  'broccoli',
  'carrot',
  'hot dog',
  'pizza',
  'donut',
  'cake',
  'chair',
  'couch',
  'potted plant',
  'bed',
  'dining table',
  'toilet',
  'tv',
  'laptop',
  'mouse',
  'remote',
  'keyboard',
  'cell phone',
  'microwave',
  'oven',
  'toaster',
  'sink',
  'refrigerator',
  'book',
  'clock',
  'vase',
  'scissors',
  'teddy bear',
  'hair drier',
  'toothbrush',
])

export type DetectorKind = 'coco-ssd' | 'mobilenet'

/** Which model recognizes label — see COCO_SSD_CLASSES's doc comment. */
export function pickDetector(label: string): DetectorKind {
  return COCO_SSD_CLASSES.has(label.toLowerCase()) ? 'coco-ssd' : 'mobilenet'
}

let cocoModelPromise: Promise<CocoSsdModel> | null = null

function loadCocoSsd(): Promise<CocoSsdModel> {
  if (!cocoModelPromise) {
    cocoModelPromise = (async () => {
      await loadScript('/vendor/tf.min.js')
      await loadScript('/vendor/coco-ssd.min.js')
      if (!window.cocoSsd) throw new Error('coco-ssd failed to load')
      return window.cocoSsd.load()
    })().catch((err: unknown) => {
      // Don't cache a permanently-failed load: a transient hiccup (slow
      // network on first camera open, script briefly unavailable) would
      // otherwise poison every detection attempt for the rest of the
      // session, since this promise is a module-level singleton. Clearing
      // it lets the next call retry from scratch.
      cocoModelPromise = null
      throw err
    })
  }
  return cocoModelPromise
}

let mobilenetModelPromise: Promise<MobileNetModel> | null = null

function loadMobilenet(): Promise<MobileNetModel> {
  if (!mobilenetModelPromise) {
    mobilenetModelPromise = (async () => {
      await loadScript('/vendor/tf.min.js')
      await loadScript('/vendor/mobilenet.min.js')
      if (!window.mobilenet) throw new Error('mobilenet failed to load')
      // v1/alpha 1.0 is the package default and best-accuracy config; its
      // weights are a similar ~16MB to COCO-SSD's, so this isn't a bigger
      // download than what the app already fetches for most items.
      return window.mobilenet.load({ version: 1, alpha: 1.0 })
    })().catch((err: unknown) => {
      mobilenetModelPromise = null
      throw err
    })
  }
  return mobilenetModelPromise
}

export interface Detection {
  class: string
  score: number
}

// How many of MobileNet's ranked predictions to consider per frame. Unlike
// COCO-SSD (which finds multiple objects in one pass), MobileNet is a
// whole-frame classifier that ranks its single best guess at what's most
// prominent — a wider top-K gives the target item a chance to match even
// when it isn't MobileNet's single most-confident guess.
const MOBILENET_TOP_K = 10

/**
 * Loads whichever model recognizes itemLabel (if needed) and runs one
 * detection pass on a video frame.
 */
export async function detectObjects(video: HTMLVideoElement, itemLabel: string): Promise<Detection[]> {
  if (pickDetector(itemLabel) === 'coco-ssd') {
    const model = await loadCocoSsd()
    const predictions = await model.detect(video)
    return predictions.map((p) => ({ class: p.class, score: p.score }))
  }
  const model = await loadMobilenet()
  const predictions = await model.classify(video, MOBILENET_TOP_K)
  return predictions.map((p) => ({ class: p.className, score: p.probability }))
}

/**
 * Kicks off loading BOTH models, without waiting for them to finish.
 * Deliberately unconditional — players are as likely to be somewhere with
 * flaky or no connectivity once play actually starts (a forest, a
 * campsite) as somewhere reliable, so waiting to see which single model a
 * round's items need and fetching only that one is a bet against exactly
 * the situation this app is built for. Loading is cached per model (see
 * loadCocoSsd / loadMobilenet), so calling this redundantly, or before
 * detectObjects ever runs, never triggers a duplicate download. Meant to
 * be called as early as possible — home screen, lobby — while there's
 * still a decent chance of a good connection, well before a player first
 * opens the camera mid-round. Failures are swallowed: a failed preload
 * isn't fatal here, since detectObjects retries the load itself the
 * moment a player actually opens the camera.
 */
export function preloadDetectors(): void {
  loadCocoSsd().catch(() => {
    // Swallowed — see doc comment above.
  })
  loadMobilenet().catch(() => {
    // Swallowed — see doc comment above.
  })
}

/**
 * Whether a detected class matches label. MobileNet sometimes reports
 * several synonyms as one comma-joined string (e.g. "tabby, tabby cat"),
 * so this checks each comma-separated part rather than requiring the whole
 * string to equal label — a no-op split for COCO-SSD, whose class names
 * never contain commas.
 */
export function classMatchesLabel(detectedClass: string, label: string): boolean {
  const target = label.toLowerCase()
  return detectedClass
    .toLowerCase()
    .split(',')
    .some((part) => part.trim() === target)
}
