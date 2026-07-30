# TODO functionnalities to add to bbtkv3


## Generating events with event generation


Start by setting thresholds and clearing memory
``` 
# set thresholds
SEPV
0
0
127
127
0
0
0
0

# Clear memory  (wait for DONE;)
SPIE
```

```

PREG
TIML
4000000 # 4 sec in microsecond
1000000,1500000,0000011  # onset, offset, port value (on)
1500000,2000000,0000000
2000000,2500000,0000011
2500000,3000000,0000000
PECG
RUEG
#  then timing data is streamed back to PC 
# in the same format as digital stimulus capture (DSCM)
# (SDAT / Number of events/ Duration /Samples/ datalines/ EDAT)
# with 20 bits datalines: 12 input lines (all set to 0)+ 8 ouput lines
```

Timings are in microseconds

The 8 bits for output ports correspond to:

`act4,act3,act2,act1,ttlout2,ttlout1,sounder2,sounder1`

Note: for the Elite BBTK, there are 16 output lines. The command is PRET


## Generatin Event Pulse Train (EGPT)



```
PRPT
TIML
0   # infinite duration
100,500,00000100   # on  period
100,500,00000000   # off period
PCPT
RUPT
```
Timings are in ms



## Digital Stimulus Response Echo (DSRE)

This mode also to send a signal on a TTLout line when an event is detected on an input line.

```
PDCR
STYP
PATT
TIML
0
000000010000,999999999999,999999999999,1,00000100,10
...
PCCR
RUSR
```
:
The format of  data lines is 

trigger1,trigger2,trigger3,RT,portout,DURATION


triggers are defined as 12 bits masks:

keypad4, keypad3, keypad2, keypad1, Opto4, Opto3, Opto2, Opto1, TTLin2, TTLin1, Mic2, Mic1


When one of the triggers is detected, a pulse of DURATION ms will be sent RT msec later, on output ports defined by portout (8 bits):

The 8 bits for output ports correspond to:

`act4,act3,act2,act1,ttlout2,ttlout1,sounder2,sounder1`




## Digital Stimulus Capture and Response (DSCAR)

Same principle as Digital Echo above, but allows to define individual events to vary RT, and will save timing data.


## Real-time event marking (EM)

The event marking functionnality allows to send 


```
PDCE
STYP
PATT
TIML
0
00000001000000000000,0000010000000000
00000000000100000000,0000100000000000
99999999999999999999,9999999999999999
99999999999999999999,9999999999999999
99999999999999999999,9999999999999999
99999999999999999999,9999999999999999
99999999999999999999,9999999999999999
99999999999999999999,9999999999999999
PCCR
RUEM
```

event=20 lines (all last 8 set to 0)
mask=16 liens (all last 8 set to 0)

# Input Line Check

The command is:

```
ICHK
```

Each time an input line change occurs, a 12 bit input port value is returned by the BBTK



# Output Line Check

```
OCHK
# send 8 bits masks
00000001
00000000
```

