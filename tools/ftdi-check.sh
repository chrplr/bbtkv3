#!/usr/bin/env bash
#
# ftdi-check.sh -- diagnose the USB link to an FTDI-based device (BBTK v3, ...)
#
# Answers one question: is the problem the physical USB link, or the software?
# Run it before debugging any of the cmd/ tools that "cannot find the BBTK".
#
# A failing USB cable does not look like a failing cable: the device still
# enumerates far enough for the kernel to create /dev/ttyUSB0 and bind
# ftdi_sio, so it appears connected.  The giveaway is that enumeration gets
# through the device and configuration descriptors and then stalls -- the
# string descriptors (manufacturer/product/serial) never arrive and every
# subsequent control transfer returns -EPIPE, which dmesg reports as:
#
#     ftdi_sio ttyUSB0: failed to get modem status: -32
#     ftdi_sio ttyUSB0: usb_serial_generic_read_bulk_callback - urb stopped: -32
#
# That is enough signal integrity to start enumerating and not enough to
# finish.  The cable supplied with the BBTK v3 failed exactly this way; a
# standard USB-B printer cable fixed it.
#
# Usage: tools/ftdi-check.sh [port]
#        port defaults to $BBTK_PORT, then /dev/ttyUSB0.

PORT="${1:-${BBTK_PORT:-/dev/ttyUSB0}}"

D=$(grep -ls '^0403$' /sys/bus/usb/devices/*/idVendor 2>/dev/null | head -1 | xargs -r dirname)
if [ -z "$D" ]; then
    echo "FAIL: no FTDI (0403:*) device enumerated -- nothing is plugged in,"
    echo "      or the device is not powered."
    exit 1
fi

echo "=== device: $(basename "$D")  $(cat "$D/idVendor"):$(cat "$D/idProduct")  bcdDevice=$(cat "$D/bcdDevice")"
echo "    speed=$(cat "$D/speed")M  bMaxPower=$(cat "$D/bMaxPower")"

echo "=== string descriptors (absent => enumeration did not complete)"
missing=0
for f in manufacturer product serial; do
    if [ -f "$D/$f" ]; then
        printf '    %-13s: %s\n' "$f" "$(cat "$D/$f")"
    else
        printf '    %-13s: MISSING\n' "$f"
        missing=1
    fi
done

echo "=== node"
if [ -e "$PORT" ]; then
    ls -l "$PORT" 2>&1 | sed 's/^/    /'
    [ -r "$PORT" ] && [ -w "$PORT" ] || echo "    WARNING: not readable/writable -- are you in the 'dialout' group?"
else
    echo "    $PORT does not exist"
fi

echo "=== TIOCMGET x5 (real control transfer; EIO == dmesg 'failed to get modem status: -32')"
python3 - "$PORT" <<'EOF'
import fcntl, termios, struct, os, sys, time
port = sys.argv[1]
names = {0x002: 'LE', 0x004: 'DTR', 0x008: 'RTS',
         0x020: 'CTS', 0x040: 'DCD', 0x080: 'RNG', 0x100: 'DSR'}
ok = 0
for i in range(5):
    try:
        fd = os.open(port, os.O_RDWR | os.O_NOCTTY | os.O_NONBLOCK)
    except OSError as e:
        print('    %d open failed: %s' % (i, e))
        continue
    try:
        v = struct.unpack('I', fcntl.ioctl(fd, termios.TIOCMGET, struct.pack('I', 0)))[0]
        print('    %d ok: 0x%03x -> %s' % (i, v, [n for b, n in names.items() if v & b]))
        ok += 1
    except OSError as e:
        print('    %d TIOCMGET failed: %s' % (i, e))
    finally:
        os.close(fd)
    time.sleep(0.2)
print('    --> %d/5 succeeded' % ok)
sys.exit(0 if ok == 5 else 1)
EOF
ctl=$?

echo "=== USB traffic (urbnum should increase)"
before=$(cat "$D/urbnum")
timeout 8 python3 - "$PORT" <<'EOF'
import sys, time
try:
    import serial
except ImportError:
    print('    (pyserial not installed -- skipping the data I/O probe)')
    sys.exit(0)
try:
    s = serial.Serial(sys.argv[1], 115200, timeout=1)
    time.sleep(0.2)
    s.reset_input_buffer()
    s.write(b'\r\n')
    s.flush()
    time.sleep(0.3)
    print('    read after CRLF: %r' % s.read(64))
    s.close()
except Exception as e:
    print('    ERROR: %s: %s' % (type(e).__name__, e))
EOF
after=$(cat "$D/urbnum")
echo "    urbnum: $before -> $after"

echo
if [ "$missing" = 0 ] && [ "$ctl" = 0 ]; then
    echo "VERDICT: USB link healthy (enumeration complete, control transfers OK)."
    echo "         Any remaining silence is device or protocol level, not the link,"
    echo "         so debugging the cmd/ tools is now worthwhile."
    exit 0
else
    echo "VERDICT: USB link FAULTY (incomplete enumeration and/or stalled control"
    echo "         transfers).  Do not debug the code.  Swap the USB cable first,"
    echo "         then try a port that is not behind a dock or hub."
    exit 1
fi
