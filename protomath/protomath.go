package protomath

import (
	"fmt"
	"log"
	"net"
	"os"
	"protogen/protoparser"
	"strings"
)

type responseType int

const (
	RESPONSE_EQPARSE_FAILED  responseType = 51
	RESPONSE_REQPARSE_FAILED responseType = 52
	RESPONSE_NON_ASCII       responseType = 53
	RESPONSE_UNSUPPORTED_OP  responseType = 54
	RESPONSE_PARSE_OK        responseType = 100
)

func ProtoMathMain(addr string) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalln(err)
	}

	for {
		conn, _ := listener.Accept()
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	var buffer [500]byte
	var eq []byte

	for {
		n, _ := conn.Read(buffer[0:])
		eq = append(eq, buffer[:n]...)

		if n != 500 {
			break
		}
	}

	answer, resp := solveEquation(eq)

	respStrByte := []byte(resp.toString())
	respStrByte = append(respStrByte, []byte{10, 10}...)
	respStrByte = append(respStrByte, answer...)

	conn.Write(respStrByte)
}

func (rp responseType) toString() string {
	respText := ""

	switch rp {
	case RESPONSE_EQPARSE_FAILED:
		respText = "EQUATION PARSE FAIL"
	case RESPONSE_PARSE_OK:
		respText = "PARSE WAS SUCCESSFUL"
	case RESPONSE_REQPARSE_FAILED:
		respText = "REQUEST PARSE FAIL"
	case RESPONSE_NON_ASCII:
		respText = "UNALLOWED BYTE DETECTED"
	case RESPONSE_UNSUPPORTED_OP:
		respText = "DETECTED UNSUPPORTED OPERATION"
	}

	return fmt.Sprintf("%d %s", rp, respText)
}

func makeSureAllowed(by []byte) bool {
	for _, b := range by {
		if b > 57 && b < 32 {
			return false
		} else {
			if b > 32 && b < 42 {
				return false
			} else if b == 46 || b == 44 {
				return false
			}
		}
	}

	return true
}

func solveEquation(eq []byte) ([]byte, responseType) {
	str := string(eq)
	if !strings.Contains(str, "PTMPv1 ") {
		return nil, RESPONSE_REQPARSE_FAILED
	}
	if strings.Contains(str, "**") {
		return nil, RESPONSE_UNSUPPORTED_OP
	}

	str = strings.Replace(str, "PTMPv1 ", "", -1)

	isAlowed := makeSureAllowed([]byte(str))
	if !isAlowed {
		return nil, RESPONSE_NON_ASCII
	}

	solution, success := protoparser.ShuntingYard(str)
	if !success {
		return nil, RESPONSE_EQPARSE_FAILED
	}

	solution += "\n\n"

	return []byte(solution), RESPONSE_PARSE_OK
}

func CleanUpProtoMath() {
	fmt.Println("Exiting ProtoGen ProtoMath on TCP")
	os.Exit(0)
}
