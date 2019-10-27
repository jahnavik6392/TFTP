Getting started:
1. Enable TFTP on windows.
2. Build the project using go build.
3. Run the project executable - which gets generated after successful go build (i.e. tftp.exe) from command line.
4. Fire the get and put TFTP commands from the command line. eg: 

>tftp -i 127.0.0.1 put C:\Go\src\archive\tar\common.go

o/p: Transfer successful: 24295 bytes in 1 second(s), 24295 bytes/s

>tftp -i 127.0.0.1 get common.go

o/p: Transfer successful: 24295 bytes in 1 second(s), 24295 bytes/s
