package requestHandler

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	logs "pbs/TFTPLogger"
	cache "pbs/cache"
	wire "pbs/wire"
	"reflect"
	"testing"
	"time"
)

//setting up the client connection
func setUp() (ClientConnection *net.UDPConn, err error) {
	stdoutWriter := io.Writer(os.Stdout)
	logs.InitializeWithRequestAndApplicationLogger(&stdoutWriter, &stdoutWriter)

	client := "127.0.0.1" + ":" + "50006"

	udpAddress, err := net.ResolveUDPAddr("udp4", client)
	if err != nil {
		return
	}
	//Setup listener for incoming UDP connection
	ClientConnection, err = net.ListenUDP("udp", udpAddress)
	return
}

func getDefaultTFTPRequest(buffer []byte) (tftpReq *TftpRequest) {
	tftpReq = &TftpRequest{
		buffer,
		len(buffer),
		"127.0.0.1:50006",
		50006,
		net.IP{127, 0, 0, 1},
	}
	return
}

func getPacketAck(buffer []byte) (packetAck *wire.PacketAck, err error) {
	var opcode uint16
	if opcode, _, err = wire.ParseUint16(buffer); err != nil {
		return
	}

	if opcode != wire.OpAck {
		fmt.Println("opcode found ", opcode)
		err = errors.New("expecting OpAck")
		return
	}
	var packetAckTmp = wire.PacketAck{}
	packetAckTmp.Parse(buffer)
	packetAck = &packetAckTmp
	return
}

//helper function
func getPacketData(buffer []byte) (packetData *wire.PacketData, err error) {
	var opcode uint16
	if opcode, _, err = wire.ParseUint16(buffer); err != nil {
		return
	}

	if opcode != wire.OpData {
		fmt.Println("opcode found ", opcode)
		err = errors.New("expecting OpData")
		return
	}

	p := wire.PacketData{}
	err = p.Parse(buffer)
	if err == nil {
		packetData = &p
	}
	return
}

//Test case for Write requests
//HappyCase with < 516 bytes file
//only 1 block with size less than 516
func TestHappyCaseWriteRequestWithLessThan516Bytes(t *testing.T) {
	ClientConnection, err := setUp()
	if err != nil {
		t.Errorf("Error received %v", err)
		return
	}
	defer ClientConnection.Close()
	request := TftpRequest{
		[]byte("\x00\x02foo\x00octet\x00"),
		30,
		"127.0.0.1:50006",
		50006,
		net.IP{127, 0, 0, 1},
	}

	go func(request TftpRequest) {
		HandleWriteRequest(request)
	}(request)
	buffer := make([]byte, 1024)
	n, address, _ := ClientConnection.ReadFromUDP(buffer)
	packetAck, err := getPacketAck(buffer[0:n])
	if err != nil {
		t.Errorf("Issues %v", err)
		return
	}
	if (*packetAck).BlockNum != 0 {
		t.Errorf("Expecting it to be blockNum 0 found %v", (*packetAck).BlockNum)
		return
	}

	//send packet to Port in the request
	packetData := wire.PacketData{BlockNum: 1, Data: []byte("fnord")}
	ClientConnection.WriteToUDP(packetData.Serialize(), address)
	//send the data to address.Port
	//check the ack
	n, _, _ = ClientConnection.ReadFromUDP(buffer)
	packetAck, err = getPacketAck(buffer[0:n])
	if packetAck.BlockNum != 1 {
		t.Errorf("Expecting it to be blockNum 1 found %v", packetAck.BlockNum)
		return
	}
}

//HappyCase with == 516 bytes file

