#!/usr/bin/env python3
import math
import shutil
import subprocess
import time
from pathlib import Path

ROOT = Path('/opt/ai-workbench')
DATA_DIR = ROOT / 'prometheus-data'
IMPORT_DIR = ROOT / 'prometheus-import' / 'fake-categraf-future'
OPENMETRICS = IMPORT_DIR / 'fake-categraf-future.om'
ULIDS_FILE = IMPORT_DIR / 'imported-ulids.txt'
STEP = 15
HOURS = 24
START = int(time.time()) - 300
END = int(time.time()) + HOURS * 3600
HOSTS = {
    '198.18.20.11': {'role': 'app', 'ident': 'biz-app-198.18.20.11', 'hostname': 'app-198-18-20-11'},
    '198.18.20.12': {'role': 'app', 'ident': 'biz-app-198.18.20.12', 'hostname': 'app-198-18-20-12'},
    '198.18.20.20': {'role': 'edge', 'ident': 'biz-edge-198.18.20.20', 'hostname': 'edge-198-18-20-20'},
    '198.18.22.11': {'role': 'oracle', 'ident': 'biz-oracle-198.18.22.11', 'hostname': 'oracle-198-18-22-11'},
    '198.18.22.12': {'role': 'oracle', 'ident': 'biz-oracle-198.18.22.12', 'hostname': 'oracle-198-18-22-12'},
    '198.18.22.13': {'role': 'oracle', 'ident': 'biz-oracle-198.18.22.13', 'hostname': 'oracle-198-18-22-13'},
}
ROLE_BASE = {'app': 0.9, 'edge': 1.7, 'oracle': 2.6}
GAUGE = 'gauge'
COUNTER = 'counter'
types = {}
lines = []

def esc(v):
    return str(v).replace('\\', '\\\\').replace('"', '\\"').replace('\n', '\\n')

def labels(base, extra=None):
    data = dict(base)
    if extra:
        data.update(extra)
    return '{' + ','.join(f'{k}="{esc(data[k])}"' for k in sorted(data)) + '}'

def sample(metric, typ, base, value, ts, extra=None):
    if metric not in types:
        types[metric] = typ
        lines.append(f'# TYPE {metric} {typ}')
    if typ == COUNTER and value < 0:
        value = 0
    lines.append(f'{metric}{labels(base, extra)} {value:.6f} {ts}.000')

def wave(idx, period, amp=1.0, phase=0.0):
    return math.sin(idx / period * 2 * math.pi + phase) * amp

def clamp(v, lo, hi):
    return max(lo, min(hi, v))

def counter(base, idx, rate, jitter=0.0):
    return base + idx * STEP * rate + max(0, wave(idx, 360, jitter))

def host_base(ip, cfg):
    return {'ident': cfg['ident'], 'instance': ip, 'hostname': cfg['hostname'], 'job': 'categraf', 'os': 'linux', 'role': cfg['role']}

