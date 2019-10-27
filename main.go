// tftp project main.go
package main

import (
	"net"
	logs "tftp/TFTPLogger"
	handleReq "tftp/requestHandler"
	wire "tftp/wire"
	"time"
)

func worker(id int, jobs <-chan handleReq.TftpRequest) {
	for tftpRequest := range jobs {
		logs.GetApplicationLogger().Println("worker", id, "started job", tftpRequest.Addr)
		if tftpRequest.LenOfBuffer > 516 {
			logs.GetApplicationLogger().Printf("Received Request with length larger than 516 from %v, worker id %v", tftpRequest.Addr, id)
			logs.GetRequestLogger().Printf("Received Unknown/Illegal request from %v, worker id %v ignoring because of large length\n", tftpRequest.Addr, id)
			continue
		}
		var opcode uint16
		var err error
		if opcode, _, err = wire.ParseUint16(tftpRequest.Buff); err != nil {
			logs.GetApplicationLogger().Fatalln("failed to parse the buffer ", err)
			return
		}

		switch opcode {
		case wire.OpRRQ:
			//Making entry to testlogfile where we are storing the input requests that are coming in.
			startTime := time.Now()
			logs.GetApplicationLogger().Printf("Received READ request from %v , worker id %v\n", tftpRequest.Addr, id)
			logs.GetRequestLogger().Printf("Received READ request from %v, worker id %v\n", tftpRequest.Addr, id)

			err := handleReq.HandleReadRequest(tftpRequest)
			logs.GetApplicationLogger().Printf("READ request from %v, worker id %v, success=%v, timetaken=%v, err=%v\n", tftpRequest.Addr, id, (err == nil), time.Now().Sub(startTime), err)
		case wire.OpWRQ:
			startTime := time.Now()
			logs.GetApplicationLogger().Printf("Received WRITE request from %v, worker id %v", tftpRequest.Addr, id)
			logs.GetRequestLogger().Printf("Received WRITE request from %v, worker id %v\n", tftpRequest.Addr, id)
			err := handleReq.HandleWriteRequest(tftpRequest)
			logs.GetApplicationLogger().Printf("Received WRITE request from %v, worker id %v, success=%v, timetaken=%v err=%v\n", tftpRequest.Addr, id, (err == nil), time.Now().Sub(startTime), err)

		case wire.OpData:
		case wire.OpAck:
		case wire.OpError:
		default:
			logs.GetRequestLogger().Printf("Received Unknown/Illegal request from %v, worker id %v, opcode %v\n", tftpRequest.Addr, id, opcode)
			logs.Error("Received Unknown/Illegal request from %v, worker id %v, opcode %v\n", tftpRequest.Addr, id, opcode)
			return
		}
		logs.GetApplicationLogger().Println("worker", id, "worker job", tftpRequest.Addr)
	}
}

//entry point
func main() {
	//Set up loggers for the application
	logs.Initialize()
	defer logs.DestroyLogger()

	logs.GetApplicationLogger().Println("TFTP Application!")

	//Handling multithreading by creating workers
	//create a channel of TFTP requests
	jobs := make(chan handleReq.TftpRequest, 100) //make 100 configurable

	//create workers
	for w := 1; w <= 3; w++ { //make 3 configurable
		go worker(w, jobs)
	}

	//Start listening on a port
	service := "0.0.0.0" + ":" + "70" //make this configurable

	logs.GetApplicationLogger().Println("Service is: ", service)

	udpAddress, err := net.ResolveUDPAddr("udp4", service)
	if err != nil {
		logs.GetApplicationLogger().Fatalln(err)
	}
	//Setup listener for incoming UDP connection
	ServerConnection, err := net.ListenUDP("udp", udpAddress)
	if err != nil {
		logs.GetApplicationLogger().Fatalln(err)
	}
	logs.GetApplicationLogger().Println("UDP server is up ")
	defer ServerConnection.Close()

	//Wait for requests to come in. If request comes in, then spawn a thread and call the connection handler.
	buffer := make([]byte, wire.MaxPacketSize)
	for {
		n, address, _ := ServerConnection.ReadFromUDP(buffer)
		var request handleReq.TftpRequest
		request.Buff = buffer
		request.LenOfBuffer = n
		request.Addr = address.String()
		request.Port = address.Port
		request.IP = address.IP
		jobs <- request
	}
}
