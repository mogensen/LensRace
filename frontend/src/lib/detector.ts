// Loads TensorFlow.js / COCO-SSD from public/vendor/ via a plain <script>
// tag rather than an ES import.
//
// Why: tfjs-converter defines a class method literally named `import`
// (`async import(keys, values) {}`). Vite's lightweight import-scanner
// misreads `import(` there as a dynamic import() call and corrupts the
// file at parse time — this happens regardless of optimizeDeps
// include/exclude settings, since it's Vite's transform pipeline itself,
// not the pre-bundler, that's doing the (mis)rewriting. Loading the UMD
// build via <script> (see scripts/copy-vendor.mjs) sidesteps Vite's JS
// transform entirely for these files. The script tag is only injected on
// first use, so the ~1.4MB payload doesn't load until the camera opens.

interface CocoSsdPrediction {
  class: string
  score: number
}

interface CocoSsdModel {
  detect(video: HTMLVideoElement): Promise<CocoSsdPrediction[]>
}

declare global {
  interface Window {
    cocoSsd?: { load(): Promise<CocoSsdModel> }
  }
}

function loadScript(src: string): Promise<void> {
  if (document.querySelector(`script[src="${src}"]`)) return Promise.resolve()
  return new Promise((resolve, reject) => {
    const script = document.createElement('script')
    script.src = src
    script.onload = () => resolve()
    script.onerror = () => reject(new Error(`Failed to load ${src}`))
    document.head.appendChild(script)
  })
}

let modelPromise: Promise<CocoSsdModel> | null = null

function loadModel(): Promise<CocoSsdModel> {
  if (!modelPromise) {
    modelPromise = (async () => {
      await loadScript('/vendor/tf.min.js')
      await loadScript('/vendor/coco-ssd.min.js')
      if (!window.cocoSsd) throw new Error('coco-ssd failed to load')
      return window.cocoSsd.load()
    })()
  }
  return modelPromise
}

export interface Detection {
  class: string
  score: number
}

/** Loads the model (if needed) and runs one detection pass on a video frame. */
export async function detectObjects(video: HTMLVideoElement): Promise<Detection[]> {
  const model = await loadModel()
  const predictions = await model.detect(video)
  return predictions.map((p) => ({ class: p.class, score: p.score }))
}
