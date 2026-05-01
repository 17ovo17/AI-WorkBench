import socket, threading
LISTEN=('0.0.0.0',18080)
TARGET=('127.0.0.1',8080)
def pipe(src,dst):
    try:
        while True:
            data=src.recv(65536)
            if not data: break
            dst.sendall(data)
    except Exception:
        pass
    finally:
        try: src.close()
        except Exception: pass
        try: dst.close()
        except Exception: pass
def handle(c):
    try:
        t=socket.create_connection(TARGET,timeout=10)
    except Exception:
        c.close(); return
    threading.Thread(target=pipe,args=(c,t),daemon=True).start()
    threading.Thread(target=pipe,args=(t,c),daemon=True).start()
s=socket.socket(); s.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1); s.bind(LISTEN); s.listen(100)
print('proxy listening', LISTEN, '->', TARGET, flush=True)
while True:
    c,a=s.accept(); threading.Thread(target=handle,args=(c,),daemon=True).start()
