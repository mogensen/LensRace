import type { MessageSchema } from './en'

export default {
  common: {
    loading: 'Indlæser…',
  },
  home: {
    tagline: 'Kapløb om at fotografere alt!',
    namePlaceholder: 'Dit navn',
    createButton: 'Opret et spil',
    creatingButton: 'Opretter…',
    or: 'ELLER',
    joinCodePlaceholder: 'KODE',
    joinButton: 'Deltag',
    errors: {
      serverUnreachable: 'Kunne ikke oprette forbindelse til serveren. Kører serveren?',
      enterName: 'Indtast dit navn først',
      enterCode: 'Indtast en spilkode',
      createFailed: 'Kunne ikke oprette spillet',
      joinFailed: 'Kunne ikke deltage i spillet',
    },
  },
  lobby: {
    shareCode: 'DEL DENNE KODE',
    category: 'Kategori',
    roundLength: 'Rundens længde',
    setByHost: 'sat af værten',
    durationMinutes: '{mins} min',
    durationMinutesSeconds: '{mins} min {secs}s',
    players: 'Spillere',
    you: 'DIG',
    host: 'VÆRT',
    ready: 'klar',
    startButton: 'Start jagten!',
    waitingMessage: 'Venter på, at værten starter…',
    errors: {
      durationFailed: 'Kunne ikke ændre rundens længde',
      categoryFailed: 'Kunne ikke ændre kategori',
      startFailed: 'Kunne ikke starte spillet',
    },
  },
  play: {
    findThese: 'Find disse {count}',
    foundCount: '{found}/{total} fundet',
    live: 'LIVE',
    points: 'point',
    cameraAccessFailed: 'Kunne ikke få adgang til kameraet',
    captureFailed: 'Kunne ikke gemme optagelsen',
  },
  camera: {
    find: 'FIND',
    cantSee: 'kan ikke se {item} — prøv igen',
    holdSteady: 'hold kameraet stille på {item}',
    detectionTrouble:
      'registreringsproblemer — tjek konsollen, eller prøv at lukke og genåbne kameraet',
    scanning: 'Scanner…',
    gotIt: 'Fanget!',
    point: '+{n} point',
    cameraUnavailable: 'Kamera utilgængeligt',
  },
  results: {
    wonTitle: '🏆 Du vandt!',
    allFoundTitle: '🎉 Alt fundet!',
    timesUpTitle: '⏰ Tiden er udløbet!',
    summary: 'Du fandt {found} af {total} · {score} point',
    playAgain: 'Spil igen',
  },
} satisfies MessageSchema
