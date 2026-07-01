// Copies the TensorFlow.js / COCO-SSD UMD (browser global) builds into
// public/vendor/, where they're served as static assets and loaded via a
// plain <script> tag instead of an ES import.
//
// Why: tfjs-converter defines a class method literally named `import`
// (`async import(keys, values) {}`), which trips up Vite's lightweight
// import-scanner — it misreads `import(` as a dynamic import() call and
// corrupts the file, regardless of optimizeDeps include/exclude settings.
// Loading the UMD build via <script> sidesteps Vite's JS transform
// pipeline for these files entirely.
import { copyFileSync, mkdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = dirname(dirname(fileURLToPath(import.meta.url)))
const vendorDir = join(root, 'public', 'vendor')
mkdirSync(vendorDir, { recursive: true })

const files = [
  ['node_modules/@tensorflow/tfjs/dist/tf.min.js', 'tf.min.js'],
  ['node_modules/@tensorflow-models/coco-ssd/dist/coco-ssd.min.js', 'coco-ssd.min.js'],
]

for (const [src, dest] of files) {
  copyFileSync(join(root, src), join(vendorDir, dest))
}

console.log('Copied TensorFlow.js vendor bundles to public/vendor/')
