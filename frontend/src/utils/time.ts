export const APP_TIME_ZONE =
  // eslint-disable-next-line @iceworks/best-practices/recommend-polyfill
  Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';

type DateTimeValue = string | number | Date | null | undefined;

// eslint-disable-next-line @iceworks/best-practices/recommend-polyfill
const DATE_TIME_FORMATTER = new Intl.DateTimeFormat('en-CA', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hour12: false,
});

// eslint-disable-next-line @iceworks/best-practices/recommend-polyfill
const DATE_FORMATTER = new Intl.DateTimeFormat('en-CA', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
});

export function formatDateTimeDisplay(value: DateTimeValue, fallback = '-'): string {
  const structured = parseStructuredDateTime(value);
  if (structured) {
    return `${structured.date} ${structured.time}`;
  }

  const timestamp = normalizeTimestamp(value);
  if (timestamp === null) {
    return fallback;
  }
  return formatDateTimeFromTimestamp(timestamp);
}

export function formatDateDisplay(value: DateTimeValue, fallback = '-'): string {
  const structured = parseStructuredDateTime(value);
  if (structured) {
    return structured.date;
  }

  const timestamp = normalizeTimestamp(value);
  if (timestamp === null) {
    return fallback;
  }
  return formatDateFromTimestamp(timestamp);
}

export function toDateTimeLocalInputValue(value: DateTimeValue, fallback = ''): string {
  const structured = parseStructuredDateTime(value);
  if (structured) {
    return `${structured.date}T${structured.time.slice(0, 5)}`;
  }

  const timestamp = normalizeTimestamp(value);
  if (timestamp === null) {
    return fallback;
  }
  return formatDateTimeFromTimestamp(timestamp).replace(' ', 'T').slice(0, 16);
}

export function dateTimeLocalInputToISOString(value: string | null | undefined): string | undefined {
  if (!value?.trim()) {
    return undefined;
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return undefined;
  }
  return parsed.toISOString();
}

function normalizeTimestamp(value: DateTimeValue): number | null {
  if (value instanceof Date) {
    return Number.isNaN(value.getTime()) ? null : value.getTime();
  }
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value;
  }
  if (typeof value !== 'string') {
    return null;
  }

  const trimmed = value.trim();
  if (!trimmed) {
    return null;
  }
  if (/^\d+$/.test(trimmed)) {
    const parsedNumber = Number(trimmed);
    return Number.isFinite(parsedNumber) ? parsedNumber : null;
  }
  if (parseStructuredDateTime(trimmed)) {
    return null;
  }

  const parsedTime = Date.parse(trimmed);
  return Number.isNaN(parsedTime) ? null : parsedTime;
}

function parseStructuredDateTime(value: DateTimeValue): { date: string; time: string } | null {
  if (typeof value !== 'string') {
    return null;
  }

  const trimmed = value.trim();
  if (!trimmed || /z$/i.test(trimmed) || /[+-]\d{2}:?\d{2}$/.test(trimmed)) {
    return null;
  }

  const match = trimmed.match(
    /^(\d{4}-\d{2}-\d{2})(?:[T\s](\d{2}:\d{2})(?::(\d{2}))?)?$/,
  );
  if (!match) {
    return null;
  }
  return {
    date: match[1],
    time: `${match[2] || '00:00'}:${match[3] || '00'}`,
  };
}

function formatDateTimeFromTimestamp(timestamp: number): string {
  const parts = DATE_TIME_FORMATTER.formatToParts(new Date(timestamp));
  const partMap: Record<string, string> = {};
  parts.forEach((part) => {
    if (part.type !== 'literal') {
      partMap[part.type] = part.value;
    }
  });
  return `${partMap.year}-${partMap.month}-${partMap.day} ${partMap.hour}:${partMap.minute}:${partMap.second}`;
}

function formatDateFromTimestamp(timestamp: number): string {
  const parts = DATE_FORMATTER.formatToParts(new Date(timestamp));
  const partMap: Record<string, string> = {};
  parts.forEach((part) => {
    if (part.type !== 'literal') {
      partMap[part.type] = part.value;
    }
  });
  return `${partMap.year}-${partMap.month}-${partMap.day}`;
}
