// Swatch palette for a camera's groups. Keep in sync with GroupColors in backend/internal/state/state.go.
// The first 4 are the legacy per-slot colors, used for groups with no stored color (old configs).
export const GROUP_COLORS = ['#2f7d4f', '#2f5f9e', '#b0611e', '#a68a1b', '#7a4fa3', '#2a8a8a', '#8a5a3c', '#a8467e'];

export const MAX_GROUPS = 8;

export function groupColor(group, index) {
  return group?.color || GROUP_COLORS[index % 4];
}