def add_basic(ip, cfg, idx, ts):
    b = host_base(ip, cfg)
    role = cfg['role']
    rb = ROLE_BASE[role]
    cpu = clamp(24 + rb*8 + wave(idx, 720, 10, rb), 4, 92)
    iowait = clamp(1.2 + rb + wave(idx, 540, 1.1, rb), 0.1, 18)
    system = clamp(cpu * 0.22, 1, 30)
    user = clamp(cpu - system - iowait, 1, 80)
    idle = clamp(100 - cpu, 2, 96)
    for name, val in {
        'cpu_usage_active': cpu, 'cpu_usage_idle': idle, 'cpu_usage_user': user, 'cpu_usage_system': system,
        'cpu_usage_iowait': iowait, 'cpu_usage_softirq': 0.8 + rb, 'cpu_usage_steal': 0.2,
        'cpu_usage_irq': 0.1, 'cpu_usage_nice': 0.0,
    }.items():
        sample(name, GAUGE, b, val, ts)
    cores = 8 if role != 'oracle' else 16
    mem_total = (32 if role != 'oracle' else 64) * 1024**3
    mem_pct = clamp(48 + rb*8 + wave(idx, 800, 7, rb), 18, 92)
    mem_used = mem_total * mem_pct / 100
    mem_avail = mem_total - mem_used
    for name, val in {
        'mem_total': mem_total, 'mem_used': mem_used, 'mem_available': mem_avail, 'mem_free': mem_avail * 0.45,
        'mem_cached': mem_total * 0.16, 'mem_buffered': mem_total * 0.03, 'mem_slab': mem_total * 0.02,
        'mem_used_percent': mem_pct, 'mem_available_percent': 100 - mem_pct,
        'swap_total': 8 * 1024**3, 'swap_used': 8 * 1024**3 * (0.04 + rb*0.01),
        'swap_free': 8 * 1024**3 * (0.95 - rb*0.01), 'swap_used_percent': 4 + rb,
    }.items():
        sample(name, GAUGE, b, val, ts)
    load1 = clamp(cpu / 100 * cores * (0.8 + rb*0.08) + wave(idx, 240, 0.4), 0, cores*2)
    sample('system_load1', GAUGE, b, load1, ts)
    sample('system_load5', GAUGE, b, load1 * 0.92, ts)
    sample('system_load15', GAUGE, b, load1 * 0.85, ts)
    sample('system_n_cpus', GAUGE, b, cores, ts)
    sample('system_uptime', GAUGE, b, 864000 + idx * STEP, ts)
    sample('system_n_users', GAUGE, b, 2 if role != 'oracle' else 4, ts)
    sample('kernel_context_switches', COUNTER, b, counter(2_000_000, idx, 500 + rb*80), ts)
    sample('kernel_processes_forked', COUNTER, b, counter(50_000, idx, 0.2 + rb*0.03), ts)
    sample('kernel_vmstat_oom_kill', COUNTER, b, 0, ts)
    sample('linux_sysctl_fs_file_max', GAUGE, b, 922337, ts)
    sample('linux_sysctl_fs_inode_nr', GAUGE, b, 180000 + idx % 500, ts)
    sample('linux_sysctl_fs_dentry_unused_nr', GAUGE, b, 90000 + idx % 300, ts)
    for path, total_gb, used_base in [('/', 100, 45), ('/data', 500 if role != 'oracle' else 2000, 52 if role != 'oracle' else 68), ('/var/log', 80, 34)]:
        pct = clamp(used_base + rb*2 + wave(idx, 1200, 2.5, len(path)), 5, 94)
        total = total_gb * 1024**3
        used = total * pct / 100
        ext = {'path': path, 'fstype': 'xfs' if path != '/' else 'ext4'}
        sample('disk_total', GAUGE, b, total, ts, ext)
        sample('disk_used', GAUGE, b, used, ts, ext)
        sample('disk_free', GAUGE, b, total - used, ts, ext)
        sample('disk_used_percent', GAUGE, b, pct, ts, ext)
        sample('disk_inodes_total', GAUGE, b, 10_000_000, ts, ext)
        sample('disk_inodes_used', GAUGE, b, 10_000_000 * pct / 120, ts, ext)
        sample('disk_inodes_free', GAUGE, b, 10_000_000 * (1 - pct / 120), ts, ext)
        sample('disk_inodes_used_percent', GAUGE, b, pct / 1.2, ts, ext)
    for dev, mult in [('vda', 1.0), ('vdb', 3.0 if role == 'oracle' else 1.8)]:
        ext = {'device': dev}
        read_rate = (2_000_000 + rb*600_000) * mult
        write_rate = (1_200_000 + rb*500_000) * mult
        sample('diskio_read_bytes', COUNTER, b, counter(80_000_000, idx, read_rate), ts, ext)
        sample('diskio_write_bytes', COUNTER, b, counter(40_000_000, idx, write_rate), ts, ext)
        sample('diskio_reads', COUNTER, b, counter(100_000, idx, 120*mult), ts, ext)
        sample('diskio_writes', COUNTER, b, counter(80_000, idx, 90*mult), ts, ext)
        sample('diskio_io_util', GAUGE, b, clamp(18 + rb*7*mult + wave(idx, 300, 5), 1, 91), ts, ext)
        sample('diskio_io_await', GAUGE, b, clamp(2 + rb*1.4*mult + wave(idx, 380, 1.2), 0.2, 45), ts, ext)
        sample('diskio_read_time', COUNTER, b, counter(5000, idx, 0.5*mult), ts, ext)
        sample('diskio_write_time', COUNTER, b, counter(4000, idx, 0.4*mult), ts, ext)
    for iface, mult in [('eth0', 1.0), ('eth1', 0.35)]:
        ext = {'interface': iface}
        in_bits = (18_000_000 + rb*4_000_000 + wave(idx, 420, 5_000_000, rb)) * mult
        out_bits = (12_000_000 + rb*3_000_000 + wave(idx, 360, 4_000_000, rb)) * mult
        sample('net_bits_recv', GAUGE, b, max(0, in_bits), ts, ext)
        sample('net_bits_sent', GAUGE, b, max(0, out_bits), ts, ext)
        sample('net_bytes_recv', COUNTER, b, counter(4_000_000_000, idx, max(1, in_bits)/8), ts, ext)
        sample('net_bytes_sent', COUNTER, b, counter(2_500_000_000, idx, max(1, out_bits)/8), ts, ext)
        sample('net_packets_recv', COUNTER, b, counter(4_000_000, idx, 1200*mult + rb*100), ts, ext)
        sample('net_packets_sent', COUNTER, b, counter(3_000_000, idx, 900*mult + rb*100), ts, ext)
        sample('net_drop_in', COUNTER, b, counter(10, idx, 0.001*rb), ts, ext)
        sample('net_drop_out', COUNTER, b, counter(8, idx, 0.0015*rb), ts, ext)
        sample('net_err_in', COUNTER, b, counter(1, idx, 0.0002*rb), ts, ext)
        sample('net_err_out', COUNTER, b, counter(1, idx, 0.0002*rb), ts, ext)
        sample('net_speed', GAUGE, b, 10000, ts, ext)
    sample('netstat_sockets_used', GAUGE, b, 260 + rb*60 + wave(idx, 300, 15), ts)
    sample('netstat_tcp_inuse', GAUGE, b, 90 + rb*35 + wave(idx, 240, 18), ts)
    sample('netstat_tcp_tw', GAUGE, b, 35 + rb*15 + wave(idx, 280, 8), ts)
    sample('netstat_tcp_established', GAUGE, b, 90 + rb*35 + wave(idx, 240, 18), ts)
    sample('netstat_tcp_time_wait', GAUGE, b, 35 + rb*15 + wave(idx, 280, 8), ts)
    sample('netstat_tcp_alloc', GAUGE, b, 110 + rb*25, ts)
    sample('netstat_udp_inuse', GAUGE, b, 12 + rb*3, ts)
    sample('processes', GAUGE, b, 220 + rb*30, ts)
    sample('processes_running', GAUGE, b, 3 + rb + wave(idx, 180, 2), ts)
    sample('processes_sleeping', GAUGE, b, 200 + rb*24, ts)
    sample('processes_blocked', GAUGE, b, 1 + max(0, wave(idx, 900, 1)), ts)
    sample('processes_zombies', GAUGE, b, 0, ts)
    sample('processes_stopped', GAUGE, b, 0, ts)