//Test case for Write requests
//HappyCase with < 516 bytes file
//only 1 block with size exactly equal to 516
func TestHappyCaseWriteRequestWithEqualTo516Bytes(t *testing.T) {
	ClientConnection, err := setUp()
	if err != nil {
		t.Errorf("Error received %v", err)
		return
	}
	defer ClientConnection.Close()
	request := TftpRequest{
		[]byte("\x00\x02foo2\x00octet\x00"),
		30,
		"127.0.0.1:50006",
		50006,
		net.IP{127, 0, 0, 1},
	}

	go func(request TftpRequest) {
		HandleWriteRequest(request)
	}(request)
	buffer := make([]byte, 1024)
	n, address, _ := ClientConnection.ReadFromUDP(buffer)
	packetAck, err := getPacketAck(buffer[0:n])
	if err != nil {
		t.Errorf("Issues %v", err)
		return
	}
	if (*packetAck).BlockNum != 0 {
		t.Errorf("Expecting it to be blockNum 0 found %v", (*packetAck).BlockNum)
		return
	}

	//send packet to Port in the request
	dataPayload := make([]byte, 512)
	for i := 0; i < 512; i++ {
		dataPayload[i] = 'a'
	}
	packetData := wire.PacketData{BlockNum: 1, Data: dataPayload}
	ClientConnection.WriteToUDP(packetData.Serialize(), address)
	//send the data to address.Port
	//check the ack
	n, _, _ = ClientConnection.ReadFromUDP(buffer)
	packetAck, err = getPacketAck(buffer[0:n])
	if packetAck.BlockNum != 1 {
		t.Errorf("Expecting it to be blockNum 1 found %v", packetAck.BlockNum)
		return
	}
	//sending the last packet
	packetData = wire.PacketData{BlockNum: 2, Data: make([]byte, 0)}
	ClientConnection.WriteToUDP(packetData.Serialize(), address)
	n, _, err = ClientConnection.ReadFromUDP(buffer)
	//checking for connection error
	if err != nil {
		t.Errorf("Expected packetAck with block num 2 found %v", err)
		return
	}
	//packet parse error
	packetAck, err = getPacketAck(buffer[0:n])
	if err != nil {
		t.Errorf("Expected packetAck with block num 2 found %v", err)
		return
	}
	//checking actual error
	if packetAck.BlockNum != 2 {
		t.Errorf("Expecting it to be blockNum 2 found %v", packetAck.BlockNum)
		return
	}
	//file content is same as the cache entry
	if reflect.DeepEqual(dataPayload, cache.Read("foo2")) == false {
		t.Errorf("Data sent is not the same as stored %v expected : %v", len(cache.Read("foo2")), len(dataPayload))
		return
	}
}

// test to check if we are getting error when writing the filename which already exists in cache
func TestSadCaseForWriteRequestWhereFileExists(t *testing.T) {
	cache.Write("foo3", make([]byte, 0))

	ClientConnection, err := setUp()
	if err != nil {
		t.Errorf("Error received %v", err)
		return
	}
	defer ClientConnection.Close()
	request := TftpRequest{
		[]byte("\x00\x02foo3\x00octet\x00"),
		30,
		"127.0.0.1:50006",
		50006,
		net.IP{127, 0, 0, 1},
	}

	go func(request TftpRequest) {
		HandleWriteRequest(request)
	}(request)
	buffer := make([]byte, 1024)
	n, address, _ := ClientConnection.ReadFromUDP(buffer)
	packetAck, err := getPacketAck(buffer[0:n])
	if err != nil {
		t.Errorf("Issues %v", err)
		return
	}
	if (*packetAck).BlockNum != 0 {
		t.Errorf("Expecting it to be blockNum 0 found %v", (*packetAck).BlockNum)
		return
	}

	//send packet to Port in the request
	dataPayload := make([]byte, 512)
	for i := 0; i < 512; i++ {
		dataPayload[i] = 'a'
	}
	packetData := wire.PacketData{BlockNum: 1, Data: dataPayload}
	ClientConnection.WriteToUDP(packetData.Serialize(), address)
	//send the data to address.Port
	//check the ack
	n, _, _ = ClientConnection.ReadFromUDP(buffer)
	packetAck, err = getPacketAck(buffer[0:n])
	if packetAck.BlockNum != 1 {
		t.Errorf("Expecting it to be blockNum 1 found %v", packetAck.BlockNum)
		return
	}
	packetData = wire.PacketData{BlockNum: 2, Data: make([]byte, 0)}
	ClientConnection.WriteToUDP(packetData.Serialize(), address)
	n, _, err = ClientConnection.ReadFromUDP(buffer)
	if err != nil {
		t.Errorf("Expected packetAck with block num 2 found %v", err)
		return
	}
	packetErr, err := getPacketError(buffer[0:n])
	if err != nil {
		t.Errorf("Expected packetAck with block num 2 found %v", err)
		return
	}
	if packetErr.Code != 6 {
		t.Errorf("Expecting errcode to be 6 found %v", packetErr.Code)
		return
	}
}

