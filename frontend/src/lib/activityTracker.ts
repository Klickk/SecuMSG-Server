type ActivityTrackerOptions = {
  idleMs: number;
  onIdle: () => void;
};

export class ActivityTracker {
  private idleMs: number;
  private onIdle: () => void;
  private timer: number | null = null;
  private active = false;

  constructor(options: ActivityTrackerOptions) {
    this.idleMs = options.idleMs;
    this.onIdle = options.onIdle;
  }

  start(): void {
    if (this.active || typeof window === "undefined") {
      return;
    }
    this.active = true;
    const events = ["mousemove", "keydown", "click", "touchstart", "scroll"];
    events.forEach((event) => {
      window.addEventListener(event, this.handleActivity, { passive: true });
    });
    this.resetTimer();
  }

  stop(): void {
    if (!this.active || typeof window === "undefined") {
      return;
    }
    this.active = false;
    const events = ["mousemove", "keydown", "click", "touchstart", "scroll"];
    events.forEach((event) => {
      window.removeEventListener(event, this.handleActivity);
    });
    if (this.timer) {
      window.clearTimeout(this.timer);
      this.timer = null;
    }
  }

  private handleActivity = (): void => {
    this.resetTimer();
  };

  private resetTimer(): void {
    if (typeof window === "undefined") {
      return;
    }
    if (this.timer) {
      window.clearTimeout(this.timer);
    }
    this.timer = window.setTimeout(() => {
      this.onIdle();
    }, this.idleMs);
  }
}
