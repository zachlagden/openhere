import type { FileInfo, Version } from "@/api/types";

export const FILE_RENDER_LIMIT = 200;

export function describeAuthor(createdBy: string | null, currentUserId: string | undefined): string {
  if (createdBy === null) {
    return "Unknown";
  }
  return createdBy === currentUserId ? "You" : "Another member";
}

export function versionLabel(version: Version): string {
  return version.id.length > 8 ? version.id.slice(0, 8) : version.id;
}

export function canRestore(version: Version): boolean {
  return version.status === "finalized" && !version.live;
}

export function canDelete(version: Version): boolean {
  return !version.live;
}

export function filterFiles(files: FileInfo[], query: string): FileInfo[] {
  const needle = query.trim().toLowerCase();
  if (needle === "") {
    return files;
  }
  return files.filter((file) => file.path.toLowerCase().includes(needle));
}