//HappyCase with retransmission of packet (UDP network ack loss case)
//sending block1 twice to see if client is creating any issues
//and also checking if the content is the same
func TestHappyCaseForWriteRequestWithRetransmission(t *testing.T) {
	ClientConnection, err := setUp()
	if err != nil {
		t.Errorf("Error received %v", err)
		return
	}
	defer ClientConnection.Close()
	request := TftpRequest{
		[]byte("\x00\x02foo4\x00octet\x00"),
		30,
		"127.0.0.1:50006",
		50006,
		net.IP{127, 0, 0, 1},
	}

	go func(request TftpRequest) {
		HandleWriteRequest(request)
	}(request)
	buffer := make([]byte, 1024)
	n, address, _ := ClientConnection.ReadFromUDP(buffer)
	packetAck, err := getPacketAck(buffer[0:n])
	if err != nil {
		t.Errorf("Issues %v", err)
		return
	}
	if (*packetAck).BlockNum != 0 {
		t.Errorf("Expecting it to be blockNum 0 found %v", (*packetAck).BlockNum)
		return
	}

	//send packet to Port in the request
	dataPayload := make([]byte, 512)
	for i := 0; i < 512; i++ {
		dataPayload[i] = 'a'
	}
	packetData := wire.PacketData{BlockNum: 1, Data: dataPayload}
	ClientConnection.WriteToUDP(packetData.Serialize(), address)
	//send the data to address.Port
	//check the ack
	n, _, _ = ClientConnection.ReadFromUDP(buffer)
	packetAck, err = getPacketAck(buffer[0:n])
	if packetAck.BlockNum != 1 {
		t.Errorf("Expecting it to be blockNum 1 found %v", packetAck.BlockNum)
		return
	}
	//Send same data packet again
	packetData = wire.PacketData{BlockNum: 1, Data: dataPayload}
	ClientConnection.WriteToUDP(packetData.Serialize(), address)
	//send the data to address.Port
	//check the ack
	n, _, _ = ClientConnection.ReadFromUDP(buffer)
	packetAck, err = getPacketAck(buffer[0:n])
	if packetAck.BlockNum != 1 {
		t.Errorf("Expecting it to be blockNum 1 found %v", packetAck.BlockNum)
		return
	}

	packetData = wire.PacketData{BlockNum: 2, Data: make([]byte, 0)}
	ClientConnection.WriteToUDP(packetData.Serialize(), address)
	n, _, err = ClientConnection.ReadFromUDP(buffer)
	if err != nil {
		t.Errorf("Expected packetAck with block num 2 found %v", err)
		return
	}
	packetAck, err = getPacketAck(buffer[0:n])
	if err != nil {
		t.Errorf("Expected packetAck with block num 2 found %v", err)
		return
	}
	if packetAck.BlockNum != 2 {
		t.Errorf("Expecting it to be blockNum 2 found %v", packetAck.BlockNum)
		return
	}

	if reflect.DeepEqual(dataPayload, cache.Read("foo4")) == false {
		t.Errorf("Data sent is not the same as stored %v expected : %v", len(cache.Read("foo4")), len(dataPayload))
		return
	}
}

