#!/usr/bin/env python3
"""生成 Categraf 风格模拟数据，输出 OpenMetrics 文件供 promtool 导入。"""
import time, random, math, sys, os

HOSTS = [
    {"ident": "clims-nginx-198.18.20.20", "ip": "198.18.20.20", "app": "clims", "role": "nginx"},
    {"ident": "clims-jvm-198.18.20.11", "ip": "198.18.20.11", "app": "clims", "role": "jvm"},
    {"ident": "clims-jvm-198.18.20.12", "ip": "198.18.20.12", "app": "clims", "role": "jvm"},
    {"ident": "clims-oracle-198.18.22.11", "ip": "198.18.22.11", "app": "clims", "role": "oracle"},
    {"ident": "clims-oracle-198.18.22.12", "ip": "198.18.22.12", "app": "clims", "role": "oracle"},
    {"ident": "clims-oracle-198.18.22.13", "ip": "198.18.22.13", "app": "clims", "role": "oracle"},
]

def g(base, j=5, h=0):
    return round(base + math.sin(h/24*math.pi*2)*j + random.uniform(-j, j), 2)

def gen(host, ts, h):
    lb = f'ident="{host["ident"]}",ip="{host["ip"]}",app="{host["app"]}"'
    out = []
    def a(n, v, e=""):
        out.append(f'{n}{{{lb+(","+e if e else "")}}} {v} {ts}')
    orc = host["role"] == "oracle"
    cb = 42 if orc else (18 if host["role"]=="nginx" else 28)
    mb = 76 if orc else (45 if host["role"]=="nginx" else 55)
    a("cpu_usage_active", g(cb,8,h))
    a("cpu_usage_idle", round(100-g(cb,8,h),2))
    a("cpu_usage_system", g(cb*0.22,3,h))
    a("cpu_usage_user", g(cb*0.65,4,h))
    a("cpu_usage_iowait", g(4.5 if orc else 1.2,2,h))
    a("cpu_usage_softirq", g(1.8 if host["role"]=="nginx" else 0.5,0.5,h))
    a("mem_used_percent", g(mb,5,h))
    a("mem_available_percent", round(100-g(mb,5,h),2))
    tm = 68719476736 if orc else 16106127360
    a("mem_used", round(tm*g(mb,5,h)/100))
    a("mem_available", round(tm*(1-g(mb,5,h)/100)))
    a("mem_cached", round(tm*random.uniform(0.1,0.2)))
    a("swap_used_percent", g(6 if orc else 2,3,h))
    for mp,dv in ([("/","vda1"),("/data","vdb1")] if orc else [("/","vda1")]):
        e=f'path="{mp}",device="{dv}"'
        bp=71 if mp=="/data" else 52
        a("disk_used_percent",g(bp,3,h),e)
        t=2147483648000 if mp=="/data" else 107374182400
        a("disk_total",t,e)
    for dv in (["vda","vdb"] if orc else ["vda"]):
        e=f'device="{dv}"'
        ib=68 if dv=="vdb" and orc else 25
        a("diskio_io_util",g(ib,10,h),e)
        a("diskio_io_await",g(14 if dv=="vdb" else 5,4,h),e)
        a("diskio_read_bytes",round(random.uniform(1e6,15e6)),e)
        a("diskio_write_bytes",round(random.uniform(5e5,10e6)),e)
    for ifc in (["eth0","eth1"] if orc else ["eth0"]):
        e=f'interface="{ifc}"'
        a("net_bits_recv",round(random.uniform(5e6,30e6)),e)
        a("net_bits_sent",round(random.uniform(3e6,25e6)),e)
        a("net_drop_in",round(random.uniform(0,200)),e)
        a("net_drop_out",round(random.uniform(0,300)),e)
        a("net_err_in",round(random.uniform(0,40)),e)
        a("net_err_out",round(random.uniform(0,40)),e)
    a("netstat_tcp_inuse",round(random.uniform(100,250)))
    a("netstat_tcp_tw",round(random.uniform(30,120)))
    a("netstat_sockets_used",round(random.uniform(300,600)))
    a("system_load1",g(6.5 if orc else 2.5,2,h))
    a("system_load5",g(6.0 if orc else 2.2,1.5,h))
    a("system_load15",g(5.5 if orc else 2.0,1,h))
    a("system_n_cpus",16 if orc else 8)
    a("kernel_context_switches",round(random.uniform(3e7,6e7)))
    a("kernel_vmstat_oom_kill",0)
    a("processes",round(random.uniform(200,400)))
    a("processes_zombies",0)
    a("processes_blocked",round(random.uniform(0,3)))
    a("linux_sysctl_fs_file_max",1048576)
    if orc:
        a("oracle_up",1)
        a("oracle_buffer_cache_hit_ratio",g(95.7,2,h))
        a("oracle_process_count",g(189,20,h))
        a("oracle_sessions",g(305,30,h))
        a("oracle_tablespace_used_percent",g(65,8,h),'tablespace="USERS"')
    return out

def main():
    f = sys.argv[1] if len(sys.argv)>1 else "/tmp/categraf-data.om"
    now = int(time.time())
    end = now + 5*86400
    step = 60
    print(f"生成 {len(HOSTS)} 台主机 5 天数据, 间隔 {step}s -> {f}")
    with open(f, "w") as fp:
        for ts in range(now, end, step):
            h = (ts%86400)/3600
            for host in HOSTS:
                for line in gen(host, ts, h):
                    fp.write(line+"\n")
            if ts % 3600 < step:
                pct = (ts-now)/(end-now)*100
                print(f"  {pct:.0f}%", end="\r", flush=True)
        fp.write("# EOF\n")
    sz = os.path.getsize(f)/1048576
    print(f"\n完成! 文件 {sz:.0f} MB")

if __name__ == "__main__":
    main()
