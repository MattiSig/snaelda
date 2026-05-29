type AnyRecord = Record<string, unknown>;

export function setAtPath(
  source: AnyRecord,
  path: ReadonlyArray<string | number>,
  value: unknown,
): AnyRecord {
  if (path.length === 0) {
    if (value && typeof value === 'object' && !Array.isArray(value)) {
      return value as AnyRecord;
    }
    return source;
  }

  const [head, ...rest] = path;
  const next = Array.isArray(source)
    ? [...(source as unknown[])]
    : { ...(source as AnyRecord) };

  if (typeof head === 'number' || /^\d+$/.test(String(head))) {
    const index = Number(head);
    const array = Array.isArray(next) ? next : [];
    while (array.length <= index) array.push(undefined);
    array[index] =
      rest.length === 0
        ? value
        : setAtPath(
            (array[index] as AnyRecord) ?? {},
            rest,
            value,
          );
    return array as unknown as AnyRecord;
  }

  const key = String(head);
  const record = next as AnyRecord;
  if (rest.length === 0) {
    if (value === undefined) {
      delete record[key];
    } else {
      record[key] = value;
    }
    return record;
  }

  const child = record[key];
  record[key] =
    child && typeof child === 'object'
      ? setAtPath(child as AnyRecord, rest, value)
      : setAtPath({}, rest, value);
  return record;
}

export function cloneProps<T>(value: T): T {
  return JSON.parse(JSON.stringify(value ?? {})) as T;
}
