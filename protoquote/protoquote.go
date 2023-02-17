package protoquote

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

type quoteResponse struct {
	Id         string   `json:"_id"`
	Conent     string   `json:"content"`
	Author     string   `json:"author"`
	AuthorSlug string   `json:"authorSlug"`
	Length     int      `json:"length"`
	Tags       []string `json:"tags"`
}

type quoteHandler struct {
	intervalMin int
	actNowChan  chan int
}

var (
	currQuote  string = ""
	currAuthor string = ""
)

func init() {
	initAuthor, initQuote := readAuthorAndQuoteFromAPI()

	currAuthor = initAuthor
	currQuote = initQuote
}

func ProtoQuoteMain(addr string, interval int) {
	createAndRunQuoteHandler(interval)

	tcpListener, err := net.Listen("tcp", addr)
	handleError(err)

	handleTcpListener(tcpListener)
}

func (qresp *quoteResponse) filterAuthorAndQute() (string, string) {
	return qresp.Author, qresp.Conent
}

func handleTcpListener(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		handleError(err)

		go handleConn(conn)
	}
}

func parsequoteResponse(inResp []byte) quoteResponse {
	var qresponse quoteResponse
	json.Unmarshal(inResp, &qresponse)

	return qresponse
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	nowTime := time.Now().String()
	applicationFrame := fmt.Sprintf("PTQP v1 R+\r\n\r\nNow = %s\r\nAuthor = %s\r\nQuote = \"%s\"\r\n", nowTime, currAuthor, currQuote)

	conn.Write([]byte(applicationFrame))
}

func handleError(err error) {
	if err != nil {
		fmt.Printf("\033[1;31mError occured:\033[0m %s\n", err)
	}
}

func readAuthorAndQuoteFromAPI() (string, string) {
	resp, err := http.Get("https://api.quotable.io/random")
	handleError(err)

	body, err := io.ReadAll(resp.Body)
	handleError(err)

	qresp := parsequoteResponse(body)

	return qresp.filterAuthorAndQute()
}

func (qh *quoteHandler) sendMessageUponInterval() {
	go func() {
		for {
			time.Sleep(time.Minute * time.Duration(qh.intervalMin))
			qh.actNowChan <- 1
		}
	}()

	for {
		<-qh.actNowChan

		newAuthor, newQuote := readAuthorAndQuoteFromAPI()

		currAuthor = newAuthor
		currQuote = newQuote
	}
}

func createAndRunQuoteHandler(interval int) {
	quoteHandler := quoteHandler{intervalMin: interval, actNowChan: make(chan int)}

	go quoteHandler.sendMessageUponInterval()
}

func CleanUpProtoQuote() {
	fmt.Println("ProtoGen's ProtoQuote server on INET/TCP has been terminated")
	os.Exit(0)
}