def add_proc(ip, cfg, idx, ts, names):
    b = host_base(ip, cfg)
    for order, name in enumerate(names):
        ext = {'process_name': name, 'pattern': name}
        factor = 1 + order * 0.25
        sample('procstat_lookup_pid_count', GAUGE, b, 1 if name not in ['oracle_pmon', 'oracle_smon'] else 3, ts, ext)
        sample('procstat_cpu_usage', GAUGE, b, clamp(3 * factor + ROLE_BASE[cfg['role']] * 2 + wave(idx, 220, 2, order), 0.1, 75), ts, ext)
        sample('procstat_memory_rss', GAUGE, b, (250 * factor + ROLE_BASE[cfg['role']] * 180) * 1024**2, ts, ext)
        sample('procstat_memory_vms', GAUGE, b, (900 * factor + ROLE_BASE[cfg['role']] * 400) * 1024**2, ts, ext)
        sample('procstat_num_threads', GAUGE, b, 25 * factor + ROLE_BASE[cfg['role']] * 8, ts, ext)
        sample('procstat_num_fds', GAUGE, b, 80 * factor + ROLE_BASE[cfg['role']] * 30, ts, ext)
        sample('procstat_read_bytes', COUNTER, b, counter(10_000_000*factor, idx, 20_000*factor), ts, ext)
        sample('procstat_write_bytes', COUNTER, b, counter(8_000_000*factor, idx, 15_000*factor), ts, ext)

