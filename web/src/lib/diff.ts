function sameValue(left: unknown, right: unknown): boolean {
  if (Array.isArray(left) && Array.isArray(right)) {
    return left.length === right.length && left.every((item, index) => sameValue(item, right[index]));
  }
  return Object.is(left, right);
}

export function changedFields<T extends object>(original: T, draft: T): Partial<T> {
  const patch: Partial<T> = {};
  for (const key of Object.keys(draft) as (keyof T)[]) {
    if (!sameValue(original[key], draft[key])) {
      patch[key] = draft[key];
    }
  }
  return patch;
}

export function isDirty<T extends object>(original: T, draft: T): boolean {
  return Object.keys(changedFields(original, draft)).length > 0;
}
