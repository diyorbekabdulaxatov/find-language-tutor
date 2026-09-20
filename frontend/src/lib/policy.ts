/**
 * Marketplace policy constants the UI quotes before a booking exists. The
 * backend is the authority (a booking carries its own `cancellationPolicy`
 * with the exact deadline); these only feed the copy on the profile and the
 * checkout, so keep NEXT_PUBLIC_FREE_CANCEL_HOURS equal to the backend's
 * BOOKINGS_FREE_CANCEL_HOURS.
 */
export const FREE_CANCEL_HOURS = (() => {
  const raw = Number(process.env.NEXT_PUBLIC_FREE_CANCEL_HOURS);
  return Number.isFinite(raw) && raw > 0 ? raw : 24;
})();
