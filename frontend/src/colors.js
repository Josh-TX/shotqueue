// Swatch palette for a camera's groups. Keep in sync with GroupColors in backend/internal/state/state.go.
// The first 4 are the legacy per-slot colors, used for groups with no stored color (old configs).
export const GROUP_COLORS = ['#389660', '#3a77c4', '#e88a24', '#e0c029', '#9a68cc', '#35b0b0', '#8a5a3c', '#d055a0'];

export const MAX_GROUPS = 8;

export function groupColor(group, index) {
  return group?.color || GROUP_COLORS[index % 4];
}
