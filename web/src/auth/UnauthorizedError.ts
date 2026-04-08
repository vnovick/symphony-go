/**
 * Thrown by authedFetch / authedEventStream when the server returns 401.
 * TanStack Query's retry guard checks `instanceof UnauthorizedError` to
 * avoid retrying failed-auth requests.
 */
export class UnauthorizedError extends Error {
  constructor(message = 'unauthorized') {
    super(message);
    this.name = 'UnauthorizedError';
  }
}
