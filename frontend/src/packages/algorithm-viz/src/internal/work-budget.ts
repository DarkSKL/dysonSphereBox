/** 全场共享的操作预算；使用：一帧内队列、快照与退场共用；边界：保留顺序与全部操作，超额留到下一帧。 */
export class FrameWorkBudget {
  private remaining: number;
  private readonly deadline: number;
  private checks = 0;

  constructor(limit = 1024, milliseconds = 4) {
    this.remaining = limit;
    this.deadline = performance.now() + milliseconds;
  }

  get exhausted(): boolean { return this.remaining <= 0; }

  /** 每十六项复核墙钟，限制计时成本；单个操作的执行时间无法在中途抢占。 */
  take(): boolean {
    if (this.remaining <= 0) return false;
    if ((this.checks++ & 15) === 0 && performance.now() >= this.deadline) { this.remaining = 0; return false; }
    this.remaining--;
    return true;
  }
}