//Sad case with filemode invalid
//i.e. sending "bar" instead of "octet"
func TestSadCaseForWriteRequestWithFileModeIsInValid(t *testing.T) {
	ClientConnection, err := setUp()
	if err != nil {
		t.Errorf("Error received %v", err)
		return
	}
	defer ClientConnection.Close()
	request := TftpRequest{
		[]byte("\x00\x02foo4\x00bar\x00"),
		30,
		"127.0.0.1:50006",
		50006,
		net.IP{127, 0, 0, 1},
	}

	go func(request TftpRequest) {
		HandleWriteRequest(request)
	}(request)
	buffer := make([]byte, 1024)
	n, _, _ := ClientConnection.ReadFromUDP(buffer)
	packetError, err := getPacketError(buffer[0:n])
	if err != nil {
		t.Errorf("Issues %v", err)
		return
	}
	if packetError.Code != 0 {
		t.Errorf("Expecting 0 Error code found %v", packetError.Code)
	}
}

//Sad case with filemode valid but not supported
//i.e. mode is "netascii" and not "octet"
func TestSadCaseForWriteRequestWithFileModeIsNotSupported(t *testing.T) {
	ClientConnection, err := setUp()
	if err != nil {
		t.Errorf("Error received %v", err)
		return
	}
	defer ClientConnection.Close()
	request := TftpRequest{
		[]byte("\x00\x02foo4\x00netascii\x00"),
		30,
		"127.0.0.1:50006",
		50006,
		net.IP{127, 0, 0, 1},
	}

	go func(request TftpRequest) {
		HandleWriteRequest(request)
	}(request)
	buffer := make([]byte, 1024)
	n, _, _ := ClientConnection.ReadFromUDP(buffer)
	packetError, err := getPacketError(buffer[0:n])
	if err != nil {
		t.Errorf("Issues %v", err)
		return
	}
	if packetError.Code != 0 {
		t.Errorf("Expecting 0 Error code found %v", packetError.Code)
	}
}

//Sad case with no response from client for 10 seconds
//test -> after we receive the final data packet, we are waiting for 11 seconds and sending the ack.
//but, the worker should close it's connection
func TestSadCaseForWriteRequestWithTimeOut(t *testing.T) {
	ClientConnection, err := setUp()
	if err != nil {
		t.Errorf("Error received %v", err)
		return
	}
	defer ClientConnection.Close()
	request := TftpRequest{
		[]byte("\x00\x02foo2\x00octet\x00"),
		30,
		"127.0.0.1:50006",
		50006,
		net.IP{127, 0, 0, 1},
	}

	go func(request TftpRequest) {
		HandleWriteRequest(request)
	}(request)
	buffer := make([]byte, 1024)
	n, address, _ := ClientConnection.ReadFromUDP(buffer)
	packetAck, err := getPacketAck(buffer[0:n])
	if err != nil {
		t.Errorf("Issues %v", err)
		return
	}
	if (*packetAck).BlockNum != 0 {
		t.Errorf("Expecting it to be blockNum 0 found %v", (*packetAck).BlockNum)
		return
	}

	//send packet to Port in the request
	dataPayload := make([]byte, 512)
	for i := 0; i < 512; i++ {
		dataPayload[i] = 'a'
	}
	packetData := wire.PacketData{BlockNum: 1, Data: dataPayload}
	ClientConnection.WriteToUDP(packetData.Serialize(), address)
	//send the data to address.Port
	//check the ack
	n, _, _ = ClientConnection.ReadFromUDP(buffer)
	packetAck, err = getPacketAck(buffer[0:n])
	if packetAck.BlockNum != 1 {
		t.Errorf("Expecting it to be blockNum 1 found %v", packetAck.BlockNum)
		return
	}
	//waiting for 11 seconds before sending the last data packet
	packetData = wire.PacketData{BlockNum: 2, Data: make([]byte, 0)}
	time.Sleep(11 * time.Second)
	//we have already closed the thread after 10 seconds - so we won't get the acknowledgement
	ClientConnection.WriteToUDP(packetData.Serialize(), address)
	ClientConnection.SetReadDeadline(time.Now().Add(time.Duration(5) * time.Second))
	n, _, err = ClientConnection.ReadFromUDP(buffer)
	if err == nil {
		t.Errorf("Expecting socket timeout error")
		return
	}
}

