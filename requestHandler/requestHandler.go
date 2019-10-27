package requestHandler

import (
	"errors"
	"fmt"
	"math"
	"net"
	logs "tftp/TFTPLogger"
	cache "tftp/cache"
	wire "tftp/wire"
	"time"
)

//global variables
const INITIAL_BLOCKNUM int = 0
const READ_TIMEOUT int = 10
const BLOCK_SIZE = 516
const DALLY_TIME = 3
const NUM_RETRIES = 3
const DATA_SIZE = 512

//TFTP request
type TftpRequest struct {
	Buff        []byte
	LenOfBuffer int
	Addr        string
	Port        int
	IP          net.IP
}

//Function to create error packet and serialize it
func CreateErrorPacket(errorCode uint16, errorMessage string) (packetErrByte []byte) {
	var packetError wire.PacketError
	packetError.Code = errorCode
	packetError.Msg = errorMessage
	packetErrorAck := packetError.Serialize()
	return packetErrorAck
}

//Function to create error packet and serialize it
func CreateAckPacket(blockNum uint16) (packetErrByte []byte) {
	var packetAck wire.PacketAck
	packetAck.BlockNum = blockNum
	packetAcknowledgement := packetAck.Serialize()
	return packetAcknowledgement
}

//Function to get the file name from the packet request buffer
func GetFileNameFromPacketRequestBuffer(buffer []byte) (fileName string) {
	pkt := wire.PacketRequest{}
	err := pkt.Parse(buffer)
	if err != nil {
		logs.GetApplicationLogger().Fatalln("failed to parse to get the fileName ", err)
	}
	var name = string(pkt.Filename)
	logs.GetApplicationLogger().Println("fileName is ", name)
	return name
}

func GetPacketRequest(buffer []byte) (packetRequest *wire.PacketRequest, err error) {
	pkt := wire.PacketRequest{}
	err = pkt.Parse(buffer)
	if err != nil {
		return
	}
	packetRequest = &pkt
	return
}

func getOpCode(buffer []byte) (opcode uint16, err error) {
	if len(buffer) > BLOCK_SIZE {
		err = errors.New("Expected packet size to be less than " + fmt.Sprint(BLOCK_SIZE))
		return
	}
	opcode, _, err = wire.ParseUint16(buffer)
	if err != nil {
		return
	}
	if opcode > 5 {
		err = errors.New("unexpected opcode " + fmt.Sprint(opcode))
	}
	return
}

func getPacketError(buffer []byte) (packetErr *wire.PacketError, err error) {
	var opcode uint16
	if opcode, _, err = wire.ParseUint16(buffer); err != nil {
		return
	}

	if opcode != wire.OpError {
		err = errors.New("expecting OpError")
		return
	}
	var packetErrTmp = wire.PacketError{}
	packetErrTmp.Parse(buffer)
	packetErr = &packetErrTmp
	return
}

