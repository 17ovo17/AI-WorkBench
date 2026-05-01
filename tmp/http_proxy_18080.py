from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import http.client
TARGET_HOST='localhost'
TARGET_PORT=8080
class Proxy(BaseHTTPRequestHandler):
    protocol_version='HTTP/1.1'
    def do_GET(self): self.forward()
    def do_POST(self): self.forward()
    def do_PUT(self): self.forward()
    def do_DELETE(self): self.forward()
    def log_message(self, fmt, *args): pass
    def forward(self):
        length=int(self.headers.get('Content-Length','0') or 0)
        body=self.rfile.read(length) if length else None
        headers={k:v for k,v in self.headers.items() if k.lower() not in ('host','connection','proxy-connection','accept-encoding')}
        conn=http.client.HTTPConnection(TARGET_HOST,TARGET_PORT,timeout=30)
        try:
            conn.request(self.command,self.path,body=body,headers=headers)
            resp=conn.getresponse()
            data=resp.read()
            self.send_response(resp.status,resp.reason)
            for k,v in resp.getheaders():
                if k.lower() not in ('transfer-encoding','connection','content-length'):
                    self.send_header(k,v)
            self.send_header('Content-Length',str(len(data)))
            self.end_headers()
            self.wfile.write(data)
        except Exception as e:
            msg=str(e).encode('utf-8','replace')
            self.send_response(502)
            self.send_header('Content-Length',str(len(msg)))
            self.end_headers(); self.wfile.write(msg)
        finally:
            conn.close()
ThreadingHTTPServer(('0.0.0.0',18080),Proxy).serve_forever()