//Malicious User sending invalid packets when expecting data packets
func TestSadCaseForWriteRequestWithPacketTampering(t *testing.T) {
	ClientConnection, err := setUp()
	if err != nil {
		t.Errorf("Error received %v", err)
		return
	}
	defer ClientConnection.Close()
	request := TftpRequest{
		[]byte("\x00\x02foo2\x00octet\x00"),
		30,
		"127.0.0.1:50006",
		50006,
		net.IP{127, 0, 0, 1},
	}

	go func(request TftpRequest) {
		HandleWriteRequest(request)
	}(request)
	buffer := make([]byte, 1024)
	n, address, _ := ClientConnection.ReadFromUDP(buffer)
	packetAck, err := getPacketAck(buffer[0:n])
	if err != nil {
		t.Errorf("Issues %v", err)
		return
	}
	if (*packetAck).BlockNum != 0 {
		t.Errorf("Expecting it to be blockNum 0 found %v", (*packetAck).BlockNum)
		return
	}

	//send packet to Port in the request
	dataPayload := make([]byte, 512)
	for i := 0; i < 512; i++ {
		dataPayload[i] = 'a'
	}
	packetData := wire.PacketData{BlockNum: 1, Data: dataPayload}
	ClientConnection.WriteToUDP(packetData.Serialize(), address)
	//send the data to address.Port
	//check the ack
	n, _, _ = ClientConnection.ReadFromUDP(buffer)
	packetAck, err = getPacketAck(buffer[0:n])
	if packetAck.BlockNum != 1 {
		t.Errorf("Expecting it to be blockNum 1 found %v", packetAck.BlockNum)
		return
	}
	ClientConnection.WriteToUDP([]byte("0xff0x090x88"), address) //opcode is invalid and even the data
	ClientConnection.SetReadDeadline(time.Now().Add(time.Duration(5) * time.Second))
	n, _, err = ClientConnection.ReadFromUDP(buffer)
	fmt.Println("buffer is ", buffer[0:n])
	if err == nil {
		t.Errorf("Expecting socket timeout error")
		return
	}
}

//Test with cache containing < 516 bytes of data
func TestHappyCaseForReadRequestWithLessThan516Bytes(t *testing.T) {
	ClientConnection, err := setUp()
	if err != nil {
		t.Errorf("Error received %v", err)
		return
	}
	dataPayload := make([]byte, 511)
	for i := 0; i < 511; i++ {
		dataPayload[i] = 'a'
	}
	cache.Write("foo10", dataPayload)
	defer ClientConnection.Close()
	request := getDefaultTFTPRequest([]byte("\x00\x01foo10\x00octet\x00"))

	go func(request TftpRequest) {
		HandleReadRequest(request)
	}(*request)
	buffer := make([]byte, 1024)
	n, address, _ := ClientConnection.ReadFromUDP(buffer)
	packetData, err := getPacketData(buffer[0:n])
	if err != nil {
		t.Errorf("Issues %v", err)
		return
	}
	if packetData.BlockNum != 1 {
		t.Errorf("block number should be 1 found %v", packetData.BlockNum)
		return
	}
	if reflect.DeepEqual(dataPayload, packetData.Data) == false {
		t.Errorf("The data is not the same")
		return
	}

	//send ack to Port in the request
	packetAck := wire.PacketAck{BlockNum: 1}
	ClientConnection.WriteToUDP(packetAck.Serialize(), address)
}