//Function to handle the read request
func HandleReadRequest(req TftpRequest) (err error) {
	/*
		1.Establish the connection
		2.Get the file name i.e. key
		3.Get the value from the map for the given file name (i.e.key)
		4.If entry from cache is null then handle file not found exception and close the connection
		5.Send the value as packets of size 512 i.e. create a PacketData, populate it and write it to the connection.
			5.1.Retry for a couple of times to get the acknowledgement.
				5.1.1 Check if the port num is the same for which we are trying to send the data
				5.1.2 If port number mismatch exists, then send error packet back
				5.1.3.If no mismatch exists, continue sending next data packets.
	*/
	//Establishing the connection
	Conn, _ := net.DialUDP("udp", nil, &net.UDPAddr{IP: req.IP, Port: req.Port, Zone: ""})
	defer Conn.Close()

	var byteArrFromCache []byte
	var blockNum uint16 = 1
	var byteCountStart int

	nextbuffer := make([]byte, wire.MaxPacketSize)

	//logic to extract the file name
	var fileName = GetFileNameFromPacketRequestBuffer(req.Buff)
	logs.GetApplicationLogger().Println("fileName is ", fileName)

	Conn.SetReadDeadline(time.Now().Add(time.Duration(READ_TIMEOUT) * time.Second))

	//logic to get the entry from the map(i.e. in-memory)
	byteArrFromCache = cache.Read(fileName)

	//handle file not found
	if byteArrFromCache == nil {
		//no entry in cache for given file name
		Conn.Write(CreateErrorPacket(uint16(1), "File not found."))
		return errors.New("file not found " + fileName)
	}

	var lengthOfCacheData = len(byteArrFromCache) //length of value
	Conn.SetReadDeadline(time.Now().Add(time.Duration(READ_TIMEOUT) * time.Second))
	for byteCountStart = 0; byteCountStart <= lengthOfCacheData; byteCountStart += DATA_SIZE {

		//send byteArrFromCache in form of packets
		var minVal float64 = math.Min(float64(byteCountStart+DATA_SIZE), float64(lengthOfCacheData))
		logs.GetApplicationLogger().Println("Length of value entry in cache is ", lengthOfCacheData)
		logs.GetApplicationLogger().Println("Minimum value is ", minVal)

		//logic to create the packetData ,populate it and write to the connection
		var packetData wire.PacketData
		packetData.BlockNum = blockNum
		packetData.Data = byteArrFromCache[byteCountStart:int(minVal)]
		pktData := packetData.Serialize()
		Conn.Write(pktData)

		var retryCount int

		for retryCount = 0; retryCount <= NUM_RETRIES; retryCount++ {
			//logic to check for acknowledgements

			n, address, err := Conn.ReadFromUDP(nextbuffer)
			if err != nil {
				return err
			}
			//check if the port num is the same or not. Else, bail out.
			logs.GetApplicationLogger().Println("Address Port number: ", address.Port)
			logs.GetApplicationLogger().Println("Request Port number: ", req.Port)

			if address.Port != req.Port {
				logs.Error("Port number mismatch!")
				Conn.WriteToUDP(CreateErrorPacket(uint16(5), "Unknown transfer ID."), address)
			}

			opCode, err := getOpCode(nextbuffer[0:n])
			if err != nil {
				logs.Error("%v", err)
				continue
			}
			if opCode == wire.OpError {
				packetErr, err := getPacketError(nextbuffer[0:n])
				if err != nil {
					return err
				}
				return errors.New("Received Error code" + fmt.Sprint(packetErr.Code) + " msg " + packetErr.Msg)
			}
			var packetAck wire.PacketAck
			ackErr := packetAck.Parse(nextbuffer)
			if ackErr != nil {
				logs.GetApplicationLogger().Println("ackErr", ackErr)
			}
			//break statement
			if blockNum == packetAck.BlockNum {
				break
			}
			if NUM_RETRIES == retryCount {
				//we did not get the acknowledgment back even after few retries
				return errors.New("retries exceeded.")
			}
			var error1 error
			_, _, error1 = Conn.ReadFromUDP(nextbuffer)
			if error1 == nil {
				//check for socket time outs
				//send back the data packet again
				Conn.Write(pktData)
			}
		}
		blockNum++
	}
	return nil
}

