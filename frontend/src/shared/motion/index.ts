/**
 * Animation features for `<LazyMotion features={loadMotionFeatures} strict>`:
 * screens animate with the light `m.*` components and fetch the engine only
 * when they render.
 */
export const loadMotionFeatures = () => import('./features').then((mod) => mod.default);