//Test with cache containing == 512 bytes of data
func TestHappyCaseForReadRequestWithEqualTo516Bytes(t *testing.T) {
	ClientConnection, err := setUp()
	if err != nil {
		t.Errorf("Error received %v", err)
		return
	}
	dataPayload := make([]byte, 512)
	for i := 0; i < 512; i++ {
		dataPayload[i] = 'a'
	}
	cache.Write("foo9", dataPayload)
	defer ClientConnection.Close()
	request := getDefaultTFTPRequest([]byte("\x00\x01foo9\x00octet\x00"))

	go func(request TftpRequest) {
		HandleReadRequest(request)
	}(*request)
	buffer := make([]byte, 1024)
	n, address, _ := ClientConnection.ReadFromUDP(buffer)
	packetData, err := getPacketData(buffer[0:n])
	if err != nil {
		t.Errorf("Issues %v", err)
		return
	}
	if packetData.BlockNum != 1 {
		t.Errorf("block number should be 1 found %v", packetData.BlockNum)
		return
	}
	if reflect.DeepEqual(dataPayload, packetData.Data) == false {
		t.Errorf("The data is not the same")
		return
	}

	//send ack to Port in the request
	packetAck := wire.PacketAck{BlockNum: 1}
	ClientConnection.WriteToUDP(packetAck.Serialize(), address)
	//send the data to address.Port
	//check the ack
	n, _, _ = ClientConnection.ReadFromUDP(buffer)
	packetData, err = getPacketData(buffer[0:n])
	if err != nil {
		t.Errorf("Issues %v", err)
		return
	}

	if packetData.BlockNum != 2 {
		t.Errorf("Expecting it to be blockNum 2 found %v", packetData.BlockNum)
		return
	}
	if len(packetData.Data) != 0 {
		t.Errorf("Expecting data length to be zero found %v", len(packetData.Data))
	}
	//send ack to Port in the request
	packetAck = wire.PacketAck{BlockNum: 2}
	ClientConnection.WriteToUDP(packetAck.Serialize(), address)

}

//Test with file not present
func TestHappyCaseForReadRequestWithFileNotPresent(t *testing.T) {
	ClientConnection, err := setUp()
	if err != nil {
		t.Errorf("Error received %v", err)
		return
	}
	defer ClientConnection.Close()
	request := getDefaultTFTPRequest([]byte("\x00\x01foo100\x00octet\x00"))

	go func(request TftpRequest) {
		HandleReadRequest(request)
	}(*request)
	buffer := make([]byte, 1024)
	n, _, _ := ClientConnection.ReadFromUDP(buffer)
	packetErr, err := getPacketError(buffer[0:n])
	if err != nil {
		t.Errorf("Issues %v", err)
		return
	}
	if packetErr.Code != 1 {
		t.Errorf("Error code should be 1 found %v", packetErr.Code)
		return
	}
}

//Test with not expected packets like getting data packet when expecting ack
func TestSadCaseForReadRequestWithUnExpectedAck(t *testing.T) {
	ClientConnection, err := setUp()
	if err != nil {
		t.Errorf("Error received %v", err)
		return
	}
	dataPayload := make([]byte, 511)
	for i := 0; i < 511; i++ {
		dataPayload[i] = 'a'
	}
	cache.Write("foo11", dataPayload)
	defer ClientConnection.Close()
	request := getDefaultTFTPRequest([]byte("\x00\x01foo11\x00octet\x00"))

	go func(request TftpRequest) {
		err = HandleReadRequest(request)
	}(*request)
	buffer := make([]byte, 1024)
	n, address, _ := ClientConnection.ReadFromUDP(buffer)
	packetData, err := getPacketData(buffer[0:n])
	if err != nil {
		t.Errorf("Issues %v", err)
		return
	}
	if packetData.BlockNum != 1 {
		t.Errorf("block number should be 1 found %v", packetData.BlockNum)
		return
	}
	if reflect.DeepEqual(dataPayload, packetData.Data) == false {
		t.Errorf("The data is not the same")
		return
	}
	//Sending invalid packet
	_, err = ClientConnection.WriteToUDP([]byte("oxff0x900x890x90"), address)
	if err != nil {
		t.Errorf("Issues %v", err)
		return
	}
}
