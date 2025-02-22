""" Small demo to capture 10s of data on the bbtkv3
    Time-stamp: 2025/02/20  <christophe@pallier.org>
"""

import sys
import time
import serial

PORT = "/dev/ttyUSB0"
BAUDRATE = 115200
CRLF = '\r\n'
WT = 0.05  # waiting time (50 ms) between writes on serial port

bbtk = serial.Serial(port=PORT,
                    baudrate=BAUDRATE,
                    bytesize=serial.EIGHTBITS,
                    parity=serial.PARITY_NONE,
                    stopbits=serial.STOPBITS_ONE,
                    timeout=1,
                    xonxoff=False,
                    rtscts=False,
                    dsrdtr=False,
                    writeTimeout=1)

def send_command(cmd):
        """write ``cmd`` on the serial port, then send a CR+LF.

        :param cmd: command to send
        :type cmd: string
        """
        bbtk.write((cmd + CRLF).encode())
        time.sleep(WT)


def read_line():
        """Read a single line from the bbtkv2 (blocking! No timeout)

        :return: the string of characters, without the '\n'.
        """
        time.sleep(WT)
        s = ''
        c = bbtk.read().decode('ascii')

        while c != '\n':
            if c != '\n':
                s = s + c
            c = bbtk.read().decode()
        return (s)


def get_response(timeout=5):
        """Accumulates characters from the serial channel, for a certain duration.

        :param timeout: time during which to wait for messages from the bbtkv2.

        :return:  the text sent by the BBTLv2.
        """
        last_time = time.time()
        data_str = ""
       
        while (bbtk.is_open and ((time.time() - last_time) < timeout)):
            if (bbtk.in_waiting > 0):
                chars = bbtk.read(bbtk.in_waiting).decode('ascii')
                data_str += chars
                last_time = time.time()
            time.sleep(0.05)

        return data_str

# %%
print('Connecting...')
send_command('CONN')
x = read_line()
if x == 'BBTK;':
    print('ok')
else:
    print('Did not get the expected response (BBTK;). Got:', x)
    sys.exit(1)
    
print('Display info on the bbtk screen')
send_command('ABOU')

print('Fimware version:', end="")
send_command('FIRM')
print(read_line())

print('Thresholds:', end="")
send_command('GEPV')
print(read_line())

print('Clear Memory')
send_command('SPIE')
time.sleep(5)
x = read_line()
if x != 'ESEC;':
    print('Did not get the expected response (ESEC;). Got:', x)
    print('Capture will probably not work. Trying nevertheless')

print("Start capture for 10s")
send_command('DSCM')
send_command('TIML')
send_command('10000000')
send_command('RUDS')
time.sleep(15)
print(get_response(5))




# %%
bbtk.close()
# %%