def add_jvm(ip, cfg, idx, ts):
    b = host_base(ip, cfg)
    app = {'application': 'order-service', 'port': '8081'}
    heap_max = 4096 * 1024**2
    heap_used = heap_max * clamp(0.48 + wave(idx, 520, 0.08), 0.25, 0.82)
    nonheap_max = 768 * 1024**2
    nonheap_used = nonheap_max * 0.62
    for area, used, maxv in [('heap', heap_used, heap_max), ('nonheap', nonheap_used, nonheap_max)]:
        ext = dict(app, area=area)
        sample('jvm_memory_used_bytes', GAUGE, b, used, ts, ext)
        sample('jvm_memory_max_bytes', GAUGE, b, maxv, ts, ext)
        sample('jvm_memory_committed_bytes', GAUGE, b, maxv * 0.9, ts, ext)
    sample('jvm_threads_live_threads', GAUGE, b, 118 + wave(idx, 300, 12), ts, app)
    sample('jvm_threads_daemon_threads', GAUGE, b, 76, ts, app)
    sample('jvm_threads_peak_threads', GAUGE, b, 150, ts, app)
    for gc in ['G1 Young Generation', 'G1 Old Generation']:
        ext = dict(app, gc=gc)
        rate = 0.03 if 'Young' in gc else 0.002
        sample('jvm_gc_pause_seconds_count', COUNTER, b, counter(1000, idx, rate), ts, ext)
        sample('jvm_gc_pause_seconds_sum', COUNTER, b, counter(70, idx, rate * 0.08), ts, ext)
    for uri, status, method, rps, latency in [('/api/order', '200', 'GET', 40, 0.06), ('/api/order', '500', 'GET', 0.2, 0.18), ('/api/pay', '200', 'POST', 8, 0.12)]:
        ext = dict(app, uri=uri, status=status, method=method)
        sample('http_server_requests_seconds_count', COUNTER, b, counter(10_000, idx, rps), ts, ext)
        sample('http_server_requests_seconds_sum', COUNTER, b, counter(700, idx, rps*latency), ts, ext)

def add_edge(ip, cfg, idx, ts):
    b = host_base(ip, cfg)
    nginx = {'port': '80', 'server': 'biz-nginx'}
    sample('nginx_up', GAUGE, b, 1, ts, nginx)
    sample('nginx_active', GAUGE, b, 260 + wave(idx, 280, 35), ts, nginx)
    sample('nginx_reading', GAUGE, b, 5 + wave(idx, 180, 2), ts, nginx)
    sample('nginx_writing', GAUGE, b, 30 + wave(idx, 220, 8), ts, nginx)
    sample('nginx_waiting', GAUGE, b, 220 + wave(idx, 260, 25), ts, nginx)
    sample('nginx_accepts', COUNTER, b, counter(1_000_000, idx, 60), ts, nginx)
    sample('nginx_handled', COUNTER, b, counter(1_000_000, idx, 60), ts, nginx)
    sample('nginx_requests', COUNTER, b, counter(2_000_000, idx, 140), ts, nginx)
    redis = {'addr': '198.18.20.20:6379'}
    sample('redis_up', GAUGE, b, 1, ts, redis)
    sample('redis_connected_clients', GAUGE, b, 86 + wave(idx, 310, 12), ts, redis)
    sample('redis_blocked_clients', GAUGE, b, 0, ts, redis)
    sample('redis_used_memory', GAUGE, b, 3.4 * 1024**3 + wave(idx, 480, 150*1024**2), ts, redis)
    sample('redis_used_memory_peak', GAUGE, b, 4.1 * 1024**3, ts, redis)
    sample('redis_mem_fragmentation_ratio', GAUGE, b, 1.18 + wave(idx, 600, 0.04), ts, redis)
    sample('redis_keyspace_hitrate', GAUGE, b, 98.4 + wave(idx, 540, 0.4), ts, redis)
    sample('redis_keyspace_hits', COUNTER, b, counter(5_000_000, idx, 1200), ts, redis)
    sample('redis_keyspace_misses', COUNTER, b, counter(40_000, idx, 8), ts, redis)
    sample('redis_expired_keys', COUNTER, b, counter(5000, idx, 0.8), ts, redis)
    sample('redis_evicted_keys', COUNTER, b, counter(20, idx, 0.002), ts, redis)
    sample('redis_rejected_connections', COUNTER, b, 0, ts, redis)
    sample('redis_instantaneous_ops_per_sec', GAUGE, b, 1800 + wave(idx, 260, 260), ts, redis)

