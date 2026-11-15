/** 按生成任务记录资源使用代次；回放和重复通知不推进代次。 */
export class GenerationCache<K> {
  private taskId = '';
  private generation = 0;
  private readonly uses = new Map<K, number>();

  begin(taskId: string): boolean {
    if (!taskId || taskId === this.taskId) return false;
    this.taskId = taskId;
    this.generation++;
    return true;
  }

  touch(key: K): void {
    this.uses.set(key, this.generation);
  }

  /** 仅返回早于上一次任务的资源；仍在显示的资源由调用方延后释放。 */
  stale(): K[] {
    return Array.from(this.uses).filter(([, used]) => used < this.generation - 1).map(([key]) => key);
  }

  forget(key: K): void {
    this.uses.delete(key);
  }

  clear(): void {
    this.uses.clear();
    this.taskId = '';
    this.generation = 0;
  }
}
