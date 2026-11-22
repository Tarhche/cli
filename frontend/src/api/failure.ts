/**
 * What went wrong, as the desktop's services say it: the cause of the error
 * a call rejects with (desktop.Failure in Go).
 */
export type Failure = {
  message: string;
  status?: number;
  code?: string;
  fields?: Record<string, string>;
  logged_out?: boolean;
  unavailable?: boolean;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/** The failure an error a call rejected with carries. */
export function failureOf(error: unknown): Failure {
  if (isRecord(error)) {
    const cause = error.cause;
    if (isRecord(cause) && typeof cause.message === "string") {
      return cause as Failure;
    }

    if (typeof error.message === "string") {
      return {message: error.message};
    }
  }

  return {message: String(error)};
}

/** Whether an error is a call that was cancelled, which is no failure. */
export function isCancelled(error: unknown): boolean {
  return isRecord(error) && (error.name === "CancelError" || error.name === "CancelledRejectionError");
}

/** What a form shows of a failure: beside its fields, and the rest above. */
export type FormFailure = {
  /** What was refused of the fields the form has, by field. */
  fields: Record<string, string>;

  /** Everything else that was said, to be shown once. */
  messages: string[];
};

/**
 * Lays a failure out on a form: what was refused of a field it shows goes
 * beside it, by the form's own field name that fieldOf answers for each of
 * the failure's; anything else, and the failure's message when it refused
 * no field, is said above the form.
 */
export function formFailure(failure: Failure | null, fieldOf: (field: string) => string | undefined): FormFailure {
  const laidOut: FormFailure = {fields: {}, messages: []};
  if (failure === null) {
    return laidOut;
  }

  const refused = Object.entries(failure.fields ?? {});
  if (refused.length === 0) {
    laidOut.messages.push(failure.message);

    return laidOut;
  }

  for (const [field, message] of refused) {
    const shown = fieldOf(field);
    if (shown === undefined) {
      laidOut.messages.push(`${field}: ${message}`);
    } else if (laidOut.fields[shown] === undefined) {
      laidOut.fields[shown] = message;
    }
  }

  return laidOut;
}