def add_oracle(ip, cfg, idx, ts):
    b = host_base(ip, cfg)
    ora = {'sid': 'ORCL', 'port': '1521'}
    sample('oracle_up', GAUGE, b, 1, ts, ora)
    sample('oracle_sessions_value', GAUGE, b, 260 + wave(idx, 330, 45), ts, ora)
    sample('oracle_process_count', GAUGE, b, 180 + wave(idx, 360, 20), ts, ora)
    sample('oracle_cache_hit_ratio_value', GAUGE, b, 96.5 + wave(idx, 700, 0.8), ts, ora)
    sample('oracle_lock_cnt', GAUGE, b, max(0, 2 + wave(idx, 900, 3)), ts, ora)
    sample('oracle_deadlock_total', COUNTER, b, 0, ts, ora)
    sample('oracle_redo_generated_bytes_total', COUNTER, b, counter(8_000_000_000, idx, 1_200_000 + ROLE_BASE['oracle']*100_000), ts, ora)
    sample('oracle_logical_reads_total', COUNTER, b, counter(30_000_000, idx, 1200), ts, ora)
    sample('oracle_physical_reads_total', COUNTER, b, counter(3_000_000, idx, 120), ts, ora)
    for tbs, pct in [('SYSTEM', 62), ('USERS', 71), ('UNDOTBS1', 55), ('DATA', 74)]:
        ext = dict(ora, tablespace=tbs)
        sample('oracle_tablespace_used_percent', GAUGE, b, pct + wave(idx, 1600, 1.2), ts, ext)
        sample('oracle_tablespace_size_bytes', GAUGE, b, (100 if tbs != 'DATA' else 800) * 1024**3, ts, ext)
        sample('oracle_tablespace_free_bytes', GAUGE, b, (100 if tbs != 'DATA' else 800) * 1024**3 * (100-pct)/100, ts, ext)
    for event, waits in [('db file sequential read', 15), ('log file sync', 4), ('CPU', 40)]:
        sample('oracle_wait_event_total', COUNTER, b, counter(100_000, idx, waits), ts, dict(ora, event=event))

def generate():
    IMPORT_DIR.mkdir(parents=True, exist_ok=True)
    for idx, ts in enumerate(range(START, END + 1, STEP)):
        for ip, cfg in HOSTS.items():
            add_basic(ip, cfg, idx, ts)
            names = ['categraf', 'catpaw']
            if cfg['role'] == 'app':
                names += ['java', 'order-service']
                add_jvm(ip, cfg, idx, ts)
            elif cfg['role'] == 'edge':
                names += ['nginx', 'redis-server']
                add_edge(ip, cfg, idx, ts)
            else:
                names += ['oracle', 'tnslsnr', 'oracle_pmon', 'oracle_smon']
                add_oracle(ip, cfg, idx, ts)
            add_proc(ip, cfg, idx, ts, names)
    lines.append('# EOF')
    OPENMETRICS.write_text('\n'.join(lines) + '\n')

def remove_old_blocks():
    if not ULIDS_FILE.exists():
        return
    for ulid in ULIDS_FILE.read_text().splitlines():
        ulid = ulid.strip()
        if not ulid:
            continue
        target = DATA_DIR / ulid
        if target.exists() and target.is_dir():
            shutil.rmtree(target)
    ULIDS_FILE.unlink(missing_ok=True)

def create_blocks():
    blocks_dir = IMPORT_DIR / 'blocks'
    if blocks_dir.exists():
        shutil.rmtree(blocks_dir)
    blocks_dir.mkdir(parents=True)
    subprocess.run(['promtool', 'tsdb', 'create-blocks-from', 'openmetrics', str(OPENMETRICS), str(blocks_dir)], check=True)
    ulids = sorted([p.name for p in blocks_dir.iterdir() if p.is_dir()])
    if not ulids:
        raise SystemExit('no blocks generated')
    remove_old_blocks()
    for ulid in ulids:
        dst = DATA_DIR / ulid
        if dst.exists():
            shutil.rmtree(dst)
        shutil.move(str(blocks_dir / ulid), str(dst))
    ULIDS_FILE.write_text('\n'.join(ulids) + '\n')
    print(f'Imported {len(ulids)} blocks into {DATA_DIR}')
    print(f'Samples file: {OPENMETRICS} ({OPENMETRICS.stat().st_size/1024/1024:.1f} MiB)')
    print(f'Time range: {START} -> {END}, step={STEP}s, hosts={len(HOSTS)}')
    print('ULIDs: ' + ', '.join(ulids))

if __name__ == '__main__':
    generate()
    create_blocks()
