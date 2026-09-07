// Fixed per-slot colors for a camera's groups. Index 0 -> group 1, etc. No longer user-chosen.
export const GROUP_SLOT_COLORS = ['#2f7d4f', '#2f5f9e', '#b0611e', '#a68a1b'];

export function groupColor(index) {
  return GROUP_SLOT_COLORS[index % GROUP_SLOT_COLORS.length];
}
