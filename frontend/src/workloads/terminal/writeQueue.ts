import {joined} from "./bytes";

/**
 * Sends what is typed into a terminal one write at a time, in the order it
 * was typed: whatever is typed while a write is on its way goes in the next
 * one, together. Calls to Go run in no particular order, so two writes on
 * their way at once could arrive the wrong way round.
 */
export class WriteQueue {
  private pending: Uint8Array[] = [];
  private sending = false;

  /**
   * @param send sends one write, and settles once it arrived.
   * @param failed is told why a write failed; what follows is still sent.
   */
  constructor(
    private readonly send: (data: Uint8Array) => Promise<unknown>,
    private readonly failed: (error: unknown) => void,
  ) {}

  /** Sends data after everything pushed before it. */
  push(data: Uint8Array): void {
    if (data.length === 0) {
      return;
    }

    this.pending.push(data);
    if (!this.sending) {
      void this.drain();
    }
  }

  /** Whether nothing is waiting to be sent, nor on its way. */
  get idle(): boolean {
    return !this.sending && this.pending.length === 0;
  }

  private async drain(): Promise<void> {
    this.sending = true;

    while (this.pending.length > 0) {
      const batch = joined(this.pending.splice(0));

      try {
        await this.send(batch);
      } catch (error) {
        this.failed(error);
      }
    }

    this.sending = false;
  }
}