//Function to handle the write request
func HandleWriteRequest(req TftpRequest) (err error) {
	/* Logic:
	1.Establishing the connection.
	2.Send first acknowledgement for block 0 i.e. to indicate a connection has been established.
	4.In a loop read from the buffer value  i.e. we want a data packet in the buffer.
		4.1.Check if the port num is the same for which we are trying to send the ack
		4.2.If port number doesn't match, then we are sending error packet to the sender who sent the data packet.
		4.3.If port num match,
			4.3.1.if block size is 516 then,
				4.3.1.1. if the block number already exists, then send acknowledgement again
						 i.e.indicating that we want the next packet
				4.3.1.2. if the block number doesn't exists, then prepare the entry for cache and send the acknowledgement
			4.3.2.if block size is less than 516 then,write to cache, send acknowledgement, dally around and if we
				  don't receive a error back, it means the final acknowledgement has been lost. So, resend the final ack.

	*/

	packetRequest, err := GetPacketRequest(req.Buff)
	if err != nil {
		return err
	}

	//Establishing the connection
	Conn, _ := net.DialUDP("udp", nil, &net.UDPAddr{IP: req.IP, Port: req.Port, Zone: ""})
	defer Conn.Close()

	//checking if mode is octet or not
	if packetRequest.Mode != "octet" {
		if packetRequest.Mode == "netascii" || packetRequest.Mode == "mail" {
			packetErr := CreateErrorPacket(0, "fileMode "+packetRequest.Mode+" not supported")
			Conn.Write(packetErr)
			return errors.New("unsupported fileMode " + packetRequest.Mode)
		}
		packetErr := CreateErrorPacket(0, "unknown fileMode "+packetRequest.Mode)
		Conn.Write(packetErr)
		return errors.New("unknown fileMode " + packetRequest.Mode)
	}

	//Send first acknowledgement for block 0 i.e. to indicate a connection has been established
	Conn.Write(CreateAckPacket(uint16(INITIAL_BLOCKNUM)))

	nextbuffer := make([]byte, wire.MaxPacketSize)

	//Reading from the buffer
	Conn.SetReadDeadline(time.Now().Add(time.Duration(READ_TIMEOUT) * time.Second))
	var counter int = 0
	var slice []byte
	for {

		n, address, err := Conn.ReadFromUDP(nextbuffer)
		if err != nil {
			return err
		}

		//check if the port num is the same for which we are trying to send. Only, then continue the operation.
		if address.Port == req.Port {
			logs.GetApplicationLogger().Println("Port number match")
			opCode, err := getOpCode(nextbuffer[0:n])
			if err != nil {
				logs.Error("%v ", err)
				continue
			} else if opCode == wire.OpError {
				packetErr, err := getPacketError(nextbuffer[0:n])
				if err != nil {
					return err
				}
				return errors.New(packetErr.Msg + fmt.Sprint(packetErr.Code))

			} else if opCode != wire.OpData {
				continue
			}
			p1 := wire.PacketData{}
			err = p1.Parse(nextbuffer[0:n])
			if err != nil {
				logs.Error("err while parsing datapacket %v", err)
				continue
			}
			if n == BLOCK_SIZE {
				logs.GetApplicationLogger().Println("Block size is 516")
				if int(p1.BlockNum) <= counter {
					//send acknowledgement back for the same block num again - indicating that i want the data for the next block
					logs.GetApplicationLogger().Printf("Received a block number %d that is already existing! Hence, resending the acknowledgement again!\n", p1.BlockNum)
					Conn.Write(CreateAckPacket(uint16(p1.BlockNum)))
				} else {
					//cache this value and increement the counter
					counter++
					logs.GetApplicationLogger().Println("Block size is 516 and the received a right block number")
					//creating a temporary slice - which we are going to persist only when we recieve the last block
					slice = append(slice, p1.Data...)
					logs.GetApplicationLogger().Println("Acknowledgement for data packet with block number: ", p1.BlockNum)
					Conn.Write(CreateAckPacket(uint16(p1.BlockNum)))
				}

			} else if n < BLOCK_SIZE {
				logs.GetApplicationLogger().Println("Block size is less than 516 for block num: ", p1.BlockNum)
				//this is the last block
				slice = append(slice, p1.Data...)

				//logic to get the file name which is used for insert into map
				var fileName = packetRequest.Filename

				//writing to cache
				err := cache.Write(fileName, slice)
				if err != nil {
					Conn.Write(CreateErrorPacket(uint16(6), ""))
					return err
				}

				//As, we received the last packet we are sending the acknowledgement
				Conn.Write(CreateAckPacket(uint16(p1.BlockNum)))

				//dally around
				time.Sleep(DALLY_TIME * time.Second)

				//check if final ack was lost and resend it.
				var error1 error
				_, _, error1 = Conn.ReadFromUDP(nextbuffer)
				if error1 == nil {
					//send back the final acknowledgement again
					Conn.Write(CreateAckPacket(uint16(p1.BlockNum)))
				}
				break
			}
		} else {
			logs.Error("Port number mismatch!")
			Conn.WriteToUDP(CreateErrorPacket(uint16(5), ""), address)
		}
	}
	return nil
}
