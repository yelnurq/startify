import confetti from 'canvas-confetti';

// Sky blue and gold of the flag, plus white.
const COLORS = ['#14a8c9', '#4cc3e1', '#82daf1', '#f5b800', '#e8c962', '#ffffff'];

function reducedMotion(): boolean {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

/** A short burst for finishing an incubator stage. */
export function celebrateStage(): void {
  if (reducedMotion()) return;
  void confetti({
    particleCount: 110,
    spread: 75,
    startVelocity: 38,
    origin: { y: 0.72 },
    colors: COLORS,
    scalar: 0.95,
    disableForReducedMotion: true,
  });
}

/** Side cannons for announcing winners on the Demo Day screen. */
export function celebrateWin(durationMs = 1800): void {
  if (reducedMotion()) return;
  const end = Date.now() + durationMs;
  const frame = () => {
    void confetti({
      particleCount: 5,
      angle: 60,
      spread: 60,
      origin: { x: 0, y: 0.75 },
      colors: COLORS,
    });
    void confetti({
      particleCount: 5,
      angle: 120,
      spread: 60,
      origin: { x: 1, y: 0.75 },
      colors: COLORS,
    });
    if (Date.now() < end) requestAnimationFrame(frame);
  };
  frame();
}
