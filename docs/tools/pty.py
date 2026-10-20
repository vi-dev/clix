import os, sys, pty, fcntl, termios, struct, subprocess, select
cols = int(sys.argv[1]); args = sys.argv[3:]
m, s = pty.openpty()
fcntl.ioctl(s, termios.TIOCSWINSZ, struct.pack('HHHH', 24, cols, 0, 0))
p = subprocess.Popen(args, stdin=s, stdout=subprocess.DEVNULL, stderr=s)
os.close(s)
out = b''
while True:
    r, _, _ = select.select([m], [], [], 0.05)
    if r:
        try:
            d = os.read(m, 65536)
        except OSError:
            break
        if not d: break
        out += d
    elif p.poll() is not None:
        break
p.wait()
sys.stdout.buffer.write(out)
sys.stderr.write("exit %d\n" % p.returncode)
