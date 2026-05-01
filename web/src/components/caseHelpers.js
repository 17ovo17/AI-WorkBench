// 知识库案例分类与配置常量
export const CATEGORIES = [
  'cpu_high', 'memory_leak', 'disk_full', 'inode_exhausted', 'io_saturation',
  'network_loss', 'tcp_timewait', 'load_high', 'mysql_slow_query', 'mysql_pool_exhausted',
  'redis_memory_full', 'redis_connection', 'oom', 'zombie_process', 'fd_exhausted'
]

export const categoryType = (cat) => {
  if (['cpu_high', 'load_high', 'memory_leak', 'oom'].includes(cat)) return 'danger'
  if (['disk_full', 'inode_exhausted', 'fd_exhausted', 'network_loss', 'tcp_timewait', 'io_saturation'].includes(cat)) return 'warning'
  if (cat?.startsWith?.('mysql_') || cat?.startsWith?.('redis_')) return 'primary'
  return 'info'
}
