/** The FMS status codes, in the order a radio's keypad shows them. */
export const fmsStatusCodes = [1, 2, 3, 4, 5, 6, 7, 8, 9, 0] as const;

/** The color of an FMS status: available (1, 2) green, on an assignment
 * (3, 4, 7, 8) orange, requests to talk (5, 0) red, not available (6) grey. */
export function fmsStatusColor(code: number): 'success' | 'warning' | 'error' | 'default' | 'info' {
  switch (code) {
    case 1:
    case 2:
      return 'success';
    case 3:
    case 4:
    case 7:
    case 8:
      return 'warning';
    case 0:
    case 5:
      return 'error';
    case 6:
      return 'default';
    default:
      return 'info';
  }
}
